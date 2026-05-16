package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const claudeMDBlock = `
<!-- gograph-start: do not remove -->
## Gograph - Go Repository Intelligence

Rules (enforced when gograph MCP server is connected):

1. NEVER use grep, rg, find, or glob to search for Go symbols, functions, structs, or types. Use gograph_query instead.
2. Before editing any Go symbol, run gograph_plan <symbol> to understand callers, tests, and risk.
3. After editing Go code, run gograph_review --uncommitted to verify test coverage and blast radius.
4. To understand a function, use gograph_context <symbol> (replaces 4-5 separate tool calls).
5. Run gograph_capabilities at the start of any Go coding session.
<!-- gograph-end: do not remove -->
`

const hookScript = `#!/bin/sh
# gograph-guard: Smart PreToolUse hook for Claude Code.
# Intercepts grep/rg calls targeting Go symbols and redirects to gograph MCP tools.
# Installed automatically by: gograph add-claude-plugin
exec gograph hook-guard
`

// installPlugin registers gograph as an MCP server, injects CLAUDE.md rules,
// and installs the smart PreToolUse hook for Claude Code.
func installPlugin() error {
	fmt.Println("🚀 Installing Gograph Claude Integration...")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	// 1. Register MCP server in Claude Desktop config
	if err := installMCPServer(home); err != nil {
		fmt.Printf("⚠️  MCP server registration skipped: %v\n", err)
	}

	// 2. Inject CLAUDE.md steering rules
	if err := installCLAUDEmd(home); err != nil {
		fmt.Printf("⚠️  CLAUDE.md injection skipped: %v\n", err)
	} else {
		fmt.Println("✅ CLAUDE.md steering rules injected (~/.claude/CLAUDE.md)")
	}

	// 3. Install hook script and register in Claude settings.json
	if err := installHook(home); err != nil {
		fmt.Printf("⚠️  Hook installation skipped: %v\n", err)
	} else {
		fmt.Println("✅ PreToolUse hook installed (~/.claude/hooks/gograph-guard.sh)")
	}

	fmt.Println("\n🔄 Restart Claude Desktop / Claude Code for all changes to take effect.")
	fmt.Println("💡 Claude Code users: also run: claude mcp add gograph -- gograph mcp .")
	return nil
}

// installMCPServer writes the gograph entry into claude_desktop_config.json.
func installMCPServer(home string) error {
	configPath := getClaudeConfigPath()
	if configPath == "" {
		return fmt.Errorf("unsupported OS")
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	gographPath, err := exec.LookPath("gograph")
	if err != nil {
		gographPath = "gograph"
	} else if absPath, err2 := filepath.Abs(gographPath); err2 == nil {
		gographPath = absPath
	}

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config = make(map[string]interface{})
		} else {
			return err
		}
	} else if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if _, ok := config["mcpServers"]; !ok {
		config["mcpServers"] = make(map[string]interface{})
	}
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("mcpServers is not a JSON object")
	}
	mcpServers["gograph"] = map[string]interface{}{
		"command": gographPath,
		"args":    []string{"mcp", "."},
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return err
	}
	fmt.Printf("✅ MCP server registered in %s\n", configPath)
	return nil
}

// installCLAUDEmd appends gograph steering rules to ~/.claude/CLAUDE.md (idempotent).
func installCLAUDEmd(home string) error {
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return err
	}
	mdPath := filepath.Join(claudeDir, "CLAUDE.md")

	existing, _ := os.ReadFile(mdPath)
	if strings.Contains(string(existing), "<!-- gograph-start: do not remove -->") {
		return nil // already installed
	}

	f, err := os.OpenFile(mdPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(claudeMDBlock)
	return err
}

// installHook writes the hook script and registers it in ~/.claude/settings.json.
func installHook(home string) error {
	hooksDir := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return err
	}

	hookPath := filepath.Join(hooksDir, "gograph-guard.sh")
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return err
	}

	// Update ~/.claude/settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]interface{})
		} else {
			return err
		}
	} else if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	// Build/merge the hooks section
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}
	preToolUse, _ := hooks["PreToolUse"].([]interface{})

	// Check if already registered
	for _, entry := range preToolUse {
		if m, ok := entry.(map[string]interface{}); ok {
			if m["matcher"] == "Bash" {
				innerHooks, _ := m["hooks"].([]interface{})
				for _, h := range innerHooks {
					if hm, ok := h.(map[string]interface{}); ok {
						if hm["command"] == hookPath {
							return nil // already registered
						}
					}
				}
			}
		}
	}

	// Append our hook entry
	preToolUse = append(preToolUse, map[string]interface{}{
		"matcher": "Bash",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": hookPath,
			},
		},
	})
	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0644)
}

func getClaudeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin": // macOS
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows": // Windows
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json")
	case "linux": // Linux (Unofficial but standard)
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	default:
		return ""
	}
}
