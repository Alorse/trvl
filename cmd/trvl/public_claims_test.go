package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicDocsAdvertiseCurrentCounts(t *testing.T) {
	t.Parallel()

	checks := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{
			path: filepath.Join("..", "..", "README.md"),
			required: []string{
				"31 travel tools for your AI assistant",
				"standalone CLI with 31 commands",
				"31 travel tools available",
				"Full v2025-11-25 — 31 tools",
				"31 commands (+ 6 watch subcommands)",
			},
			forbidden: []string{
				"29 travel tools for your AI assistant",
				"standalone CLI with 29 commands",
				"29 travel tools available",
				"Full v2025-11-25 — 29 tools",
				"29 commands (+ 6 watch subcommands)",
			},
		},
		{
			path: filepath.Join("..", "..", "AGENTS.md"),
			required: []string{
				"installed with 31 MCP tools and 5 skills",
				"You now have 31 MCP tools available.",
			},
			forbidden: []string{
				"installed with 22 MCP tools and 5 skills",
				"You now have 22 MCP tools available.",
			},
		},
		{
			path: filepath.Join("..", "..", "demo.tape"),
			required: []string{
				"# 31 MCP tools · 31 CLI commands · 17 providers · No API keys",
			},
			forbidden: []string{
				"# 29 MCP tools · 29 CLI commands · 17 providers · No API keys",
			},
		},
	}

	for _, check := range checks {
		check := check
		t.Run(filepath.Base(check.path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(check.path)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", check.path, err)
			}
			text := string(data)

			for _, needle := range check.required {
				if !strings.Contains(text, needle) {
					t.Errorf("%s missing required text %q", check.path, needle)
				}
			}
			for _, needle := range check.forbidden {
				if strings.Contains(text, needle) {
					t.Errorf("%s still contains stale text %q", check.path, needle)
				}
			}
		})
	}
}
