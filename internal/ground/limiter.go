package ground

import (
	"time"

	"golang.org/x/time/rate"
)

// newProviderLimiter creates a rate limiter that allows one request per interval
// with a burst of 1. Every provider in the ground package uses the same shape
// (burst = 1), only the interval differs.
func newProviderLimiter(interval time.Duration) *rate.Limiter {
	return rate.NewLimiter(rate.Every(interval), 1)
}
