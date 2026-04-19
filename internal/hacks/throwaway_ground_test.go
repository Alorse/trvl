package hacks

import (
	"context"
	"testing"
)

func TestDetectThrowawayGround_emptyInput(t *testing.T) {
	hacks := detectThrowawayGround(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected nil for empty input, got %d hacks", len(hacks))
	}
}

func TestDetectThrowawayGround_missingOrigin(t *testing.T) {
	hacks := detectThrowawayGround(context.Background(), DetectorInput{
		Destination: "MUC",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing origin, got %d hacks", len(hacks))
	}
}

func TestDetectThrowawayGround_missingDestination(t *testing.T) {
	hacks := detectThrowawayGround(context.Background(), DetectorInput{
		Origin: "PRG",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing destination, got %d hacks", len(hacks))
	}
}

func TestDetectThrowawayGround_validRoute(t *testing.T) {
	hacks := detectThrowawayGround(context.Background(), DetectorInput{
		Origin:      "PRG",
		Destination: "MUC",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "throwaway_ground" {
		t.Errorf("type = %q, want throwaway_ground", h.Type)
	}
	if h.Title == "" {
		t.Error("title is empty")
	}
	if h.Description == "" {
		t.Error("description is empty")
	}
	if len(h.Steps) == 0 {
		t.Error("steps are empty")
	}
	if len(h.Risks) == 0 {
		t.Error("risks are empty")
	}
}

func TestDetectThrowawayGround_containsDestination(t *testing.T) {
	hacks := detectThrowawayGround(context.Background(), DetectorInput{
		Origin:      "PRG",
		Destination: "VIE",
	})
	if len(hacks) == 0 {
		t.Fatal("expected at least 1 hack")
	}
	found := false
	for _, s := range hacks[0].Steps {
		if containsSubstring(s, "VIE") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected destination VIE in steps")
	}
}
