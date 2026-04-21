// Package afklm implements the Air France-KLM Offers API v3 as an optional
// flight provider for the trvl CLI.
//
// The provider is opt-in: users must obtain a free personal API key at
// https://developer.airfranceklm.com and store it via one of the supported
// credential backends (see auth.go). When no credential is found the provider
// returns ErrNoCredential and the CLI prints instructions for sign-up.
//
// Personal provider convention: this package is marked personal:true in
// ~/.trvl/providers/afklm.json. Registry.ListPublic() skips personal providers
// so they are never included in exports or shared configs. List() (all
// providers) still includes them for the owner's own use.
//
// Rate limits: the AF-KLM free tier allows 100 requests/day and 1 request/sec.
// The client enforces these limits defensively: a per-day file-based quota
// counter hard-refuses calls at >=95 (reserving 5 for emergency), and a
// token-bucket rate limiter enforces 1 QPS. Responses are cached on disk with
// tiered TTL by proximity to departure.
package afklm
