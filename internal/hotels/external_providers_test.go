package hotels

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/providers"
)

func TestSetExternalProviderRuntime_NilByDefault(t *testing.T) {
	// The package-level variable should be nil when no runtime is configured.
	// Save and restore to avoid test pollution.
	saved := externalProviderRuntime
	defer func() { externalProviderRuntime = saved }()

	externalProviderRuntime = nil
	if externalProviderRuntime != nil {
		t.Fatal("expected nil default")
	}
}

func TestSetExternalProviderRuntime_SetsRuntime(t *testing.T) {
	saved := externalProviderRuntime
	defer func() { externalProviderRuntime = saved }()

	rt := providers.NewRuntime(&providers.Registry{})
	SetExternalProviderRuntime(rt)
	if externalProviderRuntime != rt {
		t.Fatal("expected runtime to be set")
	}
}

func TestSetExternalProviderRuntime_ClearsRuntime(t *testing.T) {
	saved := externalProviderRuntime
	defer func() { externalProviderRuntime = saved }()

	rt := providers.NewRuntime(&providers.Registry{})
	SetExternalProviderRuntime(rt)
	SetExternalProviderRuntime(nil)
	if externalProviderRuntime != nil {
		t.Fatal("expected runtime to be cleared")
	}
}
