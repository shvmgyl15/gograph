package wiki

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// buildConcurrencyPage produces concurrency.md — all goroutine spawns, mutex
// usages, channels, and sync primitives grouped by kind.
// Returns an empty page if no concurrency primitives are found.
func buildConcurrencyPage(g *graph.Graph) WikiPage {
	if len(g.Concurrency) == 0 {
		return WikiPage{Filename: "concurrency.md", Content: ""}
	}

	// Group by kind.
	byKind := make(map[string][]graph.ConcurrencyNode)
	for _, c := range g.Concurrency {
		byKind[c.Kind] = append(byKind[c.Kind], c)
	}

	// Stable kind order.
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	var b strings.Builder
	b.WriteString("# Concurrency Primitives\n\n")
	b.WriteString(fmt.Sprintf("Total: %d sites across %d kind(s).\n\n", len(g.Concurrency), len(byKind)))
	b.WriteString("> Review these sites before any concurrent refactor.\n\n")

	for _, kind := range kinds {
		nodes := byKind[kind]
		b.WriteString(fmt.Sprintf("## %s (%d)\n\n", kind, len(nodes)))
		b.WriteString("| Function | File | Detail |\n|----------|------|--------|\n")
		for _, n := range nodes {
			detail := n.Detail
			if detail == "" {
				detail = "-"
			}
			b.WriteString(fmt.Sprintf("| `%s` | %s:%d | %s |\n",
				n.Function, n.File, n.Line, detail))
		}
		b.WriteString("\n")
	}

	return WikiPage{Filename: "concurrency.md", Content: b.String()}
}
