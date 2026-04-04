package main

import (
	"github.com/MikkoParkkola/trvl/mcp"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	var (
		httpMode bool
		port     int
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP (Model Context Protocol) server",
		Long: `Start the trvl MCP server for AI agent integration.

By default, runs in stdio mode (JSON-RPC over stdin/stdout) for use with
Claude Code and other MCP-compatible clients.

Use --http to start an HTTP server instead, suitable for gateway and remote access.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if httpMode {
				return mcp.RunHTTP(port)
			}
			return mcp.Run()
		},
	}

	cmd.Flags().BoolVar(&httpMode, "http", false, "Run as HTTP server instead of stdio")
	cmd.Flags().IntVar(&port, "port", 8080, "HTTP server port (only used with --http)")

	return cmd
}
