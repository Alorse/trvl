package flights

import (
	"os"
	"strings"
	"sync/atomic"
)

// duffelKeyCounter drives round-robin key selection across searches.
var duffelKeyCounter atomic.Uint64

// duffelKeys returns the configured Duffel API keys, read at runtime so they are
// never compiled into the binary. DUFFEL_API_KEYS (comma/whitespace-separated)
// takes precedence; otherwise a single DUFFEL_API_KEY is used.
func duffelKeys() []string {
	if v := strings.TrimSpace(os.Getenv("DUFFEL_API_KEYS")); v != "" {
		fields := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\t'
		})
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			if f = strings.TrimSpace(f); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	if v := strings.TrimSpace(os.Getenv("DUFFEL_API_KEY")); v != "" {
		return []string{v}
	}
	return nil
}

// duffelKeyOrder reorders keys round-robin so each search starts from the next
// key (load spread). Callers then fail over down the returned slice in order.
func duffelKeyOrder(keys []string) []string {
	n := len(keys)
	if n <= 1 {
		return keys
	}
	start := int((duffelKeyCounter.Add(1) - 1) % uint64(n))
	ordered := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ordered = append(ordered, keys[(start+i)%n])
	}
	return ordered
}
