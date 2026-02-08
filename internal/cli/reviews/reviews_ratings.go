package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

// ReviewsRatingsCommand returns the reviews ratings subcommand.
func ReviewsRatingsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ratings", flag.ExitOnError)

	appID := fs.String("app", "", "App Store app ID (required)")
	country := fs.String("country", "us", "Country code (e.g., us, gb, de)")
	all := fs.Bool("all", false, "Fetch ratings from all countries")
	workers := fs.Int("workers", 10, "Number of parallel workers for --all")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "ratings",
		ShortUsage: "asc reviews ratings [flags]",
		ShortHelp:  "Show App Store rating statistics.",
		LongHelp: `Show App Store rating statistics using the public iTunes API.

This command fetches aggregate rating data (average rating, rating count,
histogram) that is not available through the App Store Connect API.

No authentication is required.

Examples:
  asc reviews ratings --app "1479784361"
  asc reviews ratings --app "1479784361" --country de
  asc reviews ratings --app "1479784361" --output table
  asc reviews ratings --app "1479784361" --all
  asc reviews ratings --app "1479784361" --all --workers 20`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*appID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required")
				return flag.ErrHelp
			}

			if *workers < 1 {
				fmt.Fprintln(os.Stderr, "Error: --workers must be at least 1")
				return flag.ErrHelp
			}

			return executeRatings(ctx, *appID, *country, *all, *workers, *output, *pretty)
		},
	}
}

func executeRatings(ctx context.Context, appID, country string, all bool, workers int, output string, pretty bool) error {
	format, err := normalizeRatingsOutput(output, pretty)
	if err != nil {
		return err
	}

	client := itunes.NewClient()

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if all {
		return executeAllRatings(requestCtx, client, appID, workers, format, pretty)
	}

	return executeSingleRatings(requestCtx, client, appID, country, format, pretty)
}

func executeSingleRatings(ctx context.Context, client *itunes.Client, appID, country, output string, pretty bool) error {
	ratings, err := client.GetRatings(ctx, appID, country)
	if err != nil {
		return fmt.Errorf("reviews ratings: %w", err)
	}

	switch output {
	case "table":
		return printRatingsTable(ratings)
	case "markdown":
		return printRatingsMarkdown(ratings)
	default:
		return shared.PrintOutput(ratings, "json", pretty)
	}
}

func executeAllRatings(ctx context.Context, client *itunes.Client, appID string, workers int, output string, pretty bool) error {
	global, err := client.GetAllRatings(ctx, appID, workers)
	if err != nil {
		return fmt.Errorf("reviews ratings: %w", err)
	}

	switch output {
	case "table":
		return printGlobalRatingsTable(global)
	case "markdown":
		return printGlobalRatingsMarkdown(global)
	default:
		return shared.PrintOutput(global, "json", pretty)
	}
}

func normalizeRatingsOutput(output string, pretty bool) (string, error) {
	format := strings.ToLower(strings.TrimSpace(output))
	if format == "" {
		format = "json"
	}
	if format == "md" {
		format = "markdown"
	}

	switch format {
	case "json":
		return format, nil
	case "table", "markdown":
		if pretty {
			return "", fmt.Errorf("--pretty is only valid with JSON output")
		}
		return format, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func printRatingsTable(r *itunes.AppRatings) error {
	fmt.Printf("\n%s\n", r.AppName)
	fmt.Printf("App ID: %d | Country: %s\n", r.AppID, r.Country)
	fmt.Println(strings.Repeat("─", 40))

	fmt.Printf("Rating: %.2f (%s ratings)\n", r.AverageRating, formatNumber(r.RatingCount))

	if r.CurrentVersionRating != r.AverageRating || r.CurrentVersionCount != r.RatingCount {
		fmt.Printf("Current Version: %.2f (%s ratings)\n", r.CurrentVersionRating, formatNumber(r.CurrentVersionCount))
	}

	if len(r.Histogram) > 0 {
		printHistogram(r.Histogram)
	}
	fmt.Println()
	return nil
}

func printGlobalRatingsTable(g *itunes.GlobalRatings) error {
	fmt.Printf("\n%s\n", g.AppName)
	fmt.Printf("App ID: %d\n", g.AppID)
	fmt.Println(strings.Repeat("─", 60))

	fmt.Printf("GLOBAL: %.2f avg (%s total ratings across %d countries)\n",
		g.AverageRating, formatNumber(g.TotalCount), g.CountryCount)

	if len(g.Histogram) > 0 {
		fmt.Println("\nHistogram (Global):")
		printHistogramRows(g.Histogram)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("\n%-20s %8s %8s\n", "Country", "Rating", "Count")
	fmt.Println(strings.Repeat("─", 40))

	for _, r := range g.ByCountry {
		name := r.CountryName
		if name == "" {
			name = r.Country
		}
		if len(name) > 18 {
			name = name[:18] + ".."
		}
		fmt.Printf("%-20s %8.2f %8s\n", name, r.AverageRating, formatNumber(r.RatingCount))
	}
	fmt.Println()
	return nil
}

func printRatingsMarkdown(r *itunes.AppRatings) error {
	fmt.Printf("## %s\n\n", r.AppName)
	fmt.Printf("**App ID:** %d | **Country:** %s\n\n", r.AppID, r.Country)
	fmt.Printf("**Rating:** %.2f (%s ratings)\n\n", r.AverageRating, formatNumber(r.RatingCount))

	if len(r.Histogram) > 0 {
		fmt.Println("### Histogram")
		printHistogramMarkdown(r.Histogram)
	}
	fmt.Println()
	return nil
}

func printGlobalRatingsMarkdown(g *itunes.GlobalRatings) error {
	fmt.Printf("## %s\n\n", g.AppName)
	fmt.Printf("**App ID:** %d\n\n", g.AppID)
	fmt.Printf("**Global Rating:** %.2f (%s total ratings across %d countries)\n\n",
		g.AverageRating, formatNumber(g.TotalCount), g.CountryCount)

	if len(g.Histogram) > 0 {
		fmt.Println("### Global Histogram")
		printHistogramMarkdown(g.Histogram)
	}

	fmt.Print("\n### By Country\n\n")
	{
		headers := []string{"Country", "Rating", "Count"}
		rows := make([][]string, 0, len(g.ByCountry))
		for _, r := range g.ByCountry {
			name := r.CountryName
			if name == "" {
				name = r.Country
			}
			rows = append(rows, []string{name, fmt.Sprintf("%.2f", r.AverageRating), formatNumber(r.RatingCount)})
		}
		asc.RenderMarkdown(headers, rows)
	}
	fmt.Println()
	return nil
}

func printHistogram(histogram map[int]int64) {
	fmt.Println("\nHistogram:")
	printHistogramRows(histogram)
}

func printHistogramRows(histogram map[int]int64) {
	var total int64
	for _, count := range histogram {
		total += count
	}

	headers := []string{"Stars", "Count", "Percentage", "Bar"}
	rows := make([][]string, 0, 5)
	for star := 5; star >= 1; star-- {
		count := histogram[star]
		pct := float64(0)
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		bar := strings.Repeat("█", int(pct/5))
		rows = append(rows, []string{
			fmt.Sprintf("%d★", star),
			formatNumber(count),
			fmt.Sprintf("%.1f%%", pct),
			bar,
		})
	}
	asc.RenderTable(headers, rows)
}

func printHistogramMarkdown(histogram map[int]int64) {
	var total int64
	for _, count := range histogram {
		total += count
	}
	headers := []string{"Stars", "Count", "Percentage"}
	rows := make([][]string, 0, 5)
	for star := 5; star >= 1; star-- {
		count := histogram[star]
		pct := float64(0)
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d★", star),
			formatNumber(count),
			fmt.Sprintf("%.1f%%", pct),
		})
	}
	asc.RenderMarkdown(headers, rows)
}

func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
