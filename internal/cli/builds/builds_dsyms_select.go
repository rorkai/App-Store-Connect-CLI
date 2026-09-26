package builds

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	dsymVersionLatest          = "latest"
	dsymVersionLive            = "live"
	dsymDefaultWaitTimeout     = 15 * time.Minute
	dsymDefaultPollInterval    = 30 * time.Second
	dsymNotReadyStatus         = "dsym_not_ready"
	dsymLiveAppStoreState      = "READY_FOR_SALE"
	dsymLivePreorderStoreState = "PREORDER_READY_FOR_SALE"
)

type dsymFlagInput struct {
	BuildID        string
	AppID          string
	Version        string
	BuildNumber    string
	Platform       string
	Latest         bool
	ExcludeExpired bool
	All            bool
	MinVersion     string
	AfterRaw       string
	Wait           bool
	Timeout        time.Duration
	PollInterval   time.Duration
	TimeoutSet     bool
	PollSet        bool
}

type dsymSelection struct {
	Resolve    ResolveBuildOptions
	Live       bool
	All        bool
	MinVersion string
	After      *time.Time
	Wait       bool
	Timeout    time.Duration
	Poll       time.Duration
	Multi      bool
}

type dsymTarget struct {
	ID          string
	AppVersion  string
	BuildNumber string
	Uploaded    time.Time
}

type liveAppVersion struct {
	Version     string
	Platform    string
	CreatedDate time.Time
}

func parseDSYMSelection(input dsymFlagInput) (dsymSelection, error) {
	version := strings.TrimSpace(input.Version)
	versionToken := strings.ToLower(version)
	selection := dsymSelection{
		Resolve: ResolveBuildOptions{
			BuildID:        strings.TrimSpace(input.BuildID),
			AppID:          strings.TrimSpace(input.AppID),
			Version:        version,
			BuildNumber:    strings.TrimSpace(input.BuildNumber),
			Platform:       strings.TrimSpace(input.Platform),
			Latest:         input.Latest,
			ExcludeExpired: input.ExcludeExpired,
		},
		All:        input.All,
		MinVersion: strings.TrimSpace(input.MinVersion),
		Wait:       input.Wait,
		Timeout:    input.Timeout,
		Poll:       input.PollInterval,
	}
	if selection.Timeout <= 0 {
		selection.Timeout = dsymDefaultWaitTimeout
	}
	if selection.Poll <= 0 {
		selection.Poll = dsymDefaultPollInterval
	}

	switch versionToken {
	case dsymVersionLatest:
		selection.Resolve.Latest = true
		selection.Resolve.Version = ""
	case dsymVersionLive:
		selection.Live = true
		selection.Resolve.Version = ""
	}

	if selection.MinVersion != "" {
		if _, err := compareMarketingVersions(selection.MinVersion, selection.MinVersion); err != nil {
			return dsymSelection{}, shared.UsageErrorf("builds dsyms: --min-version must be a dotted numeric version (got %q)", selection.MinVersion)
		}
	}
	if strings.TrimSpace(input.AfterRaw) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(input.AfterRaw))
		if err != nil {
			return dsymSelection{}, shared.UsageErrorf("builds dsyms: --after-uploaded-date must be RFC3339, for example 2026-09-01T00:00:00Z (got %q)", strings.TrimSpace(input.AfterRaw))
		}
		selection.After = &parsed
	}
	if selection.Resolve.Platform != "" {
		normalized, err := shared.NormalizeAppStoreVersionPlatform(selection.Resolve.Platform)
		if err != nil {
			return dsymSelection{}, shared.UsageErrorf("builds dsyms: %s", err.Error())
		}
		selection.Resolve.Platform = normalized
	}

	selection.Multi = selection.All || selection.MinVersion != "" || selection.After != nil
	if selection.MinVersion != "" || selection.After != nil {
		selection.All = true
		selection.Multi = true
	}
	if selection.selectsByMarketingVersion() && selection.Resolve.Version != "" {
		if _, err := parseMarketingVersion(selection.Resolve.Version); err != nil {
			return dsymSelection{}, shared.UsageErrorf("builds dsyms: --version must be a dotted numeric version (got %q)", selection.Resolve.Version)
		}
	}

	if err := validateDSYMSelection(selection, input.TimeoutSet, input.PollSet, input.Timeout, input.PollInterval); err != nil {
		return dsymSelection{}, err
	}
	if !selection.selectsByMarketingVersion() {
		if err := validateResolveBuildOptions(selection.Resolve); err != nil {
			return dsymSelection{}, err
		}
	} else if shared.ResolveAppID(selection.Resolve.AppID) == "" {
		return dsymSelection{}, shared.UsageError("builds dsyms: --app is required (or set ASC_APP_ID)")
	}
	return selection, nil
}

// selectsByMarketingVersion reports selectors that list builds by App Store
// version instead of the historical --latest or --build-number lookup.
// An exact --version without --latest downloads the newest build of that
// version unless --all is also set.
func (selection dsymSelection) selectsByMarketingVersion() bool {
	if selection.Multi || selection.Live {
		return true
	}
	return selection.Resolve.BuildID == "" &&
		selection.Resolve.BuildNumber == "" &&
		!selection.Resolve.Latest &&
		selection.Resolve.Version != ""
}

func validateDSYMSelection(selection dsymSelection, timeoutSet, pollSet bool, timeout, poll time.Duration) error {
	buildID := selection.Resolve.BuildID
	hasRange := selection.All || selection.MinVersion != "" || selection.After != nil
	if buildID != "" && (selection.Resolve.Latest || selection.Live || hasRange || selection.Resolve.Version != "" || selection.Resolve.BuildNumber != "" || selection.Resolve.Platform != "" || selection.Resolve.ExcludeExpired) {
		return shared.UsageError("builds dsyms: --build-id cannot be combined with --app, --latest, --version, --build-number, --platform, --all, --min-version, --after-uploaded-date, --exclude-expired, or --not-expired")
	}
	if selection.Resolve.Latest && selection.Live {
		return shared.UsageError("builds dsyms: --version live cannot be combined with --latest")
	}
	if selection.Resolve.Latest && (selection.Resolve.BuildNumber != "") {
		return shared.UsageError("builds dsyms: --latest and --build-number are mutually exclusive")
	}
	if selection.Live && selection.Resolve.BuildNumber != "" {
		return shared.UsageError("builds dsyms: --version live cannot be combined with --build-number")
	}
	if selection.Live && selection.MinVersion != "" {
		return shared.UsageError("builds dsyms: --version live cannot be combined with --min-version")
	}
	if selection.Resolve.Latest && hasRange {
		return shared.UsageError("builds dsyms: --latest cannot be combined with --all, --min-version, or --after-uploaded-date")
	}
	if selection.Resolve.BuildNumber != "" && hasRange {
		return shared.UsageError("builds dsyms: --build-number cannot be combined with --all, --min-version, or --after-uploaded-date")
	}
	if !selection.Wait && (timeoutSet || pollSet) {
		return shared.UsageError("builds dsyms: --timeout and --poll-interval require --wait")
	}
	if selection.Wait && timeout <= 0 {
		return shared.UsageError("builds dsyms: --timeout must be greater than 0")
	}
	if selection.Wait && poll <= 0 {
		return shared.UsageError("builds dsyms: --poll-interval must be greater than 0")
	}
	return nil
}

func resolveDSYMTargets(ctx context.Context, client *asc.Client, selection dsymSelection) ([]dsymTarget, error) {
	if selection.selectsByMarketingVersion() {
		return resolveSelectedDSYMTargets(ctx, client, selection)
	}
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	buildResp, err := ResolveBuild(requestCtx, client, selection.Resolve)
	if err != nil {
		return nil, err
	}
	if buildResp == nil || strings.TrimSpace(buildResp.Data.ID) == "" {
		return nil, fmt.Errorf("builds dsyms: no build matched")
	}
	return []dsymTarget{{
		ID:          buildResp.Data.ID,
		AppVersion:  selection.Resolve.Version,
		BuildNumber: buildResp.Data.Attributes.Version,
	}}, nil
}

func resolveSelectedDSYMTargets(ctx context.Context, client *asc.Client, selection dsymSelection) ([]dsymTarget, error) {
	appID, err := shared.ResolveAppIDWithLookup(ctx, client, selection.Resolve.AppID)
	if err != nil {
		return nil, err
	}
	exactVersion := selection.Resolve.Version
	platform := selection.Resolve.Platform
	if selection.Live {
		live, err := resolveLiveAppVersion(ctx, client, appID, platform)
		if err != nil {
			return nil, err
		}
		exactVersion = live.Version
		if platform == "" {
			platform = live.Platform
		}
	}

	targets, err := listDSYMTargets(ctx, client, appID, platform, exactVersion, selection.MinVersion, selection.After, selection.Resolve.ExcludeExpired)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("builds dsyms: no builds matched")
	}
	if !selection.All {
		return targets[:1], nil
	}
	return targets, nil
}

func resolveLiveAppVersion(ctx context.Context, client *asc.Client, appID, platform string) (liveAppVersion, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsStates([]string{dsymLiveAppStoreState, dsymLivePreorderStoreState}),
		asc.WithAppStoreVersionsLimit(200),
	}
	if platform != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}
	first, err := client.GetAppStoreVersions(requestCtx, appID, opts...)
	if err != nil {
		return liveAppVersion{}, fmt.Errorf("builds dsyms: failed to list App Store versions: %w", err)
	}
	var collected []liveAppVersion
	err = asc.PaginateEach(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetAppStoreVersions(pageCtx, appID, asc.WithAppStoreVersionsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.AppStoreVersionsResponse)
		if !ok || resp == nil {
			return fmt.Errorf("unexpected app store versions page type %T", page)
		}
		for _, item := range resp.Data {
			state := strings.ToUpper(strings.TrimSpace(item.Attributes.AppStoreState))
			if state != dsymLiveAppStoreState && state != dsymLivePreorderStoreState {
				continue
			}
			created, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(item.Attributes.CreatedDate))
			if parseErr != nil {
				return fmt.Errorf("app store version %q has invalid createdDate %q", item.ID, item.Attributes.CreatedDate)
			}
			collected = append(collected, liveAppVersion{
				Version:     strings.TrimSpace(item.Attributes.VersionString),
				Platform:    strings.ToUpper(strings.TrimSpace(string(item.Attributes.Platform))),
				CreatedDate: created,
			})
		}
		return nil
	})
	if err != nil {
		return liveAppVersion{}, fmt.Errorf("builds dsyms: failed to list App Store versions: %w", err)
	}
	return chooseLiveVersion(collected, platform)
}

func chooseLiveVersion(versions []liveAppVersion, platform string) (liveAppVersion, error) {
	newest := map[string]liveAppVersion{}
	for _, version := range versions {
		if strings.TrimSpace(version.Version) == "" || version.Platform == "" {
			continue
		}
		if platform != "" && version.Platform != platform {
			continue
		}
		current, ok := newest[version.Platform]
		if !ok || version.CreatedDate.After(current.CreatedDate) {
			newest[version.Platform] = version
		}
	}
	if len(newest) == 0 {
		if platform != "" {
			return liveAppVersion{}, fmt.Errorf("builds dsyms: no live App Store version for platform %s", platform)
		}
		return liveAppVersion{}, fmt.Errorf("builds dsyms: no live App Store version")
	}
	if platform != "" {
		return newest[platform], nil
	}
	if len(newest) == 1 {
		for _, version := range newest {
			return version, nil
		}
	}
	platforms := make([]string, 0, len(newest))
	for name, version := range newest {
		platforms = append(platforms, fmt.Sprintf("%s %s", name, version.Version))
	}
	sort.Strings(platforms)
	return liveAppVersion{}, fmt.Errorf("builds dsyms: multiple live versions; pass --platform (%s)", strings.Join(platforms, ", "))
}

func listDSYMTargets(ctx context.Context, client *asc.Client, appID, platform, exactVersion, minVersion string, after *time.Time, excludeExpired bool) ([]dsymTarget, error) {
	opts := []asc.BuildsOption{
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(200),
		asc.WithBuildsInclude([]string{"preReleaseVersion"}),
	}
	if platform != "" {
		opts = append(opts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}
	if excludeExpired {
		opts = append(opts, asc.WithBuildsExpired(false))
	}
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	first, err := client.GetBuilds(requestCtx, appID, opts...)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("builds dsyms: failed to list builds: %w", err)
	}

	var targets []dsymTarget
	page := first
	for page != nil {
		pageTargets, stop, err := dsymTargetsFromPage(page, exactVersion, minVersion, after, excludeExpired)
		if err != nil {
			return nil, err
		}
		targets = append(targets, pageTargets...)
		if stop || strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		nextCtx, nextCancel := shared.ContextWithTimeout(ctx)
		page, err = client.GetBuilds(nextCtx, appID, asc.WithBuildsNextURL(page.Links.Next))
		nextCancel()
		if err != nil {
			return nil, fmt.Errorf("builds dsyms: failed to list builds: %w", err)
		}
	}
	sort.SliceStable(targets, func(i, j int) bool {
		return targets[i].Uploaded.After(targets[j].Uploaded)
	})
	return targets, nil
}

func dsymTargetsFromPage(page *asc.BuildsResponse, exactVersion, minVersion string, after *time.Time, excludeExpired bool) ([]dsymTarget, bool, error) {
	if page == nil {
		return nil, true, nil
	}
	versions := preReleaseVersionsByID(page.Included)
	targets := make([]dsymTarget, 0, len(page.Data))
	stop := false
	var oldest *time.Time
	for _, item := range page.Data {
		uploaded, err := time.Parse(time.RFC3339, strings.TrimSpace(item.Attributes.UploadedDate))
		if err != nil {
			return nil, false, fmt.Errorf("builds dsyms: build %s has invalid uploadedDate %q", item.ID, item.Attributes.UploadedDate)
		}
		if oldest == nil || uploaded.Before(*oldest) {
			value := uploaded
			oldest = &value
		}
		expired, expiredSet := item.Attributes.ExpiredValue()
		if excludeExpired && expiredSet && expired {
			continue
		}
		if after != nil && !uploaded.After(*after) {
			continue
		}
		versionID := preReleaseVersionID(item.Relationships)
		info, ok := versions[versionID]
		if exactVersion != "" || minVersion != "" {
			if !ok || strings.TrimSpace(info.Version) == "" {
				return nil, false, fmt.Errorf("builds dsyms: build %s is missing preReleaseVersion, so its marketing version cannot be selected", item.ID)
			}
		}
		marketing := ""
		if ok {
			marketing = info.Version
		}
		if exactVersion != "" {
			cmp, cmpErr := compareMarketingVersions(marketing, exactVersion)
			if cmpErr != nil {
				return nil, false, fmt.Errorf("builds dsyms: build %s has invalid marketing version %q", item.ID, marketing)
			}
			if cmp != 0 {
				continue
			}
		}
		if minVersion != "" {
			cmp, cmpErr := compareMarketingVersions(marketing, minVersion)
			if cmpErr != nil {
				return nil, false, fmt.Errorf("builds dsyms: build %s has invalid marketing version %q", item.ID, marketing)
			}
			if cmp < 0 {
				continue
			}
		}
		targets = append(targets, dsymTarget{
			ID:          item.ID,
			AppVersion:  marketing,
			BuildNumber: item.Attributes.Version,
			Uploaded:    uploaded,
		})
	}
	if after != nil && oldest != nil && !oldest.After(*after) {
		stop = true
	}
	return targets, stop, nil
}

type preReleaseVersionSummary struct {
	Version  string
	Platform string
}

func preReleaseVersionsByID(included []byte) map[string]preReleaseVersionSummary {
	if len(included) == 0 {
		return nil
	}
	var items []asc.Resource[asc.PreReleaseVersionAttributes]
	if err := json.Unmarshal(included, &items); err != nil {
		return nil
	}
	out := make(map[string]preReleaseVersionSummary, len(items))
	for _, item := range items {
		if item.Type != "preReleaseVersions" {
			continue
		}
		out[item.ID] = preReleaseVersionSummary{
			Version:  strings.TrimSpace(item.Attributes.Version),
			Platform: strings.ToUpper(strings.TrimSpace(string(item.Attributes.Platform))),
		}
	}
	return out
}

func preReleaseVersionID(relationships []byte) string {
	if len(relationships) == 0 {
		return ""
	}
	var rels struct {
		PreReleaseVersion struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"preReleaseVersion"`
	}
	if err := json.Unmarshal(relationships, &rels); err != nil {
		return ""
	}
	return strings.TrimSpace(rels.PreReleaseVersion.Data.ID)
}

func compareMarketingVersions(left, right string) (int, error) {
	leftParts, err := parseMarketingVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseMarketingVersion(right)
	if err != nil {
		return 0, err
	}
	n := len(leftParts)
	if len(rightParts) > n {
		n = len(rightParts)
	}
	for i := 0; i < n; i++ {
		var l, r int
		if i < len(leftParts) {
			l = leftParts[i]
		}
		if i < len(rightParts) {
			r = rightParts[i]
		}
		if l < r {
			return -1, nil
		}
		if l > r {
			return 1, nil
		}
	}
	return 0, nil
}

func parseMarketingVersion(raw string) ([]int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("empty version")
	}
	parts := strings.Split(trimmed, ".")
	parsed := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid version %q", trimmed)
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil, fmt.Errorf("invalid version %q", trimmed)
		}
		parsed[i] = value
	}
	return parsed, nil
}

func hashRegularFile(path string) (int64, string, bool, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	root, err := rootfs.New(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, "", false, nil
		}
		return 0, "", false, err
	}
	defer root.Close()
	file, err := root.OpenFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, "", false, nil
		}
		return 0, "", false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", false, err
	}
	if !info.Mode().IsRegular() {
		return 0, "", false, fmt.Errorf("builds dsyms: %s is not a regular file", path)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return 0, "", false, err
	}
	return info.Size(), hex.EncodeToString(hasher.Sum(nil)), true, nil
}
