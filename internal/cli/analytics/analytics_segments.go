package analytics

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAnalyticsSegmentsCommand returns the analytics segments command group.
func newAnalyticsSegmentsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("segments", flag.ExitOnError)

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
			newAnalyticsSegmentsGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAnalyticsSegmentsGetCommand retrieves a specific analytics report segment.
func newAnalyticsSegmentsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	segmentID := shared.BindResourceIDFlag(fs, "segment-id", "analyticsReportSegments", "Analytics report segment ID")
	output := d.BindOutputFlags(fs)

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
			id := strings.TrimSpace(*segmentID)
			if id == "" {
				return d.RequiredFlag("--segment-id")
			}
			var err error
			id, err = asc.ValidateResourcePathSegment(id)
			if err != nil {
				return d.UsageErrorf("analytics segments view: --segment-id: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics segments view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAnalyticsReportSegment(requestCtx, id)
			if err != nil {
				return fmt.Errorf("analytics segments view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
