package asc

import (
	"fmt"
	"strings"
)

func feedbackHasScreenshots(resp *FeedbackResponse) bool {
	for _, item := range resp.Data {
		if len(item.Attributes.Screenshots) > 0 {
			return true
		}
	}
	return false
}

func formatScreenshotURLs(images []FeedbackScreenshotImage) string {
	if len(images) == 0 {
		return ""
	}
	urls := make([]string, 0, len(images))
	for _, image := range images {
		if strings.TrimSpace(image.URL) == "" {
			continue
		}
		urls = append(urls, image.URL)
	}
	return strings.Join(urls, ", ")
}

func feedbackRows(resp *FeedbackResponse) ([]string, [][]string) {
	hasScreenshots := feedbackHasScreenshots(resp)
	var headers []string
	if hasScreenshots {
		headers = []string{"Created", "Email", "Comment", "Screenshots"}
	} else {
		headers = []string{"Created", "Email", "Comment"}
	}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		if hasScreenshots {
			rows = append(rows, []string{
				sanitizeTerminal(item.Attributes.CreatedDate),
				sanitizeTerminal(item.Attributes.Email),
				compactWhitespace(item.Attributes.Comment),
				sanitizeTerminal(formatScreenshotURLs(item.Attributes.Screenshots)),
			})
			continue
		}
		rows = append(rows, []string{
			sanitizeTerminal(item.Attributes.CreatedDate),
			sanitizeTerminal(item.Attributes.Email),
			compactWhitespace(item.Attributes.Comment),
		})
	}
	return headers, rows
}

func printFeedbackTable(resp *FeedbackResponse) error {
	h, r := feedbackRows(resp)
	RenderTable(h, r)
	return nil
}

func printFeedbackMarkdown(resp *FeedbackResponse) error {
	h, r := feedbackRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func crashesRows(resp *CrashesResponse) ([]string, [][]string) {
	headers := []string{"Created", "Email", "Device", "OS", "Comment"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			sanitizeTerminal(item.Attributes.CreatedDate),
			sanitizeTerminal(item.Attributes.Email),
			sanitizeTerminal(item.Attributes.DeviceModel),
			sanitizeTerminal(item.Attributes.OSVersion),
			compactWhitespace(item.Attributes.Comment),
		})
	}
	return headers, rows
}

func printCrashesTable(resp *CrashesResponse) error {
	h, r := crashesRows(resp)
	RenderTable(h, r)
	return nil
}

func printCrashesMarkdown(resp *CrashesResponse) error {
	h, r := crashesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func reviewsRows(resp *ReviewsResponse) ([]string, [][]string) {
	headers := []string{"Created", "Rating", "Territory", "Title"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			sanitizeTerminal(item.Attributes.CreatedDate),
			fmt.Sprintf("%d", item.Attributes.Rating),
			sanitizeTerminal(item.Attributes.Territory),
			compactWhitespace(item.Attributes.Title),
		})
	}
	return headers, rows
}

func printReviewsTable(resp *ReviewsResponse) error {
	h, r := reviewsRows(resp)
	RenderTable(h, r)
	return nil
}

func printReviewsMarkdown(resp *ReviewsResponse) error {
	h, r := reviewsRows(resp)
	RenderMarkdown(h, r)
	return nil
}
