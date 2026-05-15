package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ozgurcd/gograph/internal/graph"
)

func TestGographCapabilities(t *testing.T) {
	handlers := setupHandlers(t, &graph.Graph{})
	handler, ok := handlers["gograph_capabilities"]
	if !ok {
		t.Fatal("gograph_capabilities handler not found")
	}

	req := mcp.CallToolRequest{}
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %s", res.Content[0].(mcp.TextContent).Text)
	}

	text := res.Content[0].(mcp.TextContent).Text
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("expected JSON, got: %v", err)
	}

	// 1. Output includes tools
	tools, ok := out["tools"].([]any)
	if !ok || len(tools) < 25 {
		t.Errorf("expected tools array with at least 25 tools, got %v", tools)
	}

	toolsStr := string(text)
	expectedTools := []string{
		"gograph_capabilities", "gograph_context", "gograph_plan", "gograph_review",
		"gograph_errorflow", "gograph_api", "gograph_boundaries",
	}
	for _, tool := range expectedTools {
		if !strings.Contains(toolsStr, tool) {
			t.Errorf("expected tool %s not found in output", tool)
		}
	}

	// 2. Output includes recommended_workflows
	workflows, ok := out["recommended_workflows"].(map[string]any)
	if !ok || len(workflows) == 0 {
		t.Errorf("expected recommended_workflows object, got %v", workflows)
	}

	// 3. Output includes limitations
	limitations, ok := out["limitations"].([]any)
	if !ok || len(limitations) == 0 {
		t.Errorf("expected limitations array, got %v", limitations)
	}
	hasSSA := false
	for _, l := range limitations {
		lim := l.(string)
		if strings.Contains(lim, "SSA") {
			hasSSA = true
			break
		}
	}
	if !hasSSA {
		t.Errorf("expected SSA limitation text")
	}
}
