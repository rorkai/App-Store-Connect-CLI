package analytics

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AnalyticsRequestCommand creates a new analytics report request.
func AnalyticsRequestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("request", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	accessType := fs.String("access-type", "", "Access type: ONGOING or ONE_TIME_SNAPSHOT")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "request",
		ShortUsage: "asc analytics request [flags]",
		ShortHelp:  "Create an analytics report request.",
		LongHelp: `Create an analytics report request.

Examples:
  asc analytics request --app "123456789" --access-type ONGOING
  asc analytics request --app "123456789" --access-type ONE_TIME_SNAPSHOT`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*accessType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --access-type is required")
				return flag.ErrHelp
			}
			normalizedAccessType, err := normalizeAnalyticsAccessType(*accessType)
			if err != nil {
				return fmt.Errorf("analytics request: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics request: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			response, err := client.CreateAnalyticsReportRequest(requestCtx, resolvedAppID, normalizedAccessType)
			if err != nil {
				return fmt.Errorf("analytics request: failed to create request: %w", err)
			}

			result := &asc.AnalyticsReportRequestResult{
				RequestID:   response.Data.ID,
				AppID:       resolvedAppID,
				AccessType:  string(normalizedAccessType),
				State:       string(response.Data.Attributes.State),
				CreatedDate: response.Data.Attributes.CreatedDate,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// AnalyticsRequestsCommand lists analytics report requests.
func AnalyticsRequestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("requests", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	requestID := fs.String("request-id", "", "Filter by request ID")
	state := fs.String("state", "", "Filter by state: PROCESSING, COMPLETED, FAILED")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "requests",
		ShortUsage: "asc analytics requests [flags]",
		ShortHelp:  "List and manage analytics report requests.",
		LongHelp: `List analytics report requests.

Examples:
  asc analytics requests --app "123456789"
  asc analytics requests --app "123456789" --state COMPLETED
  asc analytics requests --app "123456789" --request-id "REQUEST_ID"
  asc analytics requests --next "<links.next>"
  asc analytics requests --app "123456789" --paginate
  asc analytics requests delete --request-id "REQUEST_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AnalyticsRequestsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > analyticsMaxLimit) {
				return fmt.Errorf("analytics requests: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("analytics requests: %w", err)
			}
			if strings.TrimSpace(*requestID) != "" {
				if err := validateUUIDFlag("--request-id", *requestID); err != nil {
					return fmt.Errorf("analytics requests: %w", err)
				}
			}

			var normalizedState asc.AnalyticsReportRequestState
			if strings.TrimSpace(*state) != "" {
				stateValue, err := normalizeAnalyticsRequestState(*state)
				if err != nil {
					return fmt.Errorf("analytics requests: %w", err)
				}
				normalizedState = stateValue
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" && strings.TrimSpace(*requestID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics requests: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var response *asc.AnalyticsReportRequestsResponse
			if strings.TrimSpace(*requestID) != "" {
				single, err := client.GetAnalyticsReportRequest(requestCtx, strings.TrimSpace(*requestID))
				if err != nil {
					return fmt.Errorf("analytics requests: failed to fetch: %w", err)
				}
				response = &asc.AnalyticsReportRequestsResponse{
					Data:  []asc.AnalyticsReportRequestResource{single.Data},
					Links: single.Links,
				}
			} else {
				opts := []asc.AnalyticsReportRequestsOption{
					asc.WithAnalyticsReportRequestsLimit(*limit),
					asc.WithAnalyticsReportRequestsNextURL(*next),
				}
				if normalizedState != "" {
					opts = append(opts, asc.WithAnalyticsReportRequestsState(string(normalizedState)))
				}

				if *paginate {
					// Fetch first page with limit set for consistent pagination
					paginateOpts := append(opts, asc.WithAnalyticsReportRequestsLimit(200))
					firstPage, err := client.GetAnalyticsReportRequests(requestCtx, resolvedAppID, paginateOpts...)
					if err != nil {
						return fmt.Errorf("analytics requests: failed to fetch: %w", err)
					}

					// Fetch all remaining pages
					paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAnalyticsReportRequests(ctx, resolvedAppID, asc.WithAnalyticsReportRequestsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("analytics requests: %w", err)
					}

					return shared.PrintOutput(paginated, *output, *pretty)
				}

				response, err = client.GetAnalyticsReportRequests(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("analytics requests: failed to fetch: %w", err)
				}
			}

			return shared.PrintOutput(response, *output, *pretty)
		},
	}
}

// AnalyticsRequestsDeleteCommand deletes an analytics report request.
func AnalyticsRequestsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	requestID := fs.String("request-id", "", "Analytics report request ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc analytics requests delete --request-id \"REQUEST_ID\" --confirm",
		ShortHelp:  "Delete an analytics report request.",
		LongHelp: `Delete an analytics report request by ID.

Examples:
  asc analytics requests delete --request-id "REQUEST_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*requestID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --request-id is required")
				return flag.ErrHelp
			}
			if err := validateUUIDFlag("--request-id", id); err != nil {
				return fmt.Errorf("analytics requests delete: %w", err)
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics requests delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAnalyticsReportRequest(requestCtx, id); err != nil {
				return fmt.Errorf("analytics requests delete: failed to delete: %w", err)
			}

			result := &asc.AnalyticsReportRequestDeleteResult{
				RequestID: id,
				Deleted:   true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// AnalyticsGetCommand retrieves analytics reports and instances for a request.
func AnalyticsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	requestID := fs.String("request-id", "", "Analytics report request ID")
	instanceID := fs.String("instance-id", "", "Filter by specific instance ID")
	date := fs.String("date", "", "Filter instances by date (YYYY-MM-DD)")
	includeSegments := fs.Bool("include-segments", false, "Include report segments with download URLs")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Paginate all reports (recommended with --date)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc analytics get [flags]",
		ShortHelp:  "Get analytics reports for a request.",
		LongHelp: `Get analytics reports for a request.

Examples:
  asc analytics get --request-id "REQUEST_ID"
  asc analytics get --request-id "REQUEST_ID" --include-segments
  asc analytics get --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"
  asc analytics get --request-id "REQUEST_ID" --date "2024-01-20" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*requestID) == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --request-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*requestID) != "" {
				if err := validateUUIDFlag("--request-id", *requestID); err != nil {
					return fmt.Errorf("analytics get: %w", err)
				}
			}
			if strings.TrimSpace(*instanceID) != "" {
				if err := validateUUIDFlag("--instance-id", *instanceID); err != nil {
					return fmt.Errorf("analytics get: %w", err)
				}
			}
			if *limit != 0 && (*limit < 1 || *limit > analyticsMaxLimit) {
				return fmt.Errorf("analytics get: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("analytics get: %w", err)
			}

			dateFilter, err := normalizeAnalyticsDateFilter(*date)
			if err != nil {
				return fmt.Errorf("analytics get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			paginateReports := strings.TrimSpace(*next) == "" && (strings.TrimSpace(*instanceID) != "" || *paginate)
			reports, links, err := fetchAnalyticsReports(requestCtx, client, strings.TrimSpace(*requestID), *limit, *next, paginateReports)
			if err != nil {
				return fmt.Errorf("analytics get: failed to fetch reports: %w", err)
			}

			result := &asc.AnalyticsReportGetResult{
				RequestID: strings.TrimSpace(*requestID),
				Links:     links,
			}

			foundInstance := false
			for _, report := range reports {
				instances, err := fetchAnalyticsReportInstances(requestCtx, client, report.ID)
				if err != nil {
					return fmt.Errorf("analytics get: failed to fetch instances: %w", err)
				}

				reportResult := asc.AnalyticsReportGetReport{
					ID:          report.ID,
					ReportType:  report.Attributes.ReportType,
					Name:        report.Attributes.Name,
					Category:    report.Attributes.Category,
					Granularity: report.Attributes.Granularity,
				}

				for _, instance := range instances {
					if strings.TrimSpace(*instanceID) != "" && instance.ID != strings.TrimSpace(*instanceID) {
						continue
					}
					if !matchAnalyticsInstanceDate(instance.Attributes, dateFilter) {
						continue
					}

					instanceResult := asc.AnalyticsReportGetInstance{
						ID:             instance.ID,
						ReportDate:     instance.Attributes.ReportDate,
						ProcessingDate: instance.Attributes.ProcessingDate,
						Granularity:    instance.Attributes.Granularity,
						Version:        instance.Attributes.Version,
					}

					if *includeSegments {
						segments, err := fetchAnalyticsReportSegments(requestCtx, client, instance.ID)
						if err != nil {
							return fmt.Errorf("analytics get: failed to fetch segments: %w", err)
						}
						for _, segment := range segments {
							instanceResult.Segments = append(instanceResult.Segments, asc.AnalyticsReportGetSegment{
								ID:                segment.ID,
								DownloadURL:       segment.Attributes.URL,
								Checksum:          segment.Attributes.Checksum,
								SizeInBytes:       segment.Attributes.SizeInBytes,
								URLExpirationDate: segment.Attributes.URLExpirationDate,
							})
						}
					}

					reportResult.Instances = append(reportResult.Instances, instanceResult)
				}

				if strings.TrimSpace(*instanceID) != "" {
					if len(reportResult.Instances) > 0 {
						result.Data = append(result.Data, reportResult)
						foundInstance = true
						break
					}
					continue
				}

				if dateFilter != "" && len(reportResult.Instances) == 0 {
					continue
				}
				result.Data = append(result.Data, reportResult)
			}

			if strings.TrimSpace(*instanceID) != "" && !foundInstance {
				return fmt.Errorf("analytics get: instance %q not found for request %q", strings.TrimSpace(*instanceID), strings.TrimSpace(*requestID))
			}
			if dateFilter != "" && len(result.Data) == 0 {
				if strings.TrimSpace(*next) == "" && !*paginate {
					return fmt.Errorf("analytics get: no instances found for date %q in the first page of reports (use --paginate or --next)", dateFilter)
				}
				return fmt.Errorf("analytics get: no instances found for date %q", dateFilter)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// AnalyticsDownloadCommand downloads analytics report data.
func AnalyticsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	requestID := fs.String("request-id", "", "Analytics report request ID")
	instanceID := fs.String("instance-id", "", "Analytics report instance ID")
	segmentID := fs.String("segment-id", "", "Analytics report segment ID (required if multiple)")
	output := fs.String("output", "", "Output file path (default: analytics_report_{requestId}_{instanceId}.csv.gz)")
	decompress := fs.Bool("decompress", false, "Decompress gzip output to .csv")
	outputFormat := fs.String("output-format", "json", "Output format for metadata: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc analytics download [flags]",
		ShortHelp:  "Download analytics report data.",
		LongHelp: `Download analytics report data.

Examples:
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --decompress
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --segment-id "SEGMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*requestID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --request-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*instanceID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --instance-id is required")
				return flag.ErrHelp
			}
			if err := validateUUIDFlag("--request-id", *requestID); err != nil {
				return fmt.Errorf("analytics download: %w", err)
			}
			if err := validateUUIDFlag("--instance-id", *instanceID); err != nil {
				return fmt.Errorf("analytics download: %w", err)
			}
			if strings.TrimSpace(*segmentID) != "" {
				if err := validateUUIDFlag("--segment-id", *segmentID); err != nil {
					return fmt.Errorf("analytics download: %w", err)
				}
			}

			defaultOutput := fmt.Sprintf("analytics_report_%s_%s.csv.gz", strings.TrimSpace(*requestID), strings.TrimSpace(*instanceID))
			compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".csv", *decompress)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics download: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			reports, _, err := fetchAnalyticsReports(requestCtx, client, strings.TrimSpace(*requestID), 0, "", true)
			if err != nil {
				return fmt.Errorf("analytics download: failed to fetch reports: %w", err)
			}

			instanceFound := false
			for _, report := range reports {
				instances, err := fetchAnalyticsReportInstances(requestCtx, client, report.ID)
				if err != nil {
					return fmt.Errorf("analytics download: failed to fetch instances: %w", err)
				}
				for _, instance := range instances {
					if instance.ID == strings.TrimSpace(*instanceID) {
						instanceFound = true
						break
					}
				}
				if instanceFound {
					break
				}
			}
			if !instanceFound {
				return fmt.Errorf("analytics download: instance %q not found for request %q", strings.TrimSpace(*instanceID), strings.TrimSpace(*requestID))
			}

			segments, err := fetchAnalyticsReportSegments(requestCtx, client, strings.TrimSpace(*instanceID))
			if err != nil {
				return fmt.Errorf("analytics download: failed to fetch segments: %w", err)
			}
			if len(segments) == 0 {
				return fmt.Errorf("analytics download: no segments available for instance %q", strings.TrimSpace(*instanceID))
			}

			selectedSegment := segments[0]
			if strings.TrimSpace(*segmentID) != "" {
				found := false
				for _, segment := range segments {
					if segment.ID == strings.TrimSpace(*segmentID) {
						selectedSegment = segment
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("analytics download: segment %q not found for instance %q", strings.TrimSpace(*segmentID), strings.TrimSpace(*instanceID))
				}
			} else if len(segments) > 1 {
				return fmt.Errorf("analytics download: multiple segments found; specify --segment-id")
			}

			downloadURL := strings.TrimSpace(selectedSegment.Attributes.URL)
			if downloadURL == "" {
				return fmt.Errorf("analytics download: segment download URL is empty")
			}

			download, err := client.DownloadAnalyticsReport(requestCtx, downloadURL)
			if err != nil {
				return fmt.Errorf("analytics download: failed to download report: %w", err)
			}
			defer download.Body.Close()

			compressedSize, err := shared.WriteStreamToFile(compressedPath, download.Body)
			if err != nil {
				return fmt.Errorf("analytics download: failed to write report: %w", err)
			}

			var decompressedSize int64
			if *decompress {
				decompressedSize, err = shared.DecompressGzipFile(compressedPath, decompressedPath)
				if err != nil {
					return fmt.Errorf("analytics download: %w", err)
				}
			}

			result := &asc.AnalyticsReportDownloadResult{
				RequestID:        strings.TrimSpace(*requestID),
				InstanceID:       strings.TrimSpace(*instanceID),
				SegmentID:        selectedSegment.ID,
				FilePath:         compressedPath,
				FileSize:         compressedSize,
				Decompressed:     *decompress,
				DecompressedPath: decompressedPath,
				DecompressedSize: decompressedSize,
			}

			return shared.PrintOutput(result, *outputFormat, *pretty)
		},
	}
}
