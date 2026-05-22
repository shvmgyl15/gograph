package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ozgurcd/gograph/internal/graph"
	mcppkg "github.com/ozgurcd/gograph/internal/mcp"
	"github.com/ozgurcd/gograph/internal/search"
)

// mockRebuild always returns the same graph
func mockRebuild(g *graph.Graph) func() (*graph.Graph, error) {
	return func() (*graph.Graph, error) {
		return g, nil
	}
}

func mockBuildGraph() func(string) (*graph.Graph, error) {
	return func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}
}

func setupHandlers(t *testing.T, g *graph.Graph) map[string]func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	t.Helper()

	prev := mcppkg.ExposeToolsForTesting
	m := make(map[string]func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error))
	mcppkg.ExposeToolsForTesting = m
	t.Cleanup(func() {
		mcppkg.ExposeToolsForTesting = prev
	})

	mcppkg.NewServer(g, mockRebuild(g), mockBuildGraph())
	return m
}

func TestNewServer(t *testing.T) {
	g := &graph.Graph{}
	s := mcppkg.NewServer(g, mockRebuild(g), mockBuildGraph())
	if s == nil {
		t.Fatal("expected NewServer to return a valid server instance")
	}
}

func TestMCPResponseSerialization(t *testing.T) {
	resp := mcppkg.MCPResponse{
		Summary: "Test plan",
		Findings: []search.Result{
			{Name: "Handler", Kind: "function"},
		},
		Risk: map[string]any{
			"public_api": "yes",
			"sql":        "no",
		},
		Tests:       []string{"auth_test.go"},
		TestResults: []search.Result{{Name: "auth_test.go", Kind: "file"}},
		Source:      "func ValidateToken() {}",
		Limitations: []string{"No SSA tracking"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal MCPResponse: %v", err)
	}

	jsonStr := string(data)
	expectedKeys := []string{
		`"summary":"Test plan"`,
		`"findings":[{"kind":"function","name":"Handler"}]`,
		`"risk":{"public_api":"yes","sql":"no"}`,
		`"tests":["auth_test.go"]`,
		`"test_results":[{"kind":"file","name":"auth_test.go"}]`,
		`"source":"func ValidateToken() {}"`,
		`"limitations":["No SSA tracking"]`,
	}

	for _, k := range expectedKeys {
		if !strings.Contains(jsonStr, k) {
			t.Errorf("expected JSON to contain %s, got %s", k, jsonStr)
		}
	}
}

func TestGographAPI_Validation(t *testing.T) {
	handlers := setupHandlers(t, &graph.Graph{})
	apiHandler, ok := handlers["gograph_api"]
	if !ok {
		t.Fatal("gograph_api handler not found")
	}

	unsafeInputs := []string{
		"main; rm -rf /",
		"main && echo bad",
		"main | cat",
		"$(whoami)",
		"`whoami`",
		"main\nother",
		"",
		"--upload-pack=bad",
		"main:bad",
		"main{bad}",
		"main[bad]",
	}

	for _, input := range unsafeInputs {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"since": input}

		res, err := apiHandler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error from handler: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error for unsafe input %q, got success", input)
		} else {
			text := res.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, "invalid since value") {
				t.Errorf("expected unsafe input error message, got %s", text)
			}
		}
	}

	safeInputs := []string{
		"main",
		"HEAD~1",
		"HEAD^",
		"origin/main",
		"feature/api-drift",
		"v1.4.41",
	}

	for _, input := range safeInputs {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"since": input}

		res, err := apiHandler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error from handler: %v", err)
		}
		// It might fail because the branch doesn't exist in the test repo,
		// but it shouldn't fail with "invalid since value".
		if res.IsError {
			text := res.Content[0].(mcp.TextContent).Text
			if strings.Contains(text, "invalid since value") {
				t.Errorf("expected safe input %q to pass validation, but got: %s", input, text)
			}
		}
	}
}

func TestGographErrorFlow(t *testing.T) {
	handlers := setupHandlers(t, &graph.Graph{})
	handler, ok := handlers["gograph_errorflow"]
	if !ok {
		t.Fatal("gograph_errorflow handler not found")
	}

	// 1. Accepts term
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"term": "ErrTest"}
	res, _ := handler(context.Background(), req)
	if res.IsError {
		t.Errorf("expected success with term, got error")
	}
	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "ErrorFlow Report for ErrTest") {
		t.Errorf("term not picked up: %s", text)
	}

	// 2. Accepts query
	req.Params.Arguments = map[string]any{"query": "ErrQuery"}
	res, _ = handler(context.Background(), req)
	text = res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "ErrorFlow Report for ErrQuery") {
		t.Errorf("query not picked up: %s", text)
	}

	// 3. Query wins over term
	req.Params.Arguments = map[string]any{"query": "ErrWinner", "term": "ErrLoser"}
	res, _ = handler(context.Background(), req)
	text = res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "ErrorFlow Report for ErrWinner") {
		t.Errorf("query did not win over term: %s", text)
	}

	// 4. Check for SSA limitation
	if !strings.Contains(text, "Errorflow uses heuristic static call-graph") || !strings.Contains(text, "It does not perform SSA") {
		t.Errorf("missing SSA limitation text: %s", text)
	}
}

func TestGographBoundaries_Structured(t *testing.T) {
	handlers := setupHandlers(t, &graph.Graph{})
	handler := handlers["gograph_boundaries"]

	// Create empty boundaries config
	tmpFile, _ := os.CreateTemp("", "boundaries.json")
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })
	if _, err := tmpFile.Write([]byte(`{"layers":[]}`)); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close tmp file: %v", err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"config": tmpFile.Name()}

	res, _ := handler(context.Background(), req)
	if res.IsError {
		t.Fatalf("expected success, got error: %s", res.Content[0].(mcp.TextContent).Text)
	}
	text := res.Content[0].(mcp.TextContent).Text

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("expected JSON output from gograph_boundaries, got: %s", text)
	}

	summary, _ := out["summary"].(string)
	if !strings.Contains(summary, "No boundary violations found.") || strings.Contains(summary, "Architecture is clean!") {
		t.Errorf("expected neutral summary, got %v", summary)
	}

	risk, ok := out["risk"].(map[string]any)
	if !ok {
		t.Fatalf("missing risk object")
	}
	if pass, _ := risk["pass"].(bool); !pass {
		t.Errorf("expected pass = true")
	}
}

func TestGographContext_Structured(t *testing.T) {
	g := &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "TargetFunc", Name: "TargetFunc", Kind: graph.KindFunction, File: "auth.go", Line: 10},
			{ID: "TestTargetFunc", Name: "TestTargetFunc", Kind: graph.KindFunction, File: "auth_test.go", Line: 20},
		},
		Calls: []graph.CallEdge{
			{CallerName: "Handler", CalleeRaw: "TargetFunc", File: "api.go", Line: 5},
			{CallerName: "TestTargetFunc", CalleeRaw: "TargetFunc", File: "auth_test.go", Line: 21},
		},
		TestEdges: []graph.TestEdge{
			{TestFunc: "TestTargetFunc", Target: "TargetFunc", File: "auth_test.go", Line: 21},
		},
	}
	handlers := setupHandlers(t, g)
	handler := handlers["gograph_context"]

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"symbol": "TargetFunc"}

	res, _ := handler(context.Background(), req)
	text := res.Content[0].(mcp.TextContent).Text

	var out mcppkg.MCPResponse
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("expected JSON, got: %v", err)
	}

	if out.Node == nil || out.Node.Name != "TargetFunc" {
		t.Errorf("expected node to be set to TargetFunc, got %v", out.Node)
	}
	if len(out.Callers) != 2 {
		t.Errorf("expected Callers array with Handler and TestTargetFunc, got %v", out.Callers)
	}

	if len(out.TestResults) != 1 || out.TestResults[0].Name != "TestTargetFunc" {
		t.Errorf("expected structured test_results to contain TestTargetFunc, got %v", out.TestResults)
	}
}
