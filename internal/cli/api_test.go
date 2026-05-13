package cli_test

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/ozgurcd/gograph/internal/cli"
)

// Helper to capture stdout and check JSON output of cli.Run
func captureRun(args []string) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command
	cli.Run(args)

	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestAPIJSONFlagOrders(t *testing.T) {
	// We want to test that the three variations produce a JSON envelope.
	// Since we don't have a valid --since argument in test env, it will return an error envelope.
	// We just need to verify it's valid JSON and kind="error" or "api".
	
	variations := [][]string{
		{"api", "--since", "nonexistent-ref", "--json"},
		{"api", "--json", "--since", "nonexistent-ref"},
		{"--json", "api", "--since", "nonexistent-ref"},
	}

	for _, args := range variations {
		out := captureRun(args)
		var env map[string]interface{}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Errorf("Failed to parse JSON for args %v: %v\nOutput: %s", args, err, out)
			continue
		}
		
		// It should be an error envelope because "nonexistent-ref" isn't a valid git ref or file
		if env["command"] != "api" {
			t.Errorf("Expected command api, got %v", env["command"])
		}
		if env["status"] != "error" {
			t.Errorf("Expected status error, got %v", env["status"])
		}
	}
}
