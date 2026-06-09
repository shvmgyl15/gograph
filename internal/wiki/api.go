package wiki

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// buildAPIPage produces api-surface.md — all exported symbols grouped by
// package, showing only signatures (no bodies).
func buildAPIPage(g *graph.Graph) WikiPage {
	modulePath := search.ReadModulePath(g.Root)

	// Group exported symbols by package path.
	type entry struct {
		kind string
		sig  string
	}
	byPkg := make(map[string][]entry)

	for _, s := range g.Symbols {
		// Exported check: first rune is uppercase.
		if len(s.Name) == 0 || s.Name[0] < 'A' || s.Name[0] > 'Z' {
			continue
		}
		pkgPath := ""
		if idx := strings.Index(s.ID, "::"); idx >= 0 {
			pkgPath = s.ID[:idx]
		}
		if pkgPath == "" {
			continue
		}
		// Internal packages only.
		if modulePath != "" && !strings.HasPrefix(pkgPath, modulePath) {
			continue
		}

		sig := s.Signature
		if sig == "" {
			sig = fmt.Sprintf("%s %s", s.Kind, s.Name)
		}
		byPkg[pkgPath] = append(byPkg[pkgPath], entry{kind: string(s.Kind), sig: sig})
	}

	// Sort packages.
	pkgPaths := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgPaths = append(pkgPaths, p)
	}
	sort.Strings(pkgPaths)

	var b strings.Builder
	b.WriteString("# API Surface\n\n")
	b.WriteString("Exported symbols only. Bodies stripped.\n\n")

	for _, pkgPath := range pkgPaths {
		shortName := pkgPath
		if modulePath != "" {
			shortName = strings.TrimPrefix(pkgPath, modulePath+"/")
		}

		b.WriteString(fmt.Sprintf("## `%s`\n\n", shortName))
		b.WriteString("```go\n")

		entries := byPkg[pkgPath]
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].sig < entries[j].sig
		})
		for _, e := range entries {
			b.WriteString(e.sig + "\n")
		}
		b.WriteString("```\n\n")
	}

	return WikiPage{Filename: "api-surface.md", Content: b.String()}
}
