package asc

import (
	"fmt"
	"strconv"
)

// CustomerReviewDigestResult summarizes customer reviews created after a cursor.
type CustomerReviewDigestResult struct {
	AppID         string                       `json:"appId"`
	Since         string                       `json:"since"`
	CursorSource  string                       `json:"cursorSource"`
	Cursor        string                       `json:"cursor"`
	Total         int                          `json:"total"`
	AverageRating float64                      `json:"averageRating"`
	Unanswered    int                          `json:"unanswered"`
	Ratings       []CustomerReviewDigestRating `json:"ratings"`
	Territories   []CustomerReviewDigestGroup  `json:"territories"`
	LowestRated   []CustomerReviewDigestReview `json:"lowestRated"`
}

// CustomerReviewDigestRating counts reviews with one star rating.
type CustomerReviewDigestRating struct {
	Rating int `json:"rating"`
	Count  int `json:"count"`
}

// CustomerReviewDigestGroup aggregates reviews for one territory.
type CustomerReviewDigestGroup struct {
	Territory     string  `json:"territory"`
	Count         int     `json:"count"`
	AverageRating float64 `json:"averageRating"`
	Unanswered    int     `json:"unanswered"`
}

// CustomerReviewDigestReview is one review listed in the digest.
type CustomerReviewDigestReview struct {
	ID          string `json:"id"`
	Rating      int    `json:"rating"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Territory   string `json:"territory"`
	CreatedDate string `json:"createdDate"`
	Answered    bool   `json:"answered"`
}

func customerReviewDigestTables(result *CustomerReviewDigestResult, render func([]string, [][]string)) error {
	render([]string{"Field", "Value"}, [][]string{
		{"App", SanitizeTerminalText(result.AppID)},
		{"Since", SanitizeTerminalText(result.Since)},
		{"Cursor Source", SanitizeTerminalText(result.CursorSource)},
		{"Cursor", SanitizeTerminalText(result.Cursor)},
		{"Reviews", strconv.Itoa(result.Total)},
		{"Average Rating", fmt.Sprintf("%.2f", result.AverageRating)},
		{"Unanswered", strconv.Itoa(result.Unanswered)},
	})

	ratingRows := make([][]string, 0, len(result.Ratings))
	for _, rating := range result.Ratings {
		ratingRows = append(ratingRows, []string{strconv.Itoa(rating.Rating), strconv.Itoa(rating.Count)})
	}
	render([]string{"Rating", "Count"}, ratingRows)

	territoryRows := make([][]string, 0, len(result.Territories))
	for _, group := range result.Territories {
		territoryRows = append(territoryRows, []string{
			SanitizeTerminalText(group.Territory),
			strconv.Itoa(group.Count),
			fmt.Sprintf("%.2f", group.AverageRating),
			strconv.Itoa(group.Unanswered),
		})
	}
	render([]string{"Territory", "Count", "Average Rating", "Unanswered"}, territoryRows)

	reviewRows := make([][]string, 0, len(result.LowestRated))
	for _, review := range result.LowestRated {
		reviewRows = append(reviewRows, []string{
			SanitizeTerminalText(review.CreatedDate),
			strconv.Itoa(review.Rating),
			SanitizeTerminalText(review.Territory),
			strconv.FormatBool(review.Answered),
			SanitizeTerminalText(review.ID),
			compactWhitespace(review.Title),
		})
	}
	render([]string{"Created", "Rating", "Territory", "Answered", "ID", "Title"}, reviewRows)
	return nil
}
