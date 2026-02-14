package assets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	assetUploadDefaultTimeout = 10 * time.Minute
	assetPollInterval         = 2 * time.Second
)

func contextWithAssetUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, asc.ResolveTimeoutWithDefault(assetUploadDefaultTimeout))
}

// ContextWithAssetUploadTimeout returns a context with the asset upload timeout.
func ContextWithAssetUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return contextWithAssetUploadTimeout(ctx)
}

func collectAssetFiles(path string) ([]string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", path)
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		files := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fullPath := filepath.Join(path, entry.Name())
			if err := asc.ValidateImageFile(fullPath); err != nil {
				return nil, err
			}
			files = append(files, fullPath)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no files found in %q", path)
		}
		sort.Strings(files)
		return files, nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file: %q", path)
	}
	if err := asc.ValidateImageFile(path); err != nil {
		return nil, err
	}
	return []string{path}, nil
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
