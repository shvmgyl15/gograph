package wiki

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// buildOverviewPage produces overview.md — a dense, single-glance summary of
// the codebase: module identity, counts, top hotspots, and worst instability.
func buildOverviewPage(g *graph.Graph) WikiPage {
	var b strings.Builder

	stats := search.Stats(g)
	hotspots := search.Hotspot(g, 5, false)
	modulePath := search.ReadModulePath(g.Root)
	coupling := search.Coupling(g, "", search.CouplingOptions{ModuleOnly: modulePath})

	b.WriteString("# Codebase Overview\n\n")

	// Module header
	fmt.Fprintf(&b, "**Module:** `%s`\n\n", modulePath)
	fmt.Fprintf(&b,
		"| Packages | Files | Symbols | Import edges | Routes | SQL queries | Env reads |\n"+
			"|----------|-------|---------|--------------|--------|-------------|----------|\n"+
			"| %d | %d | %d | %d | %d | %d | %d |\n\n",
		stats.Packages, stats.Files, stats.Symbols,
		stats.Imports, stats.Routes, stats.SQLs, stats.EnvReads,
	)

	// Top hotspots
	b.WriteString("## Top Hotspots (fan-in)\n\n")
	if len(hotspots) == 0 {
		b.WriteString("_No hotspots detected._\n\n")
	} else {
		b.WriteString("| Symbol | Callers | File |\n|--------|---------|------|\n")
		for _, h := range hotspots {
			fmt.Fprintf(&b, "| `%s` | %d | %s |\n", h.Name, h.IncomingCalls, h.File)
		}
		b.WriteString("\n")
	}

	// Worst instability packages (top 5)
	b.WriteString("## Package Instability (highest first)\n\n")
	if len(coupling) == 0 {
		b.WriteString("_No coupling data._\n\n")
	} else {
		b.WriteString("| Package | Fan-in | Fan-out | Instability |\n|---------|--------|---------|-------------|\n")
		count := 0
		for _, c := range coupling {
			if count >= 5 {
				break
			}
			fmt.Fprintf(&b, "| `%s` | %d | %d | %.2f |\n",
				c.Package, c.FanIn, c.FanOut, c.Instability)
			count++
		}
		b.WriteString("\n")
	}

	// Quick counts for linked pages
	b.WriteString("## Quick Reference\n\n")
	b.WriteString("| Topic | Detail page |\n|-------|-------------|\n")
	b.WriteString("| Architecture & layers | `architecture.md` |\n")
	b.WriteString("| Exported API surface | `api-surface.md` |\n")
	b.WriteString("| High-risk symbols | `hotspots.md` |\n")
	if stats.Routes > 0 {
		fmt.Fprintf(&b, "| HTTP routes (%d) | `routes.md` |\n", stats.Routes)
	}
	if stats.EnvReads > 0 {
		fmt.Fprintf(&b, "| Env vars (%d) | `env.md` |\n", stats.EnvReads)
	}
	if stats.SQLs > 0 {
		fmt.Fprintf(&b, "| SQL queries (%d) | `errors.md` |\n", stats.SQLs)
	}
	b.WriteString("\n")

	return WikiPage{Filename: "overview.md", Content: b.String()}
}
