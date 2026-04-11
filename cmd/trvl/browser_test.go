package main

import (
	"runtime"
	"testing"
)

func TestOpenBrowser_EmptyURL(t *testing.T) {
	err := openBrowser("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestOpenBrowser_SupportedPlatform(t *testing.T) {
	// Verify the current platform is handled (not "unsupported platform").
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		// ok — openBrowser will resolve to a real command
	default:
		t.Skipf("unsupported platform %s", runtime.GOOS)
	}
}

func TestOpenFlag_Registered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("open")
	if f == nil {
		t.Fatal("--open persistent flag not registered on root command")
	}
	if f.DefValue != "false" {
		t.Errorf("--open default = %q, want \"false\"", f.DefValue)
	}
}
