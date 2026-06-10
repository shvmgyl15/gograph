package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath returns the path to the compiled gograph binary, building it if needed.
// Relies on the Makefile-standard output location: <module_root>/bin/gograph.
func binaryPath(t *testing.T) string {
	t.Helper()
	// Walk up from this test file to find the module root (contains go.mod).
	dir := "."
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			t.Fatal("could not find module root (no go.mod found)")
		}
		abs = parent
	}
	bin := filepath.Join(abs, "bin", "gograph")
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		t.Fatalf("binary not found at %s — run 'make build' first", bin)
	}
	return bin
}

// runBinary runs the gograph binary with the given args and returns stdout, stderr, exit code.
func runBinary(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = io.Writer(&outBuf)
	cmd.Stderr = io.Writer(&errBuf)
	err := cmd.Run()
	code = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// ---------------------------------------------------------------------------
// gograph doc tests
// ---------------------------------------------------------------------------

func TestDocStdlibSymbol(t *testing.T) {
	stdout, _, code := runBinary(t, "doc", "fmt.Errorf")
	if code != 0 {
		t.Fatalf("gograph doc fmt.Errorf exited %d", code)
	}
	if !strings.Contains(stdout, "Errorf") {
		t.Errorf("expected output to contain 'Errorf', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "func") {
		t.Errorf("expected output to contain 'func', got:\n%s", stdout)
	}
}

func TestDocInterface(t *testing.T) {
	stdout, _, code := runBinary(t, "doc", "io.Reader")
	if code != 0 {
		t.Fatalf("gograph doc io.Reader exited %d", code)
	}
	if !strings.Contains(stdout, "Reader") {
		t.Errorf("expected output to contain 'Reader', got:\n%s", stdout)
	}
}

func TestDocPackageLevel(t *testing.T) {
	stdout, _, code := runBinary(t, "doc", "net/http.HandleFunc")
	if code != 0 {
		t.Fatalf("gograph doc net/http.HandleFunc exited %d", code)
	}
	if !strings.Contains(stdout, "HandleFunc") {
		t.Errorf("expected output to contain 'HandleFunc', got:\n%s", stdout)
	}
}

func TestDocUnknownSymbolReturnsError(t *testing.T) {
	_, stderr, code := runBinary(t, "doc", "doesnotexist.ZZZNonexistent99999")
	if code == 0 {
		t.Error("expected non-zero exit for unknown symbol, got 0")
	}
	// go doc outputs its error to stderr — we should have surfaced it
	if stderr == "" {
		t.Error("expected error message on stderr for unknown symbol")
	}
}

func TestDocNoArgsPrintsUsage(t *testing.T) {
	_, stderr, code := runBinary(t, "doc")
	if code == 0 {
		t.Error("expected non-zero exit when called with no args")
	}
	if !strings.Contains(stderr, "usage") && !strings.Contains(stderr, "gograph doc") {
		t.Errorf("expected usage hint in stderr, got: %s", stderr)
	}
}

func TestDocJSONMode(t *testing.T) {
	stdout, _, code := runBinary(t, "--json", "doc", "fmt.Errorf")
	if code != 0 {
		t.Fatalf("gograph --json doc fmt.Errorf exited %d", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, stdout)
	}
	if envelope["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", envelope["status"])
	}
	if envelope["command"] != "doc" {
		t.Errorf("expected command=doc, got %v", envelope["command"])
	}
	// results should be a list with at least one element
	results, ok := envelope["results"].([]any)
	if !ok || len(results) == 0 {
		t.Errorf("expected non-empty results array, got: %v", envelope["results"])
	}
	// Check the docResult has query and output fields
	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected results[0] to be an object, got %T", results[0])
	}
	if _, ok := first["output"]; !ok {
		t.Error("expected 'output' field in doc result")
	}
	if first["query"] != "fmt.Errorf" {
		t.Errorf("expected query='fmt.Errorf', got %v", first["query"])
	}
}

// ---------------------------------------------------------------------------
// gograph untested tests (integration — uses real graph.json if present)
// ---------------------------------------------------------------------------

func TestUntestedRunsWithoutError(t *testing.T) {
	// This test runs against the real gograph graph (if available).
	// It verifies the command doesn't crash and produces valid output.
	stdout, stderr, code := runBinary(t, "untested")
	if code != 0 {
		t.Fatalf("gograph untested exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// Output should be either the no-results message or the table header.
	hasHeader := strings.Contains(stdout, "FUNCTION") && strings.Contains(stdout, "CALLERS")
	hasEmpty := strings.Contains(stdout, "No untested functions")
	if !hasHeader && !hasEmpty {
		t.Errorf("unexpected output from gograph untested:\n%s", stdout)
	}
}

func TestUntestedTopFlag(t *testing.T) {
	stdout, _, code := runBinary(t, "untested", "--top", "3")
	if code != 0 {
		t.Fatalf("gograph untested --top 3 exited %d", code)
	}
	// Count data rows (non-header, non-separator lines with actual function names)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	dataRows := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "FUNCTION") || strings.HasPrefix(l, "---") ||
			strings.HasPrefix(l, "Untested") || strings.HasPrefix(l, "No untested") {
			continue
		}
		dataRows++
	}
	if dataRows > 3 {
		t.Errorf("--top 3 should return at most 3 rows, got %d data rows", dataRows)
	}
}

func TestUntestedPkgFilter(t *testing.T) {
	stdout, _, code := runBinary(t, "untested", "--pkg", "cli")
	if code != 0 {
		t.Fatalf("gograph untested --pkg cli exited %d", code)
	}
	// Every result line should mention the cli package.
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "FUNCTION") || strings.HasPrefix(l, "---") ||
			strings.HasPrefix(l, "Untested") || strings.HasPrefix(l, "No untested") {
			continue
		}
		// Data rows should relate to cli package (file contains "cli" or pkg column is "cli")
		if !strings.Contains(l, "cli") {
			t.Errorf("--pkg cli filter returned non-cli row: %s", l)
		}
	}
}

func TestUntestedJSONMode(t *testing.T) {
	stdout, _, code := runBinary(t, "--json", "untested", "--top", "5")
	if code != 0 {
		t.Fatalf("gograph --json untested exited %d", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, stdout)
	}
	if envelope["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", envelope["status"])
	}
	if envelope["command"] != "untested" {
		t.Errorf("expected command=untested, got %v", envelope["command"])
	}
	results, ok := envelope["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", envelope["results"])
	}
	// Each result must have the required fields.
	for i, item := range results {
		r, ok := item.(map[string]any)
		if !ok {
			t.Errorf("results[%d] is not an object", i)
			continue
		}
		for _, field := range []string{"name", "kind", "file", "line", "caller_count", "package"} {
			if _, exists := r[field]; !exists {
				t.Errorf("results[%d] missing field %q", i, field)
			}
		}
		// caller_count must be > 0
		if cc, ok := r["caller_count"].(float64); ok && cc <= 0 {
			t.Errorf("results[%d] has caller_count=%v, must be > 0", i, cc)
		}
		// kind must be function or method
		kind, _ := r["kind"].(string)
		if kind != "function" && kind != "method" {
			t.Errorf("results[%d] kind=%q, expected 'function' or 'method'", i, kind)
		}
	}
}

func TestUntestedInvalidTopFlag(t *testing.T) {
	_, stderr, code := runBinary(t, "untested", "--top", "notanumber")
	if code == 0 {
		t.Error("expected non-zero exit for invalid --top value")
	}
	if !strings.Contains(stderr, "invalid") && !strings.Contains(stderr, "--top") {
		t.Errorf("expected error message for invalid --top, got: %s", stderr)
	}
}
