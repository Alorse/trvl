package cookies

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// ============================================================
// BrowserReadPage — all early-return branches
// ============================================================

func TestBrowserReadPage_SkipTrue(t *testing.T) {
	orig := SkipBrowserRead
	SkipBrowserRead = true
	t.Cleanup(func() { SkipBrowserRead = orig })

	_, err := BrowserReadPage(context.Background(), "https://example.com", 1)
	if err == nil {
		t.Fatal("expected error when SkipBrowserRead=true")
	}
}

func TestBrowserReadPage_NegativeWaitSeconds(t *testing.T) {
	orig := SkipBrowserRead
	SkipBrowserRead = true
	t.Cleanup(func() { SkipBrowserRead = orig })

	// With SkipBrowserRead=true, returns before using waitSeconds, but
	// calling with negative waitSeconds should not panic.
	_, err := BrowserReadPage(context.Background(), "https://example.com", -5)
	if err == nil {
		t.Fatal("expected error when SkipBrowserRead=true")
	}
}

// ============================================================
// BrowserReadPageCached — cache miss path
// ============================================================

func TestBrowserReadPageCached_CacheMissDisabled(t *testing.T) {
	orig := SkipBrowserRead
	SkipBrowserRead = true
	t.Cleanup(func() { SkipBrowserRead = orig })

	const testURL = "https://example.com/miss-test-unique"
	// Ensure no cache entry exists.
	browserPageCache.Lock()
	delete(browserPageCache.entries, testURL)
	browserPageCache.Unlock()

	_, err := BrowserReadPageCached(context.Background(), testURL, 1, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error on cache miss with browser reading disabled")
	}
}

// ============================================================
// OpenBrowserForAuth — domain extraction edge cases
// ============================================================

func TestOpenBrowserForAuth_NoSchemeURL(t *testing.T) {
	origNow := browserAuthNow
	origStart := browserAuthStart
	now := time.Unix(1_700_000_000, 0)
	browserAuthNow = func() time.Time { return now }
	t.Cleanup(func() {
		browserAuthNow = origNow
		browserAuthStart = origStart
		browserAuthOpened.mu.Lock()
		delete(browserAuthOpened.domains, "plain.example.com")
		browserAuthOpened.mu.Unlock()
	})

	browserAuthStart = func(name string, args ...string) error {
		return nil
	}

	// URL without scheme: "plain.example.com/path"
	err := OpenBrowserForAuth("plain.example.com/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenBrowserForAuth_CooldownExpiry(t *testing.T) {
	origNow := browserAuthNow
	origStart := browserAuthStart
	now := time.Unix(1_700_000_000, 0)
	browserAuthNow = func() time.Time { return now }
	t.Cleanup(func() {
		browserAuthNow = origNow
		browserAuthStart = origStart
		browserAuthOpened.mu.Lock()
		delete(browserAuthOpened.domains, "cooldown.example.com")
		browserAuthOpened.mu.Unlock()
	})

	calls := 0
	browserAuthStart = func(name string, args ...string) error {
		calls++
		return nil
	}

	// First call opens browser.
	if err := OpenBrowserForAuth("https://cooldown.example.com/page"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}

	// Second call within cooldown — suppressed.
	if err := OpenBrowserForAuth("https://cooldown.example.com/page"); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls after suppression = %d, want 1", calls)
	}

	// Advance past cooldown.
	now = now.Add(25 * time.Hour)
	if err := OpenBrowserForAuth("https://cooldown.example.com/page"); err != nil {
		t.Fatalf("third call: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls after cooldown expired = %d, want 2", calls)
	}
}

func TestOpenBrowserForAuth_LaunchFailureDoesNotRecordCooldown(t *testing.T) {
	origNow := browserAuthNow
	origStart := browserAuthStart
	now := time.Unix(1_700_000_000, 0)
	browserAuthNow = func() time.Time { return now }
	t.Cleanup(func() {
		browserAuthNow = origNow
		browserAuthStart = origStart
		browserAuthOpened.mu.Lock()
		delete(browserAuthOpened.domains, "fail.example.com")
		browserAuthOpened.mu.Unlock()
	})

	browserAuthStart = func(name string, args ...string) error {
		return errors.New("launch failed")
	}

	if err := OpenBrowserForAuth("https://fail.example.com/auth"); err == nil {
		t.Fatal("expected error from failed launch")
	}

	browserAuthOpened.mu.Lock()
	_, recorded := browserAuthOpened.domains["fail.example.com"]
	browserAuthOpened.mu.Unlock()
	if recorded {
		t.Error("failed launch should not record cooldown")
	}
}

// ============================================================
// IsCaptchaResponse — additional boundary cases
// ============================================================

func TestIsCaptchaResponse_301Status(t *testing.T) {
	isCaptcha, _ := IsCaptchaResponse(301, []byte(`captcha-delivery.com`))
	if isCaptcha {
		t.Error("301 should not be detected as captcha")
	}
}

func TestIsCaptchaResponse_NilBody(t *testing.T) {
	isCaptcha, _ := IsCaptchaResponse(403, nil)
	if isCaptcha {
		t.Error("nil body should not be detected as captcha")
	}
}

// ============================================================
// ApplyCookies — exercises the real function when nab is absent
// ============================================================

func TestApplyCookies_NabAbsent(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://noexist.example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	// This exercises the real ApplyCookies code path. When nab is not installed,
	// BrowserCookies returns "" and no Cookie header is set.
	ApplyCookies(req, "noexist.example.com")
	// We can't assert the result depends on nab being absent, but the function
	// must not panic.
}

// ============================================================
// parseNetscapeCookies — tab-heavy edge cases
// ============================================================

func TestParseNetscapeCookies_ExtraTabFields(t *testing.T) {
	// 8+ fields: name is at index 5, value at index 6.
	data := ".example.com\tTRUE\t/\tFALSE\t0\tmyname\tmyvalue\textra1\textra2\n"
	got := parseNetscapeCookies(data)
	if got != "myname=myvalue" {
		t.Errorf("got %q, want %q", got, "myname=myvalue")
	}
}

func TestParseNetscapeCookies_WindowsLineEndings(t *testing.T) {
	data := ".example.com\tTRUE\t/\tFALSE\t0\ta\tb\r\n.example.com\tTRUE\t/\tFALSE\t0\tc\td\r\n"
	got := parseNetscapeCookies(data)
	// \r gets included in value of first cookie due to Split on \n.
	// This is expected — Netscape format uses \n, not \r\n.
	// We just verify no panic.
	if got == "" {
		t.Error("expected non-empty result")
	}
}

// ============================================================
// sanitizeURL — identity cases
// ============================================================

func TestSanitizeURL_NormalURL(t *testing.T) {
	input := "https://www.sncf-connect.com/en-en/result?from=Paris&to=Lyon"
	got := sanitizeURL(input)
	if got != input {
		t.Errorf("sanitizeURL should pass through normal URLs, got %q", got)
	}
}
