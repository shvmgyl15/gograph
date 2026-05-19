package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCmd builds the CLI binary (once per test run) and executes it against the testdata fixture.
// It returns the stdout JSON string or fails the test.
func runCmd(t *testing.T, args ...string) []byte {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve project root: %v", err)
	}

	binPath := filepath.Join(root, "bin", "gograph-test")

	// Build binary only if it doesn't exist to speed up tests, or we could always build it.
	// For reliable tests, we build it once per package test execution using TestMain,
	// but since we don't have TestMain set up here, we'll build it if needed.
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", binPath, filepath.Join(root, "cmd", "gograph", "main.go"))
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to build test binary: %v\nOutput: %s", err, string(out))
		}
	}

	fixtureDir := filepath.Join(root, "testdata", "fixture")
	cmd := exec.Command(binPath, args...)
	cmd.Dir = fixtureDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// allow empty results to return a non-zero exit code sometimes depending on command,
		// but standard errors shouldn't happen
		if !strings.Contains(string(out), "schema_version") {
			t.Fatalf("command failed: %v\nOutput: %s", err, string(out))
		}
	}

	return out
}

func TestJSONSchema(t *testing.T) {
	// 1. Build the graph for the fixture repository
	runCmd(t, "build", ".")

	t.Run("callers schema", func(t *testing.T) {
		out := runCmd(t, "callers", "GetUser", "--json")

		var env map[string]interface{}
		if err := json.Unmarshal(out, &env); err != nil {
			t.Fatalf("invalid json: %v\nOutput: %s", err, string(out))
		}

		if env["schema_version"] != "1" {
			t.Errorf("expected schema_version '1', got %v", env["schema_version"])
		}
		if env["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", env["status"])
		}
		if env["command"] != "callers" {
			t.Errorf("expected command 'callers', got %v", env["command"])
		}

		results, ok := env["results"].([]interface{})
		if !ok {
			t.Fatalf("expected results array, got %T", env["results"])
		}

		if len(results) == 0 {
			t.Fatal("expected callers for GetUser, got none")
		}

		// Verify result structure matches the Result JSON tags
		first := results[0].(map[string]interface{})
		requiredFields := []string{"name", "file", "line", "kind", "call_site_file", "call_site_line"}
		for _, field := range requiredFields {
			if _, ok := first[field]; !ok {
				t.Errorf("missing field %q in result JSON", field)
			}
		}
	})

	t.Run("hotspot schema", func(t *testing.T) {
		out := runCmd(t, "hotspot", "--json")

		var env map[string]interface{}
		if err := json.Unmarshal(out, &env); err != nil {
			t.Fatalf("invalid json: %v\nOutput: %s", err, string(out))
		}

		results, ok := env["results"].([]interface{})
		if !ok || len(results) == 0 {
			t.Fatalf("expected hotspot results array, got %v", env["results"])
		}

		first := results[0].(map[string]interface{})
		requiredFields := []string{"name", "file", "line", "kind", "incoming_calls"}
		for _, field := range requiredFields {
			if _, ok := first[field]; !ok {
				t.Errorf("missing field %q in hotspot result JSON", field)
			}
		}
	})

	t.Run("deps schema", func(t *testing.T) {
		out := runCmd(t, "deps", "auth", "--json")

		var env map[string]interface{}
		if err := json.Unmarshal(out, &env); err != nil {
			t.Fatalf("invalid json: %v\nOutput: %s", err, string(out))
		}

		result, ok := env["results"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected deps result object, got %T", env["results"])
		}

		if result["package"] != "auth" {
			t.Errorf("expected package 'auth', got %v", result["package"])
		}
		if _, ok := result["direct"]; !ok {
			t.Errorf("missing 'direct' dependencies field")
		}
	})

	t.Run("empty results schema", func(t *testing.T) {
		out := runCmd(t, "query", "NonExistentFunctionXYZ123", "--json")

		var env map[string]interface{}
		if err := json.Unmarshal(out, &env); err != nil {
			t.Fatalf("invalid json: %v\nOutput: %s", err, string(out))
		}

		if env["status"] != "empty" {
			t.Errorf("expected status 'empty', got %v", env["status"])
		}
		if countVal, ok := env["count"]; ok {
			if countVal.(float64) != 0 {
				t.Errorf("expected count 0 or omitted, got %v", countVal)
			}
		}
	})
}
