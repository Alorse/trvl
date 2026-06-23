package flights

import (
	"context"
	"errors"
	"testing"
)

func TestSerpKeyFuncInjection(t *testing.T) {
	orig := serpKeyFunc
	defer func() { serpKeyFunc = orig }()

	serpKeyFunc = func(context.Context) (string, error) { return "KEY123", nil }
	got, err := serpKeyFunc(context.Background())
	if err != nil || got != "KEY123" {
		t.Fatalf("expected KEY123, got %q err=%v", got, err)
	}

	serpKeyFunc = func(context.Context) (string, error) { return "", errors.New("no keys") }
	if _, err := serpKeyFunc(context.Background()); err == nil {
		t.Fatal("expected error when serp-key fails")
	}
}

func TestSerpKeyCmdDefault(t *testing.T) {
	t.Setenv("TRVL_SERP_KEY_CMD", "")
	if got := serpKeyCmd(); got != "serp-key" {
		t.Fatalf("expected default serp-key, got %q", got)
	}
	t.Setenv("TRVL_SERP_KEY_CMD", "/custom/serp-key")
	if got := serpKeyCmd(); got != "/custom/serp-key" {
		t.Fatalf("expected override, got %q", got)
	}
}
