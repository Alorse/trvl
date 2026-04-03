package flights

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// Compile-time assertion: DefaultProvider implements models.FlightSearcher.
var _ models.FlightSearcher = (*DefaultProvider)(nil)

func TestDefaultProviderImplementsFlightSearcher(t *testing.T) {
	var p models.FlightSearcher = &DefaultProvider{}
	if p == nil {
		t.Fatal("DefaultProvider should not be nil")
	}
}
