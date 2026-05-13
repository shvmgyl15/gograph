package search_test

import (
	"strings"
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

func TestAPIDrift(t *testing.T) {
	baseline := &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "FuncA", Name: "FuncA", Kind: graph.KindFunction, Signature: "func FuncA()"},
			{ID: "IfaceA", Name: "IfaceA", Kind: graph.KindInterface, InterfaceMethods: map[string]string{"Do": "func()"}},
			{
				ID:   "StructA",
				Name: "StructA",
				Kind: graph.KindStruct,
				StructFields: []graph.StructField{
					{Name: "Field1", Type: "string", Tag: "`json:\"field1\"`"},
					{Name: "Field2", Type: "int", Tag: "`json:\"-\"`"},
				},
			},
		},
		Routes: []graph.HTTPRoute{
			{Method: "GET", Path: "/api/v1", Handler: "HandlerA"},
		},
	}

	current := &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "FuncA", Name: "FuncA", Kind: graph.KindFunction, Signature: "func FuncA(int)"},                                               // Changed signature
			{ID: "IfaceA", Name: "IfaceA", Kind: graph.KindInterface, InterfaceMethods: map[string]string{"Do": "func()", "DoMore": "func()"}}, // Added method
			{
				ID:   "StructA",
				Name: "StructA",
				Kind: graph.KindStruct,
				StructFields: []graph.StructField{
					// Removed field1
					{Name: "Field3", Type: "bool", Tag: "`json:\"field3\"`"}, // Added field
					{Name: "Field2", Type: "int", Tag: "`json:\"-\"`"},       // Still ignored
				},
			},
			{ID: "MockIfaceA", Name: "MockIfaceA", File: "mock.go", Kind: graph.KindStruct},
		},
		Routes: []graph.HTTPRoute{
			{Method: "GET", Path: "/api/v1", Handler: "HandlerB"}, // Changed handler
		},
	}

	// Add the mock relationship properly:
	current.Implements = []graph.ImplementsEdge{
		{Interface: "IfaceA", Concrete: "MockIfaceA"},
	}

	res := search.APIDrift(baseline, current, "main")

	// Verify changed function signature
	if len(res.ExportedSymbols.Changed) != 1 || res.ExportedSymbols.Changed[0].Name != "FuncA" {
		t.Errorf("expected FuncA to be changed")
	}
	if !res.BreakingGoAPI {
		t.Errorf("expected breaking Go API")
	}

	// Verify interface changed
	if len(res.Interfaces.Changed) != 1 || res.Interfaces.Changed[0].Name != "IfaceA" {
		t.Errorf("expected IfaceA to be changed")
	}
	if !res.StaleMocksLikely {
		t.Errorf("expected stale mocks likely")
	}

	// Verify mocks
	foundMock := false
	for _, m := range res.AffectedMocks {
		if m == "MockIfaceA (likely stale)" {
			foundMock = true
		}
	}
	if !foundMock {
		t.Errorf("expected MockIfaceA (likely stale) in affected mocks, got %v", res.AffectedMocks)
	}

	// Verify struct changes with json tags
	if len(res.Structs.Changed) != 1 {
		t.Fatalf("expected 1 struct changed, got %v", res.Structs.Changed)
	}
	details := res.Structs.Changed[0].Details
	if !strings.Contains(details, "removed field field1") {
		t.Errorf("expected removal of json-tagged field1 in details, got %v", details)
	}
	if !strings.Contains(details, "added field field3; compatibility depends on validation/runtime behavior") {
		t.Errorf("expected addition of field3 in details, got %v", details)
	}

	// Verify route changed
	if len(res.Routes.Changed) != 1 {
		t.Fatalf("expected 1 route changed")
	}
	if res.Routes.Changed[0].Path != "/api/v1" {
		t.Errorf("expected /api/v1 to change")
	}
	if res.BreakingHTTPAPI != "likely" {
		t.Errorf("expected breaking HTTP API to be likely due to handler change")
	}
}
