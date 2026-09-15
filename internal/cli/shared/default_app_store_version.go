package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Default App Store version resolution picks the version a caller most likely
// means when --version is omitted: the newest editable version, else the newest
// live version. This mirrors the fallback used by EAS metadata:pull.
const (
	DefaultAppStoreVersionSourceEditable = "editable"
	DefaultAppStoreVersionSourceLive     = "live"
)

// DefaultEditableAppStoreVersionStates lists the appVersionState values whose
// metadata can still be edited in App Store Connect. The order is stable so
// requests and help text are deterministic. READY_FOR_REVIEW belongs here: it
// is the state a finished draft sits in before submission, so omitting it would
// make an app holding a ready draft alongside an older release resolve to the
// release instead of the draft being worked on.
var DefaultEditableAppStoreVersionStates = []string{
	"DEVELOPER_REJECTED",
	"INVALID_BINARY",
	"METADATA_REJECTED",
	"PREPARE_FOR_SUBMISSION",
	"READY_FOR_REVIEW",
	"REJECTED",
	"WAITING_FOR_REVIEW",
}

// defaultLiveAppStoreVersionStates lists the legacy appStoreState values that
// mark the version currently on the App Store.
var defaultLiveAppStoreVersionStates = []string{"READY_FOR_SALE"}

// defaultLiveAppVersionStates lists the modern appVersionState spelling of the
// same condition. Apple returns appStoreState and appVersionState
// inconsistently across versions and the READY_FOR_DISTRIBUTION-to-
// READY_FOR_SALE remapping is client-side only (docs/API_NOTES.md), so the
// live tier queries both spellings and merges the results. Filtering on one
// alone would report "no live version" for apps that expose only the other.
var defaultLiveAppVersionStates = []string{"READY_FOR_DISTRIBUTION"}

// DefaultAppStoreVersion describes the version selected when --version is
// omitted.
type DefaultAppStoreVersion struct {
	ID            string
	VersionString string
	Platform      string
	State         string
	// Source is DefaultAppStoreVersionSourceEditable or
	// DefaultAppStoreVersionSourceLive.
	Source string
}

// Note renders the stderr diagnostic that tells callers which version was
// picked and how to override it. overrideFlag is the flag the command reads,
// for example "--version".
func (v DefaultAppStoreVersion) Note(overrideFlag string) string {
	return fmt.Sprintf("Using version %s (%s) for platform %s; pass %s to override", v.VersionString, v.State, v.Platform, overrideFlag)
}

// AmbiguousDefaultAppStoreVersionError reports that more than one platform has
// a candidate default version, so the caller must pass --platform.
type AmbiguousDefaultAppStoreVersionError struct {
	AppID      string
	Source     string
	Candidates []DefaultAppStoreVersion
}

func (e *AmbiguousDefaultAppStoreVersionError) Error() string {
	parts := make([]string, 0, len(e.Candidates))
	for _, candidate := range e.Candidates {
		parts = append(parts, fmt.Sprintf("%s %s (%s)", candidate.Platform, candidate.VersionString, candidate.State))
	}
	return fmt.Sprintf("app %q has %s App Store versions on more than one platform (%s); pass --platform or --version", e.AppID, e.Source, strings.Join(parts, ", "))
}

// ResolveDefaultAppStoreVersion selects the app's newest editable App Store
// version, falling back to the newest live version. platform may be empty; when
// candidates exist on more than one platform the returned error is an
// *AmbiguousDefaultAppStoreVersionError.
//
// The editable preference is app-wide and is resolved before live versions are
// considered, so ambiguity is decided within the selected tier rather than
// across the union of both tiers. An app that is live on iOS and macOS but has
// a single editable iOS version therefore resolves to that editable version
// instead of failing: the editable version is the one being worked on, and
// requiring --platform there would reintroduce the friction this default
// exists to remove. The selection is never silent — callers announce the
// resolved version and its platform on stderr through Note.
func ResolveDefaultAppStoreVersion(ctx context.Context, client *asc.Client, appID, platform string) (DefaultAppStoreVersion, error) {
	if client == nil {
		return DefaultAppStoreVersion{}, fmt.Errorf("client is required")
	}
	trimmedAppID := strings.TrimSpace(appID)
	if trimmedAppID == "" {
		return DefaultAppStoreVersion{}, fmt.Errorf("app id is required")
	}
	trimmedPlatform := strings.ToUpper(strings.TrimSpace(platform))

	editable, err := listDefaultVersionCandidates(ctx, client, trimmedAppID, trimmedPlatform, asc.WithAppStoreVersionsVersionStates(DefaultEditableAppStoreVersionStates))
	if err != nil {
		return DefaultAppStoreVersion{}, err
	}
	if selected, ok, err := selectDefaultAppStoreVersion(trimmedAppID, DefaultAppStoreVersionSourceEditable, editable); ok || err != nil {
		return selected, err
	}

	live, err := listDefaultLiveVersionCandidates(ctx, client, trimmedAppID, trimmedPlatform)
	if err != nil {
		return DefaultAppStoreVersion{}, err
	}
	if selected, ok, err := selectDefaultAppStoreVersion(trimmedAppID, DefaultAppStoreVersionSourceLive, live); ok || err != nil {
		return selected, err
	}

	message := fmt.Sprintf("no editable or live App Store version found for app %q", trimmedAppID)
	if trimmedPlatform != "" {
		message += fmt.Sprintf(" on platform %s", trimmedPlatform)
	}
	return DefaultAppStoreVersion{}, NewErrorWithCause(
		fmt.Errorf("%s; create one with `asc versions create` or pass --version", message),
		asc.ErrNotFound,
	)
}

// listDefaultLiveVersionCandidates queries both live-state spellings and merges
// the results, deduplicating by version ID so a version Apple reports under
// both attributes is considered once.
func listDefaultLiveVersionCandidates(ctx context.Context, client *asc.Client, appID, platform string) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	legacy, err := listDefaultVersionCandidates(ctx, client, appID, platform, asc.WithAppStoreVersionsStates(defaultLiveAppStoreVersionStates))
	if err != nil {
		return nil, err
	}
	modern, err := listDefaultVersionCandidates(ctx, client, appID, platform, asc.WithAppStoreVersionsVersionStates(defaultLiveAppVersionStates))
	if err != nil {
		return nil, err
	}

	merged := make([]asc.Resource[asc.AppStoreVersionAttributes], 0, len(legacy)+len(modern))
	seen := make(map[string]struct{}, len(legacy)+len(modern))
	for _, group := range [][]asc.Resource[asc.AppStoreVersionAttributes]{legacy, modern} {
		for _, version := range group {
			id := strings.TrimSpace(version.ID)
			if id != "" {
				if _, duplicate := seen[id]; duplicate {
					continue
				}
				seen[id] = struct{}{}
			}
			merged = append(merged, version)
		}
	}
	return merged, nil
}

func listDefaultVersionCandidates(ctx context.Context, client *asc.Client, appID, platform string, stateOpt asc.AppStoreVersionsOption) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	opts := []asc.AppStoreVersionsOption{stateOpt, asc.WithAppStoreVersionsLimit(200)}
	if platform != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}
	firstPage, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list app store versions: %w", err)
	}
	if firstPage == nil {
		return nil, nil
	}
	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersions(ctx, appID, asc.WithAppStoreVersionsNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list app store versions: %w", err)
	}
	typed, ok := all.(*asc.AppStoreVersionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected paginated response type %T", all)
	}
	return typed.Data, nil
}

// selectDefaultAppStoreVersion keeps the newest candidate per platform. It
// returns ok=false when there are no candidates and an ambiguity error when
// more than one platform remains.
func selectDefaultAppStoreVersion(appID, source string, data []asc.Resource[asc.AppStoreVersionAttributes]) (DefaultAppStoreVersion, bool, error) {
	type candidate struct {
		version DefaultAppStoreVersion
		created time.Time
		index   int
	}
	newest := map[string]candidate{}
	for index, version := range data {
		platform := strings.ToUpper(strings.TrimSpace(string(version.Attributes.Platform)))
		current := candidate{
			version: DefaultAppStoreVersion{
				ID:            strings.TrimSpace(version.ID),
				VersionString: strings.TrimSpace(version.Attributes.VersionString),
				Platform:      platform,
				State:         asc.ResolveAppStoreVersionState(version.Attributes),
				Source:        source,
			},
			index: index,
		}
		if created, err := time.Parse(time.RFC3339, strings.TrimSpace(version.Attributes.CreatedDate)); err == nil {
			current.created = created
		}
		previous, seen := newest[platform]
		if !seen || current.created.After(previous.created) {
			newest[platform] = current
		}
	}
	if len(newest) == 0 {
		return DefaultAppStoreVersion{}, false, nil
	}
	candidates := make([]DefaultAppStoreVersion, 0, len(newest))
	for _, item := range newest {
		candidates = append(candidates, item.version)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Platform < candidates[j].Platform })
	if len(candidates) > 1 {
		return DefaultAppStoreVersion{}, false, &AmbiguousDefaultAppStoreVersionError{AppID: appID, Source: source, Candidates: candidates}
	}
	return candidates[0], true, nil
}

// ResolveAndAnnounceDefaultAppStoreVersion resolves the default App Store
// version for commands whose --version flag was omitted, prints the selection
// note to stderr, and maps platform ambiguity to an already-printed usage
// error (exit 2 without the full help page). overrideFlag is the flag name
// shown in the note.
func ResolveAndAnnounceDefaultAppStoreVersion(ctx context.Context, client *asc.Client, appID, platform, overrideFlag string) (DefaultAppStoreVersion, error) {
	resolved, err := ResolveDefaultAppStoreVersion(ctx, client, appID, platform)
	if err != nil {
		var ambiguous *AmbiguousDefaultAppStoreVersionError
		if errors.As(err, &ambiguous) {
			message := err.Error()
			fmt.Fprintf(os.Stderr, "Error: %s\n", message)
			return DefaultAppStoreVersion{}, WithDiagnostic(NewReportedUsageError(UsageErrorInvalidValue, message), DiagnosticInvalidInput, "--platform")
		}
		if errors.Is(err, asc.ErrNotFound) {
			return DefaultAppStoreVersion{}, WithDiagnostic(err, DiagnosticResourceNotFound, overrideFlag)
		}
		return DefaultAppStoreVersion{}, err
	}
	fmt.Fprintln(os.Stderr, resolved.Note(overrideFlag))
	return resolved, nil
}
