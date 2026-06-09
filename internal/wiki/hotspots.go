package wiki

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// buildHotspotsPage produces hotspots.md — highest fan-in symbols, highest
// complexity functions, and God Object candidates.
func buildHotspotsPage(g *graph.Graph) WikiPage {
	var b strings.Builder

	hotspots := search.Hotspot(g, 10, false)
	complexity := search.Complexity(g, "")
	godObjs := search.GodObjects(g, search.DefaultGodObjectParams())

	b.WriteString("# Hotspots & Risk Symbols\n\n")
	b.WriteString("> Symbols here carry the highest change risk.\n")
	b.WriteString("> Prefer wrapping them in interfaces before modifying.\n\n")

	// Fan-in hotspots
	b.WriteString("## Highest Fan-in (most-called)\n\n")
	if len(hotspots) == 0 {
		b.WriteString("_No hotspot data._\n\n")
	} else {
		b.WriteString("| Symbol | Callers | File |\n|--------|---------|------|\n")
		for _, h := range hotspots {
			fmt.Fprintf(&b, "| `%s` | %d | %s:%d |\n",
				h.Name, h.IncomingCalls, h.File, h.Line)
		}
		b.WriteString("\n")
	}

	// Complexity top 10 (HIGH or VERY HIGH only)
	b.WriteString("## Highest Complexity\n\n")
	written := 0
	header := false
	for _, c := range complexity {
		if written >= 10 {
			break
		}
		if c.Label != "HIGH" && c.Label != "VERY HIGH" {
			continue
		}
		if !header {
			b.WriteString("| Symbol | Score | Severity | File |\n|--------|-------|----------|------|\n")
			header = true
		}
		fmt.Fprintf(&b, "| `%s` | %d | %s | %s:%d |\n",
			c.Symbol, c.Score, c.Label, c.File, c.Line)
		written++
	}
	if !header {
		b.WriteString("_No HIGH or VERY HIGH complexity functions detected._\n")
	}
	b.WriteString("\n")

	// God objects
	b.WriteString("## God Object Candidates\n\n")
	if len(godObjs) == 0 {
		b.WriteString("_No God Object candidates detected._\n\n")
	} else {
		b.WriteString("| Struct | Methods | Fields | Outgoing calls | Severity |\n")
		b.WriteString("|--------|---------|--------|----------------|----------|\n")
		for _, g := range godObjs {
			fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %s |\n",
				g.Name, g.MethodCount, g.FieldCount, g.OutgoingCalls, g.Severity)
		}
		b.WriteString("\n")
	}

	return WikiPage{Filename: "hotspots.md", Content: b.String()}
}
