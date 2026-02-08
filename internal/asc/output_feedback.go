package asc

import (
	"fmt"
	"os"
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

func printFeedbackTable(resp *FeedbackResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printCrashesTable(resp *CrashesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printReviewsTable(resp *ReviewsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printFeedbackMarkdown(resp *FeedbackResponse) error {
	hasScreenshots := feedbackHasScreenshots(resp)
	if hasScreenshots {
		fmt.Fprintln(os.Stdout, "| Created | Email | Comment | Screenshots |")
		fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	} else {
		fmt.Fprintln(os.Stdout, "| Created | Email | Comment |")
		fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	}
	for _, item := range resp.Data {
		if hasScreenshots {
			fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
				escapeMarkdown(item.Attributes.CreatedDate),
				escapeMarkdown(item.Attributes.Email),
				escapeMarkdown(item.Attributes.Comment),
				escapeMarkdown(formatScreenshotURLs(item.Attributes.Screenshots)),
			)
			continue
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.Attributes.CreatedDate),
			escapeMarkdown(item.Attributes.Email),
			escapeMarkdown(item.Attributes.Comment),
		)
	}
	return nil
}

func printCrashesMarkdown(resp *CrashesResponse) error {
	fmt.Fprintln(os.Stdout, "| Created | Email | Device | OS | Comment |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.Attributes.CreatedDate),
			escapeMarkdown(item.Attributes.Email),
			escapeMarkdown(item.Attributes.DeviceModel),
			escapeMarkdown(item.Attributes.OSVersion),
			escapeMarkdown(item.Attributes.Comment),
		)
	}
	return nil
}

func printReviewsMarkdown(resp *ReviewsResponse) error {
	fmt.Fprintln(os.Stdout, "| Created | Rating | Territory | Title |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s |\n",
			escapeMarkdown(item.Attributes.CreatedDate),
			item.Attributes.Rating,
			escapeMarkdown(item.Attributes.Territory),
			escapeMarkdown(item.Attributes.Title),
		)
	}
	return nil
}
