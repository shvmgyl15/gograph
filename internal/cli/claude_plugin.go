package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// installPlugin modifies the claude_desktop_config.json to add gograph as an MCP server.
func installPlugin() error {
	fmt.Println("🚀 Installing Gograph Claude Desktop Plugin...")

	configPath := getClaudeConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine Claude Desktop config path for your OS")
	}

	// Ensure the config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Find gograph binary
	gographPath, err := exec.LookPath("gograph")
	if err != nil {
		fmt.Println("⚠️  Warning: 'gograph' is not in your PATH. The plugin may fail to start.")
		fmt.Println("   We will use 'gograph' as the command, but you may need to update the config with the absolute path.")
		gographPath = "gograph"
	} else {
		// Try to make it absolute just in case
		if absPath, err := filepath.Abs(gographPath); err == nil {
			gographPath = absPath
		}
	}

	// Read existing config or create new
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config = make(map[string]interface{})
		} else {
			return fmt.Errorf("failed to read existing config: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing config (is it valid JSON?): %w", err)
		}
	}

	// Ensure mcpServers exists
	if _, ok := config["mcpServers"]; !ok {
		config["mcpServers"] = make(map[string]interface{})
	}
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("'mcpServers' in config is not a JSON object")
	}

	// Add gograph
	mcpServers["gograph"] = map[string]interface{}{
		"command": gographPath,
		"args":    []string{"mcp", "."},
	}

	// Write back
	outData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, outData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("✅ Successfully added 'gograph' to Claude Desktop MCP config!")
	fmt.Printf("   Config updated at: %s\n\n", configPath)
	fmt.Println("🔄 Please restart Claude Desktop for the plugin to take effect.")
	fmt.Println("---------------------------------------------------------")
	fmt.Println("🛠️  If you are using Claude Code (CLI), run this instead:")
	fmt.Println("   claude mcp add gograph -- gograph mcp .")

	return nil
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
