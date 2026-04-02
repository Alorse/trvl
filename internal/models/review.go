package models

// HotelReview represents a single guest review for a hotel.
type HotelReview struct {
	Rating float64 `json:"rating"`
	Text   string  `json:"text"`
	Author string  `json:"author"`
	Date   string  `json:"date"`
}

// ReviewSummary contains aggregate review statistics for a hotel.
type ReviewSummary struct {
	AverageRating   float64        `json:"average_rating"`
	TotalReviews    int            `json:"total_reviews"`
	RatingBreakdown map[string]int `json:"rating_breakdown,omitempty"`
}

// HotelReviewResult is the top-level response for a hotel reviews lookup.
type HotelReviewResult struct {
	Success bool          `json:"success"`
	HotelID string        `json:"hotel_id"`
	Name    string        `json:"name,omitempty"`
	Summary ReviewSummary `json:"summary"`
	Reviews []HotelReview `json:"reviews"`
	Count   int           `json:"count"`
	Error   string        `json:"error,omitempty"`
}
