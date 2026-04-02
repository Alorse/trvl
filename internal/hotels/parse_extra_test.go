package hotels

import (
	"encoding/json"
	"testing"
)

// --- extractOrganicHotels ---

func TestExtractOrganicHotels_ValidStructure(t *testing.T) {
	// Build the nested structure: data[0][0][0][1] = hotel list
	hotel1 := make([]any, 12)
	hotel1[0] = nil
	hotel1[1] = "Organic Hotel 1"
	hotel1[2] = []any{[]any{60.168, 24.941}}
	hotel1[3] = []any{"4-star hotel", 4.0}
	hotel1[9] = "/g/organic1"

	hotel2 := make([]any, 12)
	hotel2[0] = nil
	hotel2[1] = "Organic Hotel 2"
	hotel2[2] = []any{[]any{60.170, 24.943}}
	hotel2[9] = "/g/organic2"

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"397419284": []any{hotel1},
			},
		},
		[]any{
			nil,
			map[string]any{
				"397419284": []any{hotel2},
			},
		},
	}

	data := []any{[]any{[]any{[]any{nil, hotelList}}}}

	hotels := extractOrganicHotels(data, "EUR")
	if len(hotels) != 2 {
		t.Fatalf("expected 2 organic hotels, got %d", len(hotels))
	}
	if hotels[0].Name != "Organic Hotel 1" {
		t.Errorf("hotel[0].Name = %q", hotels[0].Name)
	}
	if hotels[1].Name != "Organic Hotel 2" {
		t.Errorf("hotel[1].Name = %q", hotels[1].Name)
	}
}

func TestExtractOrganicHotels_SkipsSponsored(t *testing.T) {
	// Entry with key "300000000" should be skipped by organic extraction.
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil, []any{}}, // sponsored
			},
		},
	}

	data := []any{[]any{[]any{[]any{nil, hotelList}}}}

	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0 organic hotels (only sponsored), got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_NilNavigation(t *testing.T) {
	// Navigation to data[0][0][0][1] fails.
	hotels := extractOrganicHotels(nil, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0 hotels for nil input, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_NotArrayAtPath(t *testing.T) {
	data := []any{[]any{[]any{[]any{nil, "not an array"}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_EntryNotArray(t *testing.T) {
	hotelList := []any{"not an array entry"}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_EntryTooShort(t *testing.T) {
	hotelList := []any{[]any{nil}} // only 1 element, need >= 2
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_NoMapVal(t *testing.T) {
	// entryArr[1] is not a map.
	hotelList := []any{[]any{nil, "not a map"}}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_HotelArrNotArray(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"12345": "not an array",
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_HotelEntryNotArray(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"12345": []any{"not a hotel array"},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_HotelEntryTooShort(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"12345": []any{[]any{nil, "short"}}, // < 3 elements
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractOrganicHotels_EmptyName(t *testing.T) {
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "" // empty name
	hotel[2] = []any{[]any{60.0, 24.0}}

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"12345": []any{hotel},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	// Empty name hotel should be excluded.
	if len(hotels) != 0 {
		t.Errorf("expected 0 for empty-name hotel, got %d", len(hotels))
	}
}

// --- extractSponsoredHotels ---

func TestExtractSponsoredHotels_ValidStructure(t *testing.T) {
	sponsored1 := make([]any, 7)
	sponsored1[0] = "Sponsored Hotel 1"
	sponsored1[2] = "EUR 199"
	sponsored1[4] = float64(300)
	sponsored1[5] = float64(4.2)

	sponsored2 := make([]any, 7)
	sponsored2[0] = "Sponsored Hotel 2"
	sponsored2[2] = "EUR 249"
	sponsored2[4] = float64(500)
	sponsored2[5] = float64(4.5)

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil, []any{sponsored1, sponsored2}},
			},
		},
	}

	data := []any{[]any{[]any{[]any{nil, hotelList}}}}

	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 2 {
		t.Fatalf("expected 2 sponsored hotels, got %d", len(hotels))
	}
	if hotels[0].Name != "Sponsored Hotel 1" {
		t.Errorf("hotel[0].Name = %q", hotels[0].Name)
	}
}

func TestExtractSponsoredHotels_Nil(t *testing.T) {
	hotels := extractSponsoredHotels(nil, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_NoSponsoredKey(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"12345": []any{nil},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_SponsoredNotArray(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": "not an array",
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_SponsoredTooShort(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil}, // only 2 elements, need >= 3
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_HotelEntriesNotArray(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil, "not an array"},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_EntryTooShort(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil, []any{
					[]any{"name", nil, nil}, // < 6 elements
				}},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0 for too-short entry, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_EmptyName(t *testing.T) {
	entry := make([]any, 7)
	entry[0] = "" // empty name
	entry[2] = "EUR 100"

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil, []any{entry}},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0 for empty-name sponsored hotel, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_NotMapVal(t *testing.T) {
	hotelList := []any{
		[]any{nil, "not a map"},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

// --- parseHotelsFromPage with callbacks ---

func TestParseHotelsFromPage_WithOrganicHotels(t *testing.T) {
	// Build a page with an AF_initDataCallback containing hotel data.
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Page Hotel"
	hotel[2] = []any{[]any{60.168, 24.941}}
	hotel[3] = []any{"3-star", 3.0}
	hotel[9] = "/g/pagehotel"

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"397419284": []any{hotel},
			},
		},
	}
	innerData := []any{[]any{[]any{[]any{nil, hotelList}}}}
	dataJSON, _ := json.Marshal(innerData)

	page := `<html>AF_initDataCallback({key: 'ds:0', data:` + string(dataJSON) + `});</html>`

	hotels, err := parseHotelsFromPage(page, "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("expected 1 hotel, got %d", len(hotels))
	}
	if hotels[0].Name != "Page Hotel" {
		t.Errorf("Name = %q", hotels[0].Name)
	}
}

func TestParseHotelsFromPage_DeduplicateOrgAndSponsored(t *testing.T) {
	// Same hotel appears in both organic and sponsored.
	organicHotel := make([]any, 12)
	organicHotel[0] = nil
	organicHotel[1] = "Duplicate Hotel"
	organicHotel[2] = []any{[]any{60.168, 24.941}}
	organicHotel[9] = "/g/dup"

	sponsoredHotel := make([]any, 7)
	sponsoredHotel[0] = "Duplicate Hotel" // same name
	sponsoredHotel[2] = "EUR 150"
	sponsoredHotel[4] = float64(100)
	sponsoredHotel[5] = float64(4.0)

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"397419284": []any{organicHotel},
				"300000000": []any{nil, nil, []any{sponsoredHotel}},
			},
		},
	}
	innerData := []any{[]any{[]any{[]any{nil, hotelList}}}}
	dataJSON, _ := json.Marshal(innerData)

	page := `AF_initDataCallback({key: 'ds:0', data:` + string(dataJSON) + `});`

	hotels, err := parseHotelsFromPage(page, "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hotels) != 1 {
		t.Errorf("expected 1 hotel after dedup, got %d", len(hotels))
	}
}

// --- parseHotelsFromRaw ---

func TestParseHotelsFromRaw_WithHotels(t *testing.T) {
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Raw Hotel"
	hotel[2] = []any{[]any{60.168, 24.941}}

	entries := []any{
		[]any{hotel},
	}

	hotels, err := parseHotelsFromRaw(entries, "USD")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("expected 1, got %d", len(hotels))
	}
	if hotels[0].Name != "Raw Hotel" {
		t.Errorf("Name = %q", hotels[0].Name)
	}
}

func TestParseHotelsFromRaw_NoHotels(t *testing.T) {
	entries := []any{"no hotels here"}
	_, err := parseHotelsFromRaw(entries, "USD")
	if err == nil {
		t.Error("expected error for no hotels in raw")
	}
}

// --- parsePricesFromRaw ---

func TestParsePricesFromRaw_WithPrices(t *testing.T) {
	entries := []any{
		[]any{
			[]any{"Booking.com", 189.0, "USD"},
		},
	}

	prices, err := parsePricesFromRaw(entries)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(prices) != 1 {
		t.Fatalf("expected 1 price, got %d", len(prices))
	}
}

func TestParsePricesFromRaw_NoPrices(t *testing.T) {
	entries := []any{"no prices"}
	_, err := parsePricesFromRaw(entries)
	if err == nil {
		t.Error("expected error for no prices in raw")
	}
}

// --- parseHotelsFromPayload ---

func TestParseHotelsFromPayload_NoHotels(t *testing.T) {
	_, err := parseHotelsFromPayload("just a string", "USD")
	if err == nil {
		t.Error("expected error for non-hotel payload")
	}
}

func TestParseHotelsFromPayload_WithHotels(t *testing.T) {
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Payload Hotel"
	hotel[2] = []any{[]any{51.5, -0.12}}

	payload := []any{[]any{hotel}}

	hotels, err := parseHotelsFromPayload(payload, "GBP")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("expected 1, got %d", len(hotels))
	}
}

// --- findPriceArrays ---

func TestFindPriceArrays_NotArray(t *testing.T) {
	result := findPriceArrays("not an array", 0)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestFindPriceArrays_MaxDepth(t *testing.T) {
	// Create deep nesting beyond max depth.
	var data any = []any{[]any{"Booking.com", 189.0, "USD"}}
	for range 10 {
		data = []any{data}
	}
	result := findPriceArrays(data, 0)
	if len(result) != 0 {
		t.Errorf("expected 0 at max depth, got %d", len(result))
	}
}

func TestFindPriceArrays_NestedPrices(t *testing.T) {
	prices := []any{
		[]any{"Booking.com", 189.0, "USD"},
		[]any{"Expedia", 195.0, "USD"},
	}
	data := []any{nil, []any{nil, prices}}

	result := findPriceArrays(data, 0)
	if len(result) < 1 {
		t.Errorf("expected at least 1 price entry, got %d", len(result))
	}
}

// --- extractOrganicPrice edge cases ---

func TestExtractOrganicPrice_ShortInnerArray(t *testing.T) {
	// Inner array has < 4 elements — should be skipped.
	raw := []any{nil, []any{nil, nil}}
	price, cur := extractOrganicPrice(raw)
	if price != 0 || cur != "" {
		t.Errorf("expected (0, \"\"), got (%v, %q)", price, cur)
	}
}

func TestExtractOrganicPrice_ZeroPrice(t *testing.T) {
	raw := []any{nil, []any{[]any{0.0, 0.0}, nil, nil, "EUR"}}
	price, _ := extractOrganicPrice(raw)
	if price != 0 {
		t.Errorf("expected 0 for zero price, got %v", price)
	}
}

// --- Integration: full pipeline mock ---

func TestParseHotelSearchResponse_FullPipeline(t *testing.T) {
	// Build a complete mock batchexecute response.
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Pipeline Hotel"
	hotel[2] = []any{[]any{60.168, 24.941}}
	hotel[3] = []any{"4-star", 4.0}
	hotel[7] = []any{[]any{4.5, 800.0}}
	hotel[9] = "/g/pipeline"

	hotelData := []any{hotel}
	innerJSON, _ := json.Marshal(hotelData)

	entries := []any{
		[]any{
			[]any{"wrb.fr", "AtySUc", string(innerJSON), nil, nil, nil, "generic"},
		},
	}

	hotels, err := ParseHotelSearchResponse(entries, "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hotels) < 1 {
		t.Fatal("expected at least 1 hotel")
	}
}

// --- parseSponsoredHotel edge cases ---

func TestParseSponsoredHotel_NonNumericReviews(t *testing.T) {
	entry := make([]any, 7)
	entry[0] = "Hotel"
	entry[4] = "not a number" // review count should be float64
	entry[5] = "also not"     // rating should be float64

	h := parseSponsoredHotel(entry, "USD")
	if h.ReviewCount != 0 {
		t.Errorf("ReviewCount = %d, want 0", h.ReviewCount)
	}
	if h.Rating != 0 {
		t.Errorf("Rating = %v, want 0", h.Rating)
	}
}

// --- HotelSearchOptions defaults ---

func TestHotelSearchOptions_Defaults(t *testing.T) {
	opts := HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
	}

	// Verify defaults are applied in SearchHotels (guests=2, currency=USD).
	// We can't run the full search, but verify the URL building.
	url := buildTravelURL("Helsinki", opts)
	// Guests default is not applied in buildTravelURL but in SearchHotels.
	// Just verify it doesn't crash.
	if url == "" {
		t.Error("empty URL")
	}
}

// --- parseSponsoredHotel with nil price string ---

func TestParseSponsoredHotel_NilPrice(t *testing.T) {
	entry := make([]any, 7)
	entry[0] = "Hotel"
	entry[2] = nil // nil price
	h := parseSponsoredHotel(entry, "USD")
	if h.Price != 0 {
		t.Errorf("Price = %v, want 0", h.Price)
	}
}

// --- parseOrganicHotel edge cases ---

func TestParseOrganicHotel_BadCoords(t *testing.T) {
	entry := make([]any, 12)
	entry[1] = "Hotel"
	entry[2] = []any{[]any{"not", "floats"}} // bad coordinates
	h := parseOrganicHotel(entry, "USD")
	if h.Lat != 0 || h.Lon != 0 {
		t.Error("expected zero coords for non-float values")
	}
}

func TestParseOrganicHotel_EmptyLocArray(t *testing.T) {
	entry := make([]any, 12)
	entry[1] = "Hotel"
	entry[2] = []any{} // empty location array
	h := parseOrganicHotel(entry, "USD")
	if h.Lat != 0 {
		t.Errorf("Lat = %v, want 0", h.Lat)
	}
}

func TestParseOrganicHotel_BadStarRating(t *testing.T) {
	entry := make([]any, 12)
	entry[1] = "Hotel"
	entry[3] = []any{"label"} // only 1 element, need >= 2
	h := parseOrganicHotel(entry, "USD")
	if h.Stars != 0 {
		t.Errorf("Stars = %d, want 0", h.Stars)
	}
}

func TestParseOrganicHotel_BadRating(t *testing.T) {
	entry := make([]any, 12)
	entry[1] = "Hotel"
	entry[7] = []any{"not an array pair"}
	h := parseOrganicHotel(entry, "USD")
	if h.Rating != 0 {
		t.Errorf("Rating = %v, want 0", h.Rating)
	}
}

// --- extractBatchPayload edge cases ---

func TestExtractBatchPayload_EmptyHotelArr(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"12345": []any{}, // empty
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractOrganicHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

func TestExtractSponsoredHotels_EntryNotArray(t *testing.T) {
	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"300000000": []any{nil, nil, []any{
					"not an array", // should be skipped
				}},
			},
		},
	}
	data := []any{[]any{[]any{[]any{nil, hotelList}}}}
	hotels := extractSponsoredHotels(data, "USD")
	if len(hotels) != 0 {
		t.Errorf("expected 0, got %d", len(hotels))
	}
}

// Full organic+sponsored integration via parseHotelsFromPage
func TestParseHotelsFromPage_MixedOrgAndSponsored(t *testing.T) {
	organic := make([]any, 12)
	organic[0] = nil
	organic[1] = "Organic Only Hotel"
	organic[2] = []any{[]any{60.168, 24.941}}
	organic[9] = "/g/org"

	sponsored := make([]any, 7)
	sponsored[0] = "Sponsored Only Hotel"
	sponsored[2] = "EUR 200"
	sponsored[4] = float64(400)
	sponsored[5] = float64(4.1)

	hotelList := []any{
		[]any{
			nil,
			map[string]any{
				"397419284": []any{organic},
				"300000000": []any{nil, nil, []any{sponsored}},
			},
		},
	}
	innerData := []any{[]any{[]any{[]any{nil, hotelList}}}}
	dataJSON, _ := json.Marshal(innerData)

	page := `AF_initDataCallback({key: 'ds:0', data:` + string(dataJSON) + `});`

	hotels, err := parseHotelsFromPage(page, "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hotels) != 2 {
		t.Fatalf("expected 2 hotels (1 organic + 1 sponsored), got %d", len(hotels))
	}

	names := map[string]bool{}
	for _, h := range hotels {
		names[h.Name] = true
	}
	if !names["Organic Only Hotel"] || !names["Sponsored Only Hotel"] {
		t.Errorf("missing expected hotels: %v", names)
	}
}

// Verify parseOrganicHotel with all fields
func TestParseOrganicHotel_AllFields(t *testing.T) {
	entry := make([]any, 12)
	entry[0] = nil
	entry[1] = "Full Hotel"
	entry[2] = []any{[]any{51.5074, -0.1278}}
	entry[3] = []any{"5-star hotel", 5.0}
	entry[6] = []any{nil, []any{[]any{350.0, 0.0}, nil, nil, "GBP"}}
	entry[7] = []any{[]any{4.8, 2000.0}}
	entry[9] = "ChIJ123abc"
	entry[11] = []any{"123 London Road"}

	h := parseOrganicHotel(entry, "GBP")

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Name", h.Name, "Full Hotel"},
		{"Lat", h.Lat, 51.5074},
		{"Lon", h.Lon, -0.1278},
		{"Stars", h.Stars, 5},
		{"Price", h.Price, 350.0},
		{"Currency", h.Currency, "GBP"},
		{"Rating", h.Rating, 4.8},
		{"ReviewCount", h.ReviewCount, 2000},
		{"HotelID", h.HotelID, "ChIJ123abc"},
		{"Address", h.Address, "123 London Road"},
	}

	for _, c := range checks {
		switch want := c.want.(type) {
		case string:
			if c.got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, want)
			}
		case float64:
			if c.got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, want)
			}
		case int:
			if c.got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, want)
			}
		}
	}
}
