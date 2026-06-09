package search

import (
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// ContextResult bundles everything an AI agent needs to understand and work on
// a single symbol into one response, replacing 4–5 separate tool calls.
// This is the primary token-saving primitive for iterative agent workflows.
type ContextResult struct {
	// Node holds the AST details (kind, file, line, signature, doc).
	Node []Result
	// Source is the raw source code of the symbol, empty if unavailable.
	Source string
	// SourceErr holds any error from source extraction (non-fatal).
	SourceErr error
	// Callers lists functions that call this symbol.
	Callers []Result
	// Callees lists functions this symbol calls.
	Callees []Result
	// Tests lists test functions that exercise this symbol.
	Tests []Result
	// Role is a lightweight architectural classification derived from callers,
	// callees, routes, and SQL — without a full Explain computation.
	// Values: "HTTP handler", "data access", "orchestrator", "coordinator",
	//         "utility", "entry point", "internal".
	Role string
}

// Context finds the best-matching symbol for term and returns a ContextResult
// bundling its node details, source code, callers, callees, test coverage, and
// a lightweight architectural role. Returns nil if no symbol matches.
// rootDir is the repository root for source extraction (pass "." for cwd).
func Context(g *graph.Graph, rootDir, term string, exactMatch bool) *ContextResult {
	node := Node(g, term)
	if exactMatch {
		nl := strings.ToLower(term)
		var filtered []Result
		for _, r := range node {
			// Strip receiver prefix e.g. "(Foo).Bar" → "Bar"
			namePart := r.Name
			if idx := strings.LastIndex(namePart, "."); idx >= 0 {
				namePart = namePart[idx+1:]
			}
			if strings.ToLower(namePart) == nl {
				filtered = append(filtered, r)
			}
		}
		node = filtered
	}
	if len(node) == 0 {
		return nil
	}

	src, srcErr := Source(g, rootDir, term)
	callers := Callers(g, term, true, exactMatch)
	callees := Callees(g, term, true, exactMatch)

	return &ContextResult{
		Node:      node,
		Source:    src,
		SourceErr: srcErr,
		Callers:   callers,
		Callees:   callees,
		Tests:     Tests(g, term),
		Role:      quickRole(g, term, callers, callees),
	}
}

// quickRole derives an architectural role from data already computed in Context,
// without the full cost of Explain. It is intentionally coarse-grained.
func quickRole(g *graph.Graph, term string, callers, callees []Result) string {
	nl := strings.ToLower(term)

	for _, r := range g.Routes {
		if strings.ToLower(r.Handler) == nl || strings.HasSuffix(strings.ToLower(r.Handler), "."+nl) {
			return "HTTP handler"
		}
	}

	for _, sql := range g.SQLs {
		if strings.ToLower(sql.Function) == nl || strings.HasSuffix(strings.ToLower(sql.Function), "."+nl) {
			return "data access"
		}
	}

	prodCallers := 0
	for _, c := range callers {
		if !isTestFile(c.File) {
			prodCallers++
		}
	}
	calleeCount := len(callees)

	if prodCallers == 0 {
		return "entry point"
	}
	if prodCallers >= 5 && calleeCount >= 5 {
		return "orchestrator"
	}
	if prodCallers >= 5 {
		return "utility"
	}
	if calleeCount >= 5 {
		return "coordinator"
	}
	return "internal"
}
