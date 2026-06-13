package search_test

import (
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

func TestRisk_SafeSymbol(t *testing.T) {
	g := &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "auth::ValidateToken", Name: "ValidateToken", Kind: graph.KindFunction, File: "auth.go", Line: 10},
			{ID: "auth::Decrypt", Name: "Decrypt", Kind: graph.KindFunction, File: "auth.go", Line: 20},
		},
		Calls: []graph.CallEdge{
			{CallerSymbolID: "auth::ValidateToken", CalleeSymbolID: "auth::Decrypt", CalleeRaw: "Decrypt"},
		},
		TestEdges: []graph.TestEdge{
			{TestFunc: "TestValidateToken", Target: "auth::ValidateToken", File: "auth_test.go", Line: 5},
			{TestFunc: "TestValidateTokenHelper", Target: "ValidateToken", File: "auth_test.go", Line: 10},
		},
	}

	report := search.Risk(g, []string{"ValidateToken"}, "ValidateToken Test")
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	res := report.Results[0]
	if res.Symbol != "ValidateToken" {
		t.Errorf("expected symbol 'ValidateToken', got %q", res.Symbol)
	}

	// 0 callers, complexity 1 (file missing -> -1 -> score 0), testCount = 2 (TestValidateToken + TestValidateTokenHelper) -> testScore = 0, Public API = 10, SQL = 0, Env = 0.
	// Total score = 10.
	if res.Score != 10 {
		t.Errorf("expected score 10, got %d", res.Score)
	}
	if res.Verdict != "SAFE" {
		t.Errorf("expected verdict SAFE, got %q", res.Verdict)
	}
}

func TestRisk_DangerSymbol(t *testing.T) {
	g := &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "db::CreateUser", Name: "CreateUser", Kind: graph.KindFunction, File: "db.go", Line: 50},
			{ID: "db::Connect", Name: "Connect", Kind: graph.KindFunction, File: "db.go", Line: 10},
			{ID: "handler::Register", Name: "Register", Kind: graph.KindFunction, File: "handler.go", Line: 100},
			{ID: "handler::SignUp", Name: "SignUp", Kind: graph.KindFunction, File: "handler.go", Line: 120},
		},
		Calls: []graph.CallEdge{
			// 2 callers for CreateUser (Register and SignUp)
			{CallerSymbolID: "handler::Register", CalleeSymbolID: "db::CreateUser", CalleeRaw: "CreateUser"},
			{CallerSymbolID: "handler::SignUp", CalleeSymbolID: "db::CreateUser", CalleeRaw: "CreateUser"},
			// CreateUser calls Connect downstream
			{CallerSymbolID: "db::CreateUser", CalleeSymbolID: "db::Connect", CalleeRaw: "Connect"},
		},
		SQLs: []graph.SQLEdge{
			{Function: "db::Connect", Query: "INSERT INTO users...", File: "db.go", Line: 15},
		},
		EnvReads: []graph.EnvRead{
			{Key: "DATABASE_URL", Function: "db::Connect", File: "db.go", Line: 12},
		},
		TestEdges: []graph.TestEdge{
			// Connect is tested, but CreateUser has ZERO tests
			{TestFunc: "TestConnect", Target: "db::Connect", File: "db_test.go", Line: 5},
		},
	}

	report := search.Risk(g, []string{"CreateUser"}, "CreateUser Test")
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	res := report.Results[0]
	// Blast Radius: 2 callers (Register, SignUp) -> 2 * 3 = 6 points.
	// Complexity: file missing -> score -1 -> complexityScore = 0.
	// Tests: 0 tests -> testScore = 20.
	// Public API: yes (CreateUser starts with uppercase C) -> 10.
	// SQL: yes (downstream calls Connect) -> 10.
	// Env: yes (downstream calls Connect) -> 5.
	// Total: 6 + 0 + 20 + 10 + 10 + 5 = 51.
	// Verdict: REVIEW (31 <= 51 <= 70).
	if res.Score != 51 {
		t.Errorf("expected score 51, got %d", res.Score)
	}
	if res.Verdict != "REVIEW" {
		t.Errorf("expected verdict REVIEW, got %q", res.Verdict)
	}
}

func TestRisk_Empty(t *testing.T) {
	g := &graph.Graph{}
	report := search.Risk(g, nil, "Empty Test")
	if report.Message == "" {
		t.Error("expected message for empty symbols")
	}
}
