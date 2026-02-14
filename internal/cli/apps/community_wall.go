package apps

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	communityWallSourceEnv     = "ASC_WALL_SOURCE"
	communityWallRemoteURL     = "https://raw.githubusercontent.com/rudrankriyam/App-Store-Connect-CLI/main/docs/wall-of-apps.json"
	communityWallSourcePath    = "docs/wall-of-apps.json"
	defaultCommunityWallSort   = "name"
	defaultCommunityWallOutput = "table"
)

var communityWallPlatformAliases = map[string]string{
	"ios":       "IOS",
	"macos":     "MAC_OS",
	"mac_os":    "MAC_OS",
	"watchos":   "WATCH_OS",
	"watch_os":  "WATCH_OS",
	"tvos":      "TV_OS",
	"tv_os":     "TV_OS",
	"visionos":  "VISION_OS",
	"vision_os": "VISION_OS",
}

type communityWallEntry struct {
	App      string   `json:"app"`
	Link     string   `json:"link"`
	Creator  string   `json:"creator"`
	Platform []string `json:"platform"`
}

// AppsWallCommand returns the community wall subcommand.
func AppsWallCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps wall", flag.ExitOnError)

	output := shared.BindOutputFlagsWith(fs, "output", defaultCommunityWallOutput, "Output format: table (default), json, markdown")
	sortBy := fs.String("sort", defaultCommunityWallSort, "Sort by name or -name")
	limit := fs.Int("limit", 0, "Maximum number of apps to include (1-200)")
	includePlatforms := fs.String("include-platforms", "", "Filter by platform label(s), comma-separated")

	return &ffcli.Command{
		Name:       "wall",
		ShortUsage: "asc apps wall [flags]",
		ShortHelp:  "Show the community Wall of Apps from project metadata.",
		LongHelp: `Show the community Wall of Apps from project metadata.

Examples:
  asc apps wall
  asc apps wall --output markdown
  asc apps wall --include-platforms iOS,macOS --limit 20
  asc apps wall --sort -name`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return appsCommunityWall(ctx, *output.Output, *output.Pretty, *sortBy, *limit, *includePlatforms)
		},
	}
}

func appsCommunityWall(ctx context.Context, output string, pretty bool, sortBy string, limit int, includePlatforms string) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
		return flag.ErrHelp
	}
	if err := shared.ValidateSort(sortBy, "name", "-name"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		return flag.ErrHelp
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	sourceEntries, err := loadCommunityWallEntries(requestCtx)
	if err != nil {
		return fmt.Errorf("apps wall: %w", err)
	}

	entries := make([]asc.AppWallEntry, 0, len(sourceEntries))
	for _, item := range sourceEntries {
		name := strings.TrimSpace(item.App)
		link := strings.TrimSpace(item.Link)
		if name == "" || link == "" {
			continue
		}

		entries = append(entries, asc.AppWallEntry{
			Name:        name,
			AppStoreURL: link,
			Creator:     strings.TrimSpace(item.Creator),
			Platform:    normalizeCommunityPlatforms(item.Platform),
		})
	}

	entries = filterCommunityWallEntriesByPlatforms(entries, normalizeCommunityPlatformFilters(shared.SplitCSV(includePlatforms)))
	sortCommunityWallEntries(entries, sortBy)

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return shared.PrintOutput(&asc.AppsWallResult{Data: entries}, output, pretty)
}

func loadCommunityWallEntries(ctx context.Context) ([]communityWallEntry, error) {
	if source := strings.TrimSpace(os.Getenv(communityWallSourceEnv)); source != "" {
		return loadCommunityWallEntriesFromSource(ctx, source)
	}

	if localPath, ok := findCommunityWallSourcePath(); ok {
		return readCommunityWallEntriesFromFile(localPath)
	}

	return readCommunityWallEntriesFromURL(ctx, communityWallRemoteURL)
}

func loadCommunityWallEntriesFromSource(ctx context.Context, source string) ([]communityWallEntry, error) {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
		return readCommunityWallEntriesFromURL(ctx, trimmed)
	}
	return readCommunityWallEntriesFromFile(trimmed)
}

func findCommunityWallSourcePath() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	dir := wd
	for {
		candidate := filepath.Join(dir, communityWallSourcePath)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", false
}

func readCommunityWallEntriesFromFile(path string) ([]communityWallEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read community wall source %q: %w", path, err)
	}
	return decodeCommunityWallEntries(raw, path)
}

func readCommunityWallEntriesFromURL(ctx context.Context, sourceURL string) ([]communityWallEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build community wall request: %w", err)
	}

	httpClient := &http.Client{Timeout: asc.ResolveTimeout()}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch community wall source: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if strings.TrimSpace(string(body)) == "" {
			return nil, fmt.Errorf("community wall source request failed with status %s", resp.Status)
		}
		return nil, fmt.Errorf("community wall source request failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read community wall source response: %w", err)
	}

	return decodeCommunityWallEntries(raw, sourceURL)
}

func decodeCommunityWallEntries(raw []byte, source string) ([]communityWallEntry, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return nil, fmt.Errorf("community wall source %q is empty", source)
	}

	var entries []communityWallEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("invalid community wall source %q: %w", source, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("community wall source %q has no entries", source)
	}
	return entries, nil
}

func sortCommunityWallEntries(entries []asc.AppWallEntry, sortBy string) {
	if sortBy == "-name" {
		sort.SliceStable(entries, func(i, j int) bool {
			return lessCommunityWallName(entries[j], entries[i])
		})
		return
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return lessCommunityWallName(entries[i], entries[j])
	})
}

func lessCommunityWallName(left, right asc.AppWallEntry) bool {
	leftName := strings.ToLower(strings.TrimSpace(left.Name))
	rightName := strings.ToLower(strings.TrimSpace(right.Name))
	if leftName != rightName {
		return leftName < rightName
	}
	return strings.ToLower(strings.TrimSpace(left.AppStoreURL)) < strings.ToLower(strings.TrimSpace(right.AppStoreURL))
}

func normalizeCommunityPlatforms(platforms []string) []string {
	normalized := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		value := normalizeCommunityPlatform(platform)
		if value == "" {
			continue
		}
		if !containsCommunityPlatformFold(normalized, value) {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func normalizeCommunityPlatform(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	key := strings.ToLower(trimmed)
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "")
	if normalized, ok := communityWallPlatformAliases[key]; ok {
		return normalized
	}
	return trimmed
}

func normalizeCommunityPlatformFilters(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeCommunityPlatform(value)
		if normalized == "" {
			continue
		}
		allowed[strings.ToLower(normalized)] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func filterCommunityWallEntriesByPlatforms(entries []asc.AppWallEntry, allowed map[string]struct{}) []asc.AppWallEntry {
	if len(allowed) == 0 {
		return entries
	}

	filtered := make([]asc.AppWallEntry, 0, len(entries))
	for _, entry := range entries {
		if hasCommunityPlatform(entry.Platform, allowed) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func hasCommunityPlatform(platforms []string, allowed map[string]struct{}) bool {
	for _, platform := range platforms {
		if _, ok := allowed[strings.ToLower(platform)]; ok {
			return true
		}
	}
	return false
}

func containsCommunityPlatformFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
