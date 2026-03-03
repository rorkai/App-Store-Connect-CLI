package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	assetDownloadMaxAttempts  = 4
	assetDownloadInitialDelay = 200 * time.Millisecond
	assetDownloadMaxDelay     = 2 * time.Second
	assetDownloadUserAgent    = "curl/8.7.1 App-Store-Connect-CLI/asset-download"
)

type downloadHTTPStatusError struct {
	StatusCode int
	Message    string
}

func (e *downloadHTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d (%s)", e.StatusCode, e.Message)
}

func sanitizeBaseFileName(value string) string {
	base := strings.TrimSpace(value)
	if base == "" {
		return ""
	}

	// Defensive: ensure we never write outside the target directory.
	base = filepath.Base(base)
	base = strings.TrimSpace(base)

	if base == "" || base == "." || base == ".." {
		return ""
	}

	// Extra defense: normalize separators across platforms.
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	base = strings.TrimSpace(base)

	if base == "" || base == "." || base == ".." {
		return ""
	}
	return base
}

func resolveImageAssetDownloadURL(asset *asc.ImageAsset, fileName string) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("image asset is missing")
	}

	template := strings.TrimSpace(asset.TemplateURL)
	if template == "" {
		return "", fmt.Errorf("image asset template URL is missing")
	}
	if asset.Width <= 0 || asset.Height <= 0 {
		return "", fmt.Errorf("image asset dimensions are missing")
	}

	resolved := template
	resolved = strings.ReplaceAll(resolved, "{w}", fmt.Sprintf("%d", asset.Width))
	resolved = strings.ReplaceAll(resolved, "{h}", fmt.Sprintf("%d", asset.Height))
	if strings.Contains(resolved, "{f}") {
		// ASC imageAsset.templateUrl often includes "{f}" for file format.
		// Prefer the extension from the asset filename when available; fall back to png.
		format := ""
		ext := strings.TrimSpace(filepath.Ext(strings.TrimSpace(fileName)))
		if ext != "" {
			format = strings.TrimPrefix(ext, ".")
		}
		if strings.TrimSpace(format) == "" {
			format = "png"
		}
		resolved = strings.ReplaceAll(resolved, "{f}", format)
	}

	// If the URL still contains template braces, it is likely not usable as-is.
	if strings.Contains(resolved, "{") || strings.Contains(resolved, "}") {
		return "", fmt.Errorf("unresolved template URL: %q", template)
	}

	parsed, err := url.Parse(resolved)
	if err != nil {
		return "", fmt.Errorf("parse resolved URL: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		// ok
	default:
		return "", fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}

	return resolved, nil
}

func downloadURLToFile(ctx context.Context, rawURL string, outputPath string, overwrite bool) (int64, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return 0, "", fmt.Errorf("download URL is required")
	}
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return 0, "", fmt.Errorf("output path is required")
	}

	delay := assetDownloadInitialDelay
	var lastErr error
	lastContentType := ""

	for attempt := 1; attempt <= assetDownloadMaxAttempts; attempt++ {
		written, contentType, err := downloadURLToFileOnce(ctx, rawURL, outputPath, overwrite)
		if err == nil {
			return written, contentType, nil
		}

		lastErr = err
		lastContentType = contentType

		if !isRetryableDownloadError(err) || attempt == assetDownloadMaxAttempts {
			return 0, lastContentType, lastErr
		}

		if err := sleepWithContext(ctx, delay); err != nil {
			return 0, lastContentType, err
		}

		delay *= 2
		if delay > assetDownloadMaxDelay {
			delay = assetDownloadMaxDelay
		}
	}

	return 0, lastContentType, lastErr
}

func downloadURLToFileOnce(ctx context.Context, rawURL string, outputPath string, overwrite bool) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", assetDownloadUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			msg = strings.Join(strings.Fields(msg), " ")
		}
		if msg == "" {
			msg = strings.TrimSpace(resp.Status)
		}
		return 0, contentType, &downloadHTTPStatusError{
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	n, err := writeDownloadedFile(outputPath, resp.Body, overwrite)
	return n, contentType, err
}

func isRetryableDownloadError(err error) bool {
	var statusErr *downloadHTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusForbidden,
			http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writeDownloadedFile(path string, reader io.Reader, overwrite bool) (int64, error) {
	return shared.SafeWriteFileNoSymlink(
		path,
		0o600,
		overwrite,
		".asc-download-*",
		".asc-download-backup-*",
		func(f *os.File) (int64, error) {
			return io.Copy(f, reader)
		},
	)
}
