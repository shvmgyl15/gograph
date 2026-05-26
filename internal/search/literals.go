package search

import (
	"path/filepath"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Literals returns all composite-literal initialization sites for the named
// struct (e.g., User{Name: "foo"}). Case-insensitive. Run this before adding
// or removing a required field — every site returned will fail to compile.
func Literals(g *graph.Graph, structName string) []Result {
	nl := strings.ToLower(structName)
	var results []Result

	parts := strings.Split(nl, ".")
	hasDot := len(parts) == 2

	for _, lit := range g.Literals {
		matched := false
		litType := strings.ToLower(lit.TypeName)

		if litType == nl {
			matched = true
		} else if hasDot && litType == parts[1] {
			// Check if file package name matches parts[0]
			pkgDir := filepath.Base(filepath.Dir(lit.File))
			if strings.ToLower(pkgDir) == parts[0] {
				matched = true
			}
		}

		if !matched {
			continue
		}

		detail := "initialized in " + lit.Function
		if lit.Function == "" {
			detail = "package-level initialization"
		}
		results = append(results, Result{
			Kind:   "literal",
			Name:   lit.TypeName,
			File:   lit.File,
			Line:   lit.Line,
			Detail: detail,
			Score:  10,
		})
	}
	sortResults(results)
	return results
}
