package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchHotelsProviderError(t *testing.T) {
	// Mock server returning 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "error-test",
		Name:     "Error Test",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	_, err = rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err == nil {
		t.Fatal("expected error from provider returning 500")
	}

	// Verify error was marked.
	got := reg.Get("error-test")
	if got.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", got.ErrorCount)
	}
}

func TestSearchHotelsContextCanceled(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "ctx-test",
		Name:     "Ctx Test",
		Category: "hotel",
		Endpoint: "https://example.com/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 0.001, // very slow limiter to force context cancellation
			Burst:             1,
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)

	// Exhaust the burst token.
	pc := rt.getOrCreateClient(cfg)
	pc.limiter.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = rt.SearchHotels(ctx, "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

func TestSearchHotelsPostMethod(t *testing.T) {
	// Mock server that verifies POST body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}

		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		bodyStr := string(body[:n])

		if !containsSubstring(bodyStr, `"checkin":"2025-06-01"`) {
			t.Errorf("body does not contain checkin: %s", bodyStr)
		}

		resp := map[string]any{
			"results": []any{
				map[string]any{"name": "POST Hotel"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:           "post-test",
		Name:         "POST Test",
		Category:     "hotel",
		Endpoint:     srv.URL + "/search",
		Method:       "POST",
		BodyTemplate: `{"checkin":"${checkin}","checkout":"${checkout}"}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name": "name",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}
	if hotels[0].Name != "POST Hotel" {
		t.Errorf("name = %q, want 'POST Hotel'", hotels[0].Name)
	}
}

func TestSubstituteEnvVars(t *testing.T) {
	t.Run("basic substitution", func(t *testing.T) {
		t.Setenv("TRVL_TEST_VAR", "hello-world")
		got := substituteEnvVars("key=${env.TRVL_TEST_VAR}")
		if got != "key=hello-world" {
			t.Errorf("got %q, want %q", got, "key=hello-world")
		}
	})

	t.Run("missing env var replaced with empty string", func(t *testing.T) {
		got := substituteEnvVars("key=${env.TRVL_NONEXISTENT_VAR_12345}")
		if got != "key=" {
			t.Errorf("got %q, want %q", got, "key=")
		}
	})

	t.Run("no env vars returns unchanged", func(t *testing.T) {
		input := "plain string without env references"
		got := substituteEnvVars(input)
		if got != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("multiple env vars in one string", func(t *testing.T) {
		t.Setenv("TRVL_TEST_A", "alpha")
		t.Setenv("TRVL_TEST_B", "beta")
		got := substituteEnvVars("${env.TRVL_TEST_A}-and-${env.TRVL_TEST_B}")
		if got != "alpha-and-beta" {
			t.Errorf("got %q, want %q", got, "alpha-and-beta")
		}
	})

	t.Run("malformed pattern without closing brace", func(t *testing.T) {
		// ${env. without closing } should stop iteration and return what it has.
		input := "prefix${env.FOO_BAR"
		got := substituteEnvVars(input)
		// The function breaks out of the loop when it cannot find closing }.
		if got != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("empty var name", func(t *testing.T) {
		// ${env.} has an empty variable name -- os.Getenv("") returns "".
		got := substituteEnvVars("val=${env.}")
		if got != "val=" {
			t.Errorf("got %q, want %q", got, "val=")
		}
	})
}

func TestRunPreflight_POST(t *testing.T) {
	// Mock OAuth2-style token endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("preflight method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}

		// Read and verify body.
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		bodyStr := string(body[:n])
		if bodyStr != "api_key=test-key" {
			t.Errorf("preflight body = %q, want %q", bodyStr, "api_key=test-key")
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token-123","expires_in":"3600"}`)
	}))
	defer srv.Close()

	// Mock search server that verifies auth token was extracted and used.
	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token-123" {
			t.Errorf("Authorization = %q, want 'Bearer test-token-123'", authHeader)
		}
		resp := map[string]any{
			"results": []any{
				map[string]any{"name": "Token Hotel", "id": "th1"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer searchSrv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "post-preflight-test",
		Name:     "POST Preflight Test",
		Category: "hotel",
		Endpoint: searchSrv.URL + "/search",
		Method:   "GET",
		Headers: map[string]string{
			"Authorization": "Bearer ${auth_token}",
		},
		Auth: &AuthConfig{
			Type:            "preflight",
			PreflightURL:    srv.URL + "/auth",
			PreflightMethod: "POST",
			PreflightBody:   "api_key=test-key",
			PreflightHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"Accept":       "application/json",
			},
			Extractions: map[string]Extraction{
				"auth_token": {
					Pattern:  `"access_token":"([^"]+)"`,
					Variable: "auth_token",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name":     "name",
				"hotel_id": "id",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}

	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}
	if hotels[0].Name != "Token Hotel" {
		t.Errorf("name = %q, want 'Token Hotel'", hotels[0].Name)
	}
}

