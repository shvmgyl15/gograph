package parser_test

import (
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/parser"
)

func featuresDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "features")
}

func TestParseFile_Features(t *testing.T) {
	fset := token.NewFileSet()
	fixturePath := filepath.Join(featuresDir(), "features.go")
	result, err := parser.ParseFile(fset, fixturePath, "features/features.go", "example.com/features")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	byName := make(map[string]graph.SymbolNode)
	for _, s := range result.Symbols {
		byName[s.Name] = s
	}

	t.Run("Struct Embeds Extracted", func(t *testing.T) {
		adminUser, ok := byName["AdminUser"]
		if !ok {
			t.Fatal("expected AdminUser symbol")
		}
		if len(adminUser.EmbeddedStructs) == 0 {
			t.Fatal("expected AdminUser to have embedded structs")
		}
		if adminUser.EmbeddedStructs[0] != "BaseUser" {
			t.Errorf("expected embedded struct 'BaseUser', got %q", adminUser.EmbeddedStructs[0])
		}
	})

	t.Run("Errors and Panics Extracted", func(t *testing.T) {
		var foundPanic, foundErrorf, foundNew bool
		for _, call := range result.Calls {
			if call.CalleeRaw == "panic" {
				foundPanic = true
			}
			if call.CalleeRaw == "fmt.Errorf" {
				foundErrorf = true
			}
			if call.CalleeRaw == "errors.New" {
				foundNew = true
			}
		}

		if !foundPanic {
			t.Error("expected to find panic() call")
		}
		if !foundErrorf {
			t.Error("expected to find fmt.Errorf() call")
		}
		if !foundNew {
			t.Error("expected to find errors.New() call")
		}
	})

	t.Run("SQL Queries Extracted", func(t *testing.T) {
		foundQuery := false
		for _, call := range result.Calls {
			if call.CalleeRaw == "db.QueryRow" {
				foundQuery = true
			}
		}
		if !foundQuery {
			t.Error("expected to find SQL execution call via QueryRow")
		}
	})
}
