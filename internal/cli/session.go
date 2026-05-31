package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/ozgurcd/gograph/internal/session"
)

// Re-expose types/constants for compatibility if needed, or simply delegate.
func GetActiveSessionID() (string, error) {
	return session.GetActiveSessionID()
}

func StartSession(customWord string) (string, error) {
	return session.StartSession(customWord)
}

func EndSession() (string, error) {
	return session.EndSession()
}

func LogCommand(command string, args []string, intention string, elapsed time.Duration, status string) error {
	return session.LogCommand(command, args, intention, elapsed, status)
}

func RunAudit(sessionID string) int {
	return session.RunAudit(sessionID, jsonMode)
}

func CleanupSessions() (int, error) {
	return session.CleanupSessions()
}

// runSession manages the `--session` / `session` CLI subcommands.
func runSession(args []string) int {
	if len(args) == 0 {
		printSessionHelp()
		return 1
	}

	switch args[0] {
	case "create":
		customWord := ""
		if len(args) >= 2 {
			customWord = args[1]
		}
		sessionID, err := StartSession(customWord)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting session: %v\n", err)
			return 1
		}
		fmt.Printf("Session %q successfully created and activated.\n", sessionID)
		return 0

	case "end":
		sessionID, err := EndSession()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error ending session: %v\n", err)
			return 1
		}
		fmt.Printf("Session %q successfully ended.\n", sessionID)
		return 0

	case "audit":
		sessionID := ""
		if len(args) >= 2 {
			sessionID = args[1]
		}
		return RunAudit(sessionID)

	case "cleanup":
		count, err := CleanupSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning up sessions: %v\n", err)
			return 1
		}
		fmt.Printf("Successfully deleted %d stale session log files.\n", count)
		return 0

	default:
		printSessionHelp()
		return 1
	}
}

func printSessionHelp() {
	fmt.Println("Usage:")
	fmt.Println("  gograph session create [unique_identifier_word]  - Starts a new audit session")
	fmt.Println("  gograph session end                              - Ends the active audit session")
	fmt.Println("  gograph session audit [session_id]               - Audits and scores agent compliance & success")
	fmt.Println("  gograph session cleanup                          - Deletes all inactive session log files")
}

