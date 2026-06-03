package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildTestBinary compiles the gograph binary once per test run and returns
// the path.  It uses the project's bin/gograph-test name to avoid conflicts
// with the production binary.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	binPath := filepath.Join(repoRoot, "bin", "gograph-test")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", binPath, filepath.Join(repoRoot, "cmd", "gograph", "main.go"))
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build test binary: %v\nOutput: %s", err, out)
		}
	}
	return binPath
}

// setupGraphFixture creates a minimal Go project in a temp directory, builds
// the gograph index at the root, and returns (root, binPath).
func setupGraphFixture(t *testing.T) (string, string) {
	t.Helper()
	binPath := buildTestBinary(t)

	root := t.TempDir()
	// Create a simple Go source tree with a subdirectory package.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/subdir\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	mainGo := filepath.Join(root, "main.go")
	if err := os.WriteFile(mainGo, []byte(`package main

import "fmt"

func main() { fmt.Println("hello") }

func RunAudit() error { return nil }
`), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// Create a subdirectory with its own package.
	subDir := filepath.Join(root, "internal", "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.go"), []byte(`package sub

func Helper() string { return "ok" }
`), 0o644); err != nil {
		t.Fatalf("write sub.go: %v", err)
	}

	// Build the graph at root.
	cmd := exec.Command(binPath, "build", root)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gograph build: %v\n%s", err, out)
	}

	return root, binPath
}

// --- Regression tests for gograph-root-aware-graph-loading ---

// TestPlanFromRoot verifies plan works when invoked from the repo root.
func TestPlanFromRoot(t *testing.T) {
	root, bin := setupGraphFixture(t)

	cmd := exec.Command(bin, "plan", "RunAudit")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plan from root failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Change plan for RunAudit") {
		t.Errorf("expected plan output, got:\n%s", out)
	}
}

// TestPlanFromSubdirectory verifies plan works when invoked from a
// subdirectory (the key bug this feature fixes).
func TestPlanFromSubdirectory(t *testing.T) {
	root, bin := setupGraphFixture(t)
	subDir := filepath.Join(root, "internal", "sub")

	cmd := exec.Command(bin, "plan", "RunAudit")
	cmd.Dir = subDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plan from subdir failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Change plan for RunAudit") {
		t.Errorf("expected plan output from subdirectory, got:\n%s", out)
	}
}

// TestReviewFromSubdirectory verifies review works from a subdirectory.
func TestReviewFromSubdirectory(t *testing.T) {
	root, bin := setupGraphFixture(t)
	subDir := filepath.Join(root, "internal", "sub")

	cmd := exec.Command(bin, "review", "RunAudit")
	cmd.Dir = subDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("review from subdir failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Code Review for RunAudit") {
		t.Errorf("expected review output from subdirectory, got:\n%s", out)
	}
}

// TestSessionAndGraphLoading_SubdirectoryE2E exercises the full lifecycle:
//
//  1. Create a session from the root.
//  2. Chdir into a subdirectory.
//  3. Run plan with -i.
//  4. Run review with -i.
//  5. End session and audit.
//  6. Verify: total_commands >= 2, success_count >= 2, failure_count = 0,
//     plan_run = true, review_run = true, grade != "F".
func TestSessionAndGraphLoading_SubdirectoryE2E(t *testing.T) {
	root, bin := setupGraphFixture(t)
	subDir := filepath.Join(root, "internal", "sub")

	// 1. Create session from root.
	cmd := exec.Command(bin, "session", "create", "subdirgraph")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session create: %v\n%s", err, out)
	}

	// 2+3. Plan from subdirectory with intention.
	cmd = exec.Command(bin, "plan", "RunAudit", "-i", "subdir e2e plan")
	cmd.Dir = subDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plan from subdir: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Change plan for RunAudit") {
		t.Errorf("plan did not produce expected output:\n%s", out)
	}

	// 4. Review from subdirectory with intention.
	cmd = exec.Command(bin, "review", "RunAudit", "-i", "subdir e2e review")
	cmd.Dir = subDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("review from subdir: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Code Review for RunAudit") {
		t.Errorf("review did not produce expected output:\n%s", out)
	}

	// 5. End session.
	cmd = exec.Command(bin, "session", "end")
	cmd.Dir = root
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session end: %v\n%s", err, out)
	}

	// 6. Audit in JSON mode.
	cmd = exec.Command(bin, "session", "audit", "--json")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("session audit: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	var report struct {
		TotalCommands int     `json:"total_commands"`
		SuccessCount  int     `json:"success_count"`
		FailureCount  int     `json:"failure_count"`
		PlanRun       bool    `json:"plan_run"`
		ReviewRun     bool    `json:"review_run"`
		Grade         string  `json:"grade"`
		SuccessRate   float64 `json:"success_rate"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("parse audit JSON: %v\nraw: %s", err, stdout.String())
	}

	if report.TotalCommands < 2 {
		t.Errorf("total_commands = %d, want >= 2", report.TotalCommands)
	}
	if report.SuccessCount < 2 {
		t.Errorf("success_count = %d, want >= 2", report.SuccessCount)
	}
	if report.FailureCount != 0 {
		t.Errorf("failure_count = %d, want 0", report.FailureCount)
	}
	if !report.PlanRun {
		t.Error("plan_run = false, want true")
	}
	if !report.ReviewRun {
		t.Error("review_run = false, want true")
	}
	if strings.HasPrefix(report.Grade, "F") {
		t.Errorf("grade = %q, want non-F", report.Grade)
	}
}
