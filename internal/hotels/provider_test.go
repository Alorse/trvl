package hotels

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// Compile-time assertion: DefaultProvider implements models.HotelSearcher.
var _ models.HotelSearcher = (*DefaultProvider)(nil)

func TestDefaultProviderImplementsHotelSearcher(t *testing.T) {
	var p models.HotelSearcher = &DefaultProvider{}
	if p == nil {
		t.Fatal("DefaultProvider should not be nil")
	}
}
