package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type reviewIAPFinder interface {
	FindReviewIAP(ctx context.Context, appID, iapID string) (webcore.ReviewIAP, bool, error)
}

type reviewIAPMutationOutput struct {
	AppID      string                      `json:"appId"`
	IAPID      string                      `json:"iapId"`
	Operation  string                      `json:"operation"`
	Changed    bool                        `json:"changed"`
	Submission webcore.ReviewIAPSubmission `json:"submission"`
}

func buildReviewIAPMutationRows(payload reviewIAPMutationOutput) [][]string {
	return [][]string{
		{"Mutation", "App ID", valueOrNA(payload.AppID)},
		{"Mutation", "IAP ID", valueOrNA(payload.IAPID)},
		{"Mutation", "Operation", valueOrNA(payload.Operation)},
		{"Mutation", "Changed", strconv.FormatBool(payload.Changed)},
		{"Submission", "Item ID", valueOrNA(payload.Submission.ReviewSubmissionItemID)},
		{"Submission", "Review Draft ID", valueOrNA(payload.Submission.ReviewSubmissionID)},
		{"Submission", "IAP Version ID", valueOrNA(payload.Submission.InAppPurchaseVersionID)},
		{"Submission", "Next Version", strconv.FormatBool(payload.Submission.SubmitWithNextAppStoreVersion)},
	}
}

func renderReviewIAPMutationTable(payload reviewIAPMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderTable(headers, buildReviewIAPMutationRows(payload))
	return nil
}

func renderReviewIAPMutationMarkdown(payload reviewIAPMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderMarkdown(headers, buildReviewIAPMutationRows(payload))
	return nil
}

func validateReviewIAPAttachInputs(appID, iapID string, confirm bool) error {
	switch {
	case strings.TrimSpace(appID) == "":
		return shared.UsageError("--app is required (or set ASC_APP_ID)")
	case shared.SelectorNeedsLookup(appID):
		return shared.UsageError("--app must be a numeric App Store Connect app ID")
	case strings.TrimSpace(iapID) == "":
		return shared.UsageError("--iap-id is required")
	case !confirm:
		return shared.UsageError("--confirm is required")
	default:
		return nil
	}
}

func reviewIAPAmbiguousSelectionError(err error) *shared.AmbiguousSelectionError {
	var ambiguous *webcore.ReviewIAPAmbiguousError
	if !errors.As(err, &ambiguous) || ambiguous == nil {
		return nil
	}
	candidates := make([]shared.AmbiguousCandidate, 0, len(ambiguous.Matches))
	for _, match := range ambiguous.Matches {
		candidates = append(candidates, shared.AmbiguousCandidate{
			ID:    strings.TrimSpace(match.ID),
			Label: strings.TrimSpace(match.ProductID),
			Extra: strings.TrimSpace(match.ReferenceName),
		})
	}
	return &shared.AmbiguousSelectionError{
		Kind:             "in-app purchase",
		Description:      fmt.Sprintf("%q by %s", strings.TrimSpace(ambiguous.Selector), strings.TrimSpace(ambiguous.Field)),
		Flag:             "--iap-id",
		Candidates:       candidates,
		Hint:             "Use the Iris resource ID to disambiguate.",
		DisplayTextLimit: shared.AmbiguousDiagnosticTextLimit,
	}
}

func selectReviewIAPSubmission(submissions []webcore.ReviewSubmission, selectedID string) (webcore.ReviewSubmission, error) {
	var candidates []webcore.ReviewSubmission
	for _, submission := range submissions {
		if selectedID != "" && submission.ID != selectedID {
			continue
		}
		if submission.State == "READY_FOR_REVIEW" && submission.ID != "" {
			candidates = append(candidates, submission)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if selectedID != "" {
		return webcore.ReviewSubmission{}, shared.UsageError("--submission must identify a READY_FOR_REVIEW draft under --app")
	}
	if len(candidates) == 0 {
		return webcore.ReviewSubmission{}, fmt.Errorf("no READY_FOR_REVIEW draft under this app; create a review submission before attaching the IAP")
	}
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
	}
	return webcore.ReviewSubmission{}, shared.UsageErrorf("multiple READY_FOR_REVIEW drafts; select one with --submission: %s", strings.Join(ids, ", "))
}

func findReviewIAPItem(ctx context.Context, client *webcore.Client, submissionID, versionID string) (string, error) {
	items, err := client.ListReviewSubmissionItems(ctx, submissionID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.State == "REMOVED" {
			continue
		}
		for _, related := range item.Related {
			if related.Relationship == "inAppPurchaseVersion" && related.Type == "inAppPurchaseVersions" && related.ID == versionID {
				if item.Type != "reviewSubmissionItems" || item.ID == "" || item.State == "" {
					return "", fmt.Errorf("matching review item has incomplete identity or state; refusing to attach")
				}
				return item.ID, nil
			}
		}
	}
	return "", nil
}

func verifyReviewIAPBelongsToApp(ctx context.Context, client reviewIAPFinder, appID, iapID string) (webcore.ReviewIAP, error) {
	if client == nil {
		return webcore.ReviewIAP{}, fmt.Errorf("app-scoped IAP verification client is required")
	}

	iap, found, err := client.FindReviewIAP(ctx, appID, iapID)
	if err != nil {
		if ambiguous := reviewIAPAmbiguousSelectionError(err); ambiguous != nil {
			return webcore.ReviewIAP{}, fmt.Errorf("verify in-app purchase %q under app %q: %w", iapID, appID, ambiguous)
		}
		return webcore.ReviewIAP{}, fmt.Errorf("verify in-app purchase %q under app %q: %w", iapID, appID, err)
	}
	if !found {
		return webcore.ReviewIAP{}, fmt.Errorf("in-app purchase %q was not found under app %q; refusing to attach", iapID, appID)
	}
	if strings.TrimSpace(iap.ID) == "" {
		return webcore.ReviewIAP{}, fmt.Errorf("in-app purchase %q under app %q is missing an iris resource id; refusing to attach", iapID, appID)
	}
	return iap, nil
}

// WebReviewIAPsCommand groups web operations for staging non-renewing IAPs for review.
func WebReviewIAPsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review iaps", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "iaps",
		ShortUsage: "asc web review iaps <subcommand> [flags]",
		ShortHelp:  "Add non-renewing IAP versions to a review draft.",
		LongHelp: `WEB SESSION WORKFLOWS

Add a non-renewing in-app purchase version to an existing App Review draft.
This stages the IAP for review; it does not submit the draft to Apple.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewIAPsAttachCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewIAPsAttachCommand adds an IAP version to an existing review draft.
func WebReviewIAPsAttachCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review iaps attach", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	iapID := fs.String("iap-id", "", "Iris IAP resource ID, product ID, or exact reference name")
	submissionID := fs.String("submission", "", "Review draft ID (required when multiple READY_FOR_REVIEW drafts exist)")
	confirm := fs.Bool("confirm", false, "Confirm the attach operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach",
		ShortUsage: "asc web review iaps attach --app APP_ID --iap-id IAP_SELECTOR [--submission SUBMISSION_ID] --confirm [flags]",
		ShortHelp:  "Add a non-renewing IAP version to a review draft.",
		LongHelp: `WEB SESSION WORKFLOWS

Add the single editable IAP version to an existing READY_FOR_REVIEW draft.
When several drafts exist, choose one with --submission. No draft is created.
This stages the IAP for review; it does not submit the draft to Apple.
The receipt includes the review draft and IAP version IDs. Its legacy
submission.id is empty because this flow creates no legacy IAP submission;
submission.reviewSubmissionItemId identifies the actual draft item.
The legacy submitWithNextAppStoreVersion field is true only when the selected draft
already has an App Store version associated with it.

The --iap-id selector accepts the Iris resource id, the bundle-style productId,
or the exact current referenceName from ` + "`asc iap list`" + `. Apple's Iris listing does not
expose the public numeric ASC IAP id, so that numeric id is not resolved by this command.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			trimmedIAPID := strings.TrimSpace(*iapID)
			trimmedSubmissionID := strings.TrimSpace(*submissionID)
			if *submissionID != "" && trimmedSubmissionID == "" {
				return shared.UsageError("--submission must not be blank")
			}
			if err := validateReviewIAPAttachInputs(trimmedAppID, trimmedIAPID, *confirm); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			reviewIAP, err := verifyReviewIAPBelongsToApp(requestCtx, client, trimmedAppID, trimmedIAPID)
			if err != nil {
				return fmt.Errorf("web review iaps attach: %w", withReviewSelectorDiagnostic(err, "--iap-id"))
			}
			resolvedV2ID, err := client.ResolveReviewIAPV2ID(requestCtx, trimmedAppID, reviewIAP.ProductID)
			if err != nil {
				return withWebAuthHint(fmt.Errorf("web review iaps attach: resolve V2 in-app purchase: %w", err), "web review iaps attach")
			}
			versionID, err := client.ResolveReviewIAPVersionID(requestCtx, resolvedV2ID)
			if err != nil {
				return withWebAuthHint(fmt.Errorf("web review iaps attach: %w", err), "web review iaps attach")
			}
			submissions, err := client.ListReviewSubmissions(requestCtx, trimmedAppID)
			if err != nil {
				return withWebAuthHint(err, "web review iaps attach")
			}
			draft, err := selectReviewIAPSubmission(submissions, trimmedSubmissionID)
			if err != nil {
				return err
			}
			itemID, err := findReviewIAPItem(requestCtx, client, draft.ID, versionID)
			if err != nil {
				return withWebAuthHint(err, "web review iaps attach")
			}
			changed := false
			if itemID == "" {
				createdID, createErr := withWebSpinnerValue("Adding IAP version to review draft", func() (string, error) {
					return client.CreateReviewIAPSubmissionItem(requestCtx, draft.ID, versionID)
				})
				if createErr != nil && !webcore.IsAlreadyExistsConflict(createErr) {
					return withWebAuthHint(createErr, "web review iaps attach")
				}
				itemID, err = findReviewIAPItem(requestCtx, client, draft.ID, versionID)
				if createErr != nil {
					if err != nil || itemID == "" {
						return withWebAuthHint(createErr, "web review iaps attach")
					}
				} else {
					if err != nil {
						return fmt.Errorf("review item %q was created but could not be verified; inspect submission %q before retrying: %w", createdID, draft.ID, err)
					}
					if itemID != createdID {
						return fmt.Errorf("review item %q was created but is absent from submission %q; inspect it before retrying", createdID, draft.ID)
					}
					changed = true
				}
			}
			payload := reviewIAPMutationOutput{
				AppID:     trimmedAppID,
				IAPID:     trimmedIAPID,
				Operation: "attach",
				Changed:   changed,
				Submission: webcore.ReviewIAPSubmission{
					ReviewSubmissionItemID:        itemID,
					InAppPurchaseID:               resolvedV2ID,
					ReviewSubmissionID:            draft.ID,
					InAppPurchaseVersionID:        versionID,
					SubmitWithNextAppStoreVersion: draft.AppStoreVersionForReview != nil && draft.AppStoreVersionForReview.ID != "",
				},
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewIAPMutationTable(payload) },
				func() error { return renderReviewIAPMutationMarkdown(payload) },
			)
		},
	}
}
