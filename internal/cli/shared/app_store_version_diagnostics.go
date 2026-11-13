package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Version-resolution diagnostics run only after a lookup or create has
// already failed. They read the app's versions, every page within a fixed
// time budget, so the error can name what exists instead of leaving the
// caller to guess. The read is best effort: when any page fails or the budget
// runs out, the original error is returned unchanged.
const (
	appStoreVersionDiagnosticPageLimit = 200
	appStoreVersionDiagnosticShown     = 10
	appStoreVersionDiagnosticBudget    = 5 * time.Second
)

// appStoreVersionEditableStates lists states whose version string can still be
// changed with asc versions update, so the version can be renamed instead of
// creating another one.
var appStoreVersionEditableStates = map[string]struct{}{
	"DEVELOPER_REJECTED":     {},
	"INVALID_BINARY":         {},
	"METADATA_REJECTED":      {},
	"PREPARE_FOR_SUBMISSION": {},
	"READY_FOR_REVIEW":       {},
	"REJECTED":               {},
}

// appStoreVersionSettledStates lists states of versions that are live or no
// longer in progress. Any other known state is an unreleased version, which
// is what App Store Connect refuses to stack another new version on.
var appStoreVersionSettledStates = map[string]struct{}{
	"DEVELOPER_REMOVED_FROM_SALE": {},
	"NOT_APPLICABLE":              {},
	"PREORDER_READY_FOR_SALE":     {},
	"READY_FOR_DISTRIBUTION":      {},
	"READY_FOR_SALE":              {},
	"REMOVED_FROM_SALE":           {},
	"REPLACED_WITH_NEW_VERSION":   {},
}

type appStoreVersionDiagnosticRow struct {
	ID            string
	VersionString string
	Platform      string
	State         string
	createdAt     time.Time
	hasCreatedAt  bool
}

type appStoreVersionDiagnosticListing struct {
	Rows []appStoreVersionDiagnosticRow
}

// WithAppStoreVersionNotFoundDiagnostics appends the app's existing App Store
// versions, newest first, and the command that targets or creates the
// requested version to a "version not found" error. An empty platform lists
// every platform. The returned error keeps notFound's classification; when
// the extra read fails, notFound is returned unchanged.
func WithAppStoreVersionNotFoundDiagnostics(ctx context.Context, client *asc.Client, appID, version, platform string, notFound error) error {
	if notFound == nil {
		return nil
	}
	appID = strings.TrimSpace(appID)
	version = strings.TrimSpace(version)
	platform = strings.ToUpper(strings.TrimSpace(platform))
	listing, ok := readAppStoreVersionDiagnosticListing(ctx, client, appID, platform)
	if !ok {
		return notFound
	}

	safeAppID := sanitizeAmbiguousText(appID)
	safeVersion := sanitizeAmbiguousText(version)
	safePlatform := sanitizeAmbiguousText(platform)
	lines := make([]string, 0, appStoreVersionDiagnosticShown+4)
	if len(listing.Rows) == 0 {
		if safePlatform == "" {
			lines = append(lines, fmt.Sprintf("App %q has no App Store versions. Create one: asc versions create --app %q --version %q --platform PLATFORM", safeAppID, safeAppID, safeVersion))
		} else {
			lines = append(lines, noAppStoreVersionsOnPlatformLine(ctx, client, appID, safeAppID, safeVersion, safePlatform))
		}
		return appendAppStoreVersionDiagnostics(notFound, lines)
	}

	if safePlatform == "" {
		lines = append(lines, fmt.Sprintf("Existing App Store versions for app %q (newest first):", safeAppID))
	} else {
		lines = append(lines, fmt.Sprintf("Existing App Store versions for app %q on %s (newest first):", safeAppID, safePlatform))
	}
	lines = append(lines, formatAppStoreVersionDiagnosticRows(listing, safeAppID, safePlatform)...)

	if safePlatform == "" {
		lines = append(lines, "Retry with one of the listed version strings and its platform.")
	} else if editable, found := newestEditableAppStoreVersion(listing.Rows); found {
		lines = append(lines, fmt.Sprintf(
			"Retry with one of the listed version strings, or, if editable version %s is the release you meant, rename it: asc versions update --version-id %q --version %q",
			editable.VersionString, editable.ID, safeVersion,
		))
	} else {
		lines = append(lines, fmt.Sprintf("Retry with one of the listed version strings, or create it: %s", versionsCreateCommand(safeAppID, safeVersion, safePlatform)))
	}
	return appendAppStoreVersionDiagnostics(notFound, lines)
}

// WithAppStoreVersionCreateConflictDiagnostics explains an HTTP 409 from
// POST /v1/appStoreVersions. Apple answers both a reused version string and a
// platform that already has an unreleased version with "You cannot create a
// new version of the App in the current state.", so this reads the platform's
// versions and names the existing version with that string or the unreleased
// versions, with the command that reuses, renames, or releases each one. It
// reports current state without claiming which version caused the rejection.
// Other failures, and a failed diagnostic read, return createErr unchanged.
func WithAppStoreVersionCreateConflictDiagnostics(ctx context.Context, client *asc.Client, appID, version, platform string, createErr error) error {
	var apiErr *asc.APIError
	if createErr == nil || !errors.As(createErr, &apiErr) || apiErr == nil || apiErr.StatusCode != http.StatusConflict {
		return createErr
	}
	appID = strings.TrimSpace(appID)
	version = strings.TrimSpace(version)
	platform = strings.ToUpper(strings.TrimSpace(platform))
	if platform == "" {
		return createErr
	}
	listing, ok := readAppStoreVersionDiagnosticListing(ctx, client, appID, platform)
	if !ok {
		return createErr
	}

	safeAppID := sanitizeAmbiguousText(appID)
	safeVersion := sanitizeAmbiguousText(version)
	safePlatform := sanitizeAmbiguousText(platform)
	lines := make([]string, 0, 4)
	duplicateID := ""
	for _, row := range listing.Rows {
		if row.VersionString == safeVersion {
			duplicateID = row.ID
			lines = append(lines, fmt.Sprintf(
				"Version %q already exists on %s as %s (%s). Reuse it with --if-exists skip or --if-exists update, or inspect it: asc versions view --version-id %q",
				safeVersion, safePlatform, row.ID, diagnosticStateLabel(row.State), row.ID,
			))
			break
		}
	}

	unreleased := make([]appStoreVersionDiagnosticRow, 0, 1)
	for _, row := range listing.Rows {
		if row.ID != duplicateID && isUnreleasedAppStoreVersionState(row.State) {
			unreleased = append(unreleased, row)
		}
	}
	if len(unreleased) > 0 {
		lines = append(lines, fmt.Sprintf("%s versions that are not live yet (an unreleased version usually blocks creating another):", safePlatform))
		lines = append(lines, formatAppStoreVersionDiagnosticRows(appStoreVersionDiagnosticListing{Rows: unreleased}, safeAppID, safePlatform)...)
		lines = append(lines, unreleasedAppStoreVersionGuidance(unreleased[0], safeVersion))
	} else if duplicateID == "" {
		lines = append(lines, fmt.Sprintf("No unreleased %s version was found; review every version: asc versions list --app %q --platform %s", safePlatform, safeAppID, safePlatform))
	}
	return appendAppStoreVersionDiagnostics(createErr, lines)
}

func readAppStoreVersionDiagnosticListing(ctx context.Context, client *asc.Client, appID, platform string) (appStoreVersionDiagnosticListing, bool) {
	if client == nil || appID == "" || ctx.Err() != nil {
		return appStoreVersionDiagnosticListing{}, false
	}
	diagnosticCtx, cancel := context.WithTimeout(ctx, appStoreVersionDiagnosticBudget)
	defer cancel()

	opts := []asc.AppStoreVersionsOption{asc.Limit[asc.AppStoreVersionsOption](appStoreVersionDiagnosticPageLimit)}
	if platform != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}
	firstPage, err := client.GetAppStoreVersions(diagnosticCtx, appID, opts...)
	if err != nil || firstPage == nil {
		return appStoreVersionDiagnosticListing{}, false
	}
	// "Newest" guidance is only sound over every version: one page of an
	// unordered collection can miss the newest or the matching version. Read
	// every page within the budget, or give up and keep the original error.
	allPages, err := asc.PaginateAll(diagnosticCtx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersions(pageCtx, appID, asc.NextURL[asc.AppStoreVersionsOption](nextURL))
	})
	if err != nil {
		return appStoreVersionDiagnosticListing{}, false
	}
	resp, ok := allPages.(*asc.AppStoreVersionsResponse)
	if !ok || resp == nil {
		return appStoreVersionDiagnosticListing{}, false
	}

	rows := make([]appStoreVersionDiagnosticRow, 0, len(resp.Data))
	for _, item := range resp.Data {
		row := appStoreVersionDiagnosticRow{
			ID:            sanitizeAmbiguousText(item.ID, AmbiguousDiagnosticTextLimit),
			VersionString: sanitizeAmbiguousText(item.Attributes.VersionString, AmbiguousDiagnosticTextLimit),
			Platform:      sanitizeAmbiguousText(string(item.Attributes.Platform), AmbiguousDiagnosticTextLimit),
			State:         strings.ToUpper(sanitizeAmbiguousText(ResolveAppStoreVersionState(item.Attributes), AmbiguousDiagnosticTextLimit)),
		}
		if createdAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(item.Attributes.CreatedDate)); parseErr == nil {
			row.createdAt = createdAt
			row.hasCreatedAt = true
		}
		rows = append(rows, row)
	}
	// Apple does not document the collection order and the endpoint has no
	// sort parameter, so order by createdDate here. Versions without a
	// parsable createdDate keep their response order after the dated ones.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].hasCreatedAt != rows[j].hasCreatedAt {
			return rows[i].hasCreatedAt
		}
		return rows[i].createdAt.After(rows[j].createdAt)
	})
	return appStoreVersionDiagnosticListing{Rows: rows}, true
}

func formatAppStoreVersionDiagnosticRows(listing appStoreVersionDiagnosticListing, safeAppID, safePlatform string) []string {
	shown := listing.Rows
	if len(shown) > appStoreVersionDiagnosticShown {
		shown = shown[:appStoreVersionDiagnosticShown]
	}
	versionWidth, platformWidth, stateWidth := 0, 0, 0
	for _, row := range shown {
		versionWidth = max(versionWidth, len(diagnosticVersionLabel(row.VersionString)))
		platformWidth = max(platformWidth, len(diagnosticPlatformLabel(row.Platform)))
		stateWidth = max(stateWidth, len(diagnosticStateLabel(row.State)))
	}
	lines := make([]string, 0, len(shown)+1)
	for _, row := range shown {
		lines = append(lines, fmt.Sprintf(
			"  %-*s  %-*s  %-*s  %s",
			versionWidth, diagnosticVersionLabel(row.VersionString),
			platformWidth, diagnosticPlatformLabel(row.Platform),
			stateWidth, diagnosticStateLabel(row.State),
			diagnosticIDLabel(row.ID),
		))
	}

	if remaining := len(listing.Rows) - len(shown); remaining > 0 {
		listCommand := fmt.Sprintf("asc versions list --app %q", safeAppID)
		if safePlatform != "" {
			listCommand += " --platform " + safePlatform
		}
		lines = append(lines, fmt.Sprintf("  ... and %d more; list every version: %s --paginate", remaining, listCommand))
	}
	return lines
}

func newestEditableAppStoreVersion(rows []appStoreVersionDiagnosticRow) (appStoreVersionDiagnosticRow, bool) {
	for _, row := range rows {
		if _, ok := appStoreVersionEditableStates[row.State]; ok && row.ID != "" {
			return row, true
		}
	}
	return appStoreVersionDiagnosticRow{}, false
}

func isUnreleasedAppStoreVersionState(state string) bool {
	if _, known := appStoreVersionStates[state]; !known {
		return false
	}
	_, settled := appStoreVersionSettledStates[state]
	return !settled
}

func unreleasedAppStoreVersionGuidance(row appStoreVersionDiagnosticRow, safeVersion string) string {
	if _, editable := appStoreVersionEditableStates[row.State]; editable {
		return fmt.Sprintf(
			"To ship %q from editable version %s, rename it: asc versions update --version-id %q --version %q",
			safeVersion, row.VersionString, row.ID, safeVersion,
		)
	}
	if row.State == "PENDING_DEVELOPER_RELEASE" {
		return fmt.Sprintf("Version %s is approved and waiting for release; release it: asc versions release --version-id %q --confirm", row.VersionString, row.ID)
	}
	return fmt.Sprintf("Version %s is %s; wait until review finishes or it is released, or inspect it: asc versions view --version-id %q", row.VersionString, row.State, row.ID)
}

// noAppStoreVersionsOnPlatformLine explains an empty platform listing. The
// platform is often the IOS default rather than the caller's choice, so a
// create suggestion could add a platform the app never shipped on. When the
// app has versions on other platforms, point at --platform first.
func noAppStoreVersionsOnPlatformLine(ctx context.Context, client *asc.Client, appID, safeAppID, safeVersion, safePlatform string) string {
	createCommand := versionsCreateCommand(safeAppID, safeVersion, safePlatform)
	listing, ok := readAppStoreVersionDiagnosticListing(ctx, client, appID, "")
	var withVersion, others []string
	if ok {
		for _, row := range listing.Rows {
			if row.Platform == "" || row.Platform == safePlatform {
				continue
			}
			others = append(others, row.Platform)
			if row.VersionString == safeVersion {
				withVersion = append(withVersion, row.Platform)
			}
		}
	}
	slices.Sort(withVersion)
	withVersion = slices.Compact(withVersion)
	slices.Sort(others)
	others = slices.Compact(others)

	switch {
	case len(withVersion) > 0:
		return fmt.Sprintf("App %q has no App Store versions on %s, but version %q exists on %s. %s.", safeAppID, safePlatform, safeVersion, strings.Join(withVersion, ", "), retryWithPlatform(withVersion))
	case len(others) > 0:
		return fmt.Sprintf("App %q has no App Store versions on %s; it has versions on %s. %s, or create one on %s: %s", safeAppID, safePlatform, strings.Join(others, ", "), retryWithPlatform(others), safePlatform, createCommand)
	default:
		return fmt.Sprintf("App %q has no App Store versions on %s. Create one: %s", safeAppID, safePlatform, createCommand)
	}
}

func retryWithPlatform(platforms []string) string {
	if len(platforms) == 1 {
		return "Retry with --platform " + platforms[0]
	}
	return "Retry with --platform set to one of them"
}

func versionsCreateCommand(safeAppID, safeVersion, safePlatform string) string {
	return fmt.Sprintf("asc versions create --app %q --version %q --platform %s", safeAppID, safeVersion, safePlatform)
}

func appendAppStoreVersionDiagnostics(original error, lines []string) error {
	if len(lines) == 0 {
		return original
	}
	return NewErrorWithCause(
		errors.New(original.Error()+"\n"+strings.Join(lines, "\n")),
		original,
	)
}

func diagnosticVersionLabel(value string) string {
	return diagnosticLabel(value, "<no version>")
}

func diagnosticPlatformLabel(value string) string {
	return diagnosticLabel(value, "<no platform>")
}

func diagnosticStateLabel(value string) string {
	return diagnosticLabel(value, "<no state>")
}

func diagnosticIDLabel(value string) string {
	return diagnosticLabel(value, "<no id>")
}

func diagnosticLabel(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
