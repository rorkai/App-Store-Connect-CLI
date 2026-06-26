package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsIntroductoryOffersImportCommand returns the introductory offers import subcommand.
func SubscriptionsIntroductoryOffersImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers import", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	inputPath := fs.String("input", "", "Input CSV file path (required)")
	offerDuration := fs.String("offer-duration", "", "Default offer duration")
	offerMode := fs.String("offer-mode", "", "Default offer mode")
	numberOfPeriods := fs.Int("number-of-periods", 0, "Default number of periods")
	startDate := fs.String("start-date", "", "Default start date (YYYY-MM-DD)")
	endDate := fs.String("end-date", "", "Default end date (YYYY-MM-DD)")
	dryRun := fs.Bool("dry-run", false, "Validate input and print summary without creating offers")
	continueOnError := fs.Bool("continue-on-error", true, "Continue processing rows after runtime failures (default true)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc subscriptions introductory-offers import --subscription-id \"SUB_ID\" --input \"./offers.csv\" [flags]",
		ShortHelp:  "Import introductory offers from a CSV file.",
		LongHelp: `Import introductory offers from a CSV file.

CSV is UTF-8 with a required header row.

Required column:
  territory

Optional columns:
  offer_mode, offer_duration, number_of_periods, start_date, end_date, price_point_id

Header aliases:
  price_point -> price_point_id

Territory values:
  3-letter ASC territory IDs, 2-letter country codes, and English territory names

Precedence:
  Row values override command-level defaults.

Examples:
  asc subscriptions introductory-offers import --subscription-id "SUB_ID" --input "./offers.csv"
  asc subscriptions introductory-offers import --subscription-id "SUB_ID" --input "./offers.csv" --offer-duration ONE_WEEK --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions introductory-offers import --subscription-id "SUB_ID" --input "./offers.csv" --dry-run --offer-duration ONE_WEEK --offer-mode FREE_TRIAL --number-of-periods 1`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*subscriptionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*inputPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required")
				return shared.MissingRequiredUsageError()
			}
			normalizedOfferDuration := ""
			if strings.TrimSpace(*offerDuration) != "" {
				duration, err := normalizeSubscriptionOfferDuration(*offerDuration)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedOfferDuration = string(duration)
			}
			normalizedOfferMode := ""
			if strings.TrimSpace(*offerMode) != "" {
				mode, err := normalizeSubscriptionOfferMode(*offerMode)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedOfferMode = string(mode)
			}
			if *numberOfPeriods < 0 {
				fmt.Fprintln(os.Stderr, "Error: --number-of-periods must be greater than or equal to 0")
				return flag.ErrHelp
			}
			normalizedStartDate := ""
			if strings.TrimSpace(*startDate) != "" {
				date, err := shared.NormalizeDate(*startDate, "--start-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedStartDate = date
			}
			normalizedEndDate := ""
			if strings.TrimSpace(*endDate) != "" {
				date, err := shared.NormalizeDate(*endDate, "--end-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedEndDate = date
			}
			rows, err := readSubscriptionIntroductoryOffersImportCSV(*inputPath)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: %w", err)
			}
			defaults := buildSubscriptionIntroductoryOfferImportDefaults(
				normalizedOfferDuration,
				normalizedOfferMode,
				*numberOfPeriods,
				normalizedStartDate,
				normalizedEndDate,
			)
			resolvedRows, err := resolveSubscriptionIntroductoryOfferImportRows(rows, defaults)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: %w", err)
			}
			summary := &subscriptionIntroductoryOfferImportSummary{
				SubscriptionID:  strings.TrimSpace(*subscriptionID),
				InputFile:       filepath.Clean(strings.TrimSpace(*inputPath)),
				DryRun:          *dryRun,
				ContinueOnError: *continueOnError,
				Total:           len(resolvedRows),
			}

			if *dryRun {
				summary.Created = len(resolvedRows)
				for _, row := range resolvedRows {
					summary.Results = append(summary.Results, subscriptionIntroductoryOfferImportResultItem{
						Row:             row.row,
						Territory:       row.territory,
						OfferMode:       row.offerMode,
						OfferDuration:   row.offerDuration,
						NumberOfPeriods: row.numberOfPeriods,
						StartDate:       row.startDate,
						EndDate:         row.endDate,
						PricePointID:    row.pricePointID,
						TargetPlanType:  string(row.planType),
						Status:          "planned",
					})
				}
				return shared.PrintOutputWithRenderers(
					summary,
					*output.Output,
					*output.Pretty,
					func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, false) },
					func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, true) },
				)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: %w", err)
			}

			summary.SubscriptionID, err = shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
				return resolveSubscriptionLookupID(requestCtx, client, *appID, summary.SubscriptionID)
			})
			if err != nil {
				return err
			}
			stateIndex, err := fetchSubscriptionIntroductoryOfferImportState(ctx, client, summary.SubscriptionID)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: fetch existing offers: %w", err)
			}

			for _, row := range resolvedRows {
				attrs := asc.SubscriptionIntroductoryOfferCreateAttributes{
					Duration:                   asc.SubscriptionOfferDuration(row.offerDuration),
					OfferMode:                  asc.SubscriptionOfferMode(row.offerMode),
					NumberOfPeriods:            row.numberOfPeriods,
					TargetSubscriptionPlanType: row.planType,
				}
				if row.startDate != "" {
					attrs.StartDate = row.startDate
				}
				if row.endDate != "" {
					attrs.EndDate = row.endDate
				}

				status := reconciledMutationSkipped
				var rowErr error
				if !stateIndex.matches(row) {
					status, rowErr = runReconciledMutation(
						ctx,
						func(readbackCtx context.Context) (bool, error) {
							refreshed, err := fetchSubscriptionIntroductoryOfferImportState(readbackCtx, client, summary.SubscriptionID)
							if err != nil {
								return false, err
							}
							stateIndex = refreshed
							return stateIndex.matches(row), nil
						},
						func(mutationCtx context.Context) error {
							_, err := client.CreateSubscriptionIntroductoryOffer(mutationCtx, summary.SubscriptionID, attrs, row.territory, row.pricePointID)
							return err
						},
					)
				}
				if rowErr != nil {
					appendSubscriptionIntroductoryOfferImportFailure(summary, row, rowErr)
					if !*continueOnError {
						break
					}
					continue
				}

				summary.Results = append(summary.Results, subscriptionIntroductoryOfferImportResultItem{
					Row:             row.row,
					Territory:       row.territory,
					OfferMode:       row.offerMode,
					OfferDuration:   row.offerDuration,
					NumberOfPeriods: row.numberOfPeriods,
					StartDate:       row.startDate,
					EndDate:         row.endDate,
					PricePointID:    row.pricePointID,
					TargetPlanType:  string(row.planType),
					Status:          string(status),
				})
				if status != reconciledMutationSkipped {
					stateIndex.add(row)
				}
				switch status {
				case reconciledMutationCreated:
					summary.Created++
				case reconciledMutationSkipped:
					summary.Skipped++
				case reconciledMutationReconciled:
					summary.Reconciled++
				}
			}

			if summary.Failed > 0 {
				artifactPath, artifactErr := writeSubscriptionIntroductoryOfferImportFailureArtifact(summary)
				if artifactErr != nil {
					summary.FailureArtifactError = artifactErr.Error()
				} else {
					summary.FailureArtifactPath = artifactPath
				}
			}

			if err := shared.PrintOutputWithRenderers(
				summary,
				*output.Output,
				*output.Pretty,
				func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, false) },
				func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, true) },
			); err != nil {
				return err
			}
			if summary.Failed > 0 {
				rowErr := fmt.Errorf("subscriptions introductory-offers import: %d row(s) failed", summary.Failed)
				if summary.FailureArtifactError != "" {
					rowErr = errors.Join(rowErr, fmt.Errorf("write failure artifact: %s", summary.FailureArtifactError))
				}
				return shared.NewReportedError(rowErr)
			}
			return nil
		},
	}
}

type subscriptionIntroductoryOfferImportSummary struct {
	SubscriptionID       string                                              `json:"subscriptionId"`
	InputFile            string                                              `json:"inputFile"`
	DryRun               bool                                                `json:"dryRun"`
	ContinueOnError      bool                                                `json:"continueOnError"`
	Total                int                                                 `json:"total"`
	Created              int                                                 `json:"created"`
	Skipped              int                                                 `json:"skipped,omitempty"`
	Reconciled           int                                                 `json:"reconciled,omitempty"`
	Failed               int                                                 `json:"failed"`
	Failures             []subscriptionIntroductoryOfferImportSummaryFailure `json:"failures,omitempty"`
	FailureArtifactPath  string                                              `json:"failureArtifactPath,omitempty"`
	FailureArtifactError string                                              `json:"failureArtifactError,omitempty"`
	Results              []subscriptionIntroductoryOfferImportResultItem     `json:"results,omitempty"`
}

type subscriptionIntroductoryOfferImportSummaryFailure struct {
	Row       int    `json:"row"`
	Territory string `json:"territory,omitempty"`
	Error     string `json:"error"`
}

type subscriptionIntroductoryOfferImportResultItem struct {
	Row             int    `json:"row"`
	Territory       string `json:"territory,omitempty"`
	OfferMode       string `json:"offerMode,omitempty"`
	OfferDuration   string `json:"offerDuration,omitempty"`
	NumberOfPeriods int    `json:"numberOfPeriods"`
	StartDate       string `json:"startDate,omitempty"`
	EndDate         string `json:"endDate,omitempty"`
	PricePointID    string `json:"pricePointId,omitempty"`
	TargetPlanType  string `json:"targetSubscriptionPlanType"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}

type subscriptionIntroductoryOfferImportStateIndex struct {
	offers []subscriptionIntroductoryOfferImportResolvedRow
	now    time.Time
}

type subscriptionIntroductoryOfferImportFailureArtifact struct {
	SchemaVersion  int                                             `json:"schemaVersion"`
	Command        string                                          `json:"command"`
	SubscriptionID string                                          `json:"subscriptionId"`
	InputFile      string                                          `json:"inputFile"`
	Failed         int                                             `json:"failed"`
	GeneratedAt    string                                          `json:"generatedAt"`
	Results        []subscriptionIntroductoryOfferImportResultItem `json:"results"`
}

func renderSubscriptionIntroductoryOfferImportSummary(summary *subscriptionIntroductoryOfferImportSummary, markdown bool) error {
	if summary == nil {
		return fmt.Errorf("summary is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Subscription ID", "Input File", "Dry Run", "Total", "Created", "Skipped", "Reconciled", "Failed", "Failure Artifact", "Failure Artifact Error"},
		[][]string{{
			summary.SubscriptionID,
			summary.InputFile,
			fmt.Sprintf("%t", summary.DryRun),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Skipped),
			fmt.Sprintf("%d", summary.Reconciled),
			fmt.Sprintf("%d", summary.Failed),
			summary.FailureArtifactPath,
			summary.FailureArtifactError,
		}},
	)

	if len(summary.Failures) > 0 {
		rows := make([][]string, 0, len(summary.Failures))
		for _, failure := range summary.Failures {
			rows = append(rows, []string{
				fmt.Sprintf("%d", failure.Row),
				failure.Territory,
				failure.Error,
			})
		}
		render([]string{"Row", "Territory", "Error"}, rows)
	}

	return nil
}

func appendSubscriptionIntroductoryOfferImportFailure(summary *subscriptionIntroductoryOfferImportSummary, row subscriptionIntroductoryOfferImportResolvedRow, err error) {
	if summary == nil || err == nil {
		return
	}
	summary.Failed++
	summary.Failures = append(summary.Failures, subscriptionIntroductoryOfferImportSummaryFailure{
		Row:       row.row,
		Territory: row.territory,
		Error:     err.Error(),
	})
	summary.Results = append(summary.Results, subscriptionIntroductoryOfferImportResultItem{
		Row:             row.row,
		Territory:       row.territory,
		OfferMode:       row.offerMode,
		OfferDuration:   row.offerDuration,
		NumberOfPeriods: row.numberOfPeriods,
		StartDate:       row.startDate,
		EndDate:         row.endDate,
		PricePointID:    row.pricePointID,
		TargetPlanType:  string(row.planType),
		Status:          "failed",
		Error:           err.Error(),
	})
}

func fetchSubscriptionIntroductoryOfferImportState(ctx context.Context, client *asc.Client, subscriptionID string) (*subscriptionIntroductoryOfferImportStateIndex, error) {
	query := url.Values{
		"fields[subscriptionIntroductoryOffers]": []string{"startDate,endDate,duration,offerMode,numberOfPeriods,targetSubscriptionPlanType,territory,subscriptionPricePoint"},
		"include":                                []string{"territory,subscriptionPricePoint"},
		"limit":                                  []string{"200"},
	}
	fetchPage := func(nextURL string) (*asc.SubscriptionIntroductoryOffersResponse, error) {
		if nextURL != "" {
			mergedNext, err := mergeSubscriptionPricesNextQuery(nextURL, query)
			if err != nil {
				return nil, err
			}
			return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionIntroductoryOffersResponse, error) {
				return client.GetSubscriptionIntroductoryOffers(requestCtx, subscriptionID, asc.WithSubscriptionIntroductoryOffersNextURL(mergedNext))
			})
		}
		return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionIntroductoryOffersResponse, error) {
			return client.GetSubscriptionIntroductoryOffers(
				requestCtx,
				subscriptionID,
				asc.WithSubscriptionIntroductoryOffersLimit(200),
				asc.WithSubscriptionIntroductoryOffersFields([]string{"startDate", "endDate", "duration", "offerMode", "numberOfPeriods", "targetSubscriptionPlanType", "territory", "subscriptionPricePoint"}),
				asc.WithSubscriptionIntroductoryOffersInclude([]string{"territory", "subscriptionPricePoint"}),
			)
		})
	}

	firstPage, err := fetchPage("")
	if err != nil {
		return nil, err
	}
	index := &subscriptionIntroductoryOfferImportStateIndex{}
	err = asc.PaginateEach(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return fetchPage(nextURL)
	}, func(page asc.PaginatedResponse) error {
		offers, ok := page.(*asc.SubscriptionIntroductoryOffersResponse)
		if !ok {
			return fmt.Errorf("unexpected introductory offers response type %T", page)
		}
		for _, offer := range offers.Data {
			attrs := offer.Attributes
			index.offers = append(index.offers, subscriptionIntroductoryOfferImportResolvedRow{
				territory:       introductoryOfferTerritoryID(offer),
				offerMode:       string(attrs.OfferMode),
				offerDuration:   string(attrs.Duration),
				numberOfPeriods: attrs.NumberOfPeriods,
				startDate:       strings.TrimSpace(attrs.StartDate),
				endDate:         strings.TrimSpace(attrs.EndDate),
				pricePointID:    extractResourceRelationshipID(offer.Relationships, "subscriptionPricePoint"),
				planType:        attrs.TargetSubscriptionPlanType,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	index.now = subscriptionImportNow()
	return index, nil
}

func (index *subscriptionIntroductoryOfferImportStateIndex) matches(target subscriptionIntroductoryOfferImportResolvedRow) bool {
	if index == nil {
		return false
	}
	for _, offer := range index.offers {
		if strings.EqualFold(offer.territory, target.territory) &&
			offer.pricePointID == target.pricePointID &&
			offer.planType == target.planType &&
			offer.offerDuration == target.offerDuration &&
			offer.offerMode == target.offerMode &&
			offer.numberOfPeriods == target.numberOfPeriods &&
			introductoryOfferStartDateMatches(offer.startDate, target.startDate, index.now) &&
			offer.endDate == target.endDate {
			return true
		}
	}
	return false
}

func (index *subscriptionIntroductoryOfferImportStateIndex) add(target subscriptionIntroductoryOfferImportResolvedRow) {
	if index == nil {
		return
	}
	if strings.TrimSpace(target.startDate) == "" {
		target.startDate = dateOnlyUTC(index.now).Format(equalizeDateLayout)
	}
	index.offers = append(index.offers, target)
}

func introductoryOfferStartDateMatches(actual, target string, now time.Time) bool {
	actual = strings.TrimSpace(actual)
	target = strings.TrimSpace(target)
	if target != "" {
		return actual == target
	}
	if actual == "" {
		return true
	}
	parsed, err := time.Parse(equalizeDateLayout, actual)
	return err == nil && !parsed.After(dateOnlyUTC(now))
}

func extractResourceRelationshipID(relationships json.RawMessage, key string) string {
	if len(relationships) == 0 {
		return ""
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(relationships, &values); err != nil {
		return ""
	}
	var relationship struct {
		Data asc.ResourceData `json:"data"`
	}
	if err := json.Unmarshal(values[key], &relationship); err != nil {
		return ""
	}
	return strings.TrimSpace(relationship.Data.ID)
}

func writeSubscriptionIntroductoryOfferImportFailureArtifact(summary *subscriptionIntroductoryOfferImportSummary) (string, error) {
	failures := make([]subscriptionIntroductoryOfferImportResultItem, 0, summary.Failed)
	for _, result := range summary.Results {
		if result.Status == "failed" {
			failures = append(failures, result)
		}
	}
	artifact := subscriptionIntroductoryOfferImportFailureArtifact{
		SchemaVersion:  1,
		Command:        "subscriptions offers introductory import",
		SubscriptionID: summary.SubscriptionID,
		InputFile:      summary.InputFile,
		Failed:         summary.Failed,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Results:        failures,
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(".asc", "reports", "subscription-introductory-offers-import", fmt.Sprintf("failures-%d.json", time.Now().UTC().UnixNano()))
	if _, err := shared.WriteStreamToFile(path, bytes.NewReader(data)); err != nil {
		return "", err
	}
	return path, nil
}
