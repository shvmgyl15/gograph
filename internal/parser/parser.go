// Package parser extracts graph nodes from a single Go source file using the
// standard go/ast and go/parser packages. No code from the target repository
// is executed.
package parser

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// FileResult holds everything extracted from one source file.
type FileResult struct {
	File        graph.FileNode
	Symbols     []graph.SymbolNode
	Imports     []graph.ImportEdge
	Calls       []graph.CallEdge
	Env         []graph.EnvRead
	Routes      []graph.HTTPRoute
	SQLs        []graph.SQLEdge
	Errors      []graph.ErrorEdge
	Concurrency []graph.ConcurrencyNode
	TestEdges   []graph.TestEdge
	Mutations   []graph.MutationEdge
}

// ParseFile parses a single .go file and extracts its nodes.
// path must be an absolute or repo-relative path.
// relPath is the path stored in node IDs and graph edges (relative to repo root).
func ParseFile(fset *token.FileSet, path, relPath string) (*FileResult, error) {
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	tf := fset.File(f.Pos())
	lineCount := 0
	if tf != nil {
		lineCount = tf.LineCount()
	}

	pkgName := f.Name.Name

	result := &FileResult{
		File: graph.FileNode{
			ID:          relPath,
			Path:        relPath,
			PackageName: pkgName,
			Lines:       lineCount,
		},
	}

	// Imports.
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		result.Imports = append(result.Imports, graph.ImportEdge{
			FromFile:    relPath,
			FromPackage: pkgName,
			ImportPath:  importPath,
			Alias:       alias,
		})
	}

	// Top-level declarations.
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			extractGenDecl(fset, d, relPath, pkgName, result)
		case *ast.FuncDecl:
			extractFuncDecl(fset, d, relPath, pkgName, result)
		}
	}

	// Extract HTTP Routes
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		
		var method string
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			name := f.Sel.Name
			if name == "GET" || name == "POST" || name == "PUT" || name == "DELETE" || name == "PATCH" || name == "OPTIONS" || name == "HEAD" || name == "Any" || name == "Handle" || name == "HandleFunc" {
				method = name
			}
		}
		if method != "" && len(call.Args) >= 1 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				path := strings.Trim(lit.Value, "\"")
				handler := ""
				inlineBody := ""
				if len(call.Args) >= 2 {
					switch h := call.Args[1].(type) {
					case *ast.FuncLit:
						// Inline anonymous function — not a named symbol.
						// Record a descriptive label so callers know it is a closure
						// and can navigate to the exact line in the source file.
						handler = fmt.Sprintf("<inline handler at line %d>", fset.Position(h.Pos()).Line)
						// Render the full function literal source via go/printer so it
						// can be served from graph.json without re-reading the source file.
						var buf bytes.Buffer
						if err := printer.Fprint(&buf, fset, h); err == nil {
							inlineBody = buf.String()
						}
					default:
						handler = typeString(call.Args[1])
					}
				}
				result.Routes = append(result.Routes, graph.HTTPRoute{
					Method:     method,
					Path:       path,
					Handler:    handler,
					InlineBody: inlineBody,
					File:       relPath,
					Line:       fset.Position(call.Pos()).Line,
				})
			}
		}
		return true
	})

	return result, nil
}

// extractGenDecl handles type declarations (structs, interfaces).
func extractGenDecl(fset *token.FileSet, d *ast.GenDecl, relPath, pkgName string, result *FileResult) {
	for _, spec := range d.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok {
			if d.Tok == token.VAR || d.Tok == token.CONST {
				for _, name := range vs.Names {
					pos := fset.Position(name.Pos())
					endPos := fset.Position(vs.End()) // use vs.End() to capture the entire expression block
					doc := ""
					if d.Doc != nil {
						doc = d.Doc.Text()
					} else if vs.Comment != nil {
						doc = vs.Comment.Text()
					}
					
					symKind := graph.KindVar
					if d.Tok == token.CONST {
						symKind = graph.KindConst
					}

					sym := graph.SymbolNode{
						ID:          fmt.Sprintf("%s::%s", relPath, name.Name),
						Kind:        symKind,
						Name:        name.Name,
						PackageName: pkgName,
						File:        relPath,
						Line:        pos.Line,
						EndLine:     endPos.Line,
						Doc:         strings.TrimSpace(doc),
					}
					result.Symbols = append(result.Symbols, sym)
				}
			}
			continue
		}

		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		var kind graph.SymbolKind
		var methods map[string]string
		var fields []graph.StructField
		var embeds []string
		switch t := ts.Type.(type) {
		case *ast.StructType:
			kind = graph.KindStruct
			if t.Fields != nil {
				for _, f := range t.Fields.List {
					typeStr := typeString(f.Type)
					tagStr := ""
					if f.Tag != nil {
						tagStr = f.Tag.Value
					}
					if len(f.Names) == 0 {
						embeds = append(embeds, typeStr)
						fields = append(fields, graph.StructField{Name: typeStr, Type: typeStr, Tag: tagStr})
					} else {
						for _, n := range f.Names {
							fields = append(fields, graph.StructField{Name: n.Name, Type: typeStr, Tag: tagStr})
						}
					}
				}
			}
		case *ast.InterfaceType:
			kind = graph.KindInterface
			if t.Methods != nil {
				methods = make(map[string]string)
				for _, m := range t.Methods.List {
					if len(m.Names) > 0 {
						if ft, ok := m.Type.(*ast.FuncType); ok {
							sig := funcTypeSignature(ft)
							methods[m.Names[0].Name] = sig
						}
					}
				}
			}
		default:
			continue
		}

		pos := fset.Position(ts.Pos())
		endPos := fset.Position(ts.End())
		doc := ""
		if d.Doc != nil {
			doc = d.Doc.Text()
		} else if ts.Comment != nil {
			doc = ts.Comment.Text()
		}

		sym := graph.SymbolNode{
			ID:               fmt.Sprintf("%s::%s", relPath, ts.Name.Name),
			Kind:             kind,
			Name:             ts.Name.Name,
			PackageName:      pkgName,
			File:             relPath,
			Line:             pos.Line,
			EndLine:          endPos.Line,
			Doc:              strings.TrimSpace(doc),
			InterfaceMethods: methods,
			StructFields:     fields,
			EmbeddedStructs:  embeds,
		}
		result.Symbols = append(result.Symbols, sym)
	}
}

// extractFuncDecl handles function and method declarations.
func extractFuncDecl(fset *token.FileSet, d *ast.FuncDecl, relPath, pkgName string, result *FileResult) {
	pos := fset.Position(d.Pos())
	endPos := fset.Position(d.End())

	receiver := ""
	kind := graph.KindFunction
	if d.Recv != nil && len(d.Recv.List) > 0 {
		kind = graph.KindMethod
		receiver = receiverString(d.Recv.List[0].Type)
	}

	doc := ""
	if d.Doc != nil {
		doc = d.Doc.Text()
	}

	sig := funcSignature(d)
	methodSig := ""
	arity := 0
	if d.Type != nil {
		methodSig = funcTypeSignature(d.Type)
		if d.Type.Params != nil {
			for _, field := range d.Type.Params.List {
				if len(field.Names) > 0 {
					arity += len(field.Names)
				} else {
					arity += 1
				}
			}
		}
	}
	id := fmt.Sprintf("%s::%s", relPath, d.Name.Name)
	if receiver != "" {
		id = fmt.Sprintf("%s::(%s).%s", relPath, receiver, d.Name.Name)
	}

	sym := graph.SymbolNode{
		ID:              id,
		Kind:            kind,
		Name:            d.Name.Name,
		Receiver:        receiver,
		PackageName:     pkgName,
		File:            relPath,
		Line:            pos.Line,
		EndLine:         endPos.Line,
		Doc:             strings.TrimSpace(doc),
		Signature:       sig,
		MethodSignature: methodSig,
		Arity:           arity,
	}
	result.Symbols = append(result.Symbols, sym)

	// Extract call edges, env reads, concurrency, and test edges from the body.
	if d.Body != nil {
		callerName := d.Name.Name
		if receiver != "" {
			callerName = fmt.Sprintf("(%s).%s", receiver, d.Name.Name)
		}
		isTestFunc := strings.HasPrefix(d.Name.Name, "Test") || strings.HasPrefix(d.Name.Name, "Benchmark")

		// Concurrency: inspect go statements and channel sends/receives.
		ast.Inspect(d.Body, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.GoStmt:
				detail := calleeString(stmt.Call)
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "goroutine",
					Function: callerName,
					File:     relPath,
					Line:     fset.Position(stmt.Pos()).Line,
					Detail:   "go " + detail,
				})
			case *ast.SendStmt:
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "channel_send",
					Function: callerName,
					File:     relPath,
					Line:     fset.Position(stmt.Pos()).Line,
					Detail:   exprName(stmt.Chan) + " <-",
				})
			}
			return true
		})

		ast.Inspect(d.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callPos := fset.Position(call.Pos())
			callee := calleeString(call)

			// Sync primitive detection using suffix matching so "w.mu.Lock",
			// "s.mu.Lock", etc. are all caught regardless of receiver variable name.
			switch {
			case strings.HasSuffix(callee, ".Lock") && !strings.HasSuffix(callee, ".RLock"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "mutex_lock",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			case strings.HasSuffix(callee, ".RLock"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "mutex_lock",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			case strings.HasSuffix(callee, ".Unlock") && !strings.HasSuffix(callee, ".RUnlock"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "mutex_unlock",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			case strings.HasSuffix(callee, ".RUnlock"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "mutex_unlock",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			case strings.HasSuffix(callee, ".Add"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "waitgroup_add",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			case strings.HasSuffix(callee, ".Wait"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "waitgroup_wait",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			case strings.HasSuffix(callee, ".Do"):
				result.Concurrency = append(result.Concurrency, graph.ConcurrencyNode{
					Kind:     "once_do",
					Function: callerName,
					File:     relPath,
					Line:     callPos.Line,
					Detail:   callee,
				})
			}

			// Env-var detection.
			if ev, ok := envRead(call, callee, callPos.Line, relPath, callerName); ok {
				result.Env = append(result.Env, ev)
			}

			// SQL Extraction
			if callee != "" {
				parts := strings.Split(callee, ".")
				method := parts[len(parts)-1]
				isSQLMethod := method == "Query" || method == "QueryRow" || method == "Exec" || method == "QueryContext" || method == "QueryRowContext" || method == "ExecContext" || method == "Raw"
				
				if isSQLMethod && len(call.Args) > 0 {
					var queryStr string
					var found bool
					
					for _, arg := range call.Args {
						if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							queryStr = strings.Trim(lit.Value, "`\"")
							found = true
							break
						} else if id, ok := arg.(*ast.Ident); ok {
							if val, ok := resolveStringLiteral(d.Body, id.Name); ok {
								queryStr = val
								found = true
								break
							}
						}
					}
					
					if found {
						upperQ := strings.ToUpper(queryStr)
						if strings.Contains(upperQ, "SELECT ") || strings.Contains(upperQ, "INSERT ") || 
							strings.Contains(upperQ, "UPDATE ") || strings.Contains(upperQ, "DELETE ") || 
							strings.Contains(upperQ, "WITH ") || strings.Contains(upperQ, "CREATE ") || 
							strings.Contains(upperQ, "ALTER ") || strings.Contains(upperQ, "DROP ") {
							
							result.SQLs = append(result.SQLs, graph.SQLEdge{
								Query:    queryStr,
								Function: callerName,
								File:     relPath,
								Line:     callPos.Line,
							})
						}
					}
				}
			}

			// Error/Panic Extraction
			if callee == "panic" || strings.Contains(callee, "Errorf") || strings.Contains(callee, "New") {
				for _, arg := range call.Args {
					if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						result.Errors = append(result.Errors, graph.ErrorEdge{
							Message:  strings.Trim(lit.Value, "\""),
							Function: callerName,
							File:     relPath,
							Line:     callPos.Line,
						})
						break
					}
				}
			}

			// Test edge: record which production symbols a test calls.
			if isTestFunc && callee != "" && !strings.HasPrefix(callee, "t.") && !strings.HasPrefix(callee, "b.") {
				result.TestEdges = append(result.TestEdges, graph.TestEdge{
					TestFunc: d.Name.Name,
					Target:   callee,
					File:     relPath,
					Line:     callPos.Line,
				})
			}

			result.Calls = append(result.Calls, graph.CallEdge{
				CallerSymbolID: sym.ID,
				CallerName:     callerName,
				CalleeRaw:      callee,
				File:           relPath,
				Line:           callPos.Line,
			})
			return true
		})

		// Mutations Extraction
		ast.Inspect(d.Body, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			// Only track actual assignments, not declarations (:=) for global mutations
			isDecl := assign.Tok == token.DEFINE

			for _, lhs := range assign.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					result.Mutations = append(result.Mutations, graph.MutationEdge{
						Field:    sel.Sel.Name,
						Function: callerName,
						File:     relPath,
						Line:     fset.Position(assign.Pos()).Line,
					})
				} else if id, ok := lhs.(*ast.Ident); ok && !isDecl {
					result.Mutations = append(result.Mutations, graph.MutationEdge{
						Field:    id.Name,
						Function: callerName,
						File:     relPath,
						Line:     fset.Position(assign.Pos()).Line,
					})
				}
			}
			return true
		})
	}
}

// receiverString converts an AST receiver type expression to a short string.
func receiverString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + receiverString(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverString(t.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// calleeString converts a call expression to a compact string representation.
func calleeString(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		obj := exprName(fn.X)
		if obj == "" {
			return fn.Sel.Name
		}
		return obj + "." + fn.Sel.Name
	default:
		return "<complex call>"
	}
}

// exprName returns a best-effort single-identifier name for an expression.
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		prefix := exprName(e.X)
		if prefix == "" {
			return e.Sel.Name
		}
		return prefix + "." + e.Sel.Name
	default:
		return ""
	}
}

// funcSignature builds a best-effort human-readable signature string.
func funcSignature(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString("(")
		sb.WriteString(receiverString(d.Recv.List[0].Type))
		sb.WriteString(") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString("(")
	if d.Type.Params != nil {
		sb.WriteString(fieldListString(d.Type.Params))
	}
	sb.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		sb.WriteString(" (")
		sb.WriteString(fieldListString(d.Type.Results))
		sb.WriteString(")")
	}
	return sb.String()
}

// funcTypeSignature builds just the parameter and return type signature (names omitted for duck-typing)
func funcTypeSignature(ft *ast.FuncType) string {
	var sb strings.Builder
	sb.WriteString("func(")
	if ft.Params != nil {
		sb.WriteString(typeOnlyFieldListString(ft.Params))
	}
	sb.WriteString(")")
	if ft.Results != nil && len(ft.Results.List) > 0 {
		sb.WriteString(" (")
		sb.WriteString(typeOnlyFieldListString(ft.Results))
		sb.WriteString(")")
	}
	return sb.String()
}

// typeOnlyFieldListString converts an ast.FieldList to a string of types only, ignoring parameter names.
func typeOnlyFieldListString(fl *ast.FieldList) string {
	parts := make([]string, 0, len(fl.List))
	for _, f := range fl.List {
		typStr := typeString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typStr)
		} else {
			for range f.Names {
				parts = append(parts, typStr)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// fieldListString converts an ast.FieldList to a compact string.
func fieldListString(fl *ast.FieldList) string {
	parts := make([]string, 0, len(fl.List))
	for _, f := range fl.List {
		typStr := typeString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typStr)
		} else {
			names := make([]string, len(f.Names))
			for i, n := range f.Names {
				names[i] = n.Name
			}
			parts = append(parts, strings.Join(names, ", ")+" "+typStr)
		}
	}
	return strings.Join(parts, ", ")
}

// typeString converts a type expression to a compact string.
func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.SelectorExpr:
		return exprName(t.X) + "." + t.Sel.Name
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + typeString(t.Value)
	case *ast.IndexExpr:
		return typeString(t.X)
	default:
		return "_"
	}
}

// envAccessors is the set of function call patterns we detect for env reads.
var envAccessors = map[string]bool{
	"os.Getenv":       true,
	"os.LookupEnv":    true,
	"viper.GetString": true,
	"viper.GetBool":   true,
	"viper.GetInt":    true,
	"viper.Get":       true,
}

// envRead attempts to detect an environment variable read in a call expression.
func envRead(call *ast.CallExpr, callee string, line int, file, fn string) (graph.EnvRead, bool) {
	if !envAccessors[callee] {
		return graph.EnvRead{}, false
	}
	if len(call.Args) == 0 {
		return graph.EnvRead{}, false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return graph.EnvRead{}, false
	}
	key := strings.Trim(lit.Value, `"`)
	return graph.EnvRead{
		Key:      key,
		Accessor: callee,
		File:     file,
		Line:     line,
		Function: fn,
	}, true
}

func resolveStringLiteral(body *ast.BlockStmt, identName string) (string, bool) {
	if body == nil {
		return "", false
	}
	var found string
	var ok bool
	ast.Inspect(body, func(n ast.Node) bool {
		if ok {
			return false
		}
		assign, isAssign := n.(*ast.AssignStmt)
		if !isAssign {
			return true
		}
		for i, lhs := range assign.Lhs {
			if id, isId := lhs.(*ast.Ident); isId && id.Name == identName {
				if i < len(assign.Rhs) {
					rhs := assign.Rhs[i]
					if lit, isLit := rhs.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
						found = strings.Trim(lit.Value, "`\"")
						ok = true
						return false
					}
				}
			}
		}
		return true
	})
	return found, ok
}
