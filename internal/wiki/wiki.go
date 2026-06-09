// Package wiki generates a structured llm-wiki/ directory from the gograph
// static index. Every page is a self-contained, token-efficient markdown
// document designed to be injected directly into an LLM context window.
//
// No network calls are made. All data is derived from the in-memory graph.
package wiki

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ozgurcd/gograph/internal/graph"
)

// WikiPage is a named, generated markdown document.
type WikiPage struct {
	// Filename is the relative path inside the output directory, e.g.
	// "overview.md" or "packages/internal-search.md".
	Filename string
	// Content holds the full markdown text of the page.
	Content string
}

// WikiGenerator builds all wiki pages from a loaded graph.
type WikiGenerator struct {
	g *graph.Graph
}

// New returns a WikiGenerator for the given graph.
func New(g *graph.Graph) *WikiGenerator {
	return &WikiGenerator{g: g}
}

// Generate builds all wiki pages and writes them to outputDir.
// The directory is created if it does not exist.
// Returns the list of pages that were written.
func (wg *WikiGenerator) Generate(outputDir string) ([]WikiPage, error) {
	pages := wg.buildAll()

	for _, p := range pages {
		if p.Content == "" {
			// Skip empty pages (e.g. routes.md when no routes exist).
			continue
		}

		full := filepath.Join(outputDir, p.Filename)

		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, fmt.Errorf("wiki: create dir for %s: %w", p.Filename, err)
		}

		if err := os.WriteFile(full, []byte(p.Content), 0o644); err != nil {
			return nil, fmt.Errorf("wiki: write %s: %w", p.Filename, err)
		}
	}

	return pages, nil
}

// buildAll calls every page builder and collects results.
// Add new pages here as they are implemented.
func (wg *WikiGenerator) buildAll() []WikiPage {
	pages := []WikiPage{
		buildOverviewPage(wg.g),
		buildArchitecturePage(wg.g),
		buildHotspotsPage(wg.g),
		buildRoutesPage(wg.g),
		buildEnvPage(wg.g),
		buildErrorsPage(wg.g),
		buildConcurrencyPage(wg.g),
	}
	pages = append(pages, buildPackagePages(wg.g)...)
	pages = append(pages, buildAPIPage(wg.g))
	return pages
}
