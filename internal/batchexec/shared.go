package batchexec

import "sync"

var (
	sharedClient *Client
	sharedOnce   sync.Once
)

// SharedClient returns a process-wide shared Client instance.
// The client is created once and reused across all callers, enabling
// connection reuse and shared rate limiting.
func SharedClient() *Client {
	sharedOnce.Do(func() { sharedClient = NewClient() })
	return sharedClient
}
