package screenshots

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// ReviewOpenRequest configures opening a generated review HTML file.
type ReviewOpenRequest struct {
	OutputDir string
	HTMLPath  string
	DryRun    bool
}

// ReviewOpenResult describes the resolved review HTML path and open state.
type ReviewOpenResult struct {
	HTMLPath string `json:"html_path"`
	Opened   bool   `json:"opened"`
}

// ReviewApproveRequest configures updates to approved.json.
type ReviewApproveRequest struct {
	OutputDir    string
	ManifestPath string
	ApprovalPath string
	AllReady     bool
	Keys         []string
	ScreenshotID string
	Locale       string
	Device       string
}

// ReviewApproveResult summarizes approval updates.
type ReviewApproveResult struct {
	ManifestPath  string   `json:"manifest_path"`
	ApprovalPath  string   `json:"approval_path"`
	Matched       int      `json:"matched"`
	Added         int      `json:"added"`
	TotalApproved int      `json:"total_approved"`
	Keys          []string `json:"keys,omitempty"`
}

// OpenReview opens the generated review HTML in the default browser.
func OpenReview(ctx context.Context, req ReviewOpenRequest) (*ReviewOpenResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	outputDir, err := ResolveReviewOutputDir(req.OutputDir)
	if err != nil {
		return nil, err
	}
	htmlPath, err := resolveReviewArtifactPath(outputDir, strings.TrimSpace(req.HTMLPath), defaultReviewHTMLName)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("read review HTML: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("review HTML path points to a directory")
	}

	if req.DryRun {
		return &ReviewOpenResult{
			HTMLPath: htmlPath,
			Opened:   false,
		}, nil
	}
	if err := openPathInBrowser(htmlPath); err != nil {
		return nil, err
	}
	return &ReviewOpenResult{
		HTMLPath: htmlPath,
		Opened:   true,
	}, nil
}

// ApproveReview writes/updates approval keys for review entries.
func ApproveReview(ctx context.Context, req ReviewApproveRequest) (*ReviewApproveResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	outputDir, err := ResolveReviewOutputDir(req.OutputDir)
	if err != nil {
		return nil, err
	}
	manifestPath, err := resolveReviewArtifactPath(outputDir, strings.TrimSpace(req.ManifestPath), defaultReviewManifestName)
	if err != nil {
		return nil, err
	}
	approvalPath, err := resolveReviewArtifactPath(outputDir, strings.TrimSpace(req.ApprovalPath), defaultReviewApprovalsName)
	if err != nil {
		return nil, err
	}

	manifest, err := LoadReviewManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	selectedKeys, err := selectApprovalKeys(manifest, req)
	if err != nil {
		return nil, err
	}
	approvals, err := loadApprovals(approvalPath)
	if err != nil {
		return nil, err
	}

	added := 0
	for _, key := range selectedKeys {
		if approvals[key] {
			continue
		}
		approvals[key] = true
		added++
	}

	if err := SaveApprovals(approvalPath, approvals); err != nil {
		return nil, err
	}
	return &ReviewApproveResult{
		ManifestPath:  manifestPath,
		ApprovalPath:  approvalPath,
		Matched:       len(selectedKeys),
		Added:         added,
		TotalApproved: countApproved(approvals),
		Keys:          selectedKeys,
	}, nil
}

func resolveReviewArtifactPath(outputDir, override, defaultName string) (string, error) {
	path := strings.TrimSpace(override)
	if path == "" {
		path = filepath.Join(outputDir, defaultName)
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(outputDir, path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	return absPath, nil
}

func selectApprovalKeys(manifest *ReviewManifest, req ReviewApproveRequest) ([]string, error) {
	if manifest == nil {
		return nil, fmt.Errorf("review manifest is required")
	}

	keySet := make(map[string]struct{})
	for _, key := range req.Keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keySet[trimmed] = struct{}{}
	}
	localeFilter := strings.TrimSpace(req.Locale)
	deviceFilter := strings.TrimSpace(req.Device)
	idFilter := strings.TrimSpace(req.ScreenshotID)
	hasSelector := req.AllReady || len(keySet) > 0 || idFilter != "" || localeFilter != "" || deviceFilter != ""
	if !hasSelector {
		return nil, fmt.Errorf("provide at least one selector: --all-ready, --key, --id, --locale, or --device")
	}

	filterOnlySelection := !req.AllReady && len(keySet) == 0 && idFilter == "" && (localeFilter != "" || deviceFilter != "")
	entryByKey := make(map[string]ReviewEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entryByKey[entry.Key] = entry
	}

	selected := make([]string, 0)
	for key := range keySet {
		entry, ok := entryByKey[key]
		if !ok {
			return nil, fmt.Errorf("review key not found in manifest: %s", key)
		}
		if !matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			continue
		}
		selected = append(selected, key)
	}

	for _, entry := range manifest.Entries {
		if filterOnlySelection && matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			selected = append(selected, entry.Key)
		}
		if req.AllReady && entry.Status == reviewStatusReady && matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			selected = append(selected, entry.Key)
		}
		if idFilter != "" && entry.ScreenshotID == idFilter && matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			selected = append(selected, entry.Key)
		}
	}

	selected = uniqueSorted(selected)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no review entries matched approval selectors")
	}
	return selected, nil
}

func uniqueSorted(keys []string) []string {
	unique := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	slices.Sort(unique)
	return unique
}

func matchesLocaleDeviceFilters(entry ReviewEntry, locale, device string) bool {
	if locale != "" && entry.Locale != locale {
		return false
	}
	if device != "" && entry.Device != device {
		return false
	}
	return true
}

func countApproved(approvals map[string]bool) int {
	total := 0
	for _, approved := range approvals {
		if approved {
			total++
		}
	}
	return total
}

func openPathInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open review HTML: %w", err)
	}
	return nil
}
