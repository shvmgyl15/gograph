package search

import (
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// DepsResult holds the direct and optionally transitive import dependencies
// of a package.
type DepsResult struct {
	// Package is the short name of the queried package.
	Package string `json:"package"`
	// Direct lists the import paths directly imported by this package.
	Direct []string `json:"direct"`
	// Transitive lists all import paths reachable transitively (only when
	// requested). Includes Direct. Ordered BFS from the root package.
	Transitive []string `json:"transitive,omitempty"`
}

// Deps returns the import dependencies of the package whose short name or
// import-path suffix matches pkg (case-insensitive). When transitive is true,
// the full closure of dependencies is returned via BFS.
//
// Transitive resolution uses the last path segment of each ImportPath as a
// proxy for the package's short name — a best-effort approach that works
// correctly for standard Go naming conventions.
func Deps(g *graph.Graph, pkg string, transitive bool) *DepsResult {
	pl := strings.ToLower(pkg)

	// Build: packageShortName → set of direct import paths.
	directMap := make(map[string]map[string]bool)
	for _, imp := range g.Imports {
		if imp.FromPackage == "" {
			continue
		}
		if directMap[imp.FromPackage] == nil {
			directMap[imp.FromPackage] = make(map[string]bool)
		}
		directMap[imp.FromPackage][imp.ImportPath] = true
	}

	// Find the canonical short name for the queried package.
	// Accept both exact short-name match and import-path suffix match.
	target := ""
	for pkgName := range directMap {
		if strings.ToLower(pkgName) == pl {
			target = pkgName
			break
		}
	}
	// Also search by import path suffix (e.g. "internal/cli" matches "cli").
	if target == "" {
		for _, imp := range g.Imports {
			if strings.HasSuffix(strings.ToLower(imp.ImportPath), "/"+pl) ||
				strings.ToLower(imp.FromPackage) == pl {
				target = imp.FromPackage
				break
			}
		}
	}

	if target == "" {
		return nil
	}

	// Collect direct imports.
	directSet := directMap[target]
	direct := make([]string, 0, len(directSet))
	for path := range directSet {
		direct = append(direct, path)
	}
	sortStrings(direct)

	if !transitive {
		return &DepsResult{Package: target, Direct: direct}
	}

	// BFS to collect transitive closure.
	visited := make(map[string]bool)
	var transitiveList []string
	queue := []string{target}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for importPath := range directMap[cur] {
			if visited[importPath] {
				continue
			}
			visited[importPath] = true
			transitiveList = append(transitiveList, importPath)

			// Resolve import path to a package short name by taking the last
			// path segment (e.g. "github.com/foo/bar/internal/db" → "db").
			segments := strings.Split(importPath, "/")
			nextPkg := segments[len(segments)-1]
			if directMap[nextPkg] != nil {
				queue = append(queue, nextPkg)
			}
		}
	}
	sortStrings(transitiveList)

	return &DepsResult{
		Package:    target,
		Direct:     direct,
		Transitive: transitiveList,
	}
}

// Dependents returns all packages in the graph that import the named package.
// pkg is matched case-insensitively against the last path segment or the full
// import path (e.g. "auth", "internal/auth", or the full module path all work).
// Each result represents one dependent package, with its file and import line.
func Dependents(g *graph.Graph, pkg string) []Result {
	pl := strings.ToLower(pkg)

	seen := make(map[string]bool)
	var results []Result

	for _, imp := range g.Imports {
		ip := strings.ToLower(imp.ImportPath)
		if ip != pl && !strings.HasSuffix(ip, "/"+pl) {
			continue
		}
		if seen[imp.FromPackage] {
			continue
		}
		seen[imp.FromPackage] = true
		results = append(results, Result{
			Kind:   "package",
			Name:   imp.FromPackage,
			File:   imp.FromFile,
			Detail: "imports " + imp.ImportPath,
			Score:  10,
		})
	}

	sortResults(results)
	return results
}

// sortStrings sorts a string slice in-place (avoids importing sort everywhere).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
