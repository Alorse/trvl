package afklm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"
)

// ErrNoCredential is returned when no AF-KLM API key can be found via any
// of the supported credential backends. Callers treat this as "provider not
// configured; user must sign up".
var ErrNoCredential = errors.New("afklm: no API key found (set AFKLM_KEY, Keychain, or 1Password)")

// ResolveCredential resolves the AF-KLM API key using the following chain
// (first hit wins):
//
//  1. Environment variable AFKLM_KEY (non-empty).
//  2. macOS Keychain (Darwin only): security find-generic-password -a $USER -s afklm-api-key -w
//  3. 1Password CLI: op read op://Personal/Air France-KLM Developer API/credential
//
// Returns ErrNoCredential when no backend succeeds.
func ResolveCredential(ctx context.Context) (string, error) {
	// 1. Env var.
	if v := os.Getenv("AFKLM_KEY"); v != "" {
		return v, nil
	}

	// 2. macOS Keychain (Darwin only).
	if runtime.GOOS == "darwin" {
		if key, err := keychainLookup(ctx); err == nil {
			return key, nil
		}
	}

	// 3. 1Password CLI.
	if _, err := exec.LookPath("op"); err == nil {
		if key, err := opLookup(ctx); err == nil {
			return key, nil
		}
	}

	return "", ErrNoCredential
}

// Configured reports whether a credential can be resolved. It uses a
// short (500ms) context deadline so that keychain and op calls do not
// block the caller noticeably.
func Configured(ctx context.Context) bool {
	tctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	_, err := ResolveCredential(tctx)
	return err == nil
}

// keychainLookup reads from the macOS Keychain using the security CLI.
func keychainLookup(ctx context.Context) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "security",
		"find-generic-password",
		"-a", u.Username,
		"-s", "afklm-api-key",
		"-w",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(out))
	if key == "" {
		return "", ErrNoCredential
	}
	return key, nil
}

// opLookup reads from 1Password via the op CLI.
func opLookup(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "op", "read",
		"op://Personal/Air France-KLM Developer API/credential",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(out))
	if key == "" {
		return "", ErrNoCredential
	}
	return key, nil
}
