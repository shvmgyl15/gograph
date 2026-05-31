package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	sessionsDir       = ".gograph/sessions"
	activePointerPath = ".gograph/active_session.json"
)

// ActiveSessionPointer tracks the currently active session ID.
type ActiveSessionPointer struct {
	ActiveSessionID string `json:"active_session_id"`
}

// SessionStartEntry logs the start of a session.
type SessionStartEntry struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	CreatedAt string `json:"created_at"`
}

// SessionEndEntry logs the termination of a session.
type SessionEndEntry struct {
	Type    string `json:"type"`
	EndedAt string `json:"ended_at"`
	Status  string `json:"status"`
}

// CommandLogEntry logs telemetry metadata for an executed command.
type CommandLogEntry struct {
	Type        string   `json:"type"`
	Timestamp   string   `json:"timestamp"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Intention   string   `json:"intention"`
	ExecutionMs int64    `json:"execution_ms"`
	Status      string   `json:"status"`
}

// GetActiveSessionID retrieves the currently active session ID if it exists.
func GetActiveSessionID() (string, error) {
	if _, err := os.Stat(activePointerPath); os.IsNotExist(err) {
		return "", nil
	}

	data, err := os.ReadFile(activePointerPath)
	if err != nil {
		return "", fmt.Errorf("read active pointer: %w", err)
	}

	var ptr ActiveSessionPointer
	if err := json.Unmarshal(data, &ptr); err != nil {
		return "", fmt.Errorf("unmarshal active pointer: %w", err)
	}

	return ptr.ActiveSessionID, nil
}

// StartSession initializes a new session and writes the active session pointer.
func StartSession(customWord string) (string, error) {
	// 1. Check if a session is already active
	activeID, err := GetActiveSessionID()
	if err != nil {
		return "", err
	}
	if activeID != "" {
		return "", fmt.Errorf("a session is already active: %q. Please end it first", activeID)
	}

	// 2. Generate unique session ID
	timestamp := time.Now().Format("20060102_150405")
	var sessionID string
	if customWord != "" {
		// Clean the custom word (alphanumeric and underscores only)
		reg := regexp.MustCompile("[^a-zA-Z0-9_]")
		cleanWord := reg.ReplaceAllString(customWord, "")
		if cleanWord == "" {
			cleanWord = "custom"
		}
		sessionID = fmt.Sprintf("%s_%s", cleanWord, timestamp)
	} else {
		// Generate 6 random hex characters
		randBytes := make([]byte, 3)
		_, _ = rand.Read(randBytes)
		randSlug := hex.EncodeToString(randBytes)
		sessionID = fmt.Sprintf("session_%s_%s", randSlug, timestamp)
	}

	// 3. Ensure directories exist
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", fmt.Errorf("create sessions directory: %w", err)
	}

	// 4. Create and write the session start log
	logFilePath := filepath.Join(sessionsDir, fmt.Sprintf("session_%s.jsonl", sessionID))
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("create session log file: %w", err)
	}
	defer logFile.Close()

	startEntry := SessionStartEntry{
		Type:      "session_start",
		SessionID: sessionID,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	startBytes, _ := json.Marshal(startEntry)
	if _, err := logFile.Write(append(startBytes, '\n')); err != nil {
		return "", fmt.Errorf("write session start entry: %w", err)
	}

	// 5. Write the active session pointer
	ptr := ActiveSessionPointer{ActiveSessionID: sessionID}
	ptrBytes, _ := json.MarshalIndent(ptr, "", "  ")
	if err := os.WriteFile(activePointerPath, ptrBytes, 0644); err != nil {
		return "", fmt.Errorf("write active session pointer: %w", err)
	}

	return sessionID, nil
}

// EndSession ends the currently active session.
func EndSession() (string, error) {
	// 1. Get active session ID
	activeID, err := GetActiveSessionID()
	if err != nil {
		return "", err
	}
	if activeID == "" {
		return "", fmt.Errorf("no active session to end")
	}

	// 2. Append end entry to log file
	logFilePath := filepath.Join(sessionsDir, fmt.Sprintf("session_%s.jsonl", activeID))
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// If log file was manually deleted, still allow clean teardown of the active pointer
		_ = os.Remove(activePointerPath)
		return activeID, fmt.Errorf("open session log for append: %w (pointer cleaned up)", err)
	}
	defer logFile.Close()

	endEntry := SessionEndEntry{
		Type:    "session_end",
		EndedAt: time.Now().Format(time.RFC3339),
		Status:  "completed",
	}
	endBytes, _ := json.Marshal(endEntry)
	_, _ = logFile.Write(append(endBytes, '\n'))

	// 3. Remove the active pointer file
	if err := os.Remove(activePointerPath); err != nil {
		return activeID, fmt.Errorf("remove active pointer: %w", err)
	}

	return activeID, nil
}

// LogCommand Telemetry records command execution details inside the active session log if present.
func LogCommand(command string, args []string, intention string, elapsed time.Duration, status string) error {
	activeID, err := GetActiveSessionID()
	if err != nil || activeID == "" {
		return nil // No active session to log to
	}

	logFilePath := filepath.Join(sessionsDir, fmt.Sprintf("session_%s.jsonl", activeID))
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open session log for append: %w", err)
	}
	defer logFile.Close()

	entry := CommandLogEntry{
		Type:        "command",
		Timestamp:   time.Now().Format(time.RFC3339),
		Command:     command,
		Args:        args,
		Intention:   intention,
		ExecutionMs: elapsed.Milliseconds(),
		Status:      status,
	}

	entryBytes, _ := json.Marshal(entry)
	if _, err := logFile.Write(append(entryBytes, '\n')); err != nil {
		return fmt.Errorf("write command log entry: %w", err)
	}

	return nil
}

// GenericLogLine represents any log line parsed from a JSONL session file.
type GenericLogLine struct {
	Type        string   `json:"type"`
	SessionID   string   `json:"session_id"`
	CreatedAt   string   `json:"created_at"`
	EndedAt     string   `json:"ended_at"`
	Timestamp   string   `json:"timestamp"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Intention   string   `json:"intention"`
	ExecutionMs int64    `json:"execution_ms"`
	Status      string   `json:"status"`
}

// AuditReport holds the calculated compliance and success metrics of a session.
type AuditReport struct {
	SessionID       string    `json:"session_id"`
	Status          string    `json:"status"`
	CreatedAt       string    `json:"created_at"`
	EndedAt         string    `json:"ended_at"`
	DurationSeconds float64   `json:"duration_seconds"`
	TotalCommands   int       `json:"total_commands"`
	SuccessCount    int       `json:"success_count"`
	FailureCount    int       `json:"failure_count"`
	SuccessRate     float64   `json:"success_rate"`
	PlanRun         bool      `json:"plan_run"`
	ReviewRun       bool      `json:"review_run"`
	ComposedCount   int       `json:"composed_count"`
	RawQueryCount   int       `json:"raw_query_count"`
	Composability   float64   `json:"composability"`
	ComplianceScore float64   `json:"compliance_score"`
	Grade           string    `json:"grade"`
	Recommendations []string  `json:"recommendations"`
}

// FindMostRecentSessionID finds the session log file with the newest modification time.
func FindMostRecentSessionID() (string, error) {
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return "", fmt.Errorf("no sessions directory exists yet")
	}

	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", fmt.Errorf("read sessions directory: %w", err)
	}

	var newestID string
	var newestTime time.Time

	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), "session_") && strings.HasSuffix(f.Name(), ".jsonl") {
			info, err := f.Info()
			if err == nil {
				if newestID == "" || info.ModTime().After(newestTime) {
					name := strings.TrimPrefix(f.Name(), "session_")
					name = strings.TrimSuffix(name, ".jsonl")
					newestID = name
					newestTime = info.ModTime()
				}
			}
		}
	}

	if newestID == "" {
		return "", fmt.Errorf("no session logs found in %s", sessionsDir)
	}

	return newestID, nil
}

// RunAudit parses and scores a session for agent compliance and success rates.
func RunAudit(sessionID string, jsonMode bool) int {
	var err error
	if sessionID == "" {
		sessionID, err = FindMostRecentSessionID()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error locating session: %v\n", err)
			return 1
		}
	}

	logFilePath := filepath.Join(sessionsDir, fmt.Sprintf("session_%s.jsonl", sessionID))
	file, err := os.Open(logFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening session log %q: %v\n", logFilePath, err)
		return 1
	}
	defer file.Close()

	var lines []GenericLogLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		var line GenericLogLine
		if err := json.Unmarshal([]byte(text), &line); err == nil {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		fmt.Fprintf(os.Stderr, "Error: session log %q is empty or corrupt\n", logFilePath)
		return 1
	}

	// 1. Accumulate metrics
	var start time.Time
	var end time.Time
	status := "In Progress"
	var totalCommands, successCount, failureCount int
	var planRun, reviewRun bool
	var composedCount, rawQueryCount int

	for _, l := range lines {
		switch l.Type {
		case "session_start":
			start, _ = time.Parse(time.RFC3339, l.CreatedAt)
		case "session_end":
			end, _ = time.Parse(time.RFC3339, l.EndedAt)
			status = "Completed"
		case "command":
			totalCommands++
			if l.Status == "success" {
				successCount++
			} else {
				failureCount++
			}

			// Trace command signatures
			switch l.Command {
			case "plan":
				planRun = true
				composedCount++
			case "review":
				reviewRun = true
				composedCount++
			case "context", "explain", "api", "changes", "mutate":
				composedCount++
			case "node", "callers", "callees", "source":
				rawQueryCount++
			}
		}
	}

	if end.IsZero() && !start.IsZero() && len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if lastLine.Timestamp != "" {
			end, _ = time.Parse(time.RFC3339, lastLine.Timestamp)
		} else {
			end = time.Now()
		}
	}

	duration := end.Sub(start)
	if duration < 0 {
		duration = 0
	}

	successRate := 100.0
	if totalCommands > 0 {
		successRate = (float64(successCount) / float64(totalCommands)) * 100.0
	}

	var planContrib, reviewContrib, composedContrib float64
	if planRun {
		planContrib = 35.0
	}
	if reviewRun {
		reviewContrib = 35.0
	}

	composability := 100.0
	if composedCount+rawQueryCount > 0 {
		composability = (float64(composedCount) / float64(composedCount+rawQueryCount)) * 100.0
	}
	composedContrib = composability * 0.30

	complianceScore := planContrib + reviewContrib + composedContrib

	var grade string
	switch {
	case complianceScore >= 90.0:
		grade = "A (Highly Compliant)"
	case complianceScore >= 80.0:
		grade = "B (Good Compliance)"
	case complianceScore >= 70.0:
		grade = "C (Needs Improvement)"
	default:
		grade = "F (Non-Compliant)"
	}

	var recs []string
	if !planRun {
		recs = append(recs, "Agent failed to execute 'plan <symbol>' before modifying code. Advise the agent to run 'plan <symbol>' to analyze downstreams and mapped tests.")
	}
	if !reviewRun {
		recs = append(recs, "Agent failed to execute 'review --uncommitted' or 'review <symbol>' to verify its edits. Instruct the agent to run 'review --uncommitted' post-edit.")
	}
	if rawQueryCount > 3 && composedCount == 0 {
		recs = append(recs, "Agent executed multiple individual raw queries (node/callers/callees) instead of the composed single-call 'context' tool. Prompt the agent to use 'context <symbol>' to save API context window tokens.")
	}

	report := AuditReport{
		SessionID:       sessionID,
		Status:          status,
		CreatedAt:       start.Format(time.RFC3339),
		EndedAt:         end.Format(time.RFC3339),
		DurationSeconds: duration.Seconds(),
		TotalCommands:   totalCommands,
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		SuccessRate:     successRate,
		PlanRun:         planRun,
		ReviewRun:       reviewRun,
		ComposedCount:   composedCount,
		RawQueryCount:   rawQueryCount,
		Composability:   composability,
		ComplianceScore: complianceScore,
		Grade:           grade,
		Recommendations: recs,
	}

	if jsonMode {
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("GOGRAPH AGENT SESSION AUDIT\n")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Session ID      : %s\n", report.SessionID)
	fmt.Printf("Status          : %s\n", report.Status)
	fmt.Printf("Created At      : %s\n", report.CreatedAt)
	fmt.Printf("Ended At        : %s\n", report.EndedAt)
	fmt.Printf("Duration        : %v\n", duration.Round(time.Second))
	fmt.Println()
	fmt.Printf("━━━ METRICS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Total Commands  : %d\n", report.TotalCommands)
	fmt.Printf("Successful      : %d\n", report.SuccessCount)
	fmt.Printf("Failed          : %d\n", report.FailureCount)
	fmt.Printf("Success Rate    : %.1f%%\n", report.SuccessRate)
	fmt.Println()
	fmt.Printf("━━━ COMPLIANCE SCORE ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Plan Rule Run   : %t (Weight: 35%%)\n", report.PlanRun)
	fmt.Printf("Review Rule Run : %t (Weight: 35%%)\n", report.ReviewRun)
	fmt.Printf("Composability   : %.1f%% (Weight: 30%%)\n", report.Composability)
	fmt.Println()
	fmt.Printf("Overall Score   : %.1f%%\n", report.ComplianceScore)
	fmt.Printf("Compliance Grade: %s\n", report.Grade)
	fmt.Println()
	if len(report.Recommendations) > 0 {
		fmt.Printf("━━━ RECOMMENDATIONS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		for _, r := range report.Recommendations {
			fmt.Printf("* %s\n", r)
		}
	} else {
		fmt.Printf("━━━ RECOMMENDATIONS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Println("Perfect! AI agent followed all core compliance and efficiency workflow rules.")
	}
	fmt.Println(strings.Repeat("=", 80))

	return 0
}

// CleanupSessions deletes all inactive session JSONL logs. If no session is active, it deletes all logs.
func CleanupSessions() (int, error) {
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return 0, nil
	}

	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return 0, fmt.Errorf("read sessions directory: %w", err)
	}

	activeID, _ := GetActiveSessionID()
	activeFileName := ""
	if activeID != "" {
		activeFileName = fmt.Sprintf("session_%s.jsonl", activeID)
	}

	deletedCount := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), "session_") && strings.HasSuffix(f.Name(), ".jsonl") {
			if activeFileName != "" && f.Name() == activeFileName {
				continue // Skip active session log to prevent runtime telemetry corruption
			}
			filePath := filepath.Join(sessionsDir, f.Name())
			if err := os.Remove(filePath); err == nil {
				deletedCount++
			}
		}
	}

	return deletedCount, nil
}

