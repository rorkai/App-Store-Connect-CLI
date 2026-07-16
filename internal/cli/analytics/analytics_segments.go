package analytics

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AnalyticsSegmentsCommand returns the analytics segments command group.
func AnalyticsSegmentsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("segments", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "segments",
		ShortUsage: "asc analytics segments <subcommand> [flags]",
		ShortHelp:  "Get analytics report segments by ID.",
		LongHelp: `Get analytics report segments by ID.

Examples:
  asc analytics segments view --segment-id "SEGMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AnalyticsSegmentsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AnalyticsSegmentsGetCommand retrieves a specific analytics report segment.
func AnalyticsSegmentsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	segmentID := fs.String("segment-id", "", "Analytics report segment ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc analytics segments view --segment-id \"SEGMENT_ID\" [flags]",
		ShortHelp:  "View an analytics report segment by ID.",
		LongHelp: `View an analytics report segment by ID.

Examples:
  asc analytics segments view --segment-id "SEGMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*segmentID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --segment-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics segments view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAnalyticsReportSegment(requestCtx, strings.TrimSpace(*segmentID))
			if err != nil {
				return fmt.Errorf("analytics segments view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
