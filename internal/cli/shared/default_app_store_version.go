package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
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

// defaultActiveEditableAppVersionStates lists the modern appVersionState values
// for drafts whose metadata can still be edited in App Store Connect. The order
// is stable so requests and help text are deterministic. READY_FOR_REVIEW
// belongs here: it is the state a finished draft sits in before submission, so
// omitting it would make an app holding a ready draft alongside an older
// release resolve to the release instead of the draft being worked on.
var defaultActiveEditableAppVersionStates = []string{
	"DEVELOPER_REJECTED",
	"INVALID_BINARY",
	"METADATA_REJECTED",
	"PREPARE_FOR_SUBMISSION",
	"READY_FOR_REVIEW",
	"REJECTED",
	"WAITING_FOR_REVIEW",
}

// defaultRemovedEditableAppStoreVersionStates is a lower-priority editable
// fallback for apps without an active draft. Apple exposes this legacy state
// only through filter[appStoreState], so it cannot be mixed into the modern
// query above. Keeping it below active drafts prevents a removed listing from
// overriding the version currently being prepared.
var defaultRemovedEditableAppStoreVersionStates = []string{"DEVELOPER_REMOVED_FROM_SALE"}

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
// a candidate default version, so the caller must pass --platform. It unwraps
// to AmbiguousSelectionError so callers share the standard typed, sanitized
// candidate-table contract.
type AmbiguousDefaultAppStoreVersionError struct {
	AppID      string
	Source     string
	Candidates []DefaultAppStoreVersion
}

func (e *AmbiguousDefaultAppStoreVersionError) Error() string {
	return e.selectionError().Error()
}

func (e *AmbiguousDefaultAppStoreVersionError) Unwrap() error {
	return e.selectionError()
}

func (e *AmbiguousDefaultAppStoreVersionError) selectionError() *AmbiguousSelectionError {
	candidates := make([]AmbiguousCandidate, 0, len(e.Candidates))
	for _, candidate := range e.Candidates {
		label := "version " + strings.TrimSpace(candidate.VersionString)
		if id := strings.TrimSpace(candidate.ID); id != "" {
			label += fmt.Sprintf(" (%s)", id)
		}
		candidates = append(candidates, AmbiguousCandidate{
			ID:    strings.TrimSpace(candidate.Platform),
			Label: label,
			Extra: strings.TrimSpace(candidate.State),
		})
	}
	return &AmbiguousSelectionError{
		Kind:        "app store version",
		Description: fmt.Sprintf("the default %s version for app %q", strings.TrimSpace(e.Source), strings.TrimSpace(e.AppID)),
		Flag:        "--platform",
		Candidates:  candidates,
	}
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

	editable, err := listAppStoreVersionsWithState(ctx, client, trimmedAppID, trimmedPlatform, asc.WithAppStoreVersionsVersionStates(defaultActiveEditableAppVersionStates), callerRequestContext)
	if err != nil {
		return DefaultAppStoreVersion{}, err
	}
	if selected, ok, err := selectDefaultAppStoreVersion(trimmedAppID, DefaultAppStoreVersionSourceEditable, editable); ok || err != nil {
		return selected, err
	}
	removedEditable, err := listAppStoreVersionsWithState(ctx, client, trimmedAppID, trimmedPlatform, asc.WithAppStoreVersionsStates(defaultRemovedEditableAppStoreVersionStates), callerRequestContext)
	if err != nil {
		return DefaultAppStoreVersion{}, err
	}
	if selected, ok, err := selectDefaultAppStoreVersion(trimmedAppID, DefaultAppStoreVersionSourceEditable, removedEditable); ok || err != nil {
		return selected, err
	}

	// Query both live-state spellings: filtering on one alone would report "no
	// live version" for apps that expose only the other. A version the legacy
	// filter still reports as READY_FOR_SALE is dropped when its
	// appVersionState says it is no longer live.
	live, err := ListAppStoreVersionsInStates(ctx, client, trimmedAppID, trimmedPlatform, LiveAppStoreVersionStateFilter())
	if err != nil {
		return DefaultAppStoreVersion{}, err
	}
	live = slices.DeleteFunc(live, func(version asc.Resource[asc.AppStoreVersionAttributes]) bool {
		return !IsLiveAppStoreVersion(version.Attributes)
	})
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
	return resolveAndAnnounceDefaultAppStoreVersionTo(ctx, os.Stderr, client, appID, platform, overrideFlag)
}

// ResolveAndAnnounceDefaultAppStoreVersion is
// ResolveAndAnnounceDefaultAppStoreVersion with the note and the "Error:" line
// written to d's stderr.
func (d *Deps) ResolveAndAnnounceDefaultAppStoreVersion(ctx context.Context, client *asc.Client, appID, platform, overrideFlag string) (DefaultAppStoreVersion, error) {
	return resolveAndAnnounceDefaultAppStoreVersionTo(ctx, d.IO.ErrOut, client, appID, platform, overrideFlag)
}

func resolveAndAnnounceDefaultAppStoreVersionTo(ctx context.Context, stderr io.Writer, client *asc.Client, appID, platform, overrideFlag string) (DefaultAppStoreVersion, error) {
	resolved, err := ResolveDefaultAppStoreVersion(ctx, client, appID, platform)
	if err != nil {
		if IsAmbiguousSelection(err) {
			message := err.Error()
			fmt.Fprintf(stderr, "Error: %s\n", message)
			reported := WithDiagnostic(NewReportedUsageError(UsageErrorInvalidValue, message), DiagnosticInvalidInput, "--platform")
			return DefaultAppStoreVersion{}, NewErrorWithCause(reported, err)
		}
		if errors.Is(err, asc.ErrNotFound) {
			return DefaultAppStoreVersion{}, WithDiagnostic(err, DiagnosticResourceNotFound, overrideFlag)
		}
		return DefaultAppStoreVersion{}, err
	}
	fmt.Fprintln(stderr, resolved.Note(overrideFlag))
	return resolved, nil
}
