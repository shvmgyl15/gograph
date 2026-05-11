package search

import (
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
}

// Context finds the best-matching symbol for term and returns a ContextResult
// bundling its node details, source code, callers, callees, and test coverage.
// Returns nil if no symbol matches.
// rootDir is the repository root for source extraction (pass "." for cwd).
func Context(g *graph.Graph, rootDir, term string) *ContextResult {
	node := Node(g, term)
	if len(node) == 0 {
		return nil
	}

	src, srcErr := Source(g, rootDir, term)
	return &ContextResult{
		Node:      node,
		Source:    src,
		SourceErr: srcErr,
		Callers:   Callers(g, term, true),
		Callees:   Callees(g, term, true),
		Tests:     Tests(g, term),
	}
}
