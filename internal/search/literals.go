package search

import (
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Literals returns all composite-literal initialization sites for the named
// struct (e.g., User{Name: "foo"}). Case-insensitive. Run this before adding
// or removing a required field — every site returned will fail to compile.
func Literals(g *graph.Graph, structName string) []Result {
	nl := strings.ToLower(structName)
	var results []Result
	for _, lit := range g.Literals {
		if strings.ToLower(lit.TypeName) != nl {
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
