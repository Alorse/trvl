package hotels

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// parseReviewsFromBatchResponse extracts reviews from a decoded batchexecute
// response for the ocp93e rpcid.
func parseReviewsFromBatchResponse(entries []any, hotelID string, opts ReviewOptions) (*models.HotelReviewResult, error) {
	payload, err := extractBatchPayload(entries, "ocp93e")
	if err != nil {
		// Try raw extraction as fallback.
		return parseReviewsFromRawEntries(entries, hotelID, opts)
	}

	reviews := findReviewEntries(payload, 0)
	if len(reviews) == 0 {
		return nil, fmt.Errorf("no reviews found in ocp93e response")
	}

	summary := extractReviewSummary(payload)
	name := extractHotelName(payload)

	sortReviews(reviews, opts.Sort)
	if len(reviews) > opts.Limit {
		reviews = reviews[:opts.Limit]
	}

	return &models.HotelReviewResult{
		Success: true,
		HotelID: hotelID,
		Name:    name,
		Summary: summary,
		Reviews: reviews,
		Count:   len(reviews),
	}, nil
}

// parseReviewsFromRawEntries tries to extract reviews from raw batch entries.
func parseReviewsFromRawEntries(entries []any, hotelID string, opts ReviewOptions) (*models.HotelReviewResult, error) {
	var allReviews []models.HotelReview
	for _, entry := range entries {
		found := findReviewEntries(entry, 0)
		allReviews = append(allReviews, found...)
	}

	if len(allReviews) == 0 {
		return nil, fmt.Errorf("no reviews found in raw entries")
	}

	sortReviews(allReviews, opts.Sort)
	if len(allReviews) > opts.Limit {
		allReviews = allReviews[:opts.Limit]
	}

	return &models.HotelReviewResult{
		Success: true,
		HotelID: hotelID,
		Reviews: allReviews,
		Count:   len(allReviews),
	}, nil
}

// parseReviewsFromPage extracts reviews from a hotel entity page's
// AF_initDataCallback blocks.
func parseReviewsFromPage(page string, hotelID string, opts ReviewOptions) (*models.HotelReviewResult, error) {
	callbacks := extractCallbacks(page)
	if len(callbacks) == 0 {
		return nil, fmt.Errorf("no AF_initDataCallback blocks found in entity page")
	}

	var allReviews []models.HotelReview
	var summary models.ReviewSummary
	var hotelName string
	var summaryFound bool

	for _, cb := range callbacks {
		// Try to find review data in each callback.
		reviews := findReviewEntries(cb, 0)
		allReviews = append(allReviews, reviews...)

		// Extract summary if not found yet.
		if !summaryFound {
			s := extractReviewSummary(cb)
			if s.TotalReviews > 0 || s.AverageRating > 0 {
				summary = s
				summaryFound = true
			}
		}

		// Extract hotel name if not found yet.
		if hotelName == "" {
			hotelName = extractHotelName(cb)
		}
	}

	// If no reviews found via structured parsing, try text-based extraction.
	if len(allReviews) == 0 {
		allReviews = extractReviewsFromText(page)
	}

	if len(allReviews) == 0 && !summaryFound {
		return nil, fmt.Errorf("no review data found in entity page")
	}

	sortReviews(allReviews, opts.Sort)
	if opts.Limit > 0 && len(allReviews) > opts.Limit {
		allReviews = allReviews[:opts.Limit]
	}

	// If we found reviews but no summary, compute one.
	if !summaryFound && len(allReviews) > 0 {
		summary = computeSummary(allReviews)
	}

	return &models.HotelReviewResult{
		Success: true,
		HotelID: hotelID,
		Name:    hotelName,
		Summary: summary,
		Reviews: allReviews,
		Count:   len(allReviews),
	}, nil
}

// findReviewEntries recursively searches a nested JSON structure for arrays
// that look like review entries. A review entry typically has a text body,
// an author name, a numeric rating, and a date string.
func findReviewEntries(v any, depth int) []models.HotelReview {
	if depth > 12 {
		return nil
	}

	switch val := v.(type) {
	case []any:
		// Check if this array looks like a list of review entries.
		if reviews := tryParseReviewList(val); len(reviews) > 0 {
			return reviews
		}

		// Check if this single entry is a review.
		if r, ok := tryParseOneReview(val); ok {
			return []models.HotelReview{r}
		}

		// Recurse into sub-arrays.
		var results []models.HotelReview
		for _, item := range val {
			found := findReviewEntries(item, depth+1)
			if len(found) > 0 {
				results = append(results, found...)
				// If we found a batch of reviews, prefer that over scattered singles.
				if len(found) >= 3 {
					return results
				}
			}
		}
		return results

	case map[string]any:
		var results []models.HotelReview
		for _, mv := range val {
			found := findReviewEntries(mv, depth+1)
			results = append(results, found...)
		}
		return results
	}

	return nil
}

// tryParseReviewList checks if an array contains multiple review-like entries.
func tryParseReviewList(arr []any) []models.HotelReview {
	if len(arr) < 2 {
		return nil
	}

	var reviews []models.HotelReview
	for _, item := range arr {
		innerArr, ok := item.([]any)
		if !ok {
			continue
		}
		if r, ok := tryParseOneReview(innerArr); ok {
			reviews = append(reviews, r)
		}
	}

	// Only return if we found a meaningful batch.
	if len(reviews) >= 2 {
		return reviews
	}
	return nil
}

// tryParseOneReview attempts to extract review fields from an array.
//
// Google's review entries typically have this pattern:
//   - A string field with review text (>20 chars)
//   - A string field with an author name (shorter)
//   - A numeric field between 1-5 (rating)
//   - A string field matching date-like patterns
//
// The exact indices vary, so we use heuristics.
func tryParseOneReview(arr []any) (models.HotelReview, bool) {
	if len(arr) < 3 {
		return models.HotelReview{}, false
	}

	var r models.HotelReview
	var hasText, hasRating bool

	for _, v := range arr {
		switch val := v.(type) {
		case string:
			if r.Text == "" && len(val) > 20 && !isDateString(val) && !isURL(val) {
				r.Text = val
				hasText = true
			} else if r.Author == "" && len(val) > 1 && len(val) < 50 && !isDateString(val) && !isURL(val) && !hasText {
				// Author typically appears before the text.
				r.Author = val
			} else if r.Author == "" && len(val) > 1 && len(val) < 50 && !isDateString(val) && !isURL(val) && r.Text != "" && val != r.Text {
				r.Author = val
			} else if isDateString(val) && r.Date == "" {
				r.Date = val
			}
		case float64:
			if val >= 1 && val <= 5 && !hasRating {
				r.Rating = val
				hasRating = true
			}
		case []any:
			// Rating might be nested: [rating_value, ...]
			if !hasRating && len(val) > 0 {
				if f, ok := val[0].(float64); ok && f >= 1 && f <= 5 {
					r.Rating = f
					hasRating = true
				}
			}
			// Author or date might be in sub-arrays.
			if r.Author == "" {
				for _, sub := range val {
					if s, ok := sub.(string); ok && len(s) > 1 && len(s) < 50 && !isURL(s) {
						if isDateString(s) {
							if r.Date == "" {
								r.Date = s
							}
						} else if r.Author == "" {
							r.Author = s
						}
					}
				}
			}
		}
	}

	// A valid review must have text and a rating.
	if hasText && hasRating {
		return r, true
	}

	return models.HotelReview{}, false
}

// extractReviewSummary searches for aggregate review statistics (average rating,
// total count, rating breakdown) in the parsed data.
func extractReviewSummary(v any) models.ReviewSummary {
	var s models.ReviewSummary
	findSummaryData(v, &s, 0)
	return s
}

// findSummaryData recursively searches for summary-like data patterns.
func findSummaryData(v any, s *models.ReviewSummary, depth int) {
	if depth > 10 {
		return
	}

	switch val := v.(type) {
	case []any:
		// Pattern: [rating_float, review_count_float] where rating is 1-5.
		if len(val) >= 2 {
			if rating, ok := val[0].(float64); ok && rating >= 1 && rating <= 5 {
				if count, ok := val[1].(float64); ok && count > 10 {
					if s.AverageRating == 0 || count > float64(s.TotalReviews) {
						s.AverageRating = math.Round(rating*10) / 10
						s.TotalReviews = int(count)
					}
				}
			}
		}

		// Pattern: rating breakdown as [1star, 2star, 3star, 4star, 5star].
		if len(val) == 5 && s.RatingBreakdown == nil {
			allInts := true
			total := 0
			for _, item := range val {
				if f, ok := item.(float64); ok && f >= 0 && f < 100000 {
					total += int(f)
				} else {
					allInts = false
					break
				}
			}
			if allInts && total > 10 {
				s.RatingBreakdown = map[string]int{
					"1": int(val[0].(float64)),
					"2": int(val[1].(float64)),
					"3": int(val[2].(float64)),
					"4": int(val[3].(float64)),
					"5": int(val[4].(float64)),
				}
			}
		}

		for _, item := range val {
			findSummaryData(item, s, depth+1)
		}

	case map[string]any:
		for _, mv := range val {
			findSummaryData(mv, s, depth+1)
		}
	}
}

// extractHotelName searches the parsed data for the hotel name.
func extractHotelName(v any) string {
	return findHotelNameInData(v, 0)
}

func findHotelNameInData(v any, depth int) string {
	if depth > 6 {
		return ""
	}

	switch val := v.(type) {
	case []any:
		// Hotel name is often at index [1] in the top-level data.
		if len(val) > 1 {
			if name, ok := val[1].(string); ok && len(name) > 2 && len(name) < 100 && !isURL(name) {
				return name
			}
		}
		for _, item := range val {
			if name := findHotelNameInData(item, depth+1); name != "" {
				return name
			}
		}
	case map[string]any:
		for _, mv := range val {
			if name := findHotelNameInData(mv, depth+1); name != "" {
				return name
			}
		}
	}
	return ""
}

// extractReviewsFromText is a last-resort parser that looks for review patterns
// in the raw page HTML/text.
func extractReviewsFromText(page string) []models.HotelReview {
	var reviews []models.HotelReview

	// Look for JSON-like review structures in the page.
	// Google sometimes embeds review data in script tags.
	idx := 0
	for idx < len(page) {
		// Look for patterns like "rated it" or star rating indicators.
		rIdx := strings.Index(page[idx:], `"reviewRating"`)
		if rIdx < 0 {
			break
		}
		idx += rIdx

		// Try to extract a JSON block around this point.
		start := strings.LastIndex(page[:idx], "{")
		if start < 0 || idx-start > 2000 {
			idx += 15
			continue
		}

		// Try parsing the JSON block.
		chunk := page[start:]
		dec := json.NewDecoder(strings.NewReader(chunk))
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil {
			idx += 15
			continue
		}

		r := extractReviewFromJSON(obj)
		if r.Text != "" {
			reviews = append(reviews, r)
		}
		idx += 15
	}

	return reviews
}

// extractReviewFromJSON extracts review fields from a JSON-LD or schema.org review object.
func extractReviewFromJSON(obj map[string]any) models.HotelReview {
	var r models.HotelReview

	if body, ok := obj["reviewBody"].(string); ok {
		r.Text = body
	}

	if author, ok := obj["author"].(map[string]any); ok {
		if name, ok := author["name"].(string); ok {
			r.Author = name
		}
	} else if author, ok := obj["author"].(string); ok {
		r.Author = author
	}

	if rating, ok := obj["reviewRating"].(map[string]any); ok {
		if val, ok := rating["ratingValue"].(string); ok {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				r.Rating = f
			}
		} else if val, ok := rating["ratingValue"].(float64); ok {
			r.Rating = val
		}
	}

	if date, ok := obj["datePublished"].(string); ok {
		r.Date = date
	}

	return r
}

// computeSummary computes a ReviewSummary from a slice of reviews.
func computeSummary(reviews []models.HotelReview) models.ReviewSummary {
	if len(reviews) == 0 {
		return models.ReviewSummary{}
	}

	var total float64
	breakdown := map[string]int{}
	for _, r := range reviews {
		total += r.Rating
		star := fmt.Sprintf("%d", int(r.Rating))
		breakdown[star]++
	}

	return models.ReviewSummary{
		AverageRating:   math.Round(total/float64(len(reviews))*10) / 10,
		TotalReviews:    len(reviews),
		RatingBreakdown: breakdown,
	}
}

// isDateString checks if a string looks like a date or relative time expression.
// It uses word-boundary matching to avoid false positives from substrings
// (e.g., "decent" should not match "dec").
func isDateString(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}

	// YYYY-MM-DD format.
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return true
	}

	// Short strings are more likely to be dates (e.g., "2 weeks ago", "March 2026").
	// Long strings (>50 chars) are almost never date strings.
	if len(s) > 50 {
		return false
	}

	lower := strings.ToLower(s)
	words := strings.Fields(lower)

	// Relative time patterns: "X ago", "X weeks", "X months"
	relativeTime := []string{"ago", "weeks", "months", "years", "days", "hours"}
	for _, w := range words {
		for _, rt := range relativeTime {
			if w == rt {
				return true
			}
		}
	}

	// Month names (full and abbreviated) as whole words.
	monthNames := []string{
		"january", "february", "march", "april", "may", "june",
		"july", "august", "september", "october", "november", "december",
		"jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "oct", "nov", "dec",
	}
	for _, w := range words {
		// Strip trailing punctuation for matching.
		cleaned := strings.TrimRight(w, ".,;:")
		for _, month := range monthNames {
			if cleaned == month {
				return true
			}
		}
	}

	// "week" or "month" as standalone words (e.g., "1 week ago", "a month ago").
	standalone := []string{"week", "month", "year", "day", "hour"}
	for _, w := range words {
		cleaned := strings.TrimRight(w, ".,;:")
		for _, st := range standalone {
			if cleaned == st {
				return true
			}
		}
	}

	return false
}

// isURL checks if a string looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "/")
}
