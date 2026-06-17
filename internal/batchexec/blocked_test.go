package batchexec

import "testing"

func TestIsBlockedFlightResponse(t *testing.T) {
	// Valid envelope: outer[0][2] is a JSON string that parses to a []any.
	// Inner array "[1,2,[],[]]" → decodes to []any → NOT blocked.
	valid := []byte(`)]}'` + "\n" + `[["wrb.fr","GetShoppingResults","[1,2,[],[]]"]]`)
	if IsBlockedFlightResponse(valid) {
		t.Errorf("valid flight envelope reported as blocked")
	}

	// Anti-bot: outer[0][2] is an object, not a flight-array JSON string.
	blocked := []byte(`)]}'` + "\n" + `[["wrb.fr","GetShoppingResults",null,null,null,null,"generic"],["di",42],["af.httprm",42,"-1",0]]`)
	if !IsBlockedFlightResponse(blocked) {
		t.Errorf("anti-bot response not reported as blocked")
	}

	// Empty / garbage body → blocked (retryable).
	if !IsBlockedFlightResponse([]byte("")) {
		t.Errorf("empty body should be reported as blocked")
	}
	if !IsBlockedFlightResponse([]byte(")]}'\nnot json")) {
		t.Errorf("garbage body should be reported as blocked")
	}
}
