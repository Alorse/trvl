package hotels

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// ReviewOptions configures a hotel reviews lookup.
type ReviewOptions struct {
	Limit int    // Maximum number of reviews to return (default: 10)
	Sort  string // "newest", "highest", "lowest" (default: "newest")
}

// GetHotelReviews fetches guest reviews for a specific hotel.
//
// Strategy: Try batchexecute with rpcid ocp93e first. If that returns no
// reviews, fall back to scraping the hotel entity page for review data
// embedded in AF_initDataCallback blocks.
func GetHotelReviews(ctx context.Context, hotelID string, opts ReviewOptions) (*models.HotelReviewResult, error) {
	if hotelID == "" {
		return nil, fmt.Errorf("hotel ID is required")
	}
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	if opts.Sort == "" {
		opts.Sort = "newest"
	}

	client := DefaultClient()

	// Try batchexecute first.
	result, err := getReviewsViaBatchExecute(ctx, client, hotelID, opts)
	if err == nil && result != nil && len(result.Reviews) > 0 {
		return result, nil
	}

	// Fall back to page scraping.
	result, err = getReviewsViaPageScrape(ctx, client, hotelID, opts)
	if err != nil {
		return nil, fmt.Errorf("hotel reviews: %w", err)
	}

	return result, nil
}

// getReviewsViaBatchExecute tries the ocp93e rpcid for hotel reviews.
func getReviewsViaBatchExecute(ctx context.Context, client *batchexec.Client, hotelID string, opts ReviewOptions) (*models.HotelReviewResult, error) {
	encoded := batchexec.BuildHotelReviewPayload(hotelID, opts.Limit)

	status, body, err := client.BatchExecute(ctx, encoded)
	if err != nil {
		return nil, fmt.Errorf("review batchexecute request: %w", err)
	}
	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("review batchexecute returned status %d", status)
	}
	if len(body) < 50 {
		return nil, fmt.Errorf("review batchexecute returned empty response")
	}

	entries, err := batchexec.DecodeBatchResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode review response: %w", err)
	}

	return parseReviewsFromBatchResponse(entries, hotelID, opts)
}

// getReviewsViaPageScrape fetches the hotel entity page and extracts reviews
// from AF_initDataCallback blocks.
func getReviewsViaPageScrape(ctx context.Context, client *batchexec.Client, hotelID string, opts ReviewOptions) (*models.HotelReviewResult, error) {
	entityURL := fmt.Sprintf("https://www.google.com/travel/hotels/entity/%s?hl=en-US&gl=us", hotelID)

	status, body, err := client.Get(ctx, entityURL)
	if err != nil {
		return nil, fmt.Errorf("entity page request: %w", err)
	}
	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("entity page returned status %d", status)
	}
	if len(body) < 500 {
		return nil, fmt.Errorf("entity page returned empty response")
	}

	return parseReviewsFromPage(string(body), hotelID, opts)
}

// sortReviews sorts reviews in-place by the given criteria.
func sortReviews(reviews []models.HotelReview, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "highest":
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].Rating > reviews[j].Rating
		})
	case "lowest":
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].Rating < reviews[j].Rating
		})
	case "newest":
		// Reviews from page scraping are typically already in newest-first order.
		// No additional sort needed.
	}
}
