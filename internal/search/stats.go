package search

import (
	"github.com/ozgurcd/gograph/internal/graph"
)

// StatsResult holds a summary of the current graph index.
type StatsResult struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	Packages      int    `json:"packages"`
	Files         int    `json:"files"`
	Symbols       int    `json:"symbols"`
	Calls         int    `json:"calls"`
	Imports       int    `json:"imports"`
	Routes        int    `json:"routes"`
	SQLs          int    `json:"sqls"`
	EnvReads      int    `json:"env_reads"`
	TestEdges     int    `json:"test_edges"`
}

// Stats derives index health counts directly from the in-memory graph.
// It performs no I/O and requires no schema changes.
func Stats(g *graph.Graph) StatsResult {
	return StatsResult{
		SchemaVersion: g.Version,
		GeneratedAt:   g.GeneratedAt.Format("2006-01-02 15:04:05 UTC"),
		Packages:      len(g.Packages),
		Files:         len(g.Files),
		Symbols:       len(g.Symbols),
		Calls:         len(g.Calls),
		Imports:       len(g.Imports),
		Routes:        len(g.Routes),
		SQLs:          len(g.SQLs),
		EnvReads:      len(g.EnvReads),
		TestEdges:     len(g.TestEdges),
	}
}
