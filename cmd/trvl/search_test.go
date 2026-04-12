package main

import (
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/nlsearch"
)

func TestSearchCmd_NonNil(t *testing.T) {
	cmd := searchCmd()
	if cmd == nil {
		t.Fatal("searchCmd() returned nil")
	}
}

func TestSearchCmd_Use(t *testing.T) {
	cmd := searchCmd()
	if !strings.HasPrefix(cmd.Use, "search") {
		t.Errorf("Use = %q, want to start with 'search'", cmd.Use)
	}
}

func TestSearchCmd_Flags(t *testing.T) {
	cmd := searchCmd()
	for _, name := range []string{"dry-run", "json"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag --%s", name)
		}
	}
}

func TestSearchCmd_RequiresArg(t *testing.T) {
	cmd := searchCmd()
	if err := cmd.Args(cmd, nil); err == nil {
		t.Error("expected error with no args")
	}
}

func TestPrintSearchInterpretation_Smoke(t *testing.T) {
	// Just verify it doesn't panic with a populated params struct.
	p := nlsearch.Params{
		Intent:      "flight",
		Origin:      "HEL",
		Destination: "NRT",
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
	}
	printSearchInterpretation("flight HEL NRT 2026-06-15 2026-06-22", p)
}

func TestMissingFieldsHint_Flight(t *testing.T) {
	p := nlsearch.Params{Intent: "flight", Origin: "HEL"}
	err := missingFieldsHint(p, "flight", "trvl flights HEL DEST DATE")
	if err != nil {
		t.Errorf("missingFieldsHint should return nil, got %v", err)
	}
}

func TestMissingFieldsHint_Hotel(t *testing.T) {
	p := nlsearch.Params{Intent: "hotel", Location: "BCN"}
	err := missingFieldsHint(p, "hotel", `trvl hotels "BCN" ...`)
	if err != nil {
		t.Errorf("missingFieldsHint should return nil, got %v", err)
	}
}
