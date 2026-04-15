package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // register all browser cookie finders
)

// providerInteractiveKey is the context key that marks a call as interactive —
// i.e. originating from a human session where it is acceptable to launch the
// user's browser to clear a WAF/JS challenge. Non-interactive callers (unit
// tests, background jobs, MCP tool calls from headless pipelines) must leave
// this unset so the Tier 4 escape hatch never fires.
type providerInteractiveKey struct{}

// providerElicitKey is the context key for the elicitation callback.
type providerElicitKey struct{}

// ElicitConfirmFunc prompts the user with a message and returns true if they
// confirmed. This abstraction decouples the provider runtime from the MCP
// protocol layer — MCP handlers wrap their ElicitFunc into this signature.
type ElicitConfirmFunc func(message string) (confirmed bool, err error)

// WithInteractive returns a derived context marked as an interactive session.
// CLI entrypoints and MCP handlers that run with a human in the loop should
// call this so that the provider runtime may, if absolutely needed, open the
// user's browser to solve a JS bot-detection challenge.
func WithInteractive(ctx context.Context) context.Context {
	return context.WithValue(ctx, providerInteractiveKey{}, true)
}

// WithElicit returns a derived context carrying an elicitation callback.
// When the provider runtime needs user confirmation (e.g. "please visit
// booking.com to clear a WAF challenge"), it calls this function instead
// of silently opening a browser and timing out.
func WithElicit(ctx context.Context, fn ElicitConfirmFunc) context.Context {
	return context.WithValue(ctx, providerElicitKey{}, fn)
}

// getElicit returns the elicitation callback from ctx, or nil if none is set.
func getElicit(ctx context.Context) ElicitConfirmFunc {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(providerElicitKey{}).(ElicitConfirmFunc)
	return fn
}

// isInteractive reports whether ctx was marked interactive by WithInteractive.
func isInteractive(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(providerInteractiveKey{}).(bool)
	return v
}

// openerFunc is the injectable hook used by openURLInBrowser so tests can
// exercise the OS-dispatch logic without actually launching a browser. The
// default implementation shells out via exec.Command; tests override it to
// record calls and return canned errors.
type openerFunc func(goos, browserPreference, targetURL string) error

// defaultOpenURL is the production openerFunc — it dispatches to the
// platform-native "open this URL" command.
func defaultOpenURL(goos, browserPreference, targetURL string) error {
	switch goos {
	case "darwin":
		if browserPreference != "" {
			if err := exec.Command("open", "-a", browserPreference, targetURL).Run(); err == nil {
				return nil
			}
		}
		if err := exec.Command("open", targetURL).Run(); err != nil {
			return fmt.Errorf("open: %w", err)
		}
		return nil
	case "linux":
		if err := exec.Command("xdg-open", targetURL).Run(); err != nil {
			return fmt.Errorf("xdg-open: %w", err)
		}
		return nil
	case "windows":
		if err := exec.Command("cmd", "/c", "start", "", targetURL).Run(); err != nil {
			return fmt.Errorf("start: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("openURLInBrowser: unsupported OS %q", goos)
	}
}

// currentOpenURL is the active openerFunc. Tests may swap it out.
var currentOpenURL openerFunc = defaultOpenURL

// OpenURLInBrowser launches the user's browser pointed at targetURL so they
// can clear any WAF/JS challenge inline. On macOS it honours
// browserPreference (defaults to "Google Chrome"); on Linux and Windows the
// preference is ignored and the OS default is used.
//
// Callers MUST gate this on an explicit opt-in because it produces a visible
// side effect for the end user.
func OpenURLInBrowser(targetURL, browserPreference string) error {
	return openURLInBrowser(targetURL, browserPreference)
}

// openURLInBrowser is the internal implementation of OpenURLInBrowser.
func openURLInBrowser(targetURL, browserPreference string) error {
	if strings.TrimSpace(targetURL) == "" {
		return fmt.Errorf("openURLInBrowser: empty URL")
	}
	goos := runtime.GOOS
	if goos == "darwin" && strings.TrimSpace(browserPreference) == "" {
		browserPreference = "Google Chrome"
	}
	return currentOpenURL(goos, browserPreference, targetURL)
}

// cookieSourceFunc is the injectable hook used by waitForFreshCookies to
// re-read browser cookies on each tick. Production code points it at
// browserCookiesForURL; tests swap it out for an in-memory sequence.
type cookieSourceFunc func(targetURL string) []*http.Cookie

// currentCookieSource is the active cookieSourceFunc. Tests may swap it out.
var currentCookieSource cookieSourceFunc = browserCookiesForURL

// waitForFreshCookies polls the user's browser cookie stores for the given
// URL and returns as soon as the cookie set differs (by name+value) from
// prevSnapshot. Returns (newCookies, true) on a detected change, or
// (prevSnapshot, false) if maxWait elapses or ctx is cancelled.
//
// Intended as the wait step of the Tier 4 escape hatch: the caller has just
// launched the user's browser to solve a WAF challenge; this function blocks
// until the browser's cookie jar visibly updates (meaning the challenge is
// solved) or the deadline passes.
func waitForFreshCookies(ctx context.Context, targetURL string, prevSnapshot []*http.Cookie, pollInterval, maxWait time.Duration) ([]*http.Cookie, bool) {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	if maxWait <= 0 {
		maxWait = 15 * time.Second
	}
	deadline := time.Now().Add(maxWait)
	prevKey := cookieSnapshotKey(prevSnapshot)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return prevSnapshot, false
		case <-ticker.C:
			fresh := currentCookieSource(targetURL)
			if cookieSnapshotKey(fresh) != prevKey {
				return fresh, true
			}
			if time.Now().After(deadline) {
				return prevSnapshot, false
			}
		}
	}
}

// cookieSnapshotKey builds a deterministic, order-independent fingerprint of
// a cookie slice so waitForFreshCookies can detect set-level changes. Only
// name+value pairs are considered meaningful — domain and path are excluded
// because the same logical session cookie can be written under slightly
// different scopes across reads.
func cookieSnapshotKey(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	sort.Strings(parts)
	// Delimiter 0x1f (unit separator) is not valid in cookie name or value
	// per RFC 6265, so collisions between "ab|cd" + "" and "ab" + "cd" are
	// impossible.
	return strings.Join(parts, "\x1f")
}

// browserCookieLookupTimeout bounds how long we spend reading cookies from
// browser stores. Local SQLite reads are fast but Keychain prompts on macOS
// can block indefinitely; a short deadline lets us fail fast.
const browserCookieLookupTimeout = 5 * time.Second

// browserCookiesForURL reads cookies from the user's browsers matching the
// given URL's domain. Iterates all registered browser cookie stores and
// returns every cookie whose domain matches the URL host (or is a parent
// domain of it). Returns nil if the URL cannot be parsed, no cookies are
// found, or cookie access fails (e.g. user denied Keychain access on macOS).
//
// This is used as a fallback when standard HTTP preflight gets blocked by
// JavaScript bot-detection challenges (HTTP 202/403). The user's actual
// browser has already solved any JS challenges and has valid session
// cookies, which we can read directly from their disk-backed cookie jars.
func browserCookiesForURL(targetURL string) []*http.Cookie {
	u, err := url.Parse(targetURL)
	if err != nil || u.Host == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), browserCookieLookupTimeout)
	defer cancel()

	host := u.Hostname()
	cookies, err := kooky.ReadCookies(ctx, kooky.Valid, kooky.DomainHasSuffix(registrableSuffix(host)))
	if err != nil && len(cookies) == 0 {
		return nil
	}

	result := make([]*http.Cookie, 0, len(cookies))
	seen := make(map[string]struct{}, len(cookies))
	for _, c := range cookies {
		if c == nil {
			continue
		}
		if !cookieDomainMatchesHost(c.Cookie.Domain, host) {
			continue
		}
		key := c.Cookie.Name + "\x00" + c.Cookie.Domain + "\x00" + c.Cookie.Path
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		cp := c.Cookie // copy
		result = append(result, &cp)
	}
	return result
}

// registrableSuffix returns a suffix of host suitable for a DomainHasSuffix
// filter. For e.g. "www.booking.com" it returns "booking.com"; for short
// hosts it returns the original host. This is a heuristic — we filter
// precisely afterwards in cookieDomainMatchesHost.
func registrableSuffix(host string) string {
	host = strings.TrimPrefix(host, ".")
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// cookieDomainMatchesHost reports whether a cookie's Domain attribute applies
// to the given request host per RFC 6265: the cookie domain must equal the
// host or be a dot-prefixed parent domain.
func cookieDomainMatchesHost(cookieDomain, host string) bool {
	if cookieDomain == "" || host == "" {
		return false
	}
	cd := strings.ToLower(strings.TrimPrefix(cookieDomain, "."))
	h := strings.ToLower(host)
	if cd == h {
		return true
	}
	return strings.HasSuffix(h, "."+cd)
}
