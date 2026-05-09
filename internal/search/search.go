// Package search provides case-insensitive query matching over a Graph.
package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Result is a single match returned by Query.
type Result struct {
	Kind   string
	Name   string
	File   string
	Line   int
	Detail string
	Score  int
}

// String returns a compact human-readable representation.
func (r Result) String() string {
	loc := r.File
	if r.Line > 0 {
		loc = fmt.Sprintf("%s:%d", r.File, r.Line)
	}
	if r.Detail != "" {
		return fmt.Sprintf("[%s] %s — %s  (%s)", r.Kind, r.Name, r.Detail, loc)
	}
	return fmt.Sprintf("[%s] %s  (%s)", r.Kind, r.Name, loc)
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
func Callers(g *graph.Graph, name string) []Result {
	nl := strings.ToLower(name)
	seen := make(map[string]bool)
	callerSymbols := make(map[string]graph.SymbolNode)
	for _, s := range g.Symbols {
		callerSymbols[s.ID] = s
	}

	var results []Result
	for _, c := range g.Calls {
		if strings.Contains(strings.ToLower(c.CalleeRaw), nl) {
			if !seen[c.CallerSymbolID] {
				seen[c.CallerSymbolID] = true
				sym, ok := callerSymbols[c.CallerSymbolID]
				file, line := c.File, 0
				if ok {
					file, line = sym.File, sym.Line
				}
				results = append(results, Result{
					Kind:   "caller",
					Name:   c.CallerName,
					File:   file,
					Line:   line,
					Detail: fmt.Sprintf("calls %s", c.CalleeRaw),
				})
			}
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
