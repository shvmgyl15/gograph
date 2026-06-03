package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chdir changes the working directory to a temp dir for the duration of the
// test and restores it afterwards. Session uses relative paths (.gograph/…)
// so every test must run in an isolated temp directory.
func chdir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Logf("warning: could not restore working directory: %v", err)
		}
	})
}

// --- GetActiveSessionID ---

func TestGetActiveSessionID_NoPointerFile(t *testing.T) {
	chdir(t)
	id, err := GetActiveSessionID()
	if err != nil {
		t.Fatalf("expected no error when pointer file absent, got: %v", err)
	}
	if id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
}

func TestGetActiveSessionID_CorruptPointerFile(t *testing.T) {
	chdir(t)
	if err := os.MkdirAll(".gograph", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(activePointerPath, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write corrupt pointer: %v", err)
	}
	_, err := GetActiveSessionID()
	if err == nil {
		t.Fatal("expected error for corrupt JSON, got nil")
	}
}

// --- StartSession / EndSession lifecycle ---

func TestStartAndEndSession_Default(t *testing.T) {
	chdir(t)

	id, err := StartSession("")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if !strings.HasPrefix(id, "session_") {
		t.Errorf("expected id to start with 'session_', got %q", id)
	}

	// Active pointer must now exist
	activeID, err := GetActiveSessionID()
	if err != nil {
		t.Fatalf("GetActiveSessionID after start: %v", err)
	}
	if activeID != id {
		t.Errorf("active id %q != created id %q", activeID, id)
	}

	// End the session
	endedID, err := EndSession()
	if err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if endedID != id {
		t.Errorf("ended id %q != created id %q", endedID, id)
	}

	// Active pointer must be gone
	afterID, err := GetActiveSessionID()
	if err != nil {
		t.Fatalf("GetActiveSessionID after end: %v", err)
	}
	if afterID != "" {
		t.Errorf("expected no active session after end, got %q", afterID)
	}
}

func TestStartSession_CustomWord(t *testing.T) {
	chdir(t)
	id, err := StartSession("myfeature")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if !strings.HasPrefix(id, "myfeature_") {
		t.Errorf("expected id to start with 'myfeature_', got %q", id)
	}
	// cleanup
	if _, err := EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}

func TestStartSession_SpecialCharsInWord(t *testing.T) {
	chdir(t)
	// Special characters should be stripped; only alphanumeric/underscore kept.
	id, err := StartSession("my-feature!@#")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if !strings.HasPrefix(id, "myfeature_") {
		t.Errorf("expected cleaned prefix 'myfeature_', got %q", id)
	}
	_, _ = EndSession()
}

func TestStartSession_AllSpecialChars_FallsBackToCustom(t *testing.T) {
	chdir(t)
	id, err := StartSession("!@#$%")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	// When the cleaned word is empty the code uses "custom"
	if !strings.HasPrefix(id, "custom_") {
		t.Errorf("expected prefix 'custom_', got %q", id)
	}
	_, _ = EndSession()
}

func TestStartSession_AlreadyActive(t *testing.T) {
	chdir(t)
	if _, err := StartSession("first"); err != nil {
		t.Fatalf("first StartSession: %v", err)
	}
	_, err := StartSession("second")
	if err == nil {
		t.Fatal("expected error when a session is already active")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Errorf("unexpected error text: %v", err)
	}
	_, _ = EndSession()
}

func TestEndSession_NoActiveSession(t *testing.T) {
	chdir(t)
	_, err := EndSession()
	if err == nil {
		t.Fatal("expected error when no session is active")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// --- LogCommand ---

func TestLogCommand_NoActiveSession_IsNoOp(t *testing.T) {
	chdir(t)
	// Must not error when there is no active session.
	err := LogCommand("query", []string{"Foo"}, "testing", time.Millisecond*10, "success")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestLogCommand_HookGuardSuccessSkipped(t *testing.T) {
	chdir(t)
	id, err := StartSession("hooktest")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer func() { _, _ = EndSession() }()

	// hook-guard success must be silently skipped — nothing written to the log.
	if err := LogCommand("hook-guard", nil, "", 0, "success"); err != nil {
		t.Fatalf("LogCommand hook-guard success: %v", err)
	}

	// Read the session log and confirm no "command" entry was written.
	logPath := filepath.Join(sessionsDir, "session_"+id+".jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read session log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		var entry GenericLogLine
		if json.Unmarshal([]byte(line), &entry) == nil && entry.Type == "command" {
			t.Errorf("hook-guard success was logged but should have been skipped: %s", line)
		}
	}
}

func TestLogCommand_HookGuardFailureIsLogged(t *testing.T) {
	chdir(t)
	_, err := StartSession("hookfail")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer func() { _, _ = EndSession() }()

	// hook-guard failure must be logged normally.
	if err := LogCommand("hook-guard", nil, "", 0, "failure"); err != nil {
		t.Fatalf("LogCommand hook-guard failure: %v", err)
	}
	activeID, _ := GetActiveSessionID()
	logPath := filepath.Join(sessionsDir, "session_"+activeID+".jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read session log: %v", err)
	}
	if !strings.Contains(string(data), `"command"`) {
		t.Error("expected command entry in log but found none")
	}
}

func TestLogCommand_WritesEntry(t *testing.T) {
	chdir(t)
	_, err := StartSession("logtest")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer func() { _, _ = EndSession() }()

	err = LogCommand("callers", []string{"Foo"}, "find callers of Foo", 42*time.Millisecond, "success")
	if err != nil {
		t.Fatalf("LogCommand: %v", err)
	}

	activeID, _ := GetActiveSessionID()
	logPath := filepath.Join(sessionsDir, "session_"+activeID+".jsonl")
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), `"callers"`) {
		t.Error("expected 'callers' command in session log")
	}
	if !strings.Contains(string(data), `"success"`) {
		t.Error("expected 'success' status in session log")
	}
}

// --- FindMostRecentSessionID ---

func TestFindMostRecentSessionID_NoDir(t *testing.T) {
	chdir(t)
	_, err := FindMostRecentSessionID()
	if err == nil {
		t.Fatal("expected error when sessions directory does not exist")
	}
}

func TestFindMostRecentSessionID_EmptyDir(t *testing.T) {
	chdir(t)
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := FindMostRecentSessionID()
	if err == nil {
		t.Fatal("expected error for empty sessions directory")
	}
}

func TestFindMostRecentSessionID_ReturnsNewest(t *testing.T) {
	chdir(t)
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write two stub session files with different mtime by touching them sequentially.
	older := filepath.Join(sessionsDir, "session_older_20260101_000000.jsonl")
	newer := filepath.Join(sessionsDir, "session_newer_20260102_000000.jsonl")
	for _, f := range []string{older, newer} {
		if err := os.WriteFile(f, []byte(`{"type":"session_start"}`+"\n"), 0o644); err != nil {
			t.Fatalf("write stub: %v", err)
		}
	}
	// Force mtime ordering
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	id, err := FindMostRecentSessionID()
	if err != nil {
		t.Fatalf("FindMostRecentSessionID: %v", err)
	}
	if id != "newer_20260102_000000" {
		t.Errorf("expected 'newer_20260102_000000', got %q", id)
	}
}

// --- CleanupSessions ---

func TestCleanupSessions_NoSessionsDir(t *testing.T) {
	chdir(t)
	count, err := CleanupSessions()
	if err != nil {
		t.Fatalf("CleanupSessions with missing dir: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deletions, got %d", count)
	}
}

func TestCleanupSessions_DeletesInactiveLogs(t *testing.T) {
	chdir(t)
	// Start a session so we have an active log.
	id, err := StartSession("cleantest")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer func() { _, _ = EndSession() }()

	// Create an additional stale log file.
	stalePath := filepath.Join(sessionsDir, "session_stale_20260101_000000.jsonl")
	if err := os.WriteFile(stalePath, []byte(`{"type":"session_start"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write stale log: %v", err)
	}

	count, err := CleanupSessions()
	if err != nil {
		t.Fatalf("CleanupSessions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 deletion, got %d", count)
	}

	// Active session log must still exist.
	activePath := filepath.Join(sessionsDir, "session_"+id+".jsonl")
	if _, err := os.Stat(activePath); os.IsNotExist(err) {
		t.Error("active session log was incorrectly deleted")
	}
}

// --- RunAudit ---

func TestRunAudit_EmptyLog(t *testing.T) {
	chdir(t)
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	badPath := filepath.Join(sessionsDir, "session_empty_test.jsonl")
	if err := os.WriteFile(badPath, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty log: %v", err)
	}
	code := RunAudit("empty_test", false)
	if code != 1 {
		t.Errorf("expected exit code 1 for empty log, got %d", code)
	}
}

func TestRunAudit_FullSession_JSON(t *testing.T) {
	chdir(t)
	id, err := StartSession("auditme")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Log a plan, a review, and a raw callers command.
	for _, cmd := range []string{"plan", "review", "callers"} {
		_ = LogCommand(cmd, []string{"Foo"}, "intention", 10*time.Millisecond, "success")
	}

	if _, err := EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	// RunAudit should succeed (exit 0) with JSON output.
	code := RunAudit(id, true)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRunAudit_FallsBackToMostRecent(t *testing.T) {
	chdir(t)
	id, err := StartSession("recent")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	_ = LogCommand("query", []string{"X"}, "find X", 5*time.Millisecond, "success")
	if _, err := EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	// Passing empty sessionID should auto-resolve to the most recent.
	code := RunAudit("", false)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	_ = id
}
