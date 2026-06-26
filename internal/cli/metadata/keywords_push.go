package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type metadataKeywordsPushSummary struct {
	VersionID           string                           `json:"versionId"`
	InputFile           string                           `json:"inputFile"`
	ContinueOnError     bool                             `json:"continueOnError"`
	Total               int                              `json:"total"`
	Created             int                              `json:"created"`
	Updated             int                              `json:"updated"`
	Succeeded           int                              `json:"succeeded"`
	Failed              int                              `json:"failed"`
	FailureArtifactPath string                           `json:"failureArtifactPath,omitempty"`
	Results             []metadataKeywordsPushResultItem `json:"results"`
}

type metadataKeywordsPushResultItem struct {
	Locale         string `json:"locale"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	LocalizationID string `json:"localizationId,omitempty"`
	Error          string `json:"error,omitempty"`
}

type metadataKeywordsPushFailureArtifact struct {
	VersionID   string                           `json:"versionId"`
	InputFile   string                           `json:"inputFile"`
	Failed      int                              `json:"failed"`
	GeneratedAt string                           `json:"generatedAt"`
	Results     []metadataKeywordsPushResultItem `json:"results"`
}

type metadataKeywordsPushEntry struct {
	Locale   string
	Keywords string
}

type metadataKeywordsPushInputObject struct {
	Keywords *string `json:"keywords"`
}

// MetadataKeywordsPushCommand pushes locale-keyed keyword input directly to ASC.
func MetadataKeywordsPushCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords push", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	inputPath := fs.String("input", "", "Input JSON file path (required)")
	continueOnError := fs.String("continue-on-error", "true", "Continue processing locales after failures (default true)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "push",
		ShortUsage: "asc metadata keywords push --version-id \"VERSION_ID\" --input \"./keywords.json\" [flags]",
		ShortHelp:  "Push locale-keyed keywords directly to an App Store version.",
		LongHelp: `Push locale-keyed keywords directly to an App Store version.

This command bypasses local metadata files and mutates App Store Connect
version localizations directly by App Store version ID.

Input JSON must be an object keyed by locale. Each value may be either:
  - a keyword string
  - an object with a "keywords" field

Examples:
  asc metadata keywords push --version-id "VERSION_ID" --input "./keywords.json"
  asc metadata keywords push --version-id "VERSION_ID" --input "./keywords.json" --continue-on-error=false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords push does not accept positional arguments")
			}

			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}

			inputValue := strings.TrimSpace(*inputPath)
			if inputValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required")
				return shared.MissingRequiredUsageError()
			}

			entries, err := readMetadataKeywordsPushEntries(inputValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			continueOnErrorValue, err := shared.ParseBoolFlag(*continueOnError, "--continue-on-error")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("metadata keywords push: %w", err)
			}

			listCtx, listCancel := shared.ContextWithTimeout(ctx)
			existing, err := client.GetAppStoreVersionLocalizations(listCtx, vid, asc.WithAppStoreVersionLocalizationsLimit(200))
			listCancel()
			if err != nil {
				return fmt.Errorf("metadata keywords push: failed to fetch localizations: %w", err)
			}

			existingByLocale := make(map[string]asc.Resource[asc.AppStoreVersionLocalizationAttributes])
			for _, item := range existing.Data {
				localeKey := strings.ToLower(strings.TrimSpace(item.Attributes.Locale))
				if localeKey == "" {
					continue
				}
				existingByLocale[localeKey] = item
			}

			summary := &metadataKeywordsPushSummary{
				VersionID:       vid,
				InputFile:       filepath.Clean(inputValue),
				ContinueOnError: continueOnErrorValue,
				Total:           len(entries),
				Results:         make([]metadataKeywordsPushResultItem, 0, len(entries)),
			}

			for _, entry := range entries {
				result := metadataKeywordsPushResultItem{Locale: entry.Locale}
				existingItem, exists := existingByLocale[strings.ToLower(entry.Locale)]

				if exists {
					result.Action = "update"

					updateCtx, updateCancel := shared.ContextWithTimeout(ctx)
					resp, updateErr := client.UpdateAppStoreVersionLocalization(updateCtx, existingItem.ID, asc.AppStoreVersionLocalizationAttributes{
						Keywords: entry.Keywords,
					})
					updateCancel()
					if updateErr != nil {
						result.Status = "failed"
						result.Error = fmt.Sprintf("update locale %q keywords: %v", entry.Locale, updateErr)
						summary.Failed++
						summary.Results = append(summary.Results, result)
						if !continueOnErrorValue {
							break
						}
						continue
					}

					result.Status = "succeeded"
					result.LocalizationID = resp.Data.ID
					summary.Updated++
					summary.Succeeded++
					summary.Results = append(summary.Results, result)
					continue
				}

				result.Action = "create"

				createCtx, createCancel := shared.ContextWithTimeout(ctx)
				resp, createErr := client.CreateAppStoreVersionLocalization(createCtx, vid, asc.AppStoreVersionLocalizationAttributes{
					Locale:   entry.Locale,
					Keywords: entry.Keywords,
				})
				createCancel()
				if createErr != nil {
					result.Status = "failed"
					result.Error = fmt.Sprintf("create locale %q keywords: %v", entry.Locale, createErr)
					summary.Failed++
					summary.Results = append(summary.Results, result)
					if !continueOnErrorValue {
						break
					}
					continue
				}

				result.Status = "succeeded"
				result.LocalizationID = resp.Data.ID
				summary.Created++
				summary.Succeeded++
				summary.Results = append(summary.Results, result)
			}

			if summary.Failed > 0 {
				artifactPath, artifactErr := writeMetadataKeywordsPushFailureArtifact(summary)
				if artifactErr != nil {
					return fmt.Errorf("metadata keywords push: write failure artifact: %w", artifactErr)
				}
				summary.FailureArtifactPath = artifactPath
			}

			if err := shared.PrintOutputWithRenderers(
				summary,
				*output.Output,
				*output.Pretty,
				func() error { return renderMetadataKeywordsPushSummary(summary, false) },
				func() error { return renderMetadataKeywordsPushSummary(summary, true) },
			); err != nil {
				return err
			}

			if summary.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("metadata keywords push: %d locale(s) failed", summary.Failed))
			}
			return nil
		},
	}
}

func readMetadataKeywordsPushEntries(path string) ([]metadataKeywordsPushEntry, error) {
	payload, err := shared.ReadJSONFilePayload(path)
	if err != nil {
		return nil, fmt.Errorf("read input %q: %w", path, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse input %q: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("input %q must include at least one locale entry", path)
	}

	entries := make([]metadataKeywordsPushEntry, 0, len(raw))
	seen := make(map[string]string, len(raw))

	for locale, value := range raw {
		normalizedLocale := strings.TrimSpace(locale)
		if normalizedLocale == "" {
			return nil, fmt.Errorf("locale keys must not be empty")
		}
		canonicalLocale, err := validateMetadataKeywordLocale(normalizedLocale)
		if err != nil {
			return nil, err
		}

		lower := strings.ToLower(canonicalLocale)
		if previous, ok := seen[lower]; ok {
			return nil, fmt.Errorf("duplicate locale %q conflicts with %q", canonicalLocale, previous)
		}
		seen[lower] = canonicalLocale

		keywords, err := parseMetadataKeywordsPushKeywords(value)
		if err != nil {
			return nil, fmt.Errorf("invalid entry for locale %q: %w", canonicalLocale, err)
		}
		trimmedKeywords := strings.TrimSpace(keywords)
		if trimmedKeywords == "" {
			return nil, fmt.Errorf("invalid entry for locale %q: keywords must not be empty", canonicalLocale)
		}
		if err := shared.ValidateVersionLocalizationAttributes(asc.AppStoreVersionLocalizationAttributes{Keywords: trimmedKeywords}); err != nil {
			return nil, fmt.Errorf("invalid entry for locale %q: %w", canonicalLocale, err)
		}

		entries = append(entries, metadataKeywordsPushEntry{
			Locale:   canonicalLocale,
			Keywords: trimmedKeywords,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Locale) < strings.ToLower(entries[j].Locale)
	})

	return entries, nil
}

func parseMetadataKeywordsPushKeywords(value json.RawMessage) (string, error) {
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return "", fmt.Errorf("value must not be null")
	}

	var asString string
	if err := json.Unmarshal(value, &asString); err == nil {
		return asString, nil
	}

	var object metadataKeywordsPushInputObject
	if err := json.Unmarshal(value, &object); err == nil && object.Keywords != nil {
		return *object.Keywords, nil
	}

	return "", fmt.Errorf("value must be a keyword string or object with a keywords field")
}

func writeMetadataKeywordsPushFailureArtifact(summary *metadataKeywordsPushSummary) (string, error) {
	failures := make([]metadataKeywordsPushResultItem, 0, summary.Failed)
	for _, result := range summary.Results {
		if result.Status == "failed" {
			failures = append(failures, result)
		}
	}

	artifact := metadataKeywordsPushFailureArtifact{
		VersionID:   summary.VersionID,
		InputFile:   summary.InputFile,
		Failed:      summary.Failed,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Results:     failures,
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}

	path := filepath.Join(
		".asc",
		"reports",
		"metadata-keywords-push",
		fmt.Sprintf("failures-%d.json", time.Now().UTC().UnixNano()),
	)
	if _, err := shared.WriteStreamToFile(path, bytes.NewReader(data)); err != nil {
		return "", err
	}

	return filepath.Clean(path), nil
}

func renderMetadataKeywordsPushSummary(summary *metadataKeywordsPushSummary, markdown bool) error {
	if summary == nil {
		return fmt.Errorf("summary is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Version ID", "Input File", "Continue On Error", "Total", "Created", "Updated", "Succeeded", "Failed", "Failure Artifact"},
		[][]string{{
			summary.VersionID,
			summary.InputFile,
			fmt.Sprintf("%t", summary.ContinueOnError),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Updated),
			fmt.Sprintf("%d", summary.Succeeded),
			fmt.Sprintf("%d", summary.Failed),
			summary.FailureArtifactPath,
		}},
	)

	headers := []string{"Locale", "Action", "Status", "Localization ID", "Error"}
	rows := make([][]string, 0, len(summary.Results))
	for _, result := range summary.Results {
		rows = append(rows, []string{
			result.Locale,
			result.Action,
			result.Status,
			result.LocalizationID,
			result.Error,
		})
	}

	if len(rows) == 0 {
		return nil
	}

	render(headers, rows)
	return nil
}
