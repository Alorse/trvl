package main

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/watch"
)

// ---------------------------------------------------------------------------
// looksLikeGoogleHotelID — CHIJ prefix and colon form (not in v3 tests)
// ---------------------------------------------------------------------------

func TestLooksLikeGoogleHotelID_CHIJ_V26(t *testing.T) {
	cases := []string{"ChIJabc123", "chijlower", "CHIJUPPER"}
	for _, id := range cases {
		if !looksLikeGoogleHotelID(id) {
			t.Errorf("looksLikeGoogleHotelID(%q) = false, want true", id)
		}
	}
}

func TestLooksLikeGoogleHotelID_ColonForm_V26(t *testing.T) {
	// Exactly one colon and no whitespace
	if !looksLikeGoogleHotelID("namespace:identifier") {
		t.Error("looksLikeGoogleHotelID(colon form) = false, want true")
	}
	// Two colons → not an ID
	if looksLikeGoogleHotelID("a:b:c") {
		t.Error("looksLikeGoogleHotelID(two colons) = true, want false")
	}
	// Colon with space → not an ID
	if looksLikeGoogleHotelID("a: b") {
		t.Error("looksLikeGoogleHotelID(colon with space) = true, want false")
	}
}

func TestLooksLikeGoogleHotelID_Whitespace_V26(t *testing.T) {
	// Leading/trailing whitespace trimmed; /g/ after trim
	if !looksLikeGoogleHotelID("  /g/somehotel  ") {
		t.Error("expected true for /g/ with surrounding whitespace")
	}
}

// ---------------------------------------------------------------------------
// liveChecker.CheckPrice — unknown type returns error (no network call)
// ---------------------------------------------------------------------------

func TestLiveChecker_CheckPrice_UnknownType(t *testing.T) {
	c := &liveChecker{}
	ctx := context.Background()
	w := watch.Watch{Type: "unknown"}
	_, _, _, err := c.CheckPrice(ctx, w)
	if err == nil {
		t.Error("expected error for unknown watch type")
	}
}

// ---------------------------------------------------------------------------
// liveRoomChecker — satisfies watch.RoomChecker interface (compile check)
// ---------------------------------------------------------------------------

func TestLiveRoomChecker_Interface_V26(t *testing.T) {
	var _ watch.RoomChecker = &liveRoomChecker{}
}

// ---------------------------------------------------------------------------
// formatRoomsTable — no-name path (uses HotelID) and rooms with long amenities
// ---------------------------------------------------------------------------

func TestFormatRoomsTable_NoName_UsesHotelID_V26(t *testing.T) {
	result := &hotels.RoomAvailability{
		HotelID: "/g/unknown",
		Rooms: []hotels.RoomType{
			{Name: "Deluxe", Price: 200, Currency: "EUR", MaxGuests: 2,
				Amenities: []string{"wifi", "breakfast", "pool", "spa", "gym", "parking"}},
		},
	}
	err := formatRoomsTable(result)
	if err != nil {
		t.Errorf("formatRoomsTable(no name, long amenities) error: %v", err)
	}
}

func TestFormatRoomsTable_ZeroPrice_V26(t *testing.T) {
	// Room with zero price exercises the priceText="" branch.
	result := &hotels.RoomAvailability{
		Name: "Test Hotel",
		Rooms: []hotels.RoomType{
			{Name: "Free Room", Price: 0, Currency: "EUR"},
			{Name: "Paid Room", Price: 150, Currency: "EUR", MaxGuests: 2},
		},
	}
	err := formatRoomsTable(result)
	if err != nil {
		t.Errorf("formatRoomsTable(zero price) error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// maybeShowAccomHackTip — empty-checkin path (not yet in display_coverage2_test)
// ---------------------------------------------------------------------------

func TestMaybeShowAccomHackTip_EmptyCheckIn_V26(t *testing.T) {
	maybeShowAccomHackTip(context.Background(), "Helsinki", "", "2026-07-04", "EUR", 1)
}

func TestMaybeShowAccomHackTip_EmptyCheckOut_V26(t *testing.T) {
	maybeShowAccomHackTip(context.Background(), "Helsinki", "2026-07-01", "", "EUR", 1)
}

// ---------------------------------------------------------------------------
// hotelSourceLabels — deduplicated path
// ---------------------------------------------------------------------------

func TestHotelSourceLabels_Deduplication_V26(t *testing.T) {
	h := models.HotelResult{
		Sources: []models.PriceSource{
			{Provider: "google_hotels"},
			{Provider: "google_hotels"}, // duplicate
			{Provider: "booking"},
		},
	}
	got := hotelSourceLabels(h)
	if got == "" {
		t.Error("expected non-empty label for known sources")
	}
}
