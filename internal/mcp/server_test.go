package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

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
		return nil, nil
	}
}

func TestNewServer(t *testing.T) {
	g := &graph.Graph{}
	s := mcppkg.NewServer(g, mockRebuild(g), mockBuildGraph())
	if s == nil {
		t.Fatal("expected NewServer to return a valid server instance")
	}
	// We can't directly inspect the registered tools without a client,
	// but ensuring it instantiates without panicking validates the syntax.
}

func TestMCPResponseSerialization(t *testing.T) {
	resp := mcppkg.MCPResponse{
		Summary: "Test plan",
		Findings: []search.Result{
			{Name: "Handler", Kind: "function"},
		},
		Risk: map[string]string{
			"public_api": "yes",
			"sql":        "no",
		},
		Tests:       []string{"auth_test.go"},
		Source:      "func ValidateToken() {}",
		Limitations: []string{"No SSA tracking"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal MCPResponse: %v", err)
	}

	// Verify required keys are present in JSON
	jsonStr := string(data)
	expectedKeys := []string{
		`"summary":"Test plan"`,
		`"findings":[{"kind":"function","name":"Handler"}]`,
		`"risk":{"public_api":"yes","sql":"no"}`,
		`"tests":["auth_test.go"]`,
		`"source":"func ValidateToken() {}"`,
		`"limitations":["No SSA tracking"]`,
	}

	for _, k := range expectedKeys {
		if !strings.Contains(jsonStr, k) {
			t.Errorf("expected JSON to contain %s, got %s", k, jsonStr)
		}
	}
}
