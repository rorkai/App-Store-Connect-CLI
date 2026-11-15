package analytics

import (
	"context"
	"flag"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc analytics segments view --segment-id \"SEGMENT_ID\" [flags]",
		ShortHelp:  "View an analytics report segment by ID.",
		LongHelp: `View an analytics report segment by ID.

Examples:
  asc analytics segments view --segment-id "SEGMENT_ID"`,
		IDFlag:             "segment-id",
		IDUsage:            "Analytics report segment ID",
		IDType:             "analyticsReportSegments",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			segmentID := fs.Lookup("segment-id").Value
			return func() error {
				if _, err := asc.ValidateResourcePathSegment(strings.TrimSpace(segmentID.String())); err != nil {
					return shared.UsageErrorf("analytics segments view: --segment-id: %v", err)
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAnalyticsReportSegment(ctx, id)
		},
	})
}
