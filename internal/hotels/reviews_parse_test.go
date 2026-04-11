package hotels

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ============================================================
// isDateString
// ============================================================

func TestIsDateString_ISODate(t *testing.T) {
	if !isDateString("2026-07-15") {
		t.Error("expected true for ISO date")
	}
}

func TestIsDateString_RelativeAgo(t *testing.T) {
	if !isDateString("3 weeks ago") {
		t.Error("expected true for relative time")
	}
}

func TestIsDateString_MonthYear(t *testing.T) {
	if !isDateString("March 2026") {
		t.Error("expected true for month year")
	}
}

func TestIsDateString_LongText(t *testing.T) {
	long := "This is a very long review text that happens to mention the word ago but it should not match because it is over fifty characters"
	if isDateString(long) {
		t.Error("expected false for long text")
	}
}

func TestIsDateString_Empty(t *testing.T) {
	if isDateString("") {
		t.Error("expected false for empty string")
	}
}

func TestIsDateString_PlainWord(t *testing.T) {
	if isDateString("decent") {
		t.Error("expected false for 'decent' (should not match 'dec')")
	}
}

func TestIsDateString_StandaloneWeek(t *testing.T) {
	if !isDateString("a week ago") {
		t.Error("expected true for 'a week ago'")
	}
}

func TestIsDateString_AbbreviatedMonth(t *testing.T) {
	if !isDateString("Jan 2026") {
		t.Error("expected true for abbreviated month")
	}
}

// ============================================================
// isURL
// ============================================================

func TestIsURL_HTTPS(t *testing.T) {
	if !isURL("https://example.com") {
		t.Error("expected true for https URL")
	}
}

func TestIsURL_HTTP(t *testing.T) {
	if !isURL("http://example.com") {
		t.Error("expected true for http URL")
	}
}

func TestIsURL_Slash(t *testing.T) {
	if !isURL("/path/to/resource") {
		t.Error("expected true for slash-prefixed path")
	}
}

func TestIsURL_PlainText(t *testing.T) {
	if isURL("Hello World") {
		t.Error("expected false for plain text")
	}
}

func TestIsURL_Empty(t *testing.T) {
	if isURL("") {
		t.Error("expected false for empty string")
	}
}

// ============================================================
// computeSummary
// ============================================================

func TestComputeSummary_MultipleReviews(t *testing.T) {
	reviews := []models.HotelReview{
		{Rating: 4.0, Text: "Good"},
		{Rating: 5.0, Text: "Excellent"},
		{Rating: 3.0, Text: "Average"},
	}
	s := computeSummary(reviews)
	if s.TotalReviews != 3 {
		t.Errorf("TotalReviews = %d, want 3", s.TotalReviews)
	}
	if s.AverageRating != 4.0 {
		t.Errorf("AverageRating = %f, want 4.0", s.AverageRating)
	}
	if s.RatingBreakdown == nil {
		t.Fatal("RatingBreakdown should not be nil")
	}
	if s.RatingBreakdown["5"] != 1 {
		t.Errorf("5-star count = %d, want 1", s.RatingBreakdown["5"])
	}
}

func TestComputeSummary_NilSlice(t *testing.T) {
	s := computeSummary(nil)
	if s.TotalReviews != 0 {
		t.Errorf("TotalReviews = %d, want 0", s.TotalReviews)
	}
	if s.AverageRating != 0 {
		t.Errorf("AverageRating = %f, want 0", s.AverageRating)
	}
}

func TestComputeSummary_SingleReview(t *testing.T) {
	reviews := []models.HotelReview{
		{Rating: 4.5, Text: "Nice place"},
	}
	s := computeSummary(reviews)
	if s.TotalReviews != 1 {
		t.Errorf("TotalReviews = %d, want 1", s.TotalReviews)
	}
	if s.AverageRating != 4.5 {
		t.Errorf("AverageRating = %f, want 4.5", s.AverageRating)
	}
}

// ============================================================
// extractReviewFromJSON
// ============================================================

func TestExtractReviewFromJSON_FullEntry(t *testing.T) {
	obj := map[string]any{
		"reviewBody": "Great location and service",
		"author":     map[string]any{"name": "John"},
		"reviewRating": map[string]any{
			"ratingValue": "4.5",
		},
		"datePublished": "2026-03-15",
	}
	r := extractReviewFromJSON(obj)
	if r.Text != "Great location and service" {
		t.Errorf("Text = %q", r.Text)
	}
	if r.Author != "John" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Rating != 4.5 {
		t.Errorf("Rating = %f", r.Rating)
	}
	if r.Date != "2026-03-15" {
		t.Errorf("Date = %q", r.Date)
	}
}

func TestExtractReviewFromJSON_AuthorAsString(t *testing.T) {
	obj := map[string]any{
		"reviewBody": "Decent stay",
		"author":     "Jane",
	}
	r := extractReviewFromJSON(obj)
	if r.Author != "Jane" {
		t.Errorf("Author = %q, want Jane", r.Author)
	}
}

func TestExtractReviewFromJSON_RatingAsFloat(t *testing.T) {
	obj := map[string]any{
		"reviewBody":   "OK",
		"reviewRating": map[string]any{"ratingValue": 3.0},
	}
	r := extractReviewFromJSON(obj)
	if r.Rating != 3.0 {
		t.Errorf("Rating = %f, want 3.0", r.Rating)
	}
}

func TestExtractReviewFromJSON_EmptyObject(t *testing.T) {
	r := extractReviewFromJSON(map[string]any{})
	if r.Text != "" || r.Author != "" || r.Rating != 0 {
		t.Errorf("expected empty review, got %+v", r)
	}
}

// ============================================================
// looksLikeProviderEntry / looksLikePriceList
// ============================================================

func TestLooksLikeProviderEntry_Valid(t *testing.T) {
	entry := []any{"Booking.com", 189.0, "USD"}
	if !looksLikeProviderEntry(entry) {
		t.Error("expected true for valid provider entry")
	}
}

func TestLooksLikeProviderEntry_TooShort(t *testing.T) {
	entry := []any{"x"}
	if looksLikeProviderEntry(entry) {
		t.Error("expected false for too-short entry")
	}
}

func TestLooksLikeProviderEntry_NoPrice(t *testing.T) {
	entry := []any{"Booking.com", "not a price"}
	if looksLikeProviderEntry(entry) {
		t.Error("expected false for entry without numeric price")
	}
}

func TestLooksLikePriceList_Valid(t *testing.T) {
	list := []any{
		[]any{"Booking.com", 189.0},
		[]any{"Expedia", 195.0},
	}
	if !looksLikePriceList(list) {
		t.Error("expected true for valid price list")
	}
}

func TestLooksLikePriceList_Empty(t *testing.T) {
	if looksLikePriceList(nil) {
		t.Error("expected false for nil")
	}
}
