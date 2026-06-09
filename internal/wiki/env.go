package wiki

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// buildEnvPage produces env.md — all environment variable reads, by function.
// Returns an empty page if no env reads are detected.
func buildEnvPage(g *graph.Graph) WikiPage {
	if len(g.EnvReads) == 0 {
		return WikiPage{Filename: "env.md", Content: ""}
	}

	var b strings.Builder
	b.WriteString("# Environment Variables\n\n")
	b.WriteString("All `os.Getenv` / `os.LookupEnv` reads detected statically.\n\n")
	fmt.Fprintf(&b, "Total: %d env reads.\n\n", len(g.EnvReads))
	b.WriteString("| Key | Accessor | Function | File |\n")
	b.WriteString("|-----|----------|----------|------|\n")

	for _, e := range g.EnvReads {
		fn := e.Function
		if fn == "" {
			fn = "_"
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s:%d |\n",
			e.Key, e.Accessor, fn, e.File, e.Line)
	}
	b.WriteString("\n")

	return WikiPage{Filename: "env.md", Content: b.String()}
}
