package search_test

import (
	"strings"
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

func buildParityGraph() *graph.Graph {
	return &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "Handler", Name: "Handler", Kind: graph.KindFunction, File: "api.go", Line: 48, EndLine: 55},
			{ID: "User", Name: "User", Kind: graph.KindStruct, File: "user.go", Line: 10, EndLine: 15},
			{ID: "ValidateToken", Name: "ValidateToken", Kind: graph.KindFunction, File: "auth.go", Line: 20, EndLine: 30},
			{ID: "internalAuth", Name: "internalAuth", Kind: graph.KindFunction, File: "auth.go", Line: 35, EndLine: 40},
		},
		Calls: []graph.CallEdge{
			{CallerSymbolID: "Handler", CallerName: "Handler", CalleeRaw: "ValidateToken", File: "api.go", Line: 50},
			{CallerSymbolID: "ValidateToken", CallerName: "ValidateToken", CalleeRaw: "internalAuth", File: "auth.go", Line: 25},
		},
		Routes: []graph.HTTPRoute{
			{Method: "POST", Path: "/api/login", Handler: "Handler", File: "api.go", Line: 45},
		},
		EnvReads: []graph.EnvRead{
			{Key: "JWT_SECRET", Accessor: "os.Getenv", File: "auth.go", Line: 36, Function: "internalAuth"},
		},
		SQLs: []graph.SQLEdge{
			{Query: "SELECT * FROM users", Function: "internalAuth", File: "auth.go", Line: 38},
		},
		TestEdges: []graph.TestEdge{
			{TestFunc: "TestValidateToken", Target: "ValidateToken", File: "auth_test.go", Line: 10},
		},
		Errors: []graph.ErrorEdge{
			{Message: "invalid token signature", Function: "internalAuth", File: "auth.go", Line: 39},
		},
	}
}

func TestPlan(t *testing.T) {
	g := buildParityGraph()
	plan := search.Plan(g, []string{"ValidateToken"}, "ValidateToken")

	if plan.Title != "ValidateToken" {
		t.Errorf("Expected title ValidateToken, got %s", plan.Title)
	}
	if plan.PublicAPI != "yes" {
		t.Errorf("Expected PublicAPI yes, got %s", plan.PublicAPI)
	}
	if len(plan.Routes) != 1 || plan.Routes[0] != "POST /api/login" {
		t.Errorf("Expected 1 route 'POST /api/login', got %v", plan.Routes)
	}
	if len(plan.Envs) != 1 || plan.Envs[0] != "JWT_SECRET" {
		t.Errorf("Expected 1 env 'JWT_SECRET', got %v", plan.Envs)
	}
	if plan.TouchesSQL != "yes" {
		t.Errorf("Expected TouchesSQL yes, got %s", plan.TouchesSQL)
	}
	if len(plan.Tests) != 1 || plan.Tests[0] != "auth_test.go" {
		t.Errorf("Expected test file auth_test.go, got %v", plan.Tests)
	}

	// Verify string output works
	str := plan.String()
	if !strings.Contains(str, "Change plan for ValidateToken") {
		t.Errorf("String output missing expected title")
	}
}

func TestReview(t *testing.T) {
	g := buildParityGraph()
	review := search.Review(g, []string{"ValidateToken"}, "ValidateToken")

	if review.Title != "ValidateToken" {
		t.Errorf("Expected title ValidateToken, got %s", review.Title)
	}
	if len(review.Changes) != 1 {
		t.Errorf("Expected 1 change, got %v", review.Changes)
	}
	if review.PublicAPI != "yes" {
		t.Errorf("Expected PublicAPI yes, got %s", review.PublicAPI)
	}
	if len(review.Routes) != 1 || review.Routes[0] != "POST /api/login" {
		t.Errorf("Expected 1 route, got %v", review.Routes)
	}
	if len(review.Envs) != 1 || review.Envs[0] != "JWT_SECRET" {
		t.Errorf("Expected 1 env, got %v", review.Envs)
	}
	if review.TouchesSQL != "yes" {
		t.Errorf("Expected TouchesSQL yes, got %s", review.TouchesSQL)
	}
	if len(review.Tests) != 1 || review.Tests[0] != "auth_test.go" {
		t.Errorf("Expected test file auth_test.go, got %v", review.Tests)
	}
	if len(review.Errors) != 1 || review.Errors[0] != "invalid token signature" {
		t.Errorf("Expected 1 error string, got %v", review.Errors)
	}

	str := review.String()
	if !strings.Contains(str, "Code Review for ValidateToken") {
		t.Errorf("String output missing expected title")
	}
}

func TestErrorFlow(t *testing.T) {
	g := buildParityGraph()
	// Test tracing "invalid token signature"
	report := search.ErrorFlow(g, "invalid token signature")

	if report.Term != "invalid token signature" {
		t.Errorf("Expected Term invalid token signature, got %s", report.Term)
	}
	if len(report.Paths) == 0 {
		t.Fatalf("Expected at least one path, got 0")
	}

	// Paths should go internalAuth <- ValidateToken <- Handler
	path := report.Paths[0]
	if path.Error.Function != "internalAuth" {
		t.Errorf("Expected origin function internalAuth, got %s", path.Error.Function)
	}

	// Root should be "Handler"
	foundRoot := false
	for _, step := range path.Path {
		if step.Name == "Handler" {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Errorf("Expected to find Handler in error path, got %v", path.Path)
	}

	str := report.String()
	if !strings.Contains(str, "ErrorFlow Report for \"invalid token signature\"") {
		t.Errorf("String output missing expected title")
	}
}

func TestUncommittedSymbols(t *testing.T) {
	g := buildParityGraph()
	// Just verify it runs without panicking
	_, err := search.UncommittedSymbols(g)
	if err != nil {
		t.Logf("UncommittedSymbols returned err (expected if not in clean git state): %v", err)
	}
}
