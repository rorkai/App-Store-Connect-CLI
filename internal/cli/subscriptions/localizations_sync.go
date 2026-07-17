package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const localizationSyncArtifactSchemaVersion = 1

type localizationSyncEntry struct {
	Locale string            `json:"locale"`
	Fields map[string]string `json:"fields"`
}

type localizationSyncRemote struct {
	ID     string
	Locale string
	Fields map[string]string
}

type localizationSyncResult struct {
	Locale         string            `json:"locale"`
	Action         string            `json:"action"`
	Status         string            `json:"status"`
	LocalizationID string            `json:"localizationId,omitempty"`
	DesiredFields  map[string]string `json:"desiredFields,omitempty"`
	Error          string            `json:"error,omitempty"`
}

type localizationSyncSummary struct {
	Type                 string                   `json:"type"`
	TargetID             string                   `json:"targetId"`
	InputPath            string                   `json:"inputPath"`
	DryRun               bool                     `json:"dryRun"`
	Total                int                      `json:"total"`
	Planned              int                      `json:"planned"`
	Created              int                      `json:"created"`
	Updated              int                      `json:"updated"`
	Unchanged            int                      `json:"unchanged"`
	Reconciled           int                      `json:"reconciled"`
	Failed               int                      `json:"failed"`
	FailureArtifactPath  string                   `json:"failureArtifactPath,omitempty"`
	FailureArtifactError string                   `json:"failureArtifactError,omitempty"`
	Results              []localizationSyncResult `json:"results"`
}

type localizationSyncFailureArtifact struct {
	SchemaVersion int                          `json:"schemaVersion"`
	Command       string                       `json:"command"`
	Type          string                       `json:"type"`
	TargetID      string                       `json:"targetId"`
	InputPath     string                       `json:"inputPath"`
	GeneratedAt   string                       `json:"generatedAt"`
	Failures      []localizationSyncResult     `json:"failures"`
	RetryInput    map[string]map[string]string `json:"retryInput"`
}

type localizationSyncAPI struct {
	Type   string
	List   func(context.Context) ([]localizationSyncRemote, error)
	Create func(context.Context, localizationSyncEntry) (localizationSyncRemote, error)
	Update func(context.Context, localizationSyncRemote, localizationSyncEntry) (localizationSyncRemote, error)
}

type localizationSyncInputError struct {
	err error
}

func (e localizationSyncInputError) Error() string { return e.err.Error() }
func (e localizationSyncInputError) Unwrap() error { return e.err }

// SubscriptionsLocalizationsSyncCommand returns the experimental subscription localization sync command.
func SubscriptionsLocalizationsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations sync", flag.ExitOnError)
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	inputPath := fs.String("input", "", "Locale-keyed JSON input file")
	dryRun := fs.Bool("dry-run", false, "Preview changes without creating or updating localizations")
	output := shared.BindOutputFlags(fs)

	return shared.DeprecatedCommandWithGuidance(&ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc subscriptions localizations sync --subscription-id \"SUB_ID\" --input \"./localizations.json\" [flags]",
		ShortHelp:  "[experimental] Sync subscription localizations from JSON.",
		LongHelp: `Sync subscription localizations from a locale-keyed JSON file.

This command is experimental. It creates missing locales, updates only fields
present in the input, and leaves omitted locales and fields unchanged. Name
must be non-empty when present; an empty description clears that field.

Input format:
  {
    "en-US": {"name": "Pro", "description": "Premium access"},
    "de-DE": {"name": "Pro"}
  }

Examples:
  asc subscriptions localizations sync --subscription-id "SUB_ID" --input "./localizations.json"
  asc subscriptions localizations sync --subscription-id "SUB_ID" --input "./localizations.json" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("subscriptions localizations sync does not accept positional arguments: %s", strings.Join(args, " "))
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}
			path := strings.TrimSpace(*inputPath)
			if path == "" {
				return shared.UsageError("--input is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			entries, err := readLocalizationSyncEntries(path, "description")
			if err != nil {
				return shared.UsageErrorf("subscriptions localizations sync: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations sync: %w", err)
			}
			id, err = shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
				return resolveSubscriptionLookupID(requestCtx, client, *appID, id)
			})
			if err != nil {
				return err
			}

			api := newSubscriptionLocalizationSyncAPI(client, id)
			return runLocalizationSyncCommand(ctx, api, id, path, entries, *dryRun, "subscriptions localizations sync", output)
		},
	}, "asc subscriptions localizations sync", "No one-command replacement exists. Reconcile each locale through `asc subscriptions versions localizations` list/create/update/delete commands with a subscription version ID.")
}

// SubscriptionsGroupsLocalizationsSyncCommand returns the experimental group localization sync command.
func SubscriptionsGroupsLocalizationsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations sync", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Subscription group ID")
	inputPath := fs.String("input", "", "Locale-keyed JSON input file")
	dryRun := fs.Bool("dry-run", false, "Preview changes without creating or updating localizations")
	output := shared.BindOutputFlags(fs)

	return shared.DeprecatedCommandWithGuidance(&ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc subscriptions groups localizations sync --group-id \"GROUP_ID\" --input \"./localizations.json\" [flags]",
		ShortHelp:  "[experimental] Sync subscription group localizations from JSON.",
		LongHelp: `Sync subscription group localizations from a locale-keyed JSON file.

This command is experimental. It creates missing locales, updates only fields
present in the input, and leaves omitted locales and fields unchanged. Name
must be non-empty when present; an empty customAppName clears that field.

Input format:
  {
    "en-US": {"name": "Premium", "customAppName": "My App"},
    "de-DE": {"name": "Premium"}
  }

Examples:
  asc subscriptions groups localizations sync --group-id "GROUP_ID" --input "./localizations.json"
  asc subscriptions groups localizations sync --group-id "GROUP_ID" --input "./localizations.json" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("subscriptions groups localizations sync does not accept positional arguments: %s", strings.Join(args, " "))
			}
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return shared.UsageError("--group-id is required")
			}
			path := strings.TrimSpace(*inputPath)
			if path == "" {
				return shared.UsageError("--input is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			entries, err := readLocalizationSyncEntries(path, "customAppName")
			if err != nil {
				return shared.UsageErrorf("subscriptions groups localizations sync: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations sync: %w", err)
			}
			api := newSubscriptionGroupLocalizationSyncAPI(client, id)
			return runLocalizationSyncCommand(ctx, api, id, path, entries, *dryRun, "subscriptions groups localizations sync", output)
		},
	}, "asc subscriptions groups localizations sync", "No one-command replacement exists. Reconcile each locale through `asc subscriptions groups versions localizations` list/create/update/delete commands with a subscription group version ID.")
}

func runLocalizationSyncCommand(ctx context.Context, api localizationSyncAPI, targetID, inputPath string, entries []localizationSyncEntry, dryRun bool, command string, output shared.OutputFlags) error {
	summary, err := executeLocalizationSync(ctx, api, targetID, inputPath, entries, dryRun, command)
	var inputErr localizationSyncInputError
	if errors.As(err, &inputErr) {
		return shared.UsageErrorf("%s: %v", command, inputErr)
	}
	if summary == nil {
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		return fmt.Errorf("%s: empty sync result", command)
	}

	if printErr := shared.PrintOutputWithRenderers(
		summary,
		*output.Output,
		*output.Pretty,
		func() error { return renderLocalizationSyncSummary(summary, false) },
		func() error { return renderLocalizationSyncSummary(summary, true) },
	); printErr != nil {
		return printErr
	}
	if err != nil {
		return shared.NewReportedError(err)
	}
	return nil
}

func executeLocalizationSync(ctx context.Context, api localizationSyncAPI, targetID, inputPath string, entries []localizationSyncEntry, dryRun bool, command string) (*localizationSyncSummary, error) {
	remote, err := api.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch existing localizations: %w", err)
	}
	index, err := indexLocalizationSyncRemote(remote)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if _, exists := index[localizationSyncLocaleKey(entry.Locale)]; exists {
			continue
		}
		name, present := entry.Fields["name"]
		if !present || strings.TrimSpace(name) == "" {
			return nil, localizationSyncInputError{err: fmt.Errorf("locale %q does not exist remotely and requires a non-empty name", entry.Locale)}
		}
	}

	summary := &localizationSyncSummary{
		Type:      api.Type,
		TargetID:  strings.TrimSpace(targetID),
		InputPath: filepath.Clean(strings.TrimSpace(inputPath)),
		DryRun:    dryRun,
		Total:     len(entries),
		Results:   make([]localizationSyncResult, 0, len(entries)),
	}

	for i, entry := range entries {
		if err := ctx.Err(); err != nil {
			appendLocalizationSyncCancellationFailures(summary, entries[i:], index, err)
			break
		}

		key := localizationSyncLocaleKey(entry.Locale)
		existing, exists := index[key]
		action := "create"
		if exists {
			action = "update"
			if localizationSyncFieldsMatch(existing.Fields, entry.Fields) {
				summary.Unchanged++
				summary.Results = append(summary.Results, localizationSyncResult{
					Locale:         entry.Locale,
					Action:         "none",
					Status:         "unchanged",
					LocalizationID: existing.ID,
					DesiredFields:  cloneLocalizationSyncFields(entry.Fields),
				})
				continue
			}
		}

		if dryRun {
			summary.Planned++
			summary.Results = append(summary.Results, localizationSyncResult{
				Locale:         entry.Locale,
				Action:         action,
				Status:         "planned",
				LocalizationID: existing.ID,
				DesiredFields:  cloneLocalizationSyncFields(entry.Fields),
			})
			continue
		}

		value, mutationStatus, mutationErr := shared.RunReconciledMutation(
			ctx,
			func(requestCtx context.Context) (localizationSyncRemote, error) {
				if exists {
					return api.Update(requestCtx, existing, entry)
				}
				return api.Create(requestCtx, entry)
			},
			func(readbackCtx context.Context) (localizationSyncRemote, bool, error) {
				refreshed, readErr := api.List(readbackCtx)
				if readErr != nil {
					return localizationSyncRemote{}, false, readErr
				}
				refreshedIndex, indexErr := indexLocalizationSyncRemote(refreshed)
				if indexErr != nil {
					return localizationSyncRemote{}, false, indexErr
				}
				candidate, found := refreshedIndex[key]
				return candidate, found && localizationSyncFieldsMatch(candidate.Fields, entry.Fields), nil
			},
		)
		if mutationErr != nil {
			summary.Failed++
			summary.Results = append(summary.Results, localizationSyncResult{
				Locale:         entry.Locale,
				Action:         action,
				Status:         "failed",
				LocalizationID: existing.ID,
				DesiredFields:  cloneLocalizationSyncFields(entry.Fields),
				Error:          mutationErr.Error(),
			})
			continue
		}

		if strings.TrimSpace(value.ID) == "" {
			value.ID = existing.ID
		}
		if value.Fields == nil {
			value.Fields = cloneLocalizationSyncFields(existing.Fields)
		}
		value.Locale = entry.Locale
		for field, desired := range entry.Fields {
			value.Fields[field] = desired
		}
		index[key] = value

		status := "created"
		if exists {
			status = "updated"
		}
		if mutationStatus == shared.ReconciledMutationRecovered {
			status = "reconciled"
			summary.Reconciled++
		} else if exists {
			summary.Updated++
		} else {
			summary.Created++
		}
		summary.Results = append(summary.Results, localizationSyncResult{
			Locale:         entry.Locale,
			Action:         action,
			Status:         status,
			LocalizationID: value.ID,
			DesiredFields:  cloneLocalizationSyncFields(entry.Fields),
		})
	}

	if summary.Failed == 0 {
		return summary, nil
	}
	path, artifactErr := writeLocalizationSyncFailureArtifact(summary, command)
	if artifactErr != nil {
		summary.FailureArtifactError = artifactErr.Error()
	} else {
		summary.FailureArtifactPath = path
	}

	runErr := fmt.Errorf("%s: %d locale(s) failed", command, summary.Failed)
	if artifactErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("write failure artifact: %w", artifactErr))
	}
	return summary, runErr
}

func appendLocalizationSyncCancellationFailures(summary *localizationSyncSummary, entries []localizationSyncEntry, index map[string]localizationSyncRemote, err error) {
	for _, entry := range entries {
		existing, exists := index[localizationSyncLocaleKey(entry.Locale)]
		action := "create"
		if exists {
			action = "update"
		}
		summary.Failed++
		summary.Results = append(summary.Results, localizationSyncResult{
			Locale:         entry.Locale,
			Action:         action,
			Status:         "failed",
			LocalizationID: existing.ID,
			DesiredFields:  cloneLocalizationSyncFields(entry.Fields),
			Error:          err.Error(),
		})
	}
}

func readLocalizationSyncEntries(path, secondaryField string) ([]localizationSyncEntry, error) {
	payload, err := shared.ReadJSONFilePayload(path)
	if err != nil {
		return nil, fmt.Errorf("read input %q: %w", path, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	start, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("parse input %q: %w", path, err)
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("input %q must be a JSON object", path)
	}

	entries := make([]localizationSyncEntry, 0)
	seen := make(map[string]string)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("parse input %q: %w", path, err)
		}
		rawLocale, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("input %q contains a non-string locale key", path)
		}
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(rawLocale)
		if err != nil {
			return nil, err
		}
		key := localizationSyncLocaleKey(locale)
		if previous, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate canonical locale %q conflicts with %q", locale, previous)
		}
		seen[key] = rawLocale

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("locale %q: %w", locale, err)
		}
		fields, err := readLocalizationSyncFields(raw, secondaryField)
		if err != nil {
			return nil, fmt.Errorf("locale %q: %w", locale, err)
		}
		entries = append(entries, localizationSyncEntry{Locale: locale, Fields: fields})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("parse input %q: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("input %q must include at least one locale", path)
	}

	sort.Slice(entries, func(i, j int) bool {
		left := localizationSyncLocaleKey(entries[i].Locale)
		right := localizationSyncLocaleKey(entries[j].Locale)
		if left == right {
			return entries[i].Locale < entries[j].Locale
		}
		return left < right
	})
	return entries, nil
}

func readLocalizationSyncFields(payload json.RawMessage, secondaryField string) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	start, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("value must be an object")
	}

	fields := make(map[string]string)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		field, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("field name must be a string")
		}
		if _, duplicate := fields[field]; duplicate {
			return nil, fmt.Errorf("duplicate field %q", field)
		}
		if field != "name" && field != secondaryField {
			return nil, fmt.Errorf("unknown field %q", field)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, fmt.Errorf("field %q must be a string, not null", field)
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("field %q must be a string: %w", field, err)
		}
		fields[field] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if name, present := fields["name"]; present && strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("field %q must not be empty", "name")
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one of name or %s is required", secondaryField)
	}
	return fields, nil
}

func newSubscriptionLocalizationSyncAPI(client *asc.Client, subscriptionID string) localizationSyncAPI {
	return localizationSyncAPI{
		Type: "subscriptionLocalizations",
		List: func(ctx context.Context) ([]localizationSyncRemote, error) {
			return fetchSubscriptionLocalizationSyncState(ctx, client, subscriptionID)
		},
		Create: func(ctx context.Context, entry localizationSyncEntry) (localizationSyncRemote, error) {
			attrs := asc.SubscriptionLocalizationCreateAttributes{
				Locale: entry.Locale,
				Name:   entry.Fields["name"],
			}
			if description, ok := entry.Fields["description"]; ok {
				attrs.Description = description
			}
			resp, err := client.CreateSubscriptionLocalization(ctx, subscriptionID, attrs) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			if err != nil {
				return localizationSyncRemote{}, err
			}
			return subscriptionLocalizationSyncRemote(resp.Data), nil
		},
		Update: func(ctx context.Context, existing localizationSyncRemote, entry localizationSyncEntry) (localizationSyncRemote, error) {
			attrs := asc.SubscriptionLocalizationUpdateAttributes{}
			if name, ok := entry.Fields["name"]; ok {
				attrs.Name = &name
			}
			if description, ok := entry.Fields["description"]; ok {
				attrs.Description = &description
			}
			resp, err := client.UpdateSubscriptionLocalization(ctx, existing.ID, attrs) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			if err != nil {
				return localizationSyncRemote{}, err
			}
			return subscriptionLocalizationSyncRemote(resp.Data), nil
		},
	}
}

func newSubscriptionGroupLocalizationSyncAPI(client *asc.Client, groupID string) localizationSyncAPI {
	return localizationSyncAPI{
		Type: "subscriptionGroupLocalizations",
		List: func(ctx context.Context) ([]localizationSyncRemote, error) {
			return fetchSubscriptionGroupLocalizationSyncState(ctx, client, groupID)
		},
		Create: func(ctx context.Context, entry localizationSyncEntry) (localizationSyncRemote, error) {
			attrs := asc.SubscriptionGroupLocalizationCreateAttributes{
				Locale: entry.Locale,
				Name:   entry.Fields["name"],
			}
			if customAppName, ok := entry.Fields["customAppName"]; ok {
				attrs.CustomAppName = customAppName
			}
			resp, err := client.CreateSubscriptionGroupLocalization(ctx, groupID, attrs) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			if err != nil {
				return localizationSyncRemote{}, err
			}
			return subscriptionGroupLocalizationSyncRemote(resp.Data), nil
		},
		Update: func(ctx context.Context, existing localizationSyncRemote, entry localizationSyncEntry) (localizationSyncRemote, error) {
			attrs := asc.SubscriptionGroupLocalizationUpdateAttributes{}
			if name, ok := entry.Fields["name"]; ok {
				attrs.Name = &name
			}
			if customAppName, ok := entry.Fields["customAppName"]; ok {
				attrs.CustomAppName = &customAppName
			}
			resp, err := client.UpdateSubscriptionGroupLocalization(ctx, existing.ID, attrs) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			if err != nil {
				return localizationSyncRemote{}, err
			}
			return subscriptionGroupLocalizationSyncRemote(resp.Data), nil
		},
	}
}

func fetchSubscriptionLocalizationSyncState(ctx context.Context, client *asc.Client, subscriptionID string) ([]localizationSyncRemote, error) {
	first, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionLocalizationsResponse, error) {
		return client.GetSubscriptionLocalizations( //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			requestCtx,
			subscriptionID,
			asc.WithSubscriptionLocalizationsFields([]string{"description", "locale", "name"}),
			asc.WithSubscriptionLocalizationsLimit(200),
		)
	})
	if err != nil {
		return nil, err
	}
	aggregated, err := asc.PaginateAll(ctx, first, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextURL, mergeErr := shared.MergeNextURLQuery(nextURL, url.Values{
			"fields[subscriptionLocalizations]": []string{"description,locale,name"},
		})
		if mergeErr != nil {
			return nil, mergeErr
		}
		return shared.RetryReadWithFreshTimeout(pageCtx, func(requestCtx context.Context) (*asc.SubscriptionLocalizationsResponse, error) {
			return client.GetSubscriptionLocalizations(requestCtx, subscriptionID, asc.WithSubscriptionLocalizationsNextURL(nextURL)) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
		})
	})
	if err != nil {
		return nil, err
	}
	response, ok := aggregated.(*asc.SubscriptionLocalizationsResponse)
	if !ok || response == nil {
		return nil, fmt.Errorf("unexpected subscription localizations response type %T", aggregated)
	}
	result := make([]localizationSyncRemote, 0, len(response.Data))
	for _, resource := range response.Data {
		result = append(result, subscriptionLocalizationSyncRemote(resource))
	}
	return result, nil
}

func fetchSubscriptionGroupLocalizationSyncState(ctx context.Context, client *asc.Client, groupID string) ([]localizationSyncRemote, error) {
	first, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionGroupLocalizationsResponse, error) {
		return client.GetSubscriptionGroupLocalizations( //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			requestCtx,
			groupID,
			asc.WithSubscriptionGroupLocalizationsFields([]string{"customAppName", "locale", "name"}),
			asc.WithSubscriptionGroupLocalizationsLimit(200),
		)
	})
	if err != nil {
		return nil, err
	}
	aggregated, err := asc.PaginateAll(ctx, first, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextURL, mergeErr := shared.MergeNextURLQuery(nextURL, url.Values{
			"fields[subscriptionGroupLocalizations]": []string{"customAppName,locale,name"},
		})
		if mergeErr != nil {
			return nil, mergeErr
		}
		return shared.RetryReadWithFreshTimeout(pageCtx, func(requestCtx context.Context) (*asc.SubscriptionGroupLocalizationsResponse, error) {
			return client.GetSubscriptionGroupLocalizations(requestCtx, groupID, asc.WithSubscriptionGroupLocalizationsNextURL(nextURL)) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
		})
	})
	if err != nil {
		return nil, err
	}
	response, ok := aggregated.(*asc.SubscriptionGroupLocalizationsResponse)
	if !ok || response == nil {
		return nil, fmt.Errorf("unexpected subscription group localizations response type %T", aggregated)
	}
	result := make([]localizationSyncRemote, 0, len(response.Data))
	for _, resource := range response.Data {
		result = append(result, subscriptionGroupLocalizationSyncRemote(resource))
	}
	return result, nil
}

func subscriptionLocalizationSyncRemote(resource asc.Resource[asc.SubscriptionLocalizationAttributes]) localizationSyncRemote {
	return localizationSyncRemote{
		ID:     strings.TrimSpace(resource.ID),
		Locale: resource.Attributes.Locale,
		Fields: map[string]string{
			"name":        resource.Attributes.Name,
			"description": resource.Attributes.Description,
		},
	}
}

func subscriptionGroupLocalizationSyncRemote(resource asc.Resource[asc.SubscriptionGroupLocalizationAttributes]) localizationSyncRemote {
	return localizationSyncRemote{
		ID:     strings.TrimSpace(resource.ID),
		Locale: resource.Attributes.Locale,
		Fields: map[string]string{
			"name":          resource.Attributes.Name,
			"customAppName": resource.Attributes.CustomAppName,
		},
	}
}

func indexLocalizationSyncRemote(remote []localizationSyncRemote) (map[string]localizationSyncRemote, error) {
	index := make(map[string]localizationSyncRemote, len(remote))
	for _, item := range remote {
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(item.Locale)
		if err != nil {
			return nil, fmt.Errorf("remote localization %q: %w", item.ID, err)
		}
		key := localizationSyncLocaleKey(locale)
		if previous, exists := index[key]; exists {
			return nil, fmt.Errorf("duplicate remote locale %q for localizations %q and %q", locale, previous.ID, item.ID)
		}
		item.Locale = locale
		if item.Fields == nil {
			item.Fields = make(map[string]string)
		}
		index[key] = item
	}
	return index, nil
}

func localizationSyncLocaleKey(locale string) string {
	return strings.ToLower(shared.NormalizeLocaleCode(locale))
}

func localizationSyncFieldsMatch(remote, desired map[string]string) bool {
	for field, value := range desired {
		if remote[field] != value {
			return false
		}
	}
	return true
}

func cloneLocalizationSyncFields(fields map[string]string) map[string]string {
	result := make(map[string]string, len(fields))
	for field, value := range fields {
		result[field] = value
	}
	return result
}

func writeLocalizationSyncFailureArtifact(summary *localizationSyncSummary, command string) (string, error) {
	failures := make([]localizationSyncResult, 0, summary.Failed)
	for _, result := range summary.Results {
		if result.Status == "failed" {
			failures = append(failures, result)
		}
	}
	artifact := localizationSyncFailureArtifact{
		SchemaVersion: localizationSyncArtifactSchemaVersion,
		Command:       command,
		Type:          summary.Type,
		TargetID:      summary.TargetID,
		InputPath:     summary.InputPath,
		GeneratedAt:   subscriptionImportNow().Format("2006-01-02T15:04:05Z07:00"),
		Failures:      failures,
		RetryInput:    make(map[string]map[string]string, len(failures)),
	}
	for _, failure := range failures {
		artifact.RetryInput[failure.Locale] = cloneLocalizationSyncFields(failure.DesiredFields)
	}
	payload, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}
	directory := strings.ReplaceAll(strings.TrimSpace(command), " ", "-")
	path := filepath.Join(".asc", "reports", directory, fmt.Sprintf("failures-%d.json", subscriptionImportNow().UnixNano()))
	if _, err := shared.WriteStreamToFile(path, bytes.NewReader(payload)); err != nil {
		return "", err
	}
	return path, nil
}

func renderLocalizationSyncSummary(summary *localizationSyncSummary, markdown bool) error {
	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}
	render(
		[]string{"Type", "Target ID", "Input", "Dry Run", "Total", "Planned", "Created", "Updated", "Unchanged", "Reconciled", "Failed", "Failure Artifact", "Artifact Error"},
		[][]string{{
			summary.Type,
			summary.TargetID,
			summary.InputPath,
			strconv.FormatBool(summary.DryRun),
			strconv.Itoa(summary.Total),
			strconv.Itoa(summary.Planned),
			strconv.Itoa(summary.Created),
			strconv.Itoa(summary.Updated),
			strconv.Itoa(summary.Unchanged),
			strconv.Itoa(summary.Reconciled),
			strconv.Itoa(summary.Failed),
			summary.FailureArtifactPath,
			summary.FailureArtifactError,
		}},
	)
	rows := make([][]string, 0, len(summary.Results))
	for _, result := range summary.Results {
		desired, _ := json.Marshal(result.DesiredFields)
		rows = append(rows, []string{result.Locale, result.Action, result.Status, result.LocalizationID, string(desired), result.Error})
	}
	render([]string{"Locale", "Action", "Status", "Localization ID", "Desired Fields", "Error"}, rows)
	return nil
}
