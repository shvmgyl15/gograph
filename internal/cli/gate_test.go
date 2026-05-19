package cli

import (
	"os"
	"testing"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
)

func TestGatePass(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Symbols: []graph.SymbolNode{
			{Name: "Foo", Kind: "function", File: "foo.go"},
		},
		Baseline: &graph.GraphBaseline{
			OrphanCount:   10,
			CouplingEdges: 20,
		},
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", g)

	yml := `
max_complexity: 50
max_instability: 1.0
allow_new_orphans: false
max_new_coupling_edges: 5
`
	os.WriteFile(".gograph.yml", []byte(yml), 0644)

	if code := runGate(); code != 0 {
		t.Fatalf("expected gate to pass, got exit code %d", code)
	}
}

func TestGateFailOneViolation(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Imports:     make([]graph.ImportEdge, 30), // 30 current edges
		Baseline: &graph.GraphBaseline{
			CouplingEdges: 20, // max new is 5, we have 10 new, this fails
		},
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", g)

	yml := `
max_new_coupling_edges: 5
`
	os.WriteFile(".gograph.yml", []byte(yml), 0644)

	if code := runGate(); code != 1 {
		t.Fatalf("expected gate to fail, got exit code %d", code)
	}
}

func TestGateFailMultipleViolations(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Symbols: []graph.SymbolNode{
			{Name: "Foo", Kind: "method", Receiver: "Bar"},
			{Name: "Foo2", Kind: "method", Receiver: "Bar"},
			{Name: "Foo3", Kind: "method", Receiver: "Bar"},
		},
		Imports: make([]graph.ImportEdge, 30),
		Baseline: &graph.GraphBaseline{
			CouplingEdges: 20, // 10 new, limit 5 -> violation
		},
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", g)

	yml := `
max_god_object_methods: 2
max_new_coupling_edges: 5
`
	os.WriteFile(".gograph.yml", []byte(yml), 0644)

	if code := runGate(); code != 1 {
		t.Fatalf("expected gate to fail, got exit code %d", code)
	}
}
