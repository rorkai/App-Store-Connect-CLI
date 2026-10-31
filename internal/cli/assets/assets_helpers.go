package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	assetUploadDefaultTimeout = 10 * time.Minute
	assetPollInterval         = 2 * time.Second
)

func contextWithAssetUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, asc.ResolveUploadTimeoutWithDefault(assetUploadDefaultTimeout))
}

// ContextWithAssetUploadTimeout returns a context with the asset upload timeout.
func ContextWithAssetUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return contextWithAssetUploadTimeout(ctx)
}

// CollectAssetFiles validates and collects files from a path.
func CollectAssetFiles(path string) ([]string, error) {
	return collectAssetFiles(path)
}

func collectAssetPaths(path string) ([]string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, markMediaInput(err, shared.DiagnosticFileNotFound)
	}
	if !info.IsDir() {
		return []string{path}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, markMediaInput(err, shared.DiagnosticFileNotFound)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(path, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func collectAssetFiles(path string) ([]string, error) {
	files, err := collectAssetPaths(path)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, markMediaInput(fmt.Errorf("no files found in %q", path), shared.DiagnosticFileNotFound)
	}
	for _, filePath := range files {
		if err := asc.ValidateImageFile(filePath); err != nil {
			return nil, markMediaInput(err, shared.DiagnosticFileInvalidFormat)
		}
	}
	return files, nil
}

// validateLibraryUploadFlags rejects set-only flags on the Asset Library path.
func validateLibraryUploadFlags(assetLibrary bool, placementGroup string, skipExisting, replace bool) error {
	if !assetLibrary {
		if strings.TrimSpace(placementGroup) != "" {
			return shared.UsageError("--placement-group requires --asset-library")
		}
		return nil
	}
	if skipExisting {
		return shared.UsageError("--asset-library cannot be combined with --skip-existing")
	}
	if replace {
		return shared.UsageError("--asset-library cannot be combined with --replace; remove placements with asc localizations placements delete")
	}
	return nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) { found = found || f.Name == name })
	return found
}

func waitForAssetDeliveryState(ctx context.Context, assetID string, fetch func(context.Context) (*asc.AssetDeliveryState, error)) (string, error) {
	var lastState string
	_, err := asc.PollUntil(ctx, assetPollInterval, func(ctx context.Context) (struct{}, bool, error) {
		state, err := fetch(ctx)
		if err != nil {
			return struct{}{}, false, err
		}
		if state != nil {
			lastState = state.State
			switch strings.ToUpper(state.State) {
			case "COMPLETE":
				return struct{}{}, true, nil
			case "FAILED":
				return struct{}{}, false, fmt.Errorf("asset %s delivery failed: %s", assetID, formatAssetErrors(state.Errors))
			}
		}
		return struct{}{}, false, nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return lastState, fmt.Errorf("timed out waiting for asset %s delivery: %w", assetID, err)
		}
		return lastState, err
	}

	return lastState, nil
}

func formatAssetErrors(errors []asc.ErrorDetail) string {
	if len(errors) == 0 {
		return "unknown error"
	}
	parts := make([]string, 0, len(errors))
	for _, item := range errors {
		if item.Code != "" && item.Message != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", item.Code, item.Message))
			continue
		}
		if item.Message != "" {
			parts = append(parts, item.Message)
			continue
		}
		if item.Code != "" {
			parts = append(parts, item.Code)
		}
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}
