package analytics

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAnalyticsRequestCommand creates a new analytics report request.
func newAnalyticsRequestCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("request", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	accessType := fs.String("access-type", "", "Access type: ONGOING or ONE_TIME_SNAPSHOT")
	reuseExisting := fs.Bool("reuse-existing", false, "Return an existing active request with the same access type instead of creating a duplicate")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "request",
		ShortUsage: "asc analytics request [flags]",
		ShortHelp:  "Create an analytics report request.",
		LongHelp: `Create an analytics report request.

Examples:
  asc analytics request --app "123456789" --access-type ONGOING
  asc analytics request --app "123456789" --access-type ONGOING --reuse-existing
  asc analytics request --app "123456789" --access-type ONE_TIME_SNAPSHOT`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*accessType) == "" {
				return d.RequiredFlag("--access-type")
			}
			normalizedAccessType, err := normalizeAnalyticsAccessType(*accessType)
			if err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics request: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *reuseExisting {
				result, err := createOrReuseAnalyticsReportRequest(requestCtx, client, resolvedAppID, normalizedAccessType)
				if err != nil {
					return fmt.Errorf("analytics request: %w", err)
				}

				return d.PrintOutput(result, *output.Output, *output.Pretty)
			}

			response, err := client.CreateAnalyticsReportRequest(requestCtx, resolvedAppID, normalizedAccessType)
			if err != nil {
				return fmt.Errorf("analytics request: failed to create request: %w", err)
			}

			result := &asc.AnalyticsReportRequestResult{
				RequestID:              response.Data.ID,
				AppID:                  resolvedAppID,
				AccessType:             string(normalizedAccessType),
				StoppedDueToInactivity: response.Data.Attributes.StoppedDueToInactivity,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newAnalyticsRequestsCommand lists analytics report requests.
func newAnalyticsRequestsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("requests", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	requestID := shared.BindResourceIDFlag(fs, "request-id", "analyticsReportRequests", "Filter by request ID")
	accessType := fs.String("access-type", "", "Filter by access type: ONGOING, ONE_TIME_SNAPSHOT")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "requests",
		ShortUsage: "asc analytics requests [flags]",
		ShortHelp:  "List and manage analytics report requests.",
		LongHelp: `List analytics report requests.

Examples:
  asc analytics requests --app "123456789"
  asc analytics requests --app "123456789" --access-type ONGOING
  asc analytics requests --app "123456789" --request-id "REQUEST_ID"
  asc analytics requests --next "<links.next>"
  asc analytics requests --app "123456789" --paginate
  asc analytics requests delete --request-id "REQUEST_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAnalyticsRequestsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return flag.ErrHelp
			}
			if err := d.ValidatePaginationFlags(ctx, fs, "analytics requests", analyticsMaxLimit); err != nil {
				return err
			}
			resolvedAppID := d.ResolveAppID(*appID)
			if err := validateAnalyticsRequestsCursor(*next, resolvedAppID); err != nil {
				return d.UsageErrorf("analytics requests: %v", err)
			}
			if strings.TrimSpace(*requestID) != "" {
				if err := validateAnalyticsRequestID(*requestID); err != nil {
					return d.UsageErrorf("analytics requests: %v", err)
				}
			}

			var normalizedAccessType asc.AnalyticsAccessType
			if strings.TrimSpace(*accessType) != "" {
				accessTypeValue, err := normalizeAnalyticsAccessType(*accessType)
				if err != nil {
					return d.UsageError(err.Error())
				}
				normalizedAccessType = accessTypeValue
			}
			if normalizedAccessType != "" && strings.TrimSpace(*requestID) != "" {
				return d.UsageError("--access-type cannot be used with --request-id")
			}
			if normalizedAccessType != "" && strings.TrimSpace(*next) != "" {
				return d.UsageError("--access-type cannot be used with --next")
			}

			if resolvedAppID == "" && strings.TrimSpace(*next) == "" && strings.TrimSpace(*requestID) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics requests: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
					asc.Limit[asc.AnalyticsReportRequestsOption](*limit),
					asc.NextURL[asc.AnalyticsReportRequestsOption](*next),
				}
				if normalizedAccessType != "" {
					opts = append(opts, asc.WithAnalyticsReportRequestsAccessType(normalizedAccessType))
				}

				if *paginate {
					paginateOpts := append(opts, asc.Limit[asc.AnalyticsReportRequestsOption](200))
					paginated, err := d.PaginateWithSpinner(
						requestCtx,
						func(ctx context.Context) (asc.PaginatedResponse, error) {
							return client.GetAnalyticsReportRequests(ctx, resolvedAppID, paginateOpts...)
						},
						func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
							return client.GetAnalyticsReportRequests(ctx, resolvedAppID, asc.NextURL[asc.AnalyticsReportRequestsOption](nextURL))
						},
					)
					if err != nil {
						return fmt.Errorf("analytics requests: %w", err)
					}

					return d.PrintOutput(paginated, *output.Output, *output.Pretty)
				}

				response, err = client.GetAnalyticsReportRequests(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("analytics requests: failed to fetch: %w", err)
				}
			}

			return d.PrintOutput(response, *output.Output, *output.Pretty)
		},
	}
}

func validateAnalyticsRequestsCursor(next, appID string) error {
	next = strings.TrimSpace(next)
	if next == "" {
		return nil
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return err
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return err
	}
	path, hasPrefix := strings.CutPrefix(parsed.Path, "/v1/apps/")
	cursorAppID, hasSuffix := strings.CutSuffix(path, "/analyticsReportRequests")
	if _, err := asc.ValidateResourcePathSegment(cursorAppID); !hasPrefix || !hasSuffix || err != nil {
		return errors.New("--next must point to an app's analytics report requests")
	}
	if appID = strings.TrimSpace(appID); appID != "" && appID != cursorAppID {
		return errors.New("the selected app must match the app in --next")
	}
	return nil
}

func createOrReuseAnalyticsReportRequest(ctx context.Context, client *asc.Client, appID string, accessType asc.AnalyticsAccessType) (*asc.AnalyticsReportRequestReuseResult, error) {
	requests, err := listAnalyticsReportRequestsForReuse(ctx, client, appID)
	if err != nil {
		return nil, err
	}

	if request, ok := findReusableAnalyticsReportRequest(requests, accessType); ok {
		return analyticsReportRequestReuseResult(appID, request, false), nil
	}

	created, err := client.CreateAnalyticsReportRequest(ctx, appID, accessType)
	if err != nil {
		if !isAnalyticsReportRequestCreateConflict(err) {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		requests, listErr := listAnalyticsReportRequestsForReuse(ctx, client, appID)
		if listErr != nil {
			return nil, listErr
		}
		if request, ok := findReusableAnalyticsReportRequest(requests, accessType); ok {
			return analyticsReportRequestReuseResult(appID, request, false), nil
		}

		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return analyticsReportRequestReuseResult(appID, created.Data, true), nil
}

func listAnalyticsReportRequestsForReuse(ctx context.Context, client *asc.Client, appID string) (*asc.AnalyticsReportRequestsResponse, error) {
	existing, err := client.GetAnalyticsReportRequests(ctx, appID, asc.Limit[asc.AnalyticsReportRequestsOption](200))
	if err != nil {
		return nil, fmt.Errorf("failed to list requests: %w", err)
	}

	paginated, err := asc.PaginateAll(ctx, existing, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAnalyticsReportRequests(ctx, appID, asc.NextURL[asc.AnalyticsReportRequestsOption](nextURL))
	})
	if err != nil {
		return nil, err
	}

	if paginated == nil {
		return nil, fmt.Errorf("failed to list requests: empty paginated response")
	}
	requests, ok := paginated.(*asc.AnalyticsReportRequestsResponse)
	if !ok || requests == nil {
		return nil, fmt.Errorf("failed to list requests: unexpected paginated response type %T", paginated)
	}

	return requests, nil
}

func findReusableAnalyticsReportRequest(requests *asc.AnalyticsReportRequestsResponse, accessType asc.AnalyticsAccessType) (asc.AnalyticsReportRequestResource, bool) {
	if requests == nil {
		return asc.AnalyticsReportRequestResource{}, false
	}
	for _, request := range requests.Data {
		if analyticsReportRequestMatches(request, accessType) {
			return request, true
		}
	}

	return asc.AnalyticsReportRequestResource{}, false
}

func isAnalyticsReportRequestCreateConflict(err error) bool {
	var apiErr *asc.APIError
	return errors.As(err, &apiErr) && apiErr != nil && apiErr.StatusCode == http.StatusConflict
}

func analyticsReportRequestMatches(request asc.AnalyticsReportRequestResource, accessType asc.AnalyticsAccessType) bool {
	if request.Attributes.AccessType != accessType {
		return false
	}
	return request.Attributes.StoppedDueToInactivity == nil || !*request.Attributes.StoppedDueToInactivity
}

func analyticsReportRequestReuseResult(appID string, request asc.AnalyticsReportRequestResource, created bool) *asc.AnalyticsReportRequestReuseResult {
	return &asc.AnalyticsReportRequestReuseResult{
		RequestID:              request.ID,
		AppID:                  appID,
		AccessType:             string(request.Attributes.AccessType),
		StoppedDueToInactivity: request.Attributes.StoppedDueToInactivity,
		Created:                created,
	}
}

// newAnalyticsRequestsDeleteCommand deletes an analytics report request.
func newAnalyticsRequestsDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)

	requestID := shared.BindResourceIDFlag(fs, "request-id", "analyticsReportRequests", "Analytics report request ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc analytics requests delete --request-id \"REQUEST_ID\" --confirm",
		ShortHelp:  "Delete an analytics report request.",
		LongHelp: `Delete an analytics report request by ID.

Examples:
  asc analytics requests delete --request-id "REQUEST_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*requestID)
			if id == "" {
				return d.RequiredFlag("--request-id")
			}
			if err := validateAnalyticsRequestID(id); err != nil {
				return d.UsageErrorf("analytics requests delete: %v", err)
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics requests delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteAnalyticsReportRequest(requestCtx, id); err != nil {
				return fmt.Errorf("analytics requests delete: failed to delete: %w", err)
			}

			result := &asc.AnalyticsReportRequestDeleteResult{
				RequestID: id,
				Deleted:   true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func validateAnalyticsViewCursor(next, requestID string) error {
	next = strings.TrimSpace(next)
	if next == "" {
		return nil
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return err
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return err
	}
	path, hasPrefix := strings.CutPrefix(parsed.Path, "/v1/analyticsReportRequests/")
	cursorRequestID, hasSuffix := strings.CutSuffix(path, "/reports")
	if _, err := asc.ValidateResourcePathSegment(cursorRequestID); !hasPrefix || !hasSuffix || err != nil {
		return errors.New("--next must point to an analytics report request's reports")
	}
	if requestID = strings.TrimSpace(requestID); requestID != "" && !strings.EqualFold(requestID, cursorRequestID) {
		return errors.New("--request-id must match the report request in --next")
	}
	return nil
}

// newAnalyticsViewCommand retrieves analytics reports and instances for a request.
func newAnalyticsViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	requestID := shared.BindResourceIDFlag(fs, "request-id", "analyticsReportRequests", "Analytics report request ID")
	instanceID := shared.BindResourceIDFlag(fs, "instance-id", "analyticsReportInstances", "Filter by specific instance ID")
	processingDate := fs.String("processing-date", "", "Filter instances by processing date (YYYY-MM-DD)")
	granularity := fs.String("granularity", "", "Filter instances by granularity (comma-separated: DAILY, WEEKLY, MONTHLY)")
	includeSegments := fs.Bool("include-segments", false, "Include report segments with download URLs")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, "Paginate all reports (recommended with --processing-date or --next)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc analytics view [flags]",
		ShortHelp:  "View analytics reports for a request.",
		LongHelp: `View analytics reports for a request.

The --processing-date and --granularity filters are sent to App Store Connect
when fetching report instances. Granularity accepts DAILY, WEEKLY, and MONTHLY.
Use --next with a report-page links.next URL to resume from that page. Combine
--next with --paginate to search the remaining report pages; without
--paginate, only the supplied page is fetched. When selecting an exact
--instance-id, the search stops once it finds the instance.

Examples:
  asc analytics view --request-id "REQUEST_ID"
  asc analytics view --request-id "REQUEST_ID" --include-segments
  asc analytics view --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"
  asc analytics view --request-id "REQUEST_ID" --processing-date "2024-01-20" --paginate
  asc analytics view --request-id "REQUEST_ID" --processing-date "2024-01-20" --granularity "DAILY,WEEKLY" --paginate
  asc analytics view --next "<links.next>" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "analytics view", analyticsMaxLimit); err != nil {
				return err
			}
			var processingDateProvided, granularityProvided bool
			fs.Visit(func(item *flag.Flag) {
				switch item.Name {
				case "processing-date":
					processingDateProvided = true
				case "granularity":
					granularityProvided = true
				}
			})
			if strings.TrimSpace(*requestID) == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--request-id")
			}
			if strings.TrimSpace(*requestID) != "" {
				if err := validateAnalyticsRequestID(*requestID); err != nil {
					return d.UsageErrorf("analytics view: %v", err)
				}
			}
			if strings.TrimSpace(*instanceID) != "" {
				if _, err := asc.ValidateResourcePathSegment(*instanceID); err != nil {
					return d.UsageErrorf("analytics view: --instance-id: %v", err)
				}
			}
			if err := validateAnalyticsViewCursor(*next, *requestID); err != nil {
				return d.UsageErrorf("analytics view: %v", err)
			}

			var processingDateFilter string
			if processingDateProvided {
				normalized, normalizeErr := normalizeAnalyticsProcessingDateFilter(*processingDate)
				if normalizeErr != nil {
					return d.UsageErrorf("analytics view: %v", normalizeErr)
				}
				processingDateFilter = normalized
			}
			var granularities []string
			if granularityProvided {
				normalized, normalizeErr := normalizeAnalyticsGranularities(*granularity)
				if normalizeErr != nil {
					return d.UsageErrorf("analytics view: %v", normalizeErr)
				}
				granularities = normalized
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics view: %w", err)
			}

			targetInstanceID := strings.TrimSpace(*instanceID)
			paginateReports := *paginate || (strings.TrimSpace(*next) == "" && targetInstanceID != "")
			result := &asc.AnalyticsReportGetResult{RequestID: strings.TrimSpace(*requestID)}
			instanceOpts := make([]asc.AnalyticsReportInstancesOption, 0, 2)
			if processingDateFilter != "" {
				instanceOpts = append(instanceOpts, asc.WithAnalyticsReportInstancesProcessingDates([]string{processingDateFilter}))
			}
			if len(granularities) > 0 {
				instanceOpts = append(instanceOpts, asc.WithAnalyticsReportInstancesGranularities(granularities))
			}

			if targetInstanceID == "" {
				reports, links, err := fetchAnalyticsReports(ctx, d, client, result.RequestID, *limit, *next, paginateReports)
				if err != nil {
					return fmt.Errorf("analytics view: %w", err)
				}
				result.Links = links
				collected, instanceCount, err := collectAnalyticsReports(ctx, d, client, reports, instanceOpts, *includeSegments, processingDateFilter)
				if err != nil {
					return err
				}
				result.Data = collected
				if processingDateFilter == "" && len(granularities) == 0 && instanceCount > 20 {
					fmt.Fprintf(d.IO.ErrOut, "analytics view: fetched %d instances without --processing-date or --granularity; narrow the query to avoid a full fan-out\n", instanceCount)
				}
			} else {
				report, instances, links, err := findAnalyticsReportInstance(ctx, d, client, result.RequestID, targetInstanceID, *limit, *next, paginateReports, instanceOpts...)
				if err != nil {
					return fmt.Errorf("analytics view: %w", err)
				}
				result.Links = links
				if len(instances) > 0 {
					reportResult, err := analyticsReportResultFromInstances(ctx, d, client, report, instances, *includeSegments)
					if err != nil {
						return err
					}
					result.Data = append(result.Data, reportResult)
				}
			}

			if targetInstanceID != "" && len(result.Data) == 0 {
				return analyticsNotFoundError(fmt.Errorf("analytics view: instance %q not found for request %q", targetInstanceID, strings.TrimSpace(*requestID)), "--instance-id")
			}
			if processingDateFilter != "" && len(result.Data) == 0 {
				if strings.TrimSpace(*next) == "" && !*paginate {
					return analyticsNotFoundError(fmt.Errorf("analytics view: no instances found for processing date %q in the first page of reports (use --paginate or --next)", processingDateFilter), "--processing-date")
				}
				return analyticsNotFoundError(fmt.Errorf("analytics view: no instances found for processing date %q", processingDateFilter), "--processing-date")
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newAnalyticsDownloadCommand downloads analytics report data.
func newAnalyticsDownloadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("download", flag.ExitOnError)

	requestID := shared.BindResourceIDFlag(fs, "request-id", "analyticsReportRequests", "Analytics report request ID")
	instanceID := shared.BindResourceIDFlag(fs, "instance-id", "analyticsReportInstances", "Analytics report instance ID")
	segmentID := shared.BindResourceIDFlag(fs, "segment-id", "analyticsReportSegments", "Analytics report segment ID (required if multiple)")
	output := fs.String("output", "", "Output file path (default: analytics_report_{requestId}_{instanceId}.csv.gz)")
	decompress := fs.Bool("decompress", false, "Decompress gzip output to .csv")
	outputFlags := shared.BindMetadataOutputFlags(fs)
	allSegments := fs.Bool("all-segments", false, "Download every segment of the report instance")
	outputDir := fs.String("output-dir", "", "New output directory with an existing parent (required with --all-segments)")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc analytics download [flags]",
		ShortHelp:  "Download analytics report data.",
		LongHelp: `Download analytics report data.

Use --all-segments with --output-dir to get the complete report instance.
This mode uses .txt.gz files, with a .txt copy per segment when --decompress is
set. The directory is published after all segments are saved.

Examples:
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --decompress
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --segment-id "SEGMENT_ID"
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --all-segments --output-dir "./analytics-report"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.RejectOutputFormatAsPath(*output, "--output-format"); err != nil {
				return err
			}
			if strings.TrimSpace(*requestID) == "" {
				return d.RequiredFlag("--request-id")
			}
			if strings.TrimSpace(*instanceID) == "" {
				return d.RequiredFlag("--instance-id")
			}
			if err := validateAnalyticsRequestID(*requestID); err != nil {
				return d.UsageErrorf("analytics download: %v", err)
			}
			if _, err := asc.ValidateResourcePathSegment(*instanceID); err != nil {
				return d.UsageErrorf("analytics download: --instance-id: %v", err)
			}
			destination := strings.TrimSpace(*outputDir)
			if err := validateAnalyticsSegmentDownloadFlags(d, fs, *allSegments, destination); err != nil {
				return err
			}
			if *allSegments {
				if _, err := shared.ValidateOutputFormat(*outputFlags.OutputFormat, *outputFlags.Pretty); err != nil {
					return d.UsageError(err.Error())
				}
				result, err := downloadAnalyticsInstance(ctx, d, strings.TrimSpace(*requestID), strings.TrimSpace(*instanceID), destination, *decompress)
				if err != nil {
					return fmt.Errorf("analytics download: %w", err)
				}
				if err := d.PrintOutput(result, *outputFlags.OutputFormat, *outputFlags.Pretty); err != nil {
					return fmt.Errorf("analytics download: failed to print receipt for report saved to %q: %w", result.OutputDir, err)
				}
				return nil
			}
			if strings.TrimSpace(*segmentID) != "" {
				if _, err := asc.ValidateResourcePathSegment(*segmentID); err != nil {
					return d.UsageErrorf("analytics download: --segment-id: %v", err)
				}
			}

			defaultOutput := analyticsDownloadDefaultOutput(*requestID, *instanceID)
			compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".csv", *decompress)

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics download: %w", err)
			}

			// Read the instance's segments directly. Apple's instance resource has
			// no link back to its report or request, so confirming that the
			// instance belongs to --request-id meant listing the instances of
			// every report before it: about 40 requests per download against an
			// hourly limit of 3,600.
			segments, err := analyticsDownloadSegments(ctx, d, client, strings.TrimSpace(*instanceID), strings.TrimSpace(*segmentID))
			if err != nil {
				return fmt.Errorf("analytics download: %w", err)
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
					return analyticsNotFoundError(fmt.Errorf("analytics download: segment %q not found for instance %q", strings.TrimSpace(*segmentID), strings.TrimSpace(*instanceID)), "--segment-id")
				}
			} else if len(segments) > 1 {
				candidates := make([]shared.AmbiguousCandidate, 0, len(segments))
				for _, segment := range segments {
					candidates = append(candidates, shared.AmbiguousCandidate{ID: strings.TrimSpace(segment.ID), Label: fmt.Sprintf("%d bytes", segment.Attributes.SizeInBytes), Extra: strings.TrimSpace(segment.Attributes.Checksum)})
				}
				return fmt.Errorf("analytics download: %w", &shared.AmbiguousSelectionError{
					Kind:        "segment",
					Description: fmt.Sprintf("instance %q", strings.TrimSpace(*instanceID)),
					Flag:        "--segment-id",
					Candidates:  candidates,
					Hint:        "Use --all-segments --output-dir PATH to download the complete report instance.",
				})
			}

			downloadURL := strings.TrimSpace(selectedSegment.Attributes.URL)
			if downloadURL == "" {
				return analyticsNotReadyError(errors.New("analytics download: segment download URL is empty"))
			}

			compressedSize, err := downloadAnalyticsReportToFile(ctx, d, client, downloadURL, compressedPath)
			if err != nil {
				return fmt.Errorf("analytics download: %w", err)
			}

			var decompressedSize int64
			if *decompress {
				decompressedSize, err = d.DecompressGzipFile(compressedPath, decompressedPath)
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

			return d.PrintOutput(result, *outputFlags.OutputFormat, *outputFlags.Pretty)
		},
	}
}
