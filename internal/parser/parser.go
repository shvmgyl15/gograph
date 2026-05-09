// Package parser extracts graph nodes from a single Go source file using the
// standard go/ast and go/parser packages. No code from the target repository
// is executed.
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// FileResult holds everything extracted from one source file.
type FileResult struct {
	File    graph.FileNode
	Symbols []graph.SymbolNode
	Imports []graph.ImportEdge
	Calls   []graph.CallEdge
	Env     []graph.EnvRead
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

	return result, nil
}

// extractGenDecl handles type declarations (structs, interfaces).
func extractGenDecl(fset *token.FileSet, d *ast.GenDecl, relPath, pkgName string, result *FileResult) {
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		var kind graph.SymbolKind
		var methods map[string]string
		var fields []graph.StructField
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
	if d.Type != nil {
		methodSig = funcTypeSignature(d.Type)
	}
	id := fmt.Sprintf("%s::%s", relPath, d.Name.Name)
	if receiver != "" {
		id = fmt.Sprintf("%s::(%s).%s", relPath, receiver, d.Name.Name)
	}

	sym := graph.SymbolNode{
		ID:          id,
		Kind:        kind,
		Name:        d.Name.Name,
		Receiver:    receiver,
		PackageName: pkgName,
		File:        relPath,
		Line:        pos.Line,
		EndLine:     endPos.Line,
		Doc:             strings.TrimSpace(doc),
		Signature:       sig,
		MethodSignature: methodSig,
	}
	result.Symbols = append(result.Symbols, sym)

	// Extract call edges and env reads from the body.
	if d.Body != nil {
		callerName := d.Name.Name
		if receiver != "" {
			callerName = fmt.Sprintf("(%s).%s", receiver, d.Name.Name)
		}
		ast.Inspect(d.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callPos := fset.Position(call.Pos())
			callee := calleeString(call)

			// Env-var detection.
			if ev, ok := envRead(call, callee, callPos.Line, relPath, callerName); ok {
				result.Env = append(result.Env, ev)
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

// funcTypeSignature builds just the parameter and return type signature
func funcTypeSignature(ft *ast.FuncType) string {
	var sb strings.Builder
	sb.WriteString("func(")
	if ft.Params != nil {
		sb.WriteString(fieldListString(ft.Params))
	}
	sb.WriteString(")")
	if ft.Results != nil && len(ft.Results.List) > 0 {
		sb.WriteString(" (")
		sb.WriteString(fieldListString(ft.Results))
		sb.WriteString(")")
	}
	return sb.String()
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
