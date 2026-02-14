package screenshots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	defaultReviewOutputDir        = "./screenshots/review"
	defaultReviewManifestName     = "manifest.json"
	defaultReviewHTMLName         = "index.html"
	defaultReviewApprovalsName    = "approved.json"
	reviewStatusReady             = "ready"
	reviewStatusMissingRaw        = "missing_raw"
	reviewStatusInvalidSize       = "invalid_size"
	reviewStatusMissingAndInvalid = "missing_raw_invalid_size"
)

// ReviewRequest configures generation of screenshot review artifacts.
type ReviewRequest struct {
	RawDir       string // optional; missing raw files are reported
	FramedDir    string // required
	OutputDir    string // optional, defaults to ./screenshots/review
	ApprovalPath string // optional, defaults to <output-dir>/approved.json
}

// ReviewSummary aggregates status/approval totals across all entries.
type ReviewSummary struct {
	Total           int `json:"total"`
	Ready           int `json:"ready"`
	MissingRaw      int `json:"missing_raw"`
	InvalidSize     int `json:"invalid_size"`
	Approved        int `json:"approved"`
	PendingApproval int `json:"pending_approval"`
}

// ReviewEntry represents one framed screenshot row in review artifacts.
type ReviewEntry struct {
	Key               string   `json:"key"`
	ScreenshotID      string   `json:"screenshot_id"`
	Locale            string   `json:"locale,omitempty"`
	Device            string   `json:"device,omitempty"`
	FramedPath        string   `json:"framed_path"`
	FramedRelative    string   `json:"framed_relative_path"`
	RawPath           string   `json:"raw_path,omitempty"`
	RawRelative       string   `json:"raw_relative_path,omitempty"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	DisplayTypes      []string `json:"display_types,omitempty"`
	ValidAppStoreSize bool     `json:"valid_app_store_size"`
	Status            string   `json:"status"`
	Approved          bool     `json:"approved"`
	ApprovalState     string   `json:"approval_state"`
}

// ReviewManifest is the JSON artifact for agent/human checks.
type ReviewManifest struct {
	GeneratedAt  string        `json:"generated_at"`
	RawDir       string        `json:"raw_dir,omitempty"`
	FramedDir    string        `json:"framed_dir"`
	OutputDir    string        `json:"output_dir"`
	ApprovalPath string        `json:"approval_path"`
	Summary      ReviewSummary `json:"summary"`
	Entries      []ReviewEntry `json:"entries"`
}

// ReviewResult is printed by CLI after artifacts are written.
type ReviewResult struct {
	ManifestPath string `json:"manifest_path"`
	HTMLPath     string `json:"html_path"`
	ApprovalPath string `json:"approval_path"`
	FramedDir    string `json:"framed_dir"`
	Total        int    `json:"total"`
	Ready        int    `json:"ready"`
	MissingRaw   int    `json:"missing_raw"`
	InvalidSize  int    `json:"invalid_size"`
	Approved     int    `json:"approved"`
	Pending      int    `json:"pending"`
}

type reviewHTMLData struct {
	Manifest ReviewManifest
}

// GenerateReview creates manifest and HTML side-by-side report artifacts.
func GenerateReview(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	framedDir := strings.TrimSpace(req.FramedDir)
	if framedDir == "" {
		return nil, fmt.Errorf("framed directory is required")
	}
	absFramedDir, err := filepath.Abs(framedDir)
	if err != nil {
		return nil, fmt.Errorf("resolve framed directory: %w", err)
	}
	framedInfo, err := os.Stat(absFramedDir)
	if err != nil {
		return nil, fmt.Errorf("read framed directory: %w", err)
	}
	if !framedInfo.IsDir() {
		return nil, fmt.Errorf("framed directory must be a directory")
	}

	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		outputDir = defaultReviewOutputDir
	}
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}
	if err := os.MkdirAll(absOutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	approvalPath := strings.TrimSpace(req.ApprovalPath)
	if approvalPath == "" {
		approvalPath = filepath.Join(absOutputDir, defaultReviewApprovalsName)
	}
	if !filepath.IsAbs(approvalPath) {
		approvalPath = filepath.Join(absOutputDir, approvalPath)
	}
	approvals, err := loadApprovals(approvalPath)
	if err != nil {
		return nil, err
	}

	rawAvailable := false
	absRawDir := ""
	rawDir := strings.TrimSpace(req.RawDir)
	if rawDir != "" {
		absRawDir, err = filepath.Abs(rawDir)
		if err != nil {
			return nil, fmt.Errorf("resolve raw directory: %w", err)
		}
		rawInfo, statErr := os.Stat(absRawDir)
		if statErr == nil && rawInfo.IsDir() {
			rawAvailable = true
		}
	}

	rawIndex, err := buildRawIndex(absRawDir, rawAvailable)
	if err != nil {
		return nil, err
	}
	entries, err := buildReviewEntries(ctx, absFramedDir, absRawDir, rawAvailable, rawIndex, approvals)
	if err != nil {
		return nil, err
	}

	manifest := ReviewManifest{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		RawDir:       absRawDir,
		FramedDir:    absFramedDir,
		OutputDir:    absOutputDir,
		ApprovalPath: approvalPath,
		Summary:      summarizeReviewEntries(entries),
		Entries:      entries,
	}
	if !rawAvailable {
		manifest.RawDir = ""
	}

	manifestPath := filepath.Join(absOutputDir, defaultReviewManifestName)
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest JSON: %w", err)
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("write manifest JSON: %w", err)
	}

	htmlPath := filepath.Join(absOutputDir, defaultReviewHTMLName)
	htmlContent, err := renderReviewHTML(manifest)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return nil, fmt.Errorf("write review HTML: %w", err)
	}

	return &ReviewResult{
		ManifestPath: manifestPath,
		HTMLPath:     htmlPath,
		ApprovalPath: approvalPath,
		FramedDir:    absFramedDir,
		Total:        manifest.Summary.Total,
		Ready:        manifest.Summary.Ready,
		MissingRaw:   manifest.Summary.MissingRaw,
		InvalidSize:  manifest.Summary.InvalidSize,
		Approved:     manifest.Summary.Approved,
		Pending:      manifest.Summary.PendingApproval,
	}, nil
}

// ResolveReviewOutputDir resolves output dir with defaults and ensures absolute path.
func ResolveReviewOutputDir(outputDir string) (string, error) {
	dir := strings.TrimSpace(outputDir)
	if dir == "" {
		dir = defaultReviewOutputDir
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	return absDir, nil
}

// LoadReviewManifest parses a generated review manifest from disk.
func LoadReviewManifest(path string) (*ReviewManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read review manifest: %w", err)
	}
	var manifest ReviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse review manifest JSON: %w", err)
	}
	return &manifest, nil
}

func buildReviewEntries(
	ctx context.Context,
	framedDir string,
	rawDir string,
	rawAvailable bool,
	rawIndex map[string]string,
	approvals map[string]bool,
) ([]ReviewEntry, error) {
	framedFiles, err := collectImageFiles(framedDir)
	if err != nil {
		return nil, err
	}

	entries := make([]ReviewEntry, 0, len(framedFiles))
	for _, framedPath := range framedFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		relPath, err := filepath.Rel(framedDir, framedPath)
		if err != nil {
			return nil, fmt.Errorf("resolve framed relative path: %w", err)
		}
		screenshotID := strings.TrimSuffix(filepath.Base(framedPath), filepath.Ext(framedPath))
		locale, device := inferLocaleAndDevice(relPath)

		dimensions, err := asc.ReadImageDimensions(framedPath)
		if err != nil {
			return nil, fmt.Errorf("read screenshot dimensions for %q: %w", framedPath, err)
		}
		displayTypes := matchingAppDisplayTypes(dimensions.Width, dimensions.Height)
		hasValidSize := len(displayTypes) > 0

		rawPath := ""
		rawRelative := ""
		if rawAvailable {
			rawPath = rawIndex[rawIndexReviewKey(locale, device, screenshotID)]
			if rawPath == "" {
				candidate := rawIndex[rawIndexScreenshotKey(screenshotID)]
				if candidate != "" {
					if locale == "" && device == "" {
						rawPath = candidate
					} else {
						candidateRelative, relErr := filepath.Rel(rawDir, candidate)
						if relErr != nil {
							return nil, fmt.Errorf("resolve raw relative path: %w", relErr)
						}
						candidateLocale, candidateDevice := inferLocaleAndDevice(candidateRelative)
						candidateIsGeneric := candidateLocale == "" && candidateDevice == ""
						if candidateIsGeneric || ((locale == "" || locale == candidateLocale) && (device == "" || device == candidateDevice)) {
							rawPath = candidate
						}
					}
				}
			}
			if rawPath != "" {
				rawRelative, err = filepath.Rel(rawDir, rawPath)
				if err != nil {
					return nil, fmt.Errorf("resolve raw relative path: %w", err)
				}
			}
		}

		reviewKey := makeReviewKey(locale, device, screenshotID)
		approved := approvals[reviewKey]
		entry := ReviewEntry{
			Key:               reviewKey,
			ScreenshotID:      screenshotID,
			Locale:            locale,
			Device:            device,
			FramedPath:        framedPath,
			FramedRelative:    filepath.ToSlash(relPath),
			RawPath:           rawPath,
			RawRelative:       filepath.ToSlash(rawRelative),
			Width:             dimensions.Width,
			Height:            dimensions.Height,
			DisplayTypes:      displayTypes,
			ValidAppStoreSize: hasValidSize,
			Status:            deriveReviewStatus(rawPath != "", hasValidSize),
			Approved:          approved,
			ApprovalState:     approvalState(approved),
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func summarizeReviewEntries(entries []ReviewEntry) ReviewSummary {
	summary := ReviewSummary{Total: len(entries)}
	for _, entry := range entries {
		if entry.Status == reviewStatusReady {
			summary.Ready++
		}
		if strings.Contains(entry.Status, reviewStatusMissingRaw) {
			summary.MissingRaw++
		}
		if strings.Contains(entry.Status, reviewStatusInvalidSize) {
			summary.InvalidSize++
		}
		if entry.Approved {
			summary.Approved++
		} else {
			summary.PendingApproval++
		}
	}
	return summary
}

func collectImageFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !isImageFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan screenshot directory: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func buildRawIndex(rawDir string, rawAvailable bool) (map[string]string, error) {
	index := make(map[string]string)
	ambiguousScreenshotIDs := make(map[string]bool)
	if !rawAvailable {
		return index, nil
	}

	rawFiles, err := collectImageFiles(rawDir)
	if err != nil {
		return nil, fmt.Errorf("scan raw screenshot directory: %w", err)
	}
	for _, rawPath := range rawFiles {
		base := strings.TrimSuffix(filepath.Base(rawPath), filepath.Ext(rawPath))
		idKey := rawIndexScreenshotKey(base)
		if ambiguousScreenshotIDs[idKey] {
			delete(index, idKey)
		} else if existingPath, exists := index[idKey]; !exists {
			index[idKey] = rawPath
		} else if existingPath != rawPath {
			delete(index, idKey)
			ambiguousScreenshotIDs[idKey] = true
		}

		relPath, relErr := filepath.Rel(rawDir, rawPath)
		if relErr != nil {
			return nil, fmt.Errorf("resolve raw relative path: %w", relErr)
		}
		locale, device := inferLocaleAndDevice(relPath)
		reviewKey := rawIndexReviewKey(locale, device, base)
		if _, exists := index[reviewKey]; exists {
			continue
		}
		index[reviewKey] = rawPath
	}
	return index, nil
}

func rawIndexReviewKey(locale, device, screenshotID string) string {
	return "review|" + makeReviewKey(locale, device, screenshotID)
}

func rawIndexScreenshotKey(screenshotID string) string {
	return "id|" + strings.TrimSpace(screenshotID)
}

func matchingAppDisplayTypes(width, height int) []string {
	matches := make([]string, 0)
	for _, displayType := range asc.ScreenshotDisplayTypes() {
		if !strings.HasPrefix(displayType, "APP_") {
			continue
		}
		dimensions, ok := asc.ScreenshotDimensions(displayType)
		if !ok {
			continue
		}
		for _, dimension := range dimensions {
			if dimension.Width == width && dimension.Height == height {
				matches = append(matches, displayType)
				break
			}
		}
	}
	return matches
}

func makeReviewKey(locale, device, screenshotID string) string {
	return fmt.Sprintf("%s|%s|%s", strings.TrimSpace(locale), strings.TrimSpace(device), strings.TrimSpace(screenshotID))
}

func deriveReviewStatus(hasRaw bool, hasValidSize bool) string {
	switch {
	case hasRaw && hasValidSize:
		return reviewStatusReady
	case !hasRaw && hasValidSize:
		return reviewStatusMissingRaw
	case hasRaw && !hasValidSize:
		return reviewStatusInvalidSize
	default:
		return reviewStatusMissingAndInvalid
	}
}

func approvalState(approved bool) string {
	if approved {
		return "approved"
	}
	return "pending"
}

func inferLocaleAndDevice(relPath string) (string, string) {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) >= 3 {
		return parts[0], parts[1]
	}
	if len(parts) == 2 {
		if looksLikeLocale(parts[0]) {
			return parts[0], ""
		}
		return "", parts[0]
	}
	return "", ""
}

func looksLikeLocale(token string) bool {
	clean := strings.TrimSpace(strings.ReplaceAll(token, "_", "-"))
	if clean == "" {
		return false
	}
	parts := strings.Split(clean, "-")
	if len(parts) > 3 {
		return false
	}
	for index, part := range parts {
		if part == "" {
			return false
		}
		minLen, maxLen := 2, 3
		if index > 0 {
			maxLen = 8
		}
		if len(part) < minLen || len(part) > maxLen {
			return false
		}
		for _, char := range part {
			if !unicode.IsLetter(char) {
				return false
			}
		}
	}
	return true
}

func isImageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

func loadApprovals(path string) (map[string]bool, error) {
	approvals := make(map[string]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return approvals, nil
		}
		return nil, fmt.Errorf("read approvals file: %w", err)
	}

	var mapFormat map[string]bool
	if err := json.Unmarshal(data, &mapFormat); err == nil {
		for key, approved := range mapFormat {
			if approved {
				approvals[strings.TrimSpace(key)] = true
			}
		}
		return approvals, nil
	}

	var listFormat []string
	if err := json.Unmarshal(data, &listFormat); err == nil {
		for _, key := range listFormat {
			key = strings.TrimSpace(key)
			if key != "" {
				approvals[key] = true
			}
		}
		return approvals, nil
	}

	var wrapped struct {
		Approved []string `json:"approved"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil {
		for _, key := range wrapped.Approved {
			key = strings.TrimSpace(key)
			if key != "" {
				approvals[key] = true
			}
		}
		return approvals, nil
	}

	return nil, fmt.Errorf("parse approvals file %q: expected map[string]bool, []string, or {\"approved\":[]}", path)
}

// SaveApprovals writes approved keys to disk in a stable JSON format.
func SaveApprovals(path string, approvals map[string]bool) error {
	keys := make([]string, 0, len(approvals))
	for key, approved := range approvals {
		if !approved {
			continue
		}
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	sort.Strings(keys)

	payload := struct {
		Approved []string `json:"approved"`
	}{
		Approved: keys,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approvals JSON: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create approvals directory: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write approvals file: %w", err)
	}
	return nil
}

func renderReviewHTML(manifest ReviewManifest) (string, error) {
	tmpl, err := template.New("review").Funcs(template.FuncMap{
		"fileURL": localFileURL,
	}).Parse(reviewHTMLTemplate)
	if err != nil {
		return "", fmt.Errorf("parse review HTML template: %w", err)
	}

	var builder strings.Builder
	if err := tmpl.Execute(&builder, reviewHTMLData{Manifest: manifest}); err != nil {
		return "", fmt.Errorf("render review HTML template: %w", err)
	}
	return builder.String(), nil
}

func localFileURL(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	// Use an absolute path-only URL so html/template keeps it in src/href.
	// "file://" URLs are sanitized to "#ZgotmplZ" in these contexts.
	return (&url.URL{
		Path: pathOnlyURLPath(absolutePath),
	}).String()
}

func pathOnlyURLPath(absolutePath string) string {
	urlPath := filepath.ToSlash(absolutePath)
	if len(urlPath) >= 3 && urlPath[1] == ':' && urlPath[2] == '/' {
		first := urlPath[0]
		if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') {
			return "/" + urlPath
		}
	}
	return urlPath
}

const reviewHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>ASC Shots Review</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 20px; color: #1f2937; }
    h1 { margin: 0 0 8px 0; }
    .meta { margin-bottom: 18px; color: #4b5563; font-size: 14px; }
    .summary { display: grid; grid-template-columns: repeat(6, minmax(120px, 1fr)); gap: 8px; margin-bottom: 18px; }
    .card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 10px; background: #ffffff; }
    .label { font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.04em; }
    .value { font-size: 22px; font-weight: 700; margin-top: 4px; }
    table { width: 100%; border-collapse: collapse; }
    th, td { border: 1px solid #e5e7eb; padding: 8px; vertical-align: top; text-align: left; font-size: 13px; }
    th { background: #f9fafb; position: sticky; top: 0; z-index: 1; }
    .status-ready { color: #166534; font-weight: 600; }
    .status-missing { color: #92400e; font-weight: 600; }
    .status-invalid { color: #991b1b; font-weight: 600; }
    .approval-approved { color: #166534; font-weight: 600; }
    .approval-pending { color: #6b7280; font-weight: 600; }
    .shot { max-height: 340px; max-width: 220px; border: 1px solid #d1d5db; border-radius: 8px; background: #ffffff; }
    .missing { color: #9ca3af; font-style: italic; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  </style>
</head>
<body>
  <h1>ASC Shots Review</h1>
  <div class="meta">
    Generated at {{.Manifest.GeneratedAt}}<br />
    Framed: <code>{{.Manifest.FramedDir}}</code><br />
    {{if .Manifest.RawDir}}Raw: <code>{{.Manifest.RawDir}}</code><br />{{end}}
    Manifest: <code>{{.Manifest.OutputDir}}/manifest.json</code>
  </div>

  <div class="summary">
    <div class="card"><div class="label">Total</div><div class="value">{{.Manifest.Summary.Total}}</div></div>
    <div class="card"><div class="label">Ready</div><div class="value">{{.Manifest.Summary.Ready}}</div></div>
    <div class="card"><div class="label">Missing Raw</div><div class="value">{{.Manifest.Summary.MissingRaw}}</div></div>
    <div class="card"><div class="label">Invalid Size</div><div class="value">{{.Manifest.Summary.InvalidSize}}</div></div>
    <div class="card"><div class="label">Approved</div><div class="value">{{.Manifest.Summary.Approved}}</div></div>
    <div class="card"><div class="label">Pending</div><div class="value">{{.Manifest.Summary.PendingApproval}}</div></div>
  </div>

  <table>
    <thead>
      <tr>
        <th>ID</th>
        <th>Locale</th>
        <th>Device</th>
        <th>Status</th>
        <th>Approval</th>
        <th>Dimensions</th>
        <th>Display Types</th>
        <th>Raw</th>
        <th>Framed</th>
      </tr>
    </thead>
    <tbody>
      {{range .Manifest.Entries}}
      <tr>
        <td><code>{{.ScreenshotID}}</code></td>
        <td>{{if .Locale}}<code>{{.Locale}}</code>{{else}}<span class="missing">-</span>{{end}}</td>
        <td>{{if .Device}}<code>{{.Device}}</code>{{else}}<span class="missing">-</span>{{end}}</td>
        <td>
          {{if eq .Status "ready"}}<span class="status-ready">{{.Status}}</span>{{end}}
          {{if eq .Status "missing_raw"}}<span class="status-missing">{{.Status}}</span>{{end}}
          {{if eq .Status "invalid_size"}}<span class="status-invalid">{{.Status}}</span>{{end}}
          {{if eq .Status "missing_raw_invalid_size"}}<span class="status-invalid">{{.Status}}</span>{{end}}
        </td>
        <td>
          {{if .Approved}}<span class="approval-approved">approved</span>{{else}}<span class="approval-pending">pending</span>{{end}}
        </td>
        <td><code>{{.Width}}x{{.Height}}</code></td>
        <td>
          {{if .DisplayTypes}}
            {{range .DisplayTypes}}<code>{{.}}</code><br />{{end}}
          {{else}}
            <span class="missing">none</span>
          {{end}}
        </td>
        <td>
          {{if .RawPath}}
            <a href="{{fileURL .RawPath}}" target="_blank" rel="noopener">
              <img class="shot" src="{{fileURL .RawPath}}" alt="raw {{.ScreenshotID}}" />
            </a><br />
            <code>{{.RawRelative}}</code>
          {{else}}
            <span class="missing">missing</span>
          {{end}}
        </td>
        <td>
          <a href="{{fileURL .FramedPath}}" target="_blank" rel="noopener">
            <img class="shot" src="{{fileURL .FramedPath}}" alt="framed {{.ScreenshotID}}" />
          </a><br />
          <code>{{.FramedRelative}}</code>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>
`
