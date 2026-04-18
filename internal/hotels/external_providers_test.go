package hotels

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/providers"
)

func TestSetExternalProviderRuntime_NilByDefault(t *testing.T) {
	// The package-level variable should be nil when no runtime is configured.
	// Save and restore to avoid test pollution.
	saved := getExternalProviderRuntime()
	defer SetExternalProviderRuntime(saved)

	SetExternalProviderRuntime(nil)
	if getExternalProviderRuntime() != nil {
		t.Fatal("expected nil default")
	}
}

func TestSetExternalProviderRuntime_SetsRuntime(t *testing.T) {
	saved := getExternalProviderRuntime()
	defer SetExternalProviderRuntime(saved)

	rt := providers.NewRuntime(&providers.Registry{})
	SetExternalProviderRuntime(rt)
	if getExternalProviderRuntime() != rt {
		t.Fatal("expected runtime to be set")
	}
}

func TestSetExternalProviderRuntime_ClearsRuntime(t *testing.T) {
	saved := getExternalProviderRuntime()
	defer SetExternalProviderRuntime(saved)

	rt := providers.NewRuntime(&providers.Registry{})
	SetExternalProviderRuntime(rt)
	SetExternalProviderRuntime(nil)
	if getExternalProviderRuntime() != nil {
		t.Fatal("expected runtime to be cleared")
	}
}
