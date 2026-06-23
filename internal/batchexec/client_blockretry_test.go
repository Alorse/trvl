package batchexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// alwaysBlocked treats every 200 body as a block (forces the block-retry path).
func alwaysBlocked([]byte) bool { return true }

func TestBlockRetryBudgetOne(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("blocked"))
	}))
	defer srv.Close()

	c := NewTestClient(srv.URL) // fast client, no real backoff
	c.SetBaseBackoffForTest(0)  // no sleeps
	status, body, err := c.PostFormValidatedN(context.Background(), "http://x/", "f.req=x", alwaysBlocked, 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if status != 200 || string(body) != "blocked" {
		t.Fatalf("expected blocked body returned, got status=%d body=%q", status, body)
	}
	if got := hits.Load(); got != 2 { // 1 initial + 1 retry
		t.Fatalf("expected 2 attempts with budget=1, got %d", got)
	}
}

func TestBlockRetryBudgetDefault(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("blocked"))
	}))
	defer srv.Close()

	c := NewTestClient(srv.URL)
	c.SetBaseBackoffForTest(0)
	_, _, err := c.PostFormValidatedN(context.Background(), "http://x/", "f.req=x", alwaysBlocked, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := hits.Load(); got != defaultMaxRetries+1 { // default = 3 retries → 4 attempts
		t.Fatalf("expected %d attempts with default budget, got %d", defaultMaxRetries+1, got)
	}
}
