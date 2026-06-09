package wiki

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// buildRoutesPage produces routes.md — the full HTTP route table.
// Returns an empty page (skipped by Generate) if no routes are registered.
func buildRoutesPage(g *graph.Graph) WikiPage {
	if len(g.Routes) == 0 {
		return WikiPage{Filename: "routes.md", Content: ""}
	}

	var b strings.Builder
	b.WriteString("# HTTP Routes\n\n")
	b.WriteString(fmt.Sprintf("Total: %d routes registered.\n\n", len(g.Routes)))
	b.WriteString("| Method | Path | Handler | File |\n")
	b.WriteString("|--------|------|---------|------|\n")

	for _, r := range g.Routes {
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s:%d |\n",
			r.Method, r.Path, r.Handler, r.File, r.Line))
	}
	b.WriteString("\n")

	return WikiPage{Filename: "routes.md", Content: b.String()}
}
