package batchexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPostFormValidated_RetriesBlockedBody(t *testing.T) {
	var calls atomic.Int32
	blocked := `)]}'` + "\n" + `[["wrb.fr","GetShoppingResults",null,null,null,null,"generic"]]`
	good := `)]}'` + "\n" + `[["wrb.fr","GetShoppingResults","[1,2,[],[]]"]]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if calls.Add(1) <= 2 {
			_, _ = w.Write([]byte(blocked))
			return
		}
		_, _ = w.Write([]byte(good))
	}))
	defer srv.Close()

	c := NewClient()
	c.SetBaseBackoffForTest(0) // see Step 3 note: avoid slow test

	status, body, err := c.PostFormValidated(context.Background(), srv.URL, "f.req=x", IsBlockedFlightResponse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if IsBlockedFlightResponse(body) {
		t.Fatalf("final body still blocked; retries did not reach the good response")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("server calls = %d, want 3 (2 blocked + 1 good)", got)
	}
}
