package watch

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================
// buildBookingURL — hotel without return date (not in notify_test.go)
// ============================================================

func TestBuildBookingURL_HotelNoReturn(t *testing.T) {
	url := buildBookingURL(Watch{Type: "hotel", Destination: "Prague", DepartDate: "2026-07-01"})
	want := "https://www.google.com/travel/hotels/Prague?dates=2026-07-01"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestBuildBookingURL_RoomType(t *testing.T) {
	url := buildBookingURL(Watch{Type: "room"})
	if url != "" {
		t.Errorf("expected empty URL for room type, got %q", url)
	}
}

// ============================================================
// Notifier.Notify — big price drop (error fare advice)
// ============================================================

func TestNotify_BigDrop_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Origin: "HEL", Destination: "BCN", Type: "flight"},
		NewPrice:  100,
		Currency:  "EUR",
		PrevPrice: 300,
		PriceDrop: -200, // >30% drop
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "error fare") || !strings.Contains(got, "flash sale") {
		t.Errorf("expected big drop advice, got %q", got)
	}
}

func TestNotify_NormalDrop_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Origin: "HEL", Destination: "BCN", Type: "flight"},
		NewPrice:  250,
		Currency:  "EUR",
		PrevPrice: 300,
		PriceDrop: -50, // ~16% drop
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "good time to book") {
		t.Errorf("expected normal drop advice, got %q", got)
	}
}

func TestNotify_FlightTrendingUp_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Origin: "HEL", Destination: "BCN", Type: "flight"},
		NewPrice:  350,
		Currency:  "EUR",
		PrevPrice: 300,
		PriceDrop: 50,
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "trending up") {
		t.Errorf("expected trending up advice, got %q", got)
	}
}

func TestNotify_HotelTrendingUp_NoAdvice(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	// For hotel type, trending up advice should NOT appear.
	r := CheckResult{
		Watch:     Watch{Origin: "Helsinki", Destination: "Barcelona", Type: "hotel"},
		NewPrice:  150,
		Currency:  "EUR",
		PrevPrice: 100,
		PriceDrop: 50,
	}
	n.Notify(r)

	got := buf.String()
	if strings.Contains(got, "trending up") {
		t.Errorf("hotel should not show trending up advice, got %q", got)
	}
}

// ============================================================
// Notifier.Notify — below goal without depart date (no booking URL)
// ============================================================

func TestNotify_BelowGoalNoDate_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Origin: "HEL", Destination: "BCN", BelowPrice: 200, Type: "flight"},
		NewPrice:  180,
		Currency:  "EUR",
		BelowGoal: true,
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "DEAL") {
		t.Errorf("expected DEAL in output")
	}
	if strings.Contains(got, "Book:") {
		t.Errorf("should not contain booking URL without depart date")
	}
}

// ============================================================
// Notifier.notifyRoom — all paths
// ============================================================

func TestNotifyRoom_Error_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch: Watch{Type: "room", HotelName: "Grand Hotel", RoomKeywords: []string{"suite"}},
		Error: testErr{msg: "no rooms available"},
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "ERR") {
		t.Errorf("expected ERR, got %q", got)
	}
	if !strings.Contains(got, "Grand Hotel") {
		t.Errorf("expected hotel name")
	}
	if !strings.Contains(got, "suite") {
		t.Errorf("expected keywords")
	}
}

type testErr struct{ msg string }

func (e testErr) Error() string { return e.msg }

func TestNotifyRoom_NoMatches_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Type: "room", HotelName: "Grand Hotel", RoomKeywords: []string{"suite"}},
		RoomFound: false,
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "no matching rooms") {
		t.Errorf("expected 'no matching rooms', got %q", got)
	}
}

func TestNotifyRoom_FoundMultiple_Coverage(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Type: "room", HotelName: "Grand Hotel", RoomKeywords: []string{"suite"}},
		RoomFound: true,
		RoomMatches: []RoomMatch{
			{Name: "Junior Suite", Price: 200, Currency: "EUR", Provider: "booking.com"},
			{Name: "Executive Suite", Price: 350, Currency: "EUR"},
		},
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "Junior Suite") {
		t.Errorf("expected first room")
	}
	if !strings.Contains(got, "Executive Suite") {
		t.Errorf("expected second room")
	}
	if !strings.Contains(got, "booking.com") {
		t.Errorf("expected provider")
	}
}

func TestNotifyRoom_FoundNoPriceNoProvider(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Type: "room", HotelName: "Test Hotel", RoomKeywords: []string{"standard"}},
		RoomFound: true,
		RoomMatches: []RoomMatch{
			{Name: "Standard Room"},
		},
	}
	n.Notify(r)

	got := buf.String()
	if !strings.Contains(got, "ROOM AVAILABLE") {
		t.Errorf("expected ROOM AVAILABLE")
	}
	if !strings.Contains(got, "Standard Room") {
		t.Errorf("expected room name")
	}
}

// ============================================================
// Watch.Validate — additional edge cases not in watch_extra_test.go
// ============================================================

func TestWatch_Validate_CombinedDateAndRange_Coverage(t *testing.T) {
	w := Watch{
		Type:       "flight",
		DepartDate: "2026-07-01",
		DepartFrom: "2026-07-01",
		DepartTo:   "2026-07-31",
	}
	err := w.Validate()
	if err == nil {
		t.Error("expected error for combined date and range")
	}
	if !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatch_Validate_RoomWatch_MissingHotelName_Coverage(t *testing.T) {
	w := Watch{
		Type:         "room",
		RoomKeywords: []string{"suite"},
		DepartDate:   "2026-07-01",
		ReturnDate:   "2026-07-08",
	}
	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "hotel name") {
		t.Errorf("expected hotel name error, got %v", err)
	}
}

func TestWatch_Validate_RoomWatch_MissingKeywords_Coverage(t *testing.T) {
	w := Watch{
		Type:       "room",
		HotelName:  "Grand Hotel",
		DepartDate: "2026-07-01",
		ReturnDate: "2026-07-08",
	}
	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "keyword") {
		t.Errorf("expected keyword error, got %v", err)
	}
}

func TestWatch_Validate_RoomWatch_MissingDates_Coverage(t *testing.T) {
	w := Watch{
		Type:         "room",
		HotelName:    "Grand Hotel",
		RoomKeywords: []string{"suite"},
	}
	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "check-in") {
		t.Errorf("expected date error, got %v", err)
	}
}

func TestWatch_Validate_RoomWatch_Valid_Coverage(t *testing.T) {
	w := Watch{
		Type:         "room",
		HotelName:    "Grand Hotel",
		RoomKeywords: []string{"suite"},
		DepartDate:   "2026-07-01",
		ReturnDate:   "2026-07-08",
	}
	err := w.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatch_Validate_InvalidReturnDate_Coverage(t *testing.T) {
	w := Watch{
		Type:       "flight",
		DepartDate: "2026-07-01",
		ReturnDate: "07/08/2026",
	}
	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "return date") {
		t.Errorf("expected return date format error, got %v", err)
	}
}

func TestWatch_Validate_InvalidDepartFrom_Coverage(t *testing.T) {
	w := Watch{
		Type:       "flight",
		DepartFrom: "07/01/2026",
		DepartTo:   "2026-07-31",
	}
	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "date range start") {
		t.Errorf("expected date range start format error, got %v", err)
	}
}

func TestWatch_Validate_InvalidDepartTo_Coverage(t *testing.T) {
	w := Watch{
		Type:       "flight",
		DepartFrom: "2026-07-01",
		DepartTo:   "07/31/2026",
	}
	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "date range end") {
		t.Errorf("expected date range end format error, got %v", err)
	}
}

func TestWatch_Validate_RouteWatch_Coverage(t *testing.T) {
	w := Watch{Type: "flight", Origin: "HEL", Destination: "BCN"}
	if err := w.Validate(); err != nil {
		t.Errorf("unexpected error for route watch: %v", err)
	}
}

func TestWatch_Validate_ValidDateRange_Coverage(t *testing.T) {
	w := Watch{Type: "flight", DepartFrom: "2026-07-01", DepartTo: "2026-07-31"}
	if err := w.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatch_Validate_SameDayRange_Coverage(t *testing.T) {
	w := Watch{Type: "flight", DepartFrom: "2026-07-15", DepartTo: "2026-07-15"}
	if err := w.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// shortID
// ============================================================

func TestShortID_Coverage(t *testing.T) {
	id := shortID()
	if len(id) != 8 {
		t.Errorf("shortID() length = %d, want 8", len(id))
	}
	// Verify uniqueness across a small batch.
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		id := shortID()
		if seen[id] {
			t.Errorf("duplicate shortID: %q", id)
		}
		seen[id] = true
	}
}

// ============================================================
// Notify — first check (no PrevPrice)
// ============================================================

func TestNotify_FirstCheck_NoPrevPrice(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:     Watch{Origin: "HEL", Destination: "BCN", Type: "flight"},
		NewPrice:  200,
		Currency:  "EUR",
		PrevPrice: 0, // first check, no previous price
	}
	n.Notify(r)

	got := buf.String()
	// Should show price without change indicator.
	if strings.Contains(got, "(") {
		// No change indicator when PrevPrice is 0.
		if strings.Contains(got, "(unchanged)") || strings.Contains(got, "(+") || strings.Contains(got, "(-") {
			t.Errorf("first check should not show change indicator, got %q", got)
		}
	}
	if !strings.Contains(got, "200") {
		t.Errorf("expected price 200, got %q", got)
	}
}

// ============================================================
// Notify — lowest not shown when current is lowest
// ============================================================

func TestNotify_LowestEqualsCurrentNotShown(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:    Watch{Origin: "HEL", Destination: "BCN", Type: "flight", LowestPrice: 200},
		NewPrice: 200,
		Currency: "EUR",
	}
	n.Notify(r)

	got := buf.String()
	if strings.Contains(got, "lowest:") {
		t.Errorf("should not show lowest when it equals current, got %q", got)
	}
}

func TestNotify_LowestHigherThanCurrentNotShown(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf, UseColor: false}

	r := CheckResult{
		Watch:    Watch{Origin: "HEL", Destination: "BCN", Type: "flight", LowestPrice: 300},
		NewPrice: 200,
		Currency: "EUR",
	}
	n.Notify(r)

	got := buf.String()
	if strings.Contains(got, "lowest:") {
		t.Errorf("should not show lowest when it exceeds current, got %q", got)
	}
}
