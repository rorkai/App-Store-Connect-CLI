package releasenotes

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	notes "github.com/rudrankriyam/App-Store-Connect-CLI/internal/releasenotes"
)

// ReleaseNotesCommand returns the release-notes command group.
func ReleaseNotesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("release-notes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "release-notes",
		ShortUsage: "asc release-notes <subcommand> [flags]",
		ShortHelp:  "Generate and manage App Store release notes.",
		LongHelp: `Generate release notes (What's New) text from git history.

Examples:
  asc release-notes generate --since-tag "v1.2.2"
  asc release-notes generate --since-tag "v1.2.2" --until-ref "HEAD" --output markdown
  asc release-notes generate --since-ref "origin/main" --until-ref "HEAD" --max-chars 4000`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ReleaseNotesGenerateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

type releaseNotesGenerateResult struct {
	Since         string         `json:"since"`
	Until         string         `json:"until"`
	Format        string         `json:"format"`
	MaxChars      int            `json:"maxChars"`
	IncludeMerges bool           `json:"includeMerges"`
	CommitCount   int            `json:"commitCount"`
	Truncated     bool           `json:"truncated"`
	Notes         string         `json:"notes"`
	Commits       []notes.Commit `json:"commits,omitempty"`
}

// ReleaseNotesGenerateCommand returns the generate subcommand.
func ReleaseNotesGenerateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	sinceTag := fs.String("since-tag", "", "Start from tag (exclusive), e.g. v1.2.2")
	sinceRef := fs.String("since-ref", "", "Start from ref/SHA (exclusive), e.g. origin/main")
	untilRef := fs.String("until-ref", "HEAD", "End at ref/SHA (inclusive), e.g. HEAD")
	format := fs.String("format", "plain", "Notes format: plain (default), markdown")
	maxChars := fs.Int("max-chars", 4000, "Maximum characters in generated notes")
	includeMerges := fs.Bool("include-merges", false, "Include merge commits")
	output := shared.BindOutputFlagsWith(fs, "output", shared.DefaultOutputFormat(), "Output format: json, text, table, markdown")

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "asc release-notes generate [flags]",
		ShortHelp:  "Generate release notes from git history.",
		LongHelp: `Generate release notes (What's New) from local git history.

Exactly one of --since-tag or --since-ref is required.

Examples:
  asc release-notes generate --since-tag "v1.2.2"
  asc release-notes generate --since-tag "v1.2.2" --output markdown
  asc release-notes generate --since-ref "origin/main" --until-ref "HEAD" --max-chars 4000`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: unexpected arguments")
				return flag.ErrHelp
			}

			sinceTagValue := strings.TrimSpace(*sinceTag)
			sinceRefValue := strings.TrimSpace(*sinceRef)
			if sinceTagValue != "" && sinceRefValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --since-tag and --since-ref are mutually exclusive")
				return flag.ErrHelp
			}
			if sinceTagValue == "" && sinceRefValue == "" {
				fmt.Fprintln(os.Stderr, "Error: one of --since-tag or --since-ref is required")
				return flag.ErrHelp
			}

			until := strings.TrimSpace(*untilRef)
			if until == "" {
				fmt.Fprintln(os.Stderr, "Error: --until-ref is required")
				return flag.ErrHelp
			}

			formatValue := strings.ToLower(strings.TrimSpace(*format))
			switch formatValue {
			case "", "plain":
				formatValue = "plain"
			case "markdown":
				// ok
			default:
				fmt.Fprintln(os.Stderr, "Error: --format must be one of: plain, markdown")
				return flag.ErrHelp
			}

			if *maxChars < 1 {
				fmt.Fprintln(os.Stderr, "Error: --max-chars must be greater than 0")
				return flag.ErrHelp
			}

			since := sinceRefValue
			if sinceTagValue != "" {
				since = sinceTagValue
			}

			repoDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("release-notes generate: %w", err)
			}

			commits, err := notes.ListCommits(ctx, repoDir, since, until, *includeMerges)
			if err != nil {
				return fmt.Errorf("release-notes generate: %w", err)
			}

			rendered, err := notes.FormatNotes(commits, formatValue)
			if err != nil {
				// Should be unreachable due to validation, but keep it safe.
				return fmt.Errorf("release-notes generate: %w", err)
			}

			truncatedNotes, truncated := notes.TruncateNotes(rendered, *maxChars)

			result := releaseNotesGenerateResult{
				Since:         since,
				Until:         until,
				Format:        formatValue,
				MaxChars:      *maxChars,
				IncludeMerges: *includeMerges,
				CommitCount:   len(commits),
				Truncated:     truncated,
				Notes:         truncatedNotes,
				Commits:       commits,
			}

			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "json", "text", "table", "markdown")
			if err != nil {
				return fmt.Errorf("release-notes generate: %w", err)
			}

			switch normalizedOutput {
			case "json":
				return shared.PrintOutput(&result, "json", *output.Pretty)
			case "text", "markdown":
				// Notes body output (markdown is a bullet list).
				body := shared.SanitizeTerminal(truncatedNotes)
				if strings.TrimSpace(body) == "" {
					return nil
				}
				_, err := fmt.Fprintln(os.Stdout, body)
				return err
			case "table":
				tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "SHA\tSUBJECT")
				for _, c := range commits {
					sha := shared.SanitizeTerminal(strings.TrimSpace(c.SHA))
					subject := shared.SanitizeTerminal(strings.TrimSpace(c.Subject))
					_, _ = fmt.Fprintf(tw, "%s\t%s\n", sha, subject)
				}
				return tw.Flush()
			default:
				// shared.ValidateOutputFormatAllowed should prevent this.
				return fmt.Errorf("release-notes generate: unsupported format: %s", normalizedOutput)
			}
		},
	}
}
