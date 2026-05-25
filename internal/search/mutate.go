package search

import (
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Mutate searches for functions that mutate the given struct field.
// The query can be "Status" or "User.Status".
func Mutate(g *graph.Graph, query string) []Result {
	parts := strings.Split(query, ".")
	field := query
	if len(parts) > 1 {
		field = parts[len(parts)-1]
	}
	field = strings.ToLower(field)

	var results []Result
	for _, m := range g.Mutations {
		if strings.ToLower(m.Field) == field {
			detail := "mutates field " + m.Field
			// Indirect mutations carry Via — the name of the mutating
			// method or "chan<-" for sends. Surface it so the reader can
			// tell `s.field = x` from `s.field.Store(x)` without opening
			// the file.
			if m.Via != "" {
				detail = "mutates field " + m.Field + " via " + m.Via
			}
			results = append(results, Result{
				Kind:   "mutation",
				Name:   m.Function,
				File:   m.File,
				Line:   m.Line,
				Detail: detail,
				Score:  1,
			})
		}
	}

	sortResults(results)
	return results
}
