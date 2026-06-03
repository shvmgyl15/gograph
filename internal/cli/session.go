package cli

import (
	"fmt"
	"os"

	"github.com/ozgurcd/gograph/internal/session"
)

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
		sessionID, err := session.StartSession(customWord)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting session: %v\n", err)
			return 1
		}
		fmt.Printf("Session %q successfully created and activated.\n", sessionID)
		return 0

	case "end":
		sessionID, err := session.EndSession()
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
		return session.RunAudit(sessionID, jsonMode)

	case "cleanup":
		count, err := session.CleanupSessions()
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
