package wiki

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// buildPackagePages produces one markdown file per internal package under
// packages/<short-name>.md. Only packages belonging to the module are included.
func buildPackagePages(g *graph.Graph) []WikiPage {
	modulePath := search.ReadModulePath(g.Root)

	// Index coupling by full package path for O(1) lookup.
	coupling := search.Coupling(g, "", search.CouplingOptions{ModuleOnly: modulePath})
	couplingByPkg := make(map[string]search.PackageCoupling, len(coupling))
	for _, c := range coupling {
		couplingByPkg[c.Package] = c
	}

	// Index symbols by package path for quick per-package listing.
	type symEntry struct {
		name     string
		kind     string
		exported bool
	}
	symsByPkg := make(map[string][]symEntry)
	for _, s := range g.Symbols {
		pkgPath := ""
		if idx := strings.Index(s.ID, "::"); idx >= 0 {
			pkgPath = s.ID[:idx]
		}
		if pkgPath == "" {
			continue
		}
		if modulePath != "" && !strings.HasPrefix(pkgPath, modulePath) {
			continue
		}
		exported := len(s.Name) > 0 && s.Name[0] >= 'A' && s.Name[0] <= 'Z'
		symsByPkg[pkgPath] = append(symsByPkg[pkgPath], symEntry{
			name:     s.Name,
			kind:     string(s.Kind),
			exported: exported,
		})
	}

	// Collect and sort package paths.
	pkgPaths := make([]string, 0, len(symsByPkg))
	for p := range symsByPkg {
		pkgPaths = append(pkgPaths, p)
	}
	sort.Strings(pkgPaths)

	var pages []WikiPage
	for _, pkgPath := range pkgPaths {
		syms := symsByPkg[pkgPath]

		// Short display name: strip module prefix.
		shortName := pkgPath
		if modulePath != "" {
			shortName = strings.TrimPrefix(pkgPath, modulePath+"/")
		}

		// Safe filename: replace slashes with dashes.
		filename := "packages/" + strings.ReplaceAll(shortName, "/", "-") + ".md"

		var b strings.Builder
		b.WriteString(fmt.Sprintf("# Package: `%s`\n\n", shortName))
		b.WriteString(fmt.Sprintf("**Import path:** `%s`\n\n", pkgPath))

		// Coupling row.
		if c, ok := couplingByPkg[pkgPath]; ok {
			instStr := fmt.Sprintf("%.2f", c.Instability)
			if c.Instability < 0 {
				instStr = "isolated"
			}
			b.WriteString(fmt.Sprintf(
				"| Fan-in (Ca) | Fan-out (Ce) | Instability |\n"+
					"|-------------|--------------|-------------|\n"+
					"| %d | %d | %s |\n\n",
				c.FanIn, c.FanOut, instStr,
			))
		}

		// Exported symbols.
		b.WriteString("## Exported Symbols\n\n")
		exportedWritten := 0
		for _, s := range syms {
			if s.exported {
				if exportedWritten == 0 {
					b.WriteString("| Symbol | Kind |\n|--------|------|\n")
				}
				b.WriteString(fmt.Sprintf("| `%s` | %s |\n", s.name, s.kind))
				exportedWritten++
			}
		}
		if exportedWritten == 0 {
			b.WriteString("_No exported symbols._\n")
		}
		b.WriteString("\n")

		// Internal symbol count (not listed to keep page short).
		internalCount := len(syms) - exportedWritten
		if internalCount > 0 {
			b.WriteString(fmt.Sprintf("**Internal symbols:** %d (run `gograph focus %s` for details)\n\n",
				internalCount, shortName))
		}

		pages = append(pages, WikiPage{Filename: filename, Content: b.String()})
	}

	return pages
}
