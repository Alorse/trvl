package flights

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// serpKeyTimeout bounds how long we wait for the external serp-key router.
const serpKeyTimeout = 5 * time.Second

// serpKeyCmd is the external rotating-key command. Defaults to "serp-key" on
// PATH; overridable via TRVL_SERP_KEY_CMD (also the test seam for a fake script).
func serpKeyCmd() string {
	if v := strings.TrimSpace(os.Getenv("TRVL_SERP_KEY_CMD")); v != "" {
		return v
	}
	return "serp-key"
}

// serpKeyFunc fetches one rotated SerpApi key. Indirected for tests.
var serpKeyFunc = execSerpKey

// execSerpKey runs the serp-key command and returns the single key on stdout.
// Contract: stdout = raw key, exit 0 on success, exit 1 if none available.
func execSerpKey(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, serpKeyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, serpKeyCmd())
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("serp-key: %w (%s)", err, strings.TrimSpace(errb.String()))
	}
	key := strings.TrimSpace(out.String())
	if key == "" {
		return "", fmt.Errorf("serp-key: empty key")
	}
	return key, nil
}

// SerpEnabled reports whether the serp-key router command is available, gating
// SerpApi in the fallback chain. When false, SerpApi is silently skipped.
func SerpEnabled() bool {
	_, err := exec.LookPath(serpKeyCmd())
	return err == nil
}
