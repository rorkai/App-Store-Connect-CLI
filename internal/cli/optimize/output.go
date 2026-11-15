package optimize

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func renderSearchPlanTable(d *shared.Deps, report SearchPlanReport) error {
	d.RenderSection("Search Optimization Plan", []string{"Field", "Value"}, searchPlanCompactSummaryRows(report), false)
	d.RenderSection("Search Plan", searchPlanCompactHeaders(), searchPlanCompactRows(report.Rows), false)

	sourceRows := make([][]string, 0, len(report.Sources))
	for _, source := range report.Sources {
		sourceRows = append(sourceRows, []string{
			formatSearchPlanSourceName(source.Name),
			source.Status,
			strconv.Itoa(source.Count),
		})
	}
	d.RenderSection("Sources", []string{"Source", "Status", "Rows"}, sourceRows, false)

	if len(report.Notices) > 0 {
		noticeRows := make([][]string, 0, len(report.Notices))
		for index, notice := range report.Notices {
			noticeRows = append(noticeRows, []string{strconv.Itoa(index + 1), compactSearchPlanDiagnostic(notice)})
		}
		d.RenderSection("Notices", []string{"#", "Notice"}, noticeRows, false)
	}

	if len(report.Artifacts) > 0 {
		artifactRows := make([][]string, 0, len(report.Artifacts))
		for _, artifact := range report.Artifacts {
			artifactRows = append(artifactRows, []string{artifact})
		}
		d.RenderSection("Artifacts", []string{"Path"}, artifactRows, false)
	}
	return nil
}

func renderSearchPlanMarkdown(d *shared.Deps, report SearchPlanReport) error {
	renderSearchPlanHuman(d, report, true)
	return nil
}

func searchPlanCompactSummaryRows(report SearchPlanReport) [][]string {
	appName := strings.TrimSpace(report.Metadata.Name)
	if appName == "" {
		appName = report.AppID
	}
	return [][]string{
		{"App", appName},
		{"Version", strings.TrimSpace(report.Version) + " · " + formatSearchPlanPlatform(report.Platform)},
		{"Market", strings.TrimSpace(report.Country) + " · " + strings.TrimSpace(report.Locale)},
		{"Genre", formatSearchPlanSourceName(report.Genre)},
		{"Paid Window", formatSearchPlanWindow(report.Window.Start, report.Window.End)},
		{"Popularity Window", formatSearchPlanWindow(report.Window.PopularityStart, report.Window.PopularityEnd)},
		{"Terms", strconv.Itoa(report.Summary.Terms)},
		{"Source Coverage", fmt.Sprintf("%d available · %d empty · %d unavailable", report.Summary.AvailableSources, report.Summary.EmptySources, report.Summary.UnavailableSources)},
	}
}

func searchPlanCompactHeaders() []string {
	return []string{"Term", "Popularity", "Genre Rank", "Share", "Installs", "CPA", "Next Step", "Confidence"}
}

func searchPlanCompactRows(rows []SearchPlanRow) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, []string{
			row.Term,
			formatSearchPlanCompactPopularity(row),
			formatOptionalInt(row.RankInGenre),
			formatSearchPlanShare(row.ImpressionShareLow, row.ImpressionShareHigh),
			formatOptionalInt64(row.TotalInstalls),
			formatSearchPlanMoney(row.CPA),
			formatSearchPlanCompactActions(row.Actions),
			row.Confidence,
		})
	}
	return result
}

func formatSearchPlanCompactPopularity(row SearchPlanRow) string {
	popularity5 := row.Popularity5
	if popularity5 == nil {
		popularity5 = row.ImpressionSharePopularity5
	}
	if popularity5 == nil && row.Popularity100 == nil {
		if row.SuggestionPopularity != nil {
			return "suggested " + strconv.Itoa(*row.SuggestionPopularity)
		}
		return "—"
	}
	return formatOptionalInt(popularity5) + " / " + formatOptionalInt(row.Popularity100)
}

func formatSearchPlanCompactActions(actions []string) string {
	labels := make([]string, 0, len(actions))
	for _, action := range actions {
		label := map[string]string{
			"promote_exact":      "promote exact",
			"negative_candidate": "add negative",
			"metadata_candidate": "metadata",
			"defend":             "defend",
			"saturated":          "saturated",
			"untested_candidate": "test",
		}[action]
		if label == "" {
			label = strings.ReplaceAll(action, "_", " ")
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return "—"
	}
	return strings.Join(labels, " · ")
}

func formatSearchPlanPlatform(platform string) string {
	switch strings.ToUpper(strings.TrimSpace(platform)) {
	case "IOS":
		return "iOS"
	case "MAC_OS", "MACOS":
		return "macOS"
	case "TV_OS", "TVOS":
		return "tvOS"
	case "VISION_OS", "VISIONOS":
		return "visionOS"
	default:
		return strings.TrimSpace(platform)
	}
}

func formatSearchPlanSourceName(value string) string {
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "_", " "))
	for index, word := range words {
		if word == "cpa" {
			words[index] = "CPA"
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func compactSearchPlanDiagnostic(value string) string {
	compact := strings.TrimSpace(value)
	lower := strings.ToLower(compact)
	htmlIndex := len(compact)
	for _, marker := range []string{"<!doctype html", "<html", "<head", "<body"} {
		if index := strings.Index(lower, marker); index >= 0 && index < htmlIndex {
			htmlIndex = index
		}
	}
	if htmlIndex < len(compact) {
		compact = strings.TrimSpace(compact[:htmlIndex])
	}
	compact = strings.TrimSpace(strings.TrimSuffix(compact, ":"))
	compact = strings.Join(strings.Fields(compact), " ")
	const maxRunes = 72
	runes := []rune(compact)
	if len(runes) > maxRunes {
		compact = string(runes[:maxRunes-1]) + "…"
	}
	return compact
}

func searchPlanHeaders() []string {
	return []string{"Term", "Popularity 1-5", "Popularity 1-100", "Genre Rank", "Share", "Installs", "CPA", "Actions", "Confidence", "Sources"}
}

func searchPlanRows(rows []SearchPlanRow) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, []string{
			row.Term,
			formatSearchPlanPopularity5(row),
			formatOptionalInt(row.Popularity100),
			formatOptionalInt(row.RankInGenre),
			formatSearchPlanShare(row.ImpressionShareLow, row.ImpressionShareHigh),
			formatOptionalInt64(row.TotalInstalls),
			formatSearchPlanMoney(row.CPA),
			strings.Join(row.Actions, ","),
			row.Confidence,
			strings.Join(row.Sources, ","),
		})
	}
	return result
}

func renderSearchPlanHuman(d *shared.Deps, report SearchPlanReport, markdown bool) {
	d.RenderSection("Summary", []string{"Field", "Value"}, [][]string{
		{"App", report.AppID},
		{"Version", report.Version},
		{"Platform", report.Platform},
		{"Store", report.Country},
		{"Genre", report.Genre},
		{"Locale", report.Locale},
		{"Paid Window", formatSearchPlanWindow(report.Window.Start, report.Window.End)},
		{"Popularity Window", formatSearchPlanWindow(report.Window.PopularityStart, report.Window.PopularityEnd)},
		{"Terms", strconv.Itoa(report.Summary.Terms)},
		{"Daily Budget Recommendations", strconv.Itoa(report.Summary.DailyBudgetRecommendations)},
		{"Target CPA Recommendations", strconv.Itoa(report.Summary.TargetCPARecommendations)},
	}, markdown)

	sourceRows := make([][]string, 0, len(report.Sources))
	for _, source := range report.Sources {
		sourceRows = append(sourceRows, []string{source.Name, source.Status, strconv.Itoa(source.Count), source.Error})
	}
	d.RenderSection("Sources", []string{"Source", "Status", "Count", "Error"}, sourceRows, markdown)

	if len(report.Notices) > 0 {
		noticeRows := make([][]string, 0, len(report.Notices))
		for index, notice := range report.Notices {
			noticeRows = append(noticeRows, []string{strconv.Itoa(index + 1), notice})
		}
		d.RenderSection("Notices", []string{"#", "Notice"}, noticeRows, markdown)
	}

	d.RenderSection("Search Plan", searchPlanHeaders(), searchPlanRows(report.Rows), markdown)

	if len(report.Artifacts) > 0 {
		artifactRows := make([][]string, 0, len(report.Artifacts))
		for _, artifact := range report.Artifacts {
			artifactRows = append(artifactRows, []string{artifact})
		}
		d.RenderSection("Artifacts", []string{"Path"}, artifactRows, markdown)
	}
}

func formatSearchPlanPopularity5(row SearchPlanRow) string {
	if row.Popularity5 != nil {
		return strconv.Itoa(*row.Popularity5)
	}
	return formatOptionalInt(row.ImpressionSharePopularity5)
}

func formatOptionalInt(value *int) string {
	if value == nil {
		return "—"
	}
	return strconv.Itoa(*value)
}

func formatOptionalInt64(value *int64) string {
	if value == nil {
		return "—"
	}
	return strconv.FormatInt(*value, 10)
}

func formatSearchPlanShare(low, high *float64) string {
	if low == nil || high == nil {
		return "—"
	}
	if *low == *high {
		return strconv.FormatFloat(*low*100, 'f', 0, 64) + "%"
	}
	return strconv.FormatFloat(*low*100, 'f', 0, 64) + "–" + strconv.FormatFloat(*high*100, 'f', 0, 64) + "%"
}

func formatSearchPlanMoney(value *SearchPlanMoney) string {
	if value == nil {
		return "—"
	}
	return value.Amount + " " + value.Currency
}

func formatSearchPlanWindow(start, end string) string {
	if start == "" && end == "" {
		return "—"
	}
	return fmt.Sprintf("%s through %s", shared.OrNA(start), shared.OrNA(end))
}
