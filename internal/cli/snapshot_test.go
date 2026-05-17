package cli

import (
	"os"
	"testing"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
)

func TestSnapshotSaveAndList(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Symbols: []graph.SymbolNode{
			{Name: "Foo"},
		},
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", g)

	// Save
	if code := runSnapshot([]string{"save", "v1"}); code != 0 {
		t.Fatalf("expected snapshot save to succeed, got %d", code)
	}

	// List
	if code := runSnapshot([]string{"list"}); code != 0 {
		t.Fatalf("expected snapshot list to succeed, got %d", code)
	}
}

func TestSnapshotDiffImproved(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// Old graph (lots of imports)
	gOld := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Imports:     make([]graph.ImportEdge, 100),
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", gOld)
	runSnapshot([]string{"save", "base"})

	// New graph (fewer imports -> improved coupling edges)
	gNew := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Imports:     make([]graph.ImportEdge, 50),
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", gNew)

	if code := runSnapshot([]string{"diff", "base"}); code != 0 {
		t.Fatalf("expected snapshot diff to succeed, got %d", code)
	}
}

func TestSnapshotDiffWorse(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	gOld := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Imports:     make([]graph.ImportEdge, 10),
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", gOld)
	runSnapshot([]string{"save", "base"})

	// New graph (more imports -> worse coupling edges)
	gNew := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
		Imports:     make([]graph.ImportEdge, 50),
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", gNew)

	if code := runSnapshot([]string{"diff", "base"}); code != 0 {
		t.Fatalf("expected snapshot diff to succeed, got %d", code)
	}
}

func TestSnapshotDrop(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now(),
	}
	os.MkdirAll(".gograph", 0750)
	writeJSON(".gograph/graph.json", g)
	runSnapshot([]string{"save", "v1"})

	// Drop existing
	if code := runSnapshot([]string{"drop", "v1"}); code != 0 {
		t.Fatalf("expected snapshot drop to succeed, got %d", code)
	}

	// Drop missing
	if code := runSnapshot([]string{"drop", "v1"}); code != 1 {
		t.Fatalf("expected snapshot drop missing to fail, got %d", code)
	}
}
