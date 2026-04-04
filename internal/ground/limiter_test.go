package ground

import (
	"math"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func assertLimiterConfiguration(t *testing.T, limiter *rate.Limiter, wantEvery time.Duration, wantBurst int) {
	t.Helper()

	wantLimit := rate.Every(wantEvery)
	gotLimit := limiter.Limit()
	if math.Abs(float64(gotLimit-wantLimit)) > 1e-9 {
		t.Fatalf("limiter limit = %v, want %v", gotLimit, wantLimit)
	}
	if gotBurst := limiter.Burst(); gotBurst != wantBurst {
		t.Fatalf("limiter burst = %d, want %d", gotBurst, wantBurst)
	}

	probe := rate.NewLimiter(gotLimit, wantBurst)
	now := time.Unix(0, 0)

	first := probe.ReserveN(now, 1)
	if !first.OK() {
		t.Fatal("fresh limiter should allow first reservation")
	}
	if delay := first.DelayFrom(now); delay != 0 {
		t.Fatalf("fresh limiter first reservation delay = %v, want 0", delay)
	}

	second := probe.ReserveN(now, 1)
	if !second.OK() {
		t.Fatal("fresh limiter should allow second reservation")
	}

	delay := second.DelayFrom(now)
	tolerance := max(wantEvery/20, 10*time.Millisecond)
	if diff := delay - wantEvery; diff < -tolerance || diff > tolerance {
		t.Fatalf("fresh limiter second reservation delay = %v, want %v (+/-%v)", delay, wantEvery, tolerance)
	}
}
