package docs

import (
	"strings"

	docsembed "github.com/rudrankriyam/App-Store-Connect-CLI/docs"
)

type guideEntry struct {
	Slug        string
	Description string
	Content     string
}

type guideSummary struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

var guideRegistry = []guideEntry{
	{
		Slug:        "api-notes",
		Description: "API quirks: date formats, finance reports, sandbox testers",
		Content:     docsembed.APINotesGuide,
	},
	{
		Slug:        "reference",
		Description: "Repo-local ASC quick reference (also available via 'asc init')",
		Content:     ascTemplate,
	},
	{
		Slug:        "workflows",
		Description: "Task-based workflow recipes for release, TestFlight, and Xcode flows",
		Content:     docsembed.WorkflowsGuide,
	},
}

func listGuideSummaries() []guideSummary {
	out := make([]guideSummary, 0, len(guideRegistry))
	for _, guide := range guideRegistry {
		out = append(out, guideSummary{
			Slug:        guide.Slug,
			Description: guide.Description,
		})
	}
	return out
}

func guideRows() [][]string {
	rows := make([][]string, 0, len(guideRegistry))
	for _, guide := range guideRegistry {
		rows = append(rows, []string{guide.Slug, guide.Description})
	}
	return rows
}

func guideSlugs() []string {
	slugs := make([]string, 0, len(guideRegistry))
	for _, guide := range guideRegistry {
		slugs = append(slugs, guide.Slug)
	}
	return slugs
}

func findGuide(slug string) (guideEntry, bool) {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	for _, guide := range guideRegistry {
		if guide.Slug == normalized {
			return guide, true
		}
	}
	return guideEntry{}, false
}
