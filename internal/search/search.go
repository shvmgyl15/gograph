// Package search provides case-insensitive query matching over a Graph.
package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Result is a single match returned by any search function.
type Result struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Detail string `json:"detail,omitempty"`
	Score  int    `json:"-"` // internal ranking only, not serialised

	// CallSiteFile and CallSiteLine carry the exact location of the call
	// expression that produced this result (callers/callees). Empty for
	// non-call results. This is the provenance field.
	CallSiteFile string `json:"call_site_file,omitempty"`
	CallSiteLine int    `json:"call_site_line,omitempty"`
}

// String returns a compact human-readable representation.
func (r Result) String() string {
	loc := r.File
	if r.Line > 0 {
		loc = fmt.Sprintf("%s:%d", r.File, r.Line)
	}
	provenance := ""
	if r.CallSiteFile != "" {
		provenance = fmt.Sprintf(" [call @ %s:%d]", r.CallSiteFile, r.CallSiteLine)
	}
	if r.Detail != "" {
		return fmt.Sprintf("[%s] %s — %s  (%s)%s", r.Kind, r.Name, r.Detail, loc, provenance)
	}
	return fmt.Sprintf("[%s] %s  (%s)%s", r.Kind, r.Name, loc, provenance)
}

// Query searches g for all occurrences of any of the given terms (OR logic)
// and returns ranked results. Matching is case-insensitive.
func Query(g *graph.Graph, terms []string) []Result {
	lterms := make([]string, len(terms))
	for i, t := range terms {
		lterms[i] = strings.ToLower(t)
	}
	match := func(s string) int {
		sl := strings.ToLower(s)
		for _, t := range lterms {
			if strings.Contains(sl, t) {
				return 1
			}
		}
		return 0
	}

	seen := make(map[string]bool)
	var results []Result
	add := func(r Result) {
		key := fmt.Sprintf("%s|%s|%d", r.Kind, r.Name, r.Line)
		if !seen[key] {
			seen[key] = true
			results = append(results, r)
		}
	}

	for _, pkg := range g.Packages {
		if match(pkg.Name)+match(pkg.Dir) > 0 {
			add(Result{Kind: "package", Name: pkg.Name, File: pkg.Dir, Score: 1})
		}
	}
	for _, f := range g.Files {
		if match(f.Path) > 0 {
			add(Result{Kind: "file", Name: f.Path, File: f.Path, Score: 1})
		}
	}
	for _, s := range g.Symbols {
		score := match(s.Name) + match(s.Doc) + match(s.Receiver)
		if score > 0 {
			name := s.Name
			if s.Receiver != "" {
				name = fmt.Sprintf("(%s).%s", s.Receiver, s.Name)
			}
			add(Result{Kind: string(s.Kind), Name: name, File: s.File, Line: s.Line, Detail: string(s.Kind), Score: score})
		}
	}
	for _, imp := range g.Imports {
		if match(imp.ImportPath) > 0 {
			add(Result{Kind: "import", Name: imp.ImportPath, File: imp.FromFile, Detail: "imported by " + imp.FromPackage, Score: 1})
		}
	}
	for _, c := range g.Calls {
		if match(c.CalleeRaw) > 0 {
			add(Result{Kind: "call", Name: c.CalleeRaw, File: c.File, Line: c.Line, Detail: "called by " + c.CallerName, Score: 1})
		}
	}

	sortResults(results)
	return results
}

// Node finds symbols/packages/files whose name matches (exact or partial).
func Node(g *graph.Graph, name string) []Result {
	nl := strings.ToLower(name)
	seen := make(map[string]bool)
	var results []Result
	add := func(r Result) {
		key := fmt.Sprintf("%s|%s|%d", r.Kind, r.Name, r.Line)
		if !seen[key] {
			seen[key] = true
			results = append(results, r)
		}
	}

	for _, pkg := range g.Packages {
		if strings.ToLower(pkg.Name) == nl {
			add(Result{Kind: "package", Name: pkg.Name, File: pkg.Dir})
		}
	}
	for _, f := range g.Files {
		fl := strings.ToLower(f.Path)
		if fl == nl || fl == nl+".go" {
			add(Result{Kind: "file", Name: f.Path, File: f.Path})
		}
	}
	for _, s := range g.Symbols {
		sname := strings.ToLower(s.Name)
		full := strings.ToLower(fmt.Sprintf("(%s).%s", s.Receiver, s.Name))
		if sname == nl || strings.Contains(full, nl) || strings.Contains(sname, nl) {
			n2 := s.Name
			if s.Receiver != "" {
				n2 = fmt.Sprintf("(%s).%s", s.Receiver, s.Name)
			}
			add(Result{Kind: string(s.Kind), Name: n2, File: s.File, Line: s.Line, Detail: s.Signature})
		}
	}

	sortResults(results)
	return results
}

// Callers returns functions/methods that contain a call expression matching name.
// Each result includes call-site provenance (CallSiteFile, CallSiteLine) pointing
// to the exact line of the call expression, not just the enclosing function.
func Callers(g *graph.Graph, name string) []Result {
	nl := strings.ToLower(name)
	callerSymbols := make(map[string]graph.SymbolNode)
	for _, s := range g.Symbols {
		callerSymbols[s.ID] = s
	}

	// Collect all matching call edges (one per unique call site).
	type siteKey struct{ id, callFile string; callLine int }
	seen := make(map[siteKey]bool)
	var results []Result
	for _, c := range g.Calls {
		if strings.Contains(strings.ToLower(c.CalleeRaw), nl) {
			k := siteKey{c.CallerSymbolID, c.File, c.Line}
			if seen[k] {
				continue
			}
			seen[k] = true
			sym, ok := callerSymbols[c.CallerSymbolID]
			file, line := c.File, 0
			if ok {
				file, line = sym.File, sym.Line
			}
			results = append(results, Result{
				Kind:         "caller",
				Name:         c.CallerName,
				File:         file,
				Line:         line,
				Detail:       fmt.Sprintf("calls %s", c.CalleeRaw),
				CallSiteFile: c.File,
				CallSiteLine: c.Line,
			})
		}
	}
	sortResults(results)
	return results
}

// Callees returns call expressions found inside the given function/method name.
func Callees(g *graph.Graph, name string) []Result {
	nl := strings.ToLower(name)
	matchedIDs := make(map[string]bool)
	for _, s := range g.Symbols {
		sname := strings.ToLower(s.Name)
		full := strings.ToLower(fmt.Sprintf("(%s).%s", s.Receiver, s.Name))
		if sname == nl || strings.Contains(full, nl) || strings.Contains(sname, nl) {
			matchedIDs[s.ID] = true
		}
	}

	var results []Result
	for _, c := range g.Calls {
		if matchedIDs[c.CallerSymbolID] {
			results = append(results, Result{
				Kind:   "callee",
				Name:   c.CalleeRaw,
				File:   c.File,
				Line:   c.Line,
				Detail: fmt.Sprintf("called by %s", c.CallerName),
			})
		}
	}
	sortResults(results)
	return results
}

func sortResults(results []Result) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0; j-- {
			a, b := results[j-1], results[j]
			swap := a.Score < b.Score || (a.Score == b.Score && a.Name > b.Name)
			if swap {
				results[j-1], results[j] = results[j], results[j-1]
			} else {
				break
			}
		}
	}
}

// Focus returns all files, symbols, imports, and calls associated with a specific package.
func Focus(g *graph.Graph, pkgName string) []Result {
	nl := strings.ToLower(pkgName)
	var results []Result

	// Find the package(s)
	var targetPkgs []graph.PackageNode
	for _, pkg := range g.Packages {
		if strings.ToLower(pkg.Name) == nl || strings.ToLower(pkg.Dir) == nl {
			targetPkgs = append(targetPkgs, pkg)
			results = append(results, Result{Kind: "package", Name: pkg.Name, File: pkg.Dir, Score: 10})
		}
	}

	if len(targetPkgs) == 0 {
		return results
	}

	// Make a set of files belonging to this package
	pkgFiles := make(map[string]bool)
	for _, pkg := range targetPkgs {
		for _, f := range pkg.Files {
			pkgFiles[f] = true
		}
	}

	// Add files and symbols
	for _, f := range g.Files {
		if pkgFiles[f.Path] {
			results = append(results, Result{Kind: "file", Name: f.Path, File: f.Path, Score: 9})
		}
	}

	for _, s := range g.Symbols {
		if pkgFiles[s.File] {
			name := s.Name
			if s.Receiver != "" {
				name = fmt.Sprintf("(%s).%s", s.Receiver, s.Name)
			}
			results = append(results, Result{Kind: string(s.Kind), Name: name, File: s.File, Line: s.Line, Score: 8})
		}
	}

	// Add things this package imports
	for _, imp := range g.Imports {
		if pkgFiles[imp.FromFile] {
			results = append(results, Result{Kind: "imports", Name: imp.ImportPath, File: imp.FromFile, Detail: "dependency", Score: 7})
		}
	}

	// Add things that call into this package
	for _, c := range g.Calls {
		if pkgFiles[c.File] {
			results = append(results, Result{Kind: "callee", Name: c.CalleeRaw, File: c.File, Line: c.Line, Detail: "called by " + c.CallerName, Score: 6})
		} else {
			for _, pkg := range targetPkgs {
				prefix := pkg.Name + "."
				if strings.HasPrefix(c.CalleeRaw, prefix) {
					results = append(results, Result{Kind: "caller", Name: c.CallerName, File: c.File, Line: c.Line, Detail: "calls " + c.CalleeRaw, Score: 5})
					break
				}
			}
		}
	}

	sortResults(results)
	return results
}

// Implementers finds all structs that implement the given interface.
func Implementers(g *graph.Graph, interfaceName string) []Result {
	nl := strings.ToLower(interfaceName)
	var iface *graph.SymbolNode
	var results []Result

	for i, s := range g.Symbols {
		if s.Kind == graph.KindInterface && strings.ToLower(s.Name) == nl {
			iface = &g.Symbols[i]
			break
		}
	}

	if iface == nil || len(iface.InterfaceMethods) == 0 {
		return results
	}

	// 1. Precise Fast-Path (if --precise was used)
	if len(g.Implements) > 0 {
		for _, edge := range g.Implements {
			if strings.ToLower(edge.Interface) == nl {
				// Find the concrete symbol
				for _, s := range g.Symbols {
					if s.Kind == graph.KindStruct && s.Name == edge.Concrete {
						results = append(results, Result{
							Kind:   "struct",
							Name:   s.Name,
							File:   s.File,
							Line:   s.Line,
							Detail: "implements " + iface.Name + " (type-checked)",
							Score:  10,
						})
					}
				}
			}
		}
		if len(results) > 0 {
			sortResults(results)
			return results
		}
	}

	// 2. AST Heuristic Fallback (duck-typing)
	var structs []graph.SymbolNode
	for _, s := range g.Symbols {
		if s.Kind == graph.KindStruct {
			structs = append(structs, s)
		}
	}

	methodsByReceiver := make(map[string][]graph.SymbolNode)
	for _, s := range g.Symbols {
		if s.Kind == graph.KindMethod && s.Receiver != "" {
			recv := strings.TrimPrefix(s.Receiver, "*")
			methodsByReceiver[recv] = append(methodsByReceiver[recv], s)
		}
	}

	for _, str := range structs {
		methods := methodsByReceiver[str.Name]
		if len(methods) < len(iface.InterfaceMethods) {
			continue
		}

		implemented := 0
		for reqName, reqSig := range iface.InterfaceMethods {
			for _, m := range methods {
				if m.Name == reqName && m.MethodSignature == reqSig {
					implemented++
					break
				}
			}
		}

		if implemented == len(iface.InterfaceMethods) {
			results = append(results, Result{
				Kind:   "struct",
				Name:   str.Name,
				File:   str.File,
				Line:   str.Line,
				Detail: "implements " + iface.Name,
				Score:  10,
			})
		}
	}

	sortResults(results)
	return results
}

// Source extracts the exact source code lines for a given symbol.
func Source(g *graph.Graph, rootDir, symbolName string) (string, error) {
	var target *graph.SymbolNode
	nl := strings.ToLower(symbolName)
	
	// Exact match preferred
	for i, s := range g.Symbols {
		if strings.ToLower(s.Name) == nl || strings.ToLower(s.ID) == nl {
			target = &g.Symbols[i]
			break
		}
	}
	
	if target == nil {
		return "", fmt.Errorf("symbol '%s' not found", symbolName)
	}

	absPath := filepath.Join(rootDir, target.File)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", target.File, err)
	}

	lines := strings.Split(string(data), "\n")
	start := target.Line - 1
	end := target.EndLine
	
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return "", fmt.Errorf("invalid line range: %d to %d", target.Line, target.EndLine)
	}

	extracted := strings.Join(lines[start:end], "\n")
	return fmt.Sprintf("// %s (%s:%d-%d)\n%s", target.ID, target.File, target.Line, target.EndLine, extracted), nil
}

// Orphans finds functions and methods that are never explicitly called in the codebase.
func Orphans(g *graph.Graph) []Result {
	var results []Result
	calledNames := make(map[string]bool)

	for _, c := range g.Calls {
		parts := strings.Split(c.CalleeRaw, ".")
		calledNames[parts[len(parts)-1]] = true
		calledNames[c.CalleeRaw] = true
	}

	for _, s := range g.Symbols {
		if s.Kind != graph.KindFunction && s.Kind != graph.KindMethod {
			continue
		}
		if s.Name == "main" || s.Name == "init" {
			continue
		}

		// Check if the name is explicitly called
		if !calledNames[s.Name] {
			results = append(results, Result{
				Kind:   string(s.Kind),
				Name:   s.Name,
				File:   s.File,
				Line:   s.Line,
				Detail: "0 explicit calls found",
				Score:  5,
			})
		}
	}

	sortResults(results)
	return results
}

// Fields extracts all fields for a given struct.
func Fields(g *graph.Graph, structName string) []Result {
	var results []Result
	nl := strings.ToLower(structName)

	for _, s := range g.Symbols {
		if s.Kind == graph.KindStruct && (strings.ToLower(s.Name) == nl || strings.ToLower(s.ID) == nl) {
			for _, f := range s.StructFields {
				detail := f.Type
				if f.Tag != "" {
					detail += " " + f.Tag
				}
				results = append(results, Result{
					Kind:   "field",
					Name:   f.Name,
					File:   s.File,
					Line:   s.Line,
					Detail: detail,
					Score:  10,
				})
			}
			break
		}
	}

	return results
}

// Impact traverses the call graph backwards to find all symbols that eventually call the target symbol.
func Impact(g *graph.Graph, name string) []Result {
	return ImpactMultiple(g, []string{name}, "downstream impact of "+name)
}

// ImpactMultiple calculates blast radius for multiple root symbols simultaneously.
func ImpactMultiple(g *graph.Graph, names []string, reason string) []Result {
	callerSymbols := make(map[string]graph.SymbolNode)
	for _, s := range g.Symbols {
		callerSymbols[s.ID] = s
	}

	var queue []string
	visitedTerms := make(map[string]bool)
	for _, name := range names {
		nl := strings.ToLower(name)
		queue = append(queue, nl)
		visitedTerms[nl] = true
	}
	
	var results []Result
	seenIDs := make(map[string]bool)

	for len(queue) > 0 {
		term := queue[0]
		queue = queue[1:]

		for _, c := range g.Calls {
			if strings.Contains(strings.ToLower(c.CalleeRaw), term) {
				callerID := c.CallerSymbolID
				if !seenIDs[callerID] {
					seenIDs[callerID] = true
					
					sym, ok := callerSymbols[callerID]
					if ok {
						results = append(results, Result{
							Kind:   "impact",
							Name:   sym.Name,
							File:   sym.File,
							Line:   sym.Line,
							Detail: reason,
							Score:  8,
						})
						
						nextTerm := strings.ToLower(sym.Name)
						if !visitedTerms[nextTerm] {
							visitedTerms[nextTerm] = true
							queue = append(queue, nextTerm)
						}
					}
				}
			}
		}
	}
	
	sortResults(results)
	return results
}

// Routes extracts all HTTP REST API routes found in the codebase.
func Routes(g *graph.Graph) []Result {
	var results []Result
	for _, r := range g.Routes {
		results = append(results, Result{
			Kind:   "route",
			Name:   fmt.Sprintf("%s %s", r.Method, r.Path),
			File:   r.File,
			Line:   r.Line,
			Detail: "handled by " + r.Handler,
			Score:  10,
		})
	}
	sortResults(results)
	return results
}

// ExternalImports tracks which files import a specific external package.
func ExternalImports(g *graph.Graph, pkg string) []Result {
	var results []Result
	nl := strings.ToLower(pkg)
	for _, i := range g.Imports {
		if strings.Contains(strings.ToLower(i.ImportPath), nl) {
			results = append(results, Result{
				Kind:   "import",
				Name:   i.ImportPath,
				File:   i.FromFile,
				Line:   0,
				Detail: "imported by " + i.FromPackage,
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// SQL extracts all database SQL queries found in the codebase.
func SQL(g *graph.Graph, term string) []Result {
	var results []Result
	nl := strings.ToLower(term)
	for _, sql := range g.SQLs {
		if term == "" || strings.Contains(strings.ToLower(sql.Query), nl) {
			results = append(results, Result{
				Kind:   "sql",
				Name:   sql.Query,
				File:   sql.File,
				Line:   sql.Line,
				Detail: "executed by " + sql.Function,
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// Errors extracts all custom error messages and panics.
func Errors(g *graph.Graph, term string) []Result {
	var results []Result
	nl := strings.ToLower(term)
	for _, err := range g.Errors {
		if term == "" || strings.Contains(strings.ToLower(err.Message), nl) {
			results = append(results, Result{
				Kind:   "error",
				Name:   err.Message,
				File:   err.File,
				Line:   err.Line,
				Detail: "thrown by " + err.Function,
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// Embeds shows what structs embed the target struct.
func Embeds(g *graph.Graph, structName string) []Result {
	var results []Result
	for _, s := range g.Symbols {
		for _, e := range s.EmbeddedStructs {
			if e == structName || e == "*"+structName {
				results = append(results, Result{
					Kind:   "embed",
					Name:   s.Name,
					File:   s.File,
					Line:   s.Line,
					Detail: "embeds " + structName,
					Score:  10,
				})
			}
		}
	}
	sortResults(results)
	return results
}

// Public shows only the exported symbols of a package.
func Public(g *graph.Graph, pkgName string) []Result {
	var results []Result
	for _, s := range g.Symbols {
		// Basic check: is the package name correct, and does it start with a capital letter?
		if s.PackageName == pkgName && len(s.Name) > 0 && s.Name[0] >= 'A' && s.Name[0] <= 'Z' {
			results = append(results, Result{
				Kind:   string(s.Kind),
				Name:   s.Name,
				File:   s.File,
				Line:   s.Line,
				Detail: s.Signature,
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// Envs returns all environment variable reads in the graph,
// optionally filtered by a keyword term.
func Envs(g *graph.Graph, term string) []Result {
	nl := strings.ToLower(term)
	var results []Result
	for _, ev := range g.EnvReads {
		if term == "" || strings.Contains(strings.ToLower(ev.Key), nl) || strings.Contains(strings.ToLower(ev.Accessor), nl) {
			detail := fmt.Sprintf("read via %s", ev.Accessor)
			if ev.Function != "" {
				detail += " in " + ev.Function
			}
			results = append(results, Result{
				Kind:   "env",
				Name:   ev.Key,
				File:   ev.File,
				Line:   ev.Line,
				Detail: detail,
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// Interfaces returns all interfaces that the named struct satisfies,
// using duck-typing: a struct satisfies an interface if it has all methods
// listed in InterfaceMethods (by name). Only interfaces defined in the graph
// are checked.
func Interfaces(g *graph.Graph, structName string) []Result {
	var results []Result

	// 1. Precise Fast-Path (if --precise was used)
	if len(g.Implements) > 0 {
		for _, edge := range g.Implements {
			if strings.EqualFold(edge.Concrete, structName) {
				// Find the interface symbol
				for _, s := range g.Symbols {
					if s.Kind == graph.KindInterface && s.Name == edge.Interface {
						results = append(results, Result{
							Kind:   "interface",
							Name:   s.Name,
							File:   s.File,
							Line:   s.Line,
							Detail: fmt.Sprintf("%s satisfies %s (type-checked)", structName, s.Name),
							Score:  10,
						})
					}
				}
			}
		}
		if len(results) > 0 {
			sortResults(results)
			return results
		}
	}

	// 2. AST Heuristic Fallback (duck-typing)
	// Build a map of method names owned by the struct (via method receivers).
	structMethods := make(map[string]bool)
	for _, s := range g.Symbols {
		if s.Kind != graph.KindMethod {
			continue
		}
		// Receiver can be "Foo" or "*Foo".
		recv := strings.TrimPrefix(s.Receiver, "*")
		if recv == structName {
			structMethods[s.Name] = true
		}
	}

	for _, iface := range g.Symbols {
		if iface.Kind != graph.KindInterface || len(iface.InterfaceMethods) == 0 {
			continue
		}
		// Check that the struct implements every method on the interface.
		satisfied := true
		for methodName := range iface.InterfaceMethods {
			if !structMethods[methodName] {
				satisfied = false
				break
			}
		}
		if satisfied {
			results = append(results, Result{
				Kind:   "interface",
				Name:   iface.Name,
				File:   iface.File,
				Line:   iface.Line,
				Detail: fmt.Sprintf("%s satisfies %s (%d methods)", structName, iface.Name, len(iface.InterfaceMethods)),
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// Concurrency returns all concurrency primitives in the graph,
// optionally filtered by a kind keyword (e.g. "goroutine", "mutex").
func Concurrency(g *graph.Graph, term string) []Result {
	nl := strings.ToLower(term)
	var results []Result
	for _, c := range g.Concurrency {
		if term == "" || strings.Contains(strings.ToLower(c.Kind), nl) || strings.Contains(strings.ToLower(c.Function), nl) {
			results = append(results, Result{
				Kind:   "concurrency",
				Name:   c.Kind,
				File:   c.File,
				Line:   c.Line,
				Detail: fmt.Sprintf("%s — %s", c.Function, c.Detail),
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}

// Tests returns all test functions that exercise the named symbol.
// Pass an empty term to list all test edges.
func Tests(g *graph.Graph, term string) []Result {
	nl := strings.ToLower(term)
	var results []Result
	seen := make(map[string]bool)
	for _, te := range g.TestEdges {
		if term == "" || strings.Contains(strings.ToLower(te.Target), nl) || strings.Contains(strings.ToLower(te.TestFunc), nl) {
			key := fmt.Sprintf("%s|%s", te.TestFunc, te.Target)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, Result{
				Kind:   "test",
				Name:   te.TestFunc,
				File:   te.File,
				Line:   te.Line,
				Detail: fmt.Sprintf("exercises %s", te.Target),
				Score:  10,
			})
		}
	}
	sortResults(results)
	return results
}
