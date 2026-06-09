package wiki

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// buildArchitecturePage produces architecture.md — the Mermaid package diagram
// plus a coupling metrics table for all internal packages.
func buildArchitecturePage(g *graph.Graph) WikiPage {
	var b strings.Builder

	modulePath := search.ReadModulePath(g.Root)
	coupling := search.Coupling(g, "", search.CouplingOptions{ModuleOnly: modulePath})

	b.WriteString("# Architecture\n\n")

	// Package dependency diagram (internal packages only, module group view).
	b.WriteString("## Package Dependency Diagram\n\n")
	diagram := search.DiagramToMermaid(g, "module", 0, false)
	b.WriteString(diagram)
	b.WriteString("\n\n")

	// Coupling metrics table.
	b.WriteString("## Package Coupling\n\n")
	b.WriteString("Sorted by instability (1.0 = maximally unstable, 0.0 = maximally stable).\n\n")

	if len(coupling) == 0 {
		b.WriteString("_No coupling data available._\n\n")
	} else {
		b.WriteString("| Package | Fan-in (Ca) | Fan-out (Ce) | Instability |\n")
		b.WriteString("|---------|-------------|--------------|-------------|\n")
		for _, c := range coupling {
			// Shorten the package path for readability.
			name := c.Package
			if modulePath != "" {
				name = strings.TrimPrefix(name, modulePath+"/")
			}
			instability := fmt.Sprintf("%.2f", c.Instability)
			if c.Instability < 0 {
				instability = "isolated"
			}
			b.WriteString(fmt.Sprintf("| `%s` | %d | %d | %s |\n",
				name, c.FanIn, c.FanOut, instability))
		}
		b.WriteString("\n")
	}

	// Derived layer table: packages grouped by how many internal packages they
	// depend on (fan-out against internal-only coupling data). Layer 0 has no
	// internal imports; higher layers depend on lower ones.
	b.WriteString("## Topological Layers\n\n")
	b.WriteString("Packages grouped by fan-out (0 = foundation/stable, higher = more dependent).\n\n")

	type layerEntry struct {
		name   string
		fanOut int
	}
	layers := make(map[int][]string)
	for _, c := range coupling {
		name := c.Package
		if modulePath != "" {
			name = strings.TrimPrefix(name, modulePath+"/")
		}
		layers[c.FanOut] = append(layers[c.FanOut], name)
	}

	// Find max layer.
	maxLayer := 0
	for k := range layers {
		if k > maxLayer {
			maxLayer = k
		}
	}

	b.WriteString("| Layer (fan-out) | Packages |\n|-----------------|----------|\n")
	for l := 0; l <= maxLayer; l++ {
		pkgs, ok := layers[l]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("| %d | %s |\n", l, strings.Join(pkgs, ", ")))
	}
	b.WriteString("\n")

	return WikiPage{Filename: "architecture.md", Content: b.String()}
}
