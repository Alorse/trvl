package ground

import "net/http"

// SetHTTPClient replaces the package-level httpClient used by FlixBus,
// RegioJet, and other providers that go through rateLimitedDo. Tests use
// this to inject an httptest.Server-backed client.
func SetHTTPClient(c *http.Client) { httpClient = c }

// SetEckeroLineClient replaces the Eckerö Line HTTP client (for testing).
func SetEckeroLineClient(c *http.Client) { eckerolineClient = c }
