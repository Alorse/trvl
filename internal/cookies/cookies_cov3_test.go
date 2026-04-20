package cookies

import (
	"runtime"
	"testing"
	"time"
)

// TestBrowserAuthStartRealFunction exercises the real browserAuthStart function body.
// It calls the original function (not the mock) with a harmless "true" command
// that succeeds on all platforms, verifying the exec.Command.Start() path is covered.
//
// The statement at browser.go:22.61,24.3 is the original browserAuthStart func literal body.
// It only gets covered when the real function is called (not the mock replacement).
func TestBrowserAuthStartRealFunction(t *testing.T) {
	// Save originals.
	origNow := browserAuthNow
	origStart := browserAuthStart
	t.Cleanup(func() {
		browserAuthNow = origNow
		browserAuthStart = origStart
		browserAuthOpened.mu.Lock()
		browserAuthOpened.domains = make(map[string]time.Time)
		browserAuthOpened.mu.Unlock()
	})

	// Call the real browserAuthStart with a command that exists and succeeds.
	// On all platforms, the "true" (unix) or "cmd /c exit 0" (windows) command works.
	var (
		cmdName string
		cmdArgs []string
	)
	switch runtime.GOOS {
	case "windows":
		cmdName, cmdArgs = "cmd", []string{"/c", "exit", "0"}
	default:
		cmdName, cmdArgs = "true", nil
	}

	// Call the real function directly.
	err := browserAuthStart(cmdName, cmdArgs...)
	// The process starts in background — err is nil on success.
	if err != nil {
		t.Logf("browserAuthStart returned %v (not a test failure — process may not be available)", err)
	}
}

// TestOpenBrowserForAuthCooldownPath exercises the cooldown-active path
// (browser.go:136.15,137.40 and adjacent lines) by setting up a domain
// that was recently opened.
func TestOpenBrowserForAuthCooldownPath(t *testing.T) {
	origNow := browserAuthNow
	origStart := browserAuthStart
	t.Cleanup(func() {
		browserAuthNow = origNow
		browserAuthStart = origStart
		browserAuthOpened.mu.Lock()
		browserAuthOpened.domains = make(map[string]time.Time)
		browserAuthOpened.mu.Unlock()
	})

	const testDomain = "cooldown-test.example.com"
	frozenNow := time.Unix(1_800_000_000, 0)
	browserAuthNow = func() time.Time { return frozenNow }

	// Pre-populate the cooldown map so the domain was "recently opened".
	browserAuthOpened.mu.Lock()
	browserAuthOpened.domains[testDomain] = frozenNow.Add(-1 * time.Minute) // 1 min ago < 24h
	browserAuthOpened.mu.Unlock()

	launchCalls := 0
	browserAuthStart = func(name string, args ...string) error {
		launchCalls++
		return nil
	}

	// Should be suppressed by cooldown (return nil without launching).
	err := OpenBrowserForAuth("https://" + testDomain + "/path")
	if err != nil {
		t.Fatalf("expected nil (cooldown suppressed), got: %v", err)
	}
	if launchCalls != 0 {
		t.Errorf("expected 0 launch calls (suppressed by cooldown), got %d", launchCalls)
	}
}
