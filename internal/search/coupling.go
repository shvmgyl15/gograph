package search

import (
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// PackageCoupling holds fan-in and fan-out metrics for a single package.
type PackageCoupling struct {
	// Package is the import path of the package (e.g. "github.com/foo/bar/internal/auth").
	Package string
	// FanOut is the number of distinct packages this package imports.
	// A high fan-out means this package depends on many others — it is
	// sensitive to changes in its dependencies.
	FanOut int
	// FanIn is the number of distinct packages that import this package.
	// A high fan-in means this package is a dependency of many others —
	// changes to it have a wide blast radius.
	FanIn int
	// Instability is FanOut / (FanIn + FanOut), range [0.0, 1.0].
	// 0.0 = maximally stable (nothing it depends on changes),
	// 1.0 = maximally unstable (depends on everything, nothing depends on it).
	// -1.0 is reported when FanIn + FanOut == 0 (isolated package).
	Instability float64
}

// Coupling computes fan-in, fan-out, and instability for every package in the
// graph. Results are sorted by instability descending (most unstable first).
// Pass a term to filter results by package name substring (case-insensitive).
func Coupling(g *graph.Graph, term string) []PackageCoupling {
	tl := strings.ToLower(term)

	// Build a set of all known packages — both importers and importees.
	allPkgs := make(map[string]bool)
	for _, imp := range g.Imports {
		if imp.FromPackage != "" {
			allPkgs[imp.FromPackage] = true
		}
		// Also include packages that are only imported (leaf/stable packages
		// that import nothing themselves still deserve fan-in metrics).
		if imp.ImportPath != "" {
			allPkgs[imp.ImportPath] = true
		}
	}

	// fan-out: how many distinct packages does pkg import?
	// fan-in: how many distinct packages import pkg?
	fanOut := make(map[string]map[string]bool) // pkg -> set of imported paths
	fanIn := make(map[string]map[string]bool)  // pkg -> set of packages that import it

	for _, imp := range g.Imports {
		from := imp.FromPackage
		to := imp.ImportPath
		if from == "" || to == "" {
			continue
		}
		if fanOut[from] == nil {
			fanOut[from] = make(map[string]bool)
		}
		fanOut[from][to] = true

		if fanIn[to] == nil {
			fanIn[to] = make(map[string]bool)
		}
		fanIn[to][from] = true
	}

	// Collect results for all known packages.
	var results []PackageCoupling
	for pkg := range allPkgs {
		if tl != "" && !strings.Contains(strings.ToLower(pkg), tl) {
			continue
		}
		fo := len(fanOut[pkg])
		fi := len(fanIn[pkg])
		instability := -1.0
		if fo+fi > 0 {
			instability = float64(fo) / float64(fo+fi)
		}
		results = append(results, PackageCoupling{
			Package:     pkg,
			FanOut:      fo,
			FanIn:       fi,
			Instability: instability,
		})
	}

	// Sort by instability descending; break ties by fan-out descending.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Instability != results[j].Instability {
			return results[i].Instability > results[j].Instability
		}
		return results[i].FanOut > results[j].FanOut
	})

	return results
}
