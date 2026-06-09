package wiki

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// buildErrorsPage produces errors.md — all error creation sites.
// Returns an empty page if no error edges are in the graph.
func buildErrorsPage(g *graph.Graph) WikiPage {
	if len(g.Errors) == 0 {
		return WikiPage{Filename: "errors.md", Content: ""}
	}

	var b strings.Builder
	b.WriteString("# Error Definitions\n\n")
	b.WriteString("All `errors.New`, `fmt.Errorf`, and sentinel `var` declarations.\n\n")
	b.WriteString(fmt.Sprintf("Total: %d error sites.\n\n", len(g.Errors)))
	b.WriteString("| Message | Function | File |\n")
	b.WriteString("|---------|----------|------|\n")

	for _, e := range g.Errors {
		msg := e.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s:%d |\n",
			msg, e.Function, e.File, e.Line))
	}
	b.WriteString("\n")

	return WikiPage{Filename: "errors.md", Content: b.String()}
}
