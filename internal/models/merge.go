package models

import (
	"math"
	"strings"
)

// MergeHotelResults deduplicates hotels from multiple sources. When the same
// hotel appears from different providers, sources are merged into a single
// HotelResult with the lowest price as the primary and all provider prices
// preserved in Sources.
//
// Matching uses case-insensitive name normalization. When names match but
// could be ambiguous (e.g. "Hilton" in different cities), geo-proximity
// within maxDistanceMeters is used as a tiebreaker.
func MergeHotelResults(sources ...[]HotelResult) []HotelResult {
	const maxDistanceMeters = 200.0

	type key struct {
		name string
	}

	merged := make(map[key]*HotelResult)
	var order []key // preserve insertion order

	for _, batch := range sources {
		for _, h := range batch {
			k := key{name: normalizeName(h.Name)}

			if existing, ok := merged[k]; ok {
				// Check geo-proximity if both have coordinates.
				if existing.Lat != 0 && h.Lat != 0 {
					dist := haversineMeters(existing.Lat, existing.Lon, h.Lat, h.Lon)
					if dist > maxDistanceMeters {
						// Same name but different location — treat as separate.
						// Use a disambiguated key.
						dk := key{name: normalizeName(h.Name) + "|" + h.Address}
						if _, exists := merged[dk]; !exists {
							clone := h
							clone.Sources = buildSources(clone)
							merged[dk] = &clone
							order = append(order, dk)
						}
						continue
					}
				}

				// Merge: add this provider's price as a source.
				for _, src := range buildSources(h) {
					existing.Sources = append(existing.Sources, src)
				}

				// Update primary price to the lowest.
				if h.Price > 0 && (existing.Price == 0 || h.Price < existing.Price) {
					existing.Price = h.Price
					existing.Currency = h.Currency
					existing.BookingURL = h.BookingURL
				}

				// Merge fields that the primary might be missing.
				if existing.Rating == 0 && h.Rating > 0 {
					existing.Rating = h.Rating
				}
				if existing.ReviewCount == 0 && h.ReviewCount > 0 {
					existing.ReviewCount = h.ReviewCount
				}
				if existing.Stars == 0 && h.Stars > 0 {
					existing.Stars = h.Stars
				}
				if existing.Address == "" && h.Address != "" {
					existing.Address = h.Address
				}
				if existing.Lat == 0 && h.Lat != 0 {
					existing.Lat = h.Lat
					existing.Lon = h.Lon
				}
			} else {
				clone := h
				clone.Sources = buildSources(clone)
				merged[k] = &clone
				order = append(order, k)
			}
		}
	}

	result := make([]HotelResult, 0, len(order))
	for _, k := range order {
		result = append(result, *merged[k])
	}
	return result
}

// buildSources creates a Sources slice from a HotelResult's own price.
func buildSources(h HotelResult) []PriceSource {
	if h.Price == 0 {
		return nil
	}
	provider := "unknown"
	for _, s := range h.Sources {
		provider = s.Provider
		break
	}
	if len(h.Sources) > 0 {
		return h.Sources
	}
	return []PriceSource{{
		Provider:   provider,
		Price:      h.Price,
		Currency:   h.Currency,
		BookingURL: h.BookingURL,
	}}
}

// normalizeName lowercases, trims whitespace, and collapses internal spaces.
func normalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	// Collapse multiple spaces.
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}
	return name
}

// haversineMeters returns the distance in meters between two lat/lon points.
func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMeters = 6_371_000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}
