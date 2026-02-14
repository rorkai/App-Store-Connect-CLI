package screenshots

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	ProviderAXe = "axe"
)

// Provider captures a single screenshot and returns the path to the PNG.
type Provider interface {
	Capture(ctx context.Context, req CaptureRequest) (pngPath string, err error)
}

// Capture runs the appropriate provider and validates the output file.
func Capture(ctx context.Context, req CaptureRequest) (*CaptureResult, error) {
	return CaptureWithProvider(ctx, req, nil)
}

// CaptureWithProvider runs the given provider (or selects by req.Provider if nil) and validates the output file.
// Used for testing with a mock provider.
func CaptureWithProvider(ctx context.Context, req CaptureRequest, p Provider) (*CaptureResult, error) {
	req.Provider = strings.TrimSpace(strings.ToLower(req.Provider))
	req.Name = strings.TrimSpace(req.Name)
	req.OutputDir = strings.TrimSpace(req.OutputDir)
	if err := validateCaptureDestination(req.Name, req.OutputDir); err != nil {
		return nil, err
	}

	if p == nil {
		switch req.Provider {
		case ProviderAXe:
			p = &AXeProvider{}
		default:
			return nil, fmt.Errorf("unknown provider %q", req.Provider)
		}
	}

	pngPath, err := p.Capture(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := asc.ValidateImageFile(pngPath); err != nil {
		return nil, fmt.Errorf("captured file invalid: %w", err)
	}
	dims, err := asc.ReadImageDimensions(pngPath)
	if err != nil {
		return nil, fmt.Errorf("read image dimensions: %w", err)
	}

	absPath, err := filepath.Abs(pngPath)
	if err != nil {
		return nil, fmt.Errorf("resolve captured path: %w", err)
	}
	return &CaptureResult{
		Path:     absPath,
		Provider: req.Provider,
		Width:    dims.Width,
		Height:   dims.Height,
		BundleID: req.BundleID,
		UDID:     req.UDID,
	}, nil
}

func validateCaptureDestination(name, outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if name == "" {
		return fmt.Errorf("screenshot name is required")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("screenshot name must be a file name without path separators")
	}
	return nil
}
