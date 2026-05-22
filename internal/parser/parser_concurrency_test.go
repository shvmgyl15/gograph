package parser_test

import (
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/parser"
)

func concurrentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "concurrent")
}

func TestParseConcurrency(t *testing.T) {
	fset := token.NewFileSet()
	fixturePath := filepath.Join(concurrentDir(), "concurrent.go")
	result, err := parser.ParseFile(fset, fixturePath, "concurrent/concurrent.go", "example.com/concurrent")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	t.Run("Goroutines captured", func(t *testing.T) {
		found := false
		for _, c := range result.Concurrency {
			if c.Kind == "goroutine" {
				found = true
			}
		}
		if !found {
			t.Error("expected at least one goroutine node, found none")
		}
	})

	t.Run("Channel sends captured", func(t *testing.T) {
		found := false
		for _, c := range result.Concurrency {
			if c.Kind == "channel_send" {
				found = true
			}
		}
		if !found {
			t.Error("expected at least one channel_send node, found none")
		}
	})

	t.Run("Mutex lock captured", func(t *testing.T) {
		found := false
		for _, c := range result.Concurrency {
			if c.Kind == "mutex_lock" {
				found = true
			}
		}
		if !found {
			t.Error("expected at least one mutex_lock node, found none")
		}
	})

	t.Run("WaitGroup captured", func(t *testing.T) {
		foundAdd, foundWait := false, false
		for _, c := range result.Concurrency {
			if c.Kind == "waitgroup_add" {
				foundAdd = true
			}
			if c.Kind == "waitgroup_wait" {
				foundWait = true
			}
		}
		if !foundAdd {
			t.Error("expected waitgroup_add, found none")
		}
		if !foundWait {
			t.Error("expected waitgroup_wait, found none")
		}
	})
}

func TestParseEnvRead(t *testing.T) {
	fset := token.NewFileSet()
	fixturePath := filepath.Join(concurrentDir(), "concurrent.go")
	result, err := parser.ParseFile(fset, fixturePath, "concurrent/concurrent.go", "example.com/concurrent")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	found := false
	for _, ev := range result.Env {
		if ev.Key == "WORKER_NAME" && ev.Accessor == "os.Getenv" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected env read WORKER_NAME via os.Getenv; got: %+v", result.Env)
	}
}

func TestParseInterfaceMethods(t *testing.T) {
	fset := token.NewFileSet()
	fixturePath := filepath.Join(concurrentDir(), "concurrent.go")
	result, err := parser.ParseFile(fset, fixturePath, "concurrent/concurrent.go", "example.com/concurrent")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	// Find the Stringer interface and verify its methods are captured.
	found := false
	for _, sym := range result.Symbols {
		if sym.Name == "Stringer" && sym.Kind == graph.KindInterface {
			if _, ok := sym.InterfaceMethods["String"]; ok {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected Stringer interface to have 'String' in InterfaceMethods")
	}
}
