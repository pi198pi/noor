package main

import (
	"fmt"
	"os"
)

func main() {
	// Subcommand: noor config validate
	if len(os.Args) >= 3 && os.Args[1] == "config" && os.Args[2] == "validate" {
		errs := validateConfig()
		if errs > 0 {
			os.Exit(1)
		}
		return
	}

	cfg := parseFlags()
	loadConfig(&cfg)

	if cfg.Debug || os.Getenv("DEBUG") == "1" {
		EnableDebugLogging()
	}

	if cfg.Theme != "" {
		applyTheme(cfg.Theme)
	}

	if cfg.APIKey == "" {
		printError("OPENROUTER_API_KEY is not set. Set it in the environment or ~/.config/" + AppName + "/config")
		os.Exit(1)
	}

	var mcp *MCPClient
	if cfg.MCPServer != "" {
		var err error
		mcp, err = NewMCPClient(cfg.MCPServer)
		if err != nil {
			printError("MCP server failed to start: " + err.Error())
			os.Exit(1)
		}
		// chatLoop owns the MCP lifecycle (it can swap clients via /mcp)
		printSuccess(fmt.Sprintf("MCP: loaded %d tools", len(mcp.Tools())))
	}

	setupSignals(mcp)
	chatLoop(&cfg, mcp)
}
