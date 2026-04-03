package ground

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// Compile-time assertion: DefaultProvider implements models.GroundSearcher.
var _ models.GroundSearcher = (*DefaultProvider)(nil)

func TestDefaultProviderImplementsGroundSearcher(t *testing.T) {
	var p models.GroundSearcher = &DefaultProvider{}
	if p == nil {
		t.Fatal("DefaultProvider should not be nil")
	}
}
