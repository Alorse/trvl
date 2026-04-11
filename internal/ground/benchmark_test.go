package ground

import (
	"fmt"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// makeGroundRoutes creates n GroundRoute values. Every other route is a
// duplicate (same provider+time+price) to give deduplication real work.
func makeGroundRoutes(n int) []models.GroundRoute {
	routes := make([]models.GroundRoute, n)
	for i := 0; i < n; i++ {
		// Pair routes so even/odd share the same key — 50% duplicates.
		idx := i / 2
		routes[i] = models.GroundRoute{
			Provider: []string{"flixbus", "regiojet", "db", "oebb", "ns"}[idx%5],
			Type:     "bus",
			Price:    float64(10 + idx*3),
			Currency: "EUR",
			Duration: 120 + idx*5,
			Departure: models.GroundStop{
				City: "Helsinki",
				Time: fmt.Sprintf("2026-04-10T%02d:00:00", 6+idx%12),
			},
			Arrival: models.GroundStop{
				City: "Tallinn",
				Time: fmt.Sprintf("2026-04-10T%02d:30:00", 8+idx%12),
			},
		}
	}
	return routes
}

// makeMixedGroundRoutes creates n routes: half with price > 0 (available),
// half with price = 0 and non-schedule-only provider (unavailable).
func makeMixedGroundRoutes(n int) []models.GroundRoute {
	routes := make([]models.GroundRoute, n)
	for i := 0; i < n; i++ {
		price := float64(10 + i*2)
		provider := "flixbus"
		if i%2 == 0 {
			// Unavailable: zero price + provider not in scheduleOnlyProviders.
			price = 0
			provider = "flixbus"
		}
		routes[i] = models.GroundRoute{
			Provider: provider,
			Type:     "bus",
			Price:    price,
			Currency: "EUR",
			Duration: 120,
			Departure: models.GroundStop{
				City: "Berlin",
				Time: fmt.Sprintf("2026-04-10T%02d:00:00", 6+i%12),
			},
			Arrival: models.GroundStop{
				City: "Prague",
				Time: fmt.Sprintf("2026-04-10T%02d:30:00", 10+i%12),
			},
		}
	}
	return routes
}

func BenchmarkDeduplicateGroundRoutes(b *testing.B) {
	routes := makeGroundRoutes(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := make([]models.GroundRoute, len(routes))
		copy(input, routes)
		_ = deduplicateGroundRoutes(input)
	}
}

func BenchmarkFilterUnavailableGroundRoutes(b *testing.B) {
	routes := makeMixedGroundRoutes(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := make([]models.GroundRoute, len(routes))
		copy(input, routes)
		_ = filterUnavailableGroundRoutes(input)
	}
}

func BenchmarkFilterGroundRoutes(b *testing.B) {
	routes := append(makeGroundRoutes(100), makeMixedGroundRoutes(100)...)
	opts := SearchOptions{
		MaxPrice: 100,
		Type:     "bus",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := make([]models.GroundRoute, len(routes))
		copy(input, routes)
		_ = filterGroundRoutes(input, opts)
	}
}
