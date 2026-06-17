package batchexec

// IsBlockedFlightResponse reports whether a flight response body is an anti-bot
// rejection rather than real flight data. Google returns its anti-abuse
// ErrorResponse with HTTP 200, so status alone cannot detect it.
//
// A real response (even one with zero results) decodes into a []any flight
// array via DecodeFlightResponse. A blocked response decodes into the error
// object (not a []any), or fails to decode entirely. Either case is treated as
// retryable. A valid empty-results array returns false (not blocked) so genuine
// "no flights" answers are not retried.
func IsBlockedFlightResponse(body []byte) bool {
	inner, err := DecodeFlightResponse(body)
	if err != nil {
		return true
	}
	_, ok := inner.([]any)
	return !ok
}
