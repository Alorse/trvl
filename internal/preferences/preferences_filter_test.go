package preferences

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestIsDormitory_HostelChains(t *testing.T) {
	tests := []struct {
		name string
		h    models.HotelResult
		want bool
	}{
		{name: "st christophers", h: models.HotelResult{Name: "St Christopher's Inn Prague"}, want: true},
		{name: "generator", h: models.HotelResult{Name: "Generator Berlin"}, want: true},
		{name: "meininger", h: models.HotelResult{Name: "MEININGER Hotel Amsterdam"}, want: true},
		{name: "wombat", h: models.HotelResult{Name: "Wombats City Hostel"}, want: true},
		{name: "clink", h: models.HotelResult{Name: "Clink78 London"}, want: true},
		{name: "safestay", h: models.HotelResult{Name: "Safestay Holland Park"}, want: true},
		{name: "rygerfjord", h: models.HotelResult{Name: "Rygerfjord Hotel & Hostel"}, want: true},
		{name: "citybox", h: models.HotelResult{Name: "Citybox Oslo"}, want: true},
		{name: "a&o", h: models.HotelResult{Name: "A&O Berlin Mitte"}, want: true},
		{name: "yha", h: models.HotelResult{Name: "YHA London Central"}, want: true},
		{name: "travelodge", h: models.HotelResult{Name: "Travelodge London Central"}, want: true},
		{name: "generic hostel", h: models.HotelResult{Name: "Prague Central Hostel"}, want: true},
		{name: "generic dorm", h: models.HotelResult{Name: "Downtown Dormitory"}, want: true},
		{name: "capsule", h: models.HotelResult{Name: "Tokyo Capsule Hotel"}, want: true},
		{name: "backpacker", h: models.HotelResult{Name: "Backpacker Haven"}, want: true},
		{name: "pod hotel", h: models.HotelResult{Name: "Pod Hotel NYC"}, want: true},
		{name: "bunk", h: models.HotelResult{Name: "Bunk Hostel"}, want: true},
		{name: "room in", h: models.HotelResult{Name: "Room in City Center"}, want: true},
		{name: "private room", h: models.HotelResult{Name: "Private Room at House"}, want: true},
		{name: "shared room", h: models.HotelResult{Name: "Shared Room Budget"}, want: true},
		{name: "guesthouse", h: models.HotelResult{Name: "Old Town Guesthouse"}, want: true},
		{name: "guest house", h: models.HotelResult{Name: "Cozy Guest House"}, want: true},
		// Non-hostels
		{name: "hilton", h: models.HotelResult{Name: "Hilton Prague"}, want: false},
		{name: "marriott", h: models.HotelResult{Name: "Marriott Amsterdam"}, want: false},
		{name: "novotel", h: models.HotelResult{Name: "Novotel Berlin"}, want: false},
		{name: "ibis", h: models.HotelResult{Name: "ibis Budget Paris"}, want: false},
		{name: "radisson", h: models.HotelResult{Name: "Radisson Blu Helsinki"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDormitory(tt.h)
			if got != tt.want {
				t.Errorf("isDormitory(%q) = %v, want %v", tt.h.Name, got, tt.want)
			}
		})
	}
}

func TestIsDormitory_SubListingPattern(t *testing.T) {
	tests := []struct {
		name string
		h    models.HotelResult
		want bool
	}{
		{name: "sub-listing double room", h: models.HotelResult{Name: "Main Square Rooms - Small Double Room"}, want: true},
		{name: "sub-listing studio", h: models.HotelResult{Name: "City Inn - Deluxe Studio"}, want: true},
		{name: "sub-listing apartment", h: models.HotelResult{Name: "Grand Place - One Bedroom Apartment"}, want: true},
		{name: "sub-listing suite", h: models.HotelResult{Name: "River View - Executive Suite"}, want: true},
		{name: "sub-listing twin", h: models.HotelResult{Name: "Budget Stay - Twin Room"}, want: true},
		{name: "sub-listing single", h: models.HotelResult{Name: "Central - Single Room"}, want: true},
		{name: "sub-listing triple", h: models.HotelResult{Name: "Hostel X - Triple Room"}, want: true},
		{name: "sub-listing dorm", h: models.HotelResult{Name: "City Place - 6 Bed Dorm"}, want: true},
		// Normal hotel names with hyphens but NOT sub-listings
		{name: "normal hyphen name", h: models.HotelResult{Name: "Hotel Negresco"}, want: false},
		{name: "city hyphen", h: models.HotelResult{Name: "Park Inn by Radisson"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDormitory(tt.h)
			if got != tt.want {
				t.Errorf("isDormitory(%q) = %v, want %v", tt.h.Name, got, tt.want)
			}
		})
	}
}

func TestIsDormitory_Amenities(t *testing.T) {
	h := models.HotelResult{
		Name:      "Regular Hotel",
		Amenities: []string{"wifi", "shared room available"},
	}
	if !isDormitory(h) {
		t.Error("isDormitory should detect 'shared room' in amenities")
	}

	h2 := models.HotelResult{
		Name:      "Regular Hotel",
		Amenities: []string{"wifi", "dorm-style bed"},
	}
	if !isDormitory(h2) {
		t.Error("isDormitory should detect 'dorm' in amenities")
	}
}

func TestLacksPrivateBathroom(t *testing.T) {
	tests := []struct {
		name string
		h    models.HotelResult
		want bool
	}{
		{
			name: "shared bathroom in amenities",
			h:    models.HotelResult{Name: "Budget Inn", Amenities: []string{"shared bathroom", "wifi"}},
			want: true,
		},
		{
			name: "communal bathroom in address",
			h:    models.HotelResult{Name: "Old Town Stay", Address: "communal bathroom on each floor"},
			want: true,
		},
		{
			name: "shared bath keyword",
			h:    models.HotelResult{Name: "Cheap Hotel", Amenities: []string{"shared bath"}},
			want: true,
		},
		{
			name: "shared facilities",
			h:    models.HotelResult{Name: "Hostel B", Amenities: []string{"shared facilities"}},
			want: true,
		},
		{
			name: "common bathroom",
			h:    models.HotelResult{Name: "Pension", Amenities: []string{"common bathroom"}},
			want: true,
		},
		{
			name: "normal hotel no signal",
			h:    models.HotelResult{Name: "Hilton", Amenities: []string{"pool", "wifi", "gym"}},
			want: false,
		},
		{
			name: "no amenities at all",
			h:    models.HotelResult{Name: "Mystery Hotel"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lacksPrivateBathroom(tt.h)
			if got != tt.want {
				t.Errorf("lacksPrivateBathroom(%q) = %v, want %v", tt.h.Name, got, tt.want)
			}
		})
	}
}

func TestDropAirportAndSuburbHotels(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Central Hotel", Address: "Centrum, Amsterdam"},
		{Name: "Airport Hotel", Address: "Schiphol, Hoofddorp"},
		{Name: "City Inn", Address: "Dam Square, Amsterdam"},
		{Name: "Roissy Hub", Address: "Roissy-en-France, Paris"},
		{Name: "Fiumicino Rest", Address: "Fiumicino, Rome"},
	}

	got := dropAirportAndSuburbHotels(hotels, "Amsterdam")
	if len(got) != 2 {
		t.Fatalf("expected 2 hotels after dropping airport ones, got %d", len(got))
	}
	for _, h := range got {
		nameLow := h.Name
		if nameLow == "Airport Hotel" || nameLow == "Roissy Hub" || nameLow == "Fiumicino Rest" {
			t.Errorf("airport hotel %q should have been dropped", h.Name)
		}
	}
}

func TestDropAirportAndSuburbHotels_AllDropped_ReturnsOriginal(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Airport Inn", Address: "Schiphol, Airport Area"},
		{Name: "Gatwick Rest", Address: "Near Gatwick"},
	}

	got := dropAirportAndSuburbHotels(hotels, "London")
	if len(got) != 2 {
		t.Errorf("when all hotels dropped, should return original list; got %d", len(got))
	}
}

func TestDropAirportAndSuburbHotels_AllAirportKeywords(t *testing.T) {
	tests := []struct {
		keyword string
	}{
		{"airport"}, {"aéroport"}, {"aeroport"},
		{"villepinte"}, {"roissy"}, {"tremblay-en-france"},
		{"orly"}, {"rungis"}, {"massy"},
		{"stansted"}, {"luton"}, {"gatwick"},
		{"schiphol"}, {"hoofddorp"},
		{"fiumicino"},
		{"vantaa airport"},
		{"barajas"},
		{"zaventem"},
		{"malpensa"},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			hotels := []models.HotelResult{
				{Name: "Good Hotel", Address: "City Center"},
				{Name: "Bad Hotel", Address: tt.keyword},
			}
			got := dropAirportAndSuburbHotels(hotels, "Anywhere")
			if len(got) != 1 {
				t.Errorf("keyword %q: expected 1 hotel after filter, got %d", tt.keyword, len(got))
			}
		})
	}
}

func TestFilterByDistrict(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel A", Address: "Prague 1, Old Town"},
		{Name: "Hotel B", Address: "Prague 8, Suburbs"},
		{Name: "Hotel C", Address: "Vinohrady, Prague 3"},
	}

	got, matched := filterByDistrict(hotels, []string{"Prague 1", "Prague 3"})
	if !matched {
		t.Fatal("filterByDistrict should have matched")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 hotels, got %d", len(got))
	}
	if got[0].Name != "Hotel A" || got[1].Name != "Hotel C" {
		t.Errorf("unexpected hotels: %v", got)
	}
}

func TestFilterByDistrict_NoMatch(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel A", Address: "Suburbs"},
	}
	got, matched := filterByDistrict(hotels, []string{"City Center"})
	if matched {
		t.Error("filterByDistrict should not match")
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d", len(got))
	}
}

func TestPrioritiseByDistrict(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Suburban Hotel", Address: "Far Away"},
		{Name: "Central Hotel", Address: "Jordaan, Amsterdam"},
		{Name: "Canal Hotel", Address: "Grachtengordel, Amsterdam"},
	}

	got := prioritiseByDistrict(hotels, []string{"jordaan", "grachtengordel"})
	if len(got) != 3 {
		t.Fatalf("expected 3 hotels, got %d", len(got))
	}
	// Preferred hotels should come first.
	if got[0].Name != "Central Hotel" {
		t.Errorf("first hotel should be Central Hotel, got %q", got[0].Name)
	}
	if got[1].Name != "Canal Hotel" {
		t.Errorf("second hotel should be Canal Hotel, got %q", got[1].Name)
	}
	if got[2].Name != "Suburban Hotel" {
		t.Errorf("third hotel should be Suburban Hotel, got %q", got[2].Name)
	}
}

func TestPrioritiseByDistrict_NoMatches(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel A", Address: "Unknown"},
		{Name: "Hotel B", Address: "Somewhere"},
	}
	got := prioritiseByDistrict(hotels, []string{"centrum"})
	if len(got) != 2 {
		t.Fatalf("expected 2 hotels, got %d", len(got))
	}
	// Order should be preserved.
	if got[0].Name != "Hotel A" || got[1].Name != "Hotel B" {
		t.Error("order should be preserved when no matches")
	}
}

func TestSubListingPattern(t *testing.T) {
	positive := []string{
		"Hotel X - Double Room",
		"Inn - Single Room",
		"Apartments - Studio",
		"Place - One Bedroom Apartment",
		"Suites - Executive Suite",
		"Budget - Twin Room",
		"Hostel - 4 Bed Dorm",
		"Villa - Triple Room",
	}
	for _, s := range positive {
		if !subListingPattern.MatchString(s) {
			t.Errorf("subListingPattern should match %q", s)
		}
	}

	negative := []string{
		"Hotel Negresco",
		"Park Inn by Radisson",
		"Hilton Garden Inn",
		"Room Mate Aitana",
		"Double Tree by Hilton",
	}
	for _, s := range negative {
		if subListingPattern.MatchString(s) {
			t.Errorf("subListingPattern should NOT match %q", s)
		}
	}
}

func TestFilterHotels_MinReviewThreshold(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Popular", Rating: 4.5, ReviewCount: 500},
		{Name: "Few Reviews", Rating: 4.8, ReviewCount: 10},
		{Name: "No Reviews", Rating: 0, ReviewCount: 0},
		{Name: "Decent", Rating: 4.2, ReviewCount: 50},
	}
	p := Default()
	p.MinHotelRating = 4.0

	got := FilterHotels(hotels, "London", p)
	names := make(map[string]bool)
	for _, h := range got {
		names[h.Name] = true
	}
	if names["Few Reviews"] {
		t.Error("hotel with <20 reviews should be filtered when MinHotelRating >= 4.0")
	}
	if names["No Reviews"] {
		t.Error("hotel with 0 reviews and 0 rating should be filtered when MinHotelRating > 0")
	}
	if !names["Popular"] {
		t.Error("Popular hotel should survive filter")
	}
	if !names["Decent"] {
		t.Error("Decent hotel should survive filter")
	}
}

func TestFilterHotels_DefaultDistricts_Prioritises(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Suburb Hotel", Address: "Far Away"},
		{Name: "Jordaan Hotel", Address: "Jordaan, Amsterdam"},
	}
	p := Default()

	got := FilterHotels(hotels, "Amsterdam", p)
	if len(got) < 1 {
		t.Fatal("expected at least 1 hotel")
	}
	// Jordaan should be prioritised to front.
	if got[0].Name != "Jordaan Hotel" {
		t.Errorf("Jordaan hotel should be first, got %q", got[0].Name)
	}
}
