package ads

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// PlatformAssetUploadCommand returns the dedicated Platform API asset upload
// command. Unlike the generated mutation commands, this sends multipart image
// data rather than a JSON payload.
func PlatformAssetUploadCommand() *ffcli.Command {
	uploadSpec, ok := appleads.PlatformEndpointByCommandPath("assets", "upload")
	if !ok {
		panic("missing Apple Ads asset upload endpoint metadata")
	}
	fs := flag.NewFlagSet("ads assets upload", flag.ExitOnError)
	filePath := fs.String("file", "", "Path to image file (PNG, JPEG, or HEIC) (required)")
	brandID := fs.String("brand", "", "Apple Ads business brand ID (required)")
	common := commonFlags{
		AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
		AdAccount:  fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID env)"),
	}
	output := bindAdsRawOutputFlags(fs)
	longHelp := `Upload an Apple Ads brand image asset.

The image is sent as multipart/form-data with the promoted object type
BUSINESS_BRAND. Supported filename extensions are .png, .jpg, .jpeg, and .heic.
Apple processes the asset after upload. Poll "asc ads assets view"
until eligibility.status is ELIGIBLE before using the asset in a creative.
LIMITED assets require checking allowedGroups; PENDING and INELIGIBLE assets
must not be used.`
	longHelp += endpointBodyHelp(uploadSpec)
	longHelp += `

Example:
  asc ads assets upload --file ./brand.png --brand "BRAND_ID" --ad-account "AD_ACCOUNT_ID"`
	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc ads assets upload --file IMAGE --brand ID --ad-account ID",
		ShortHelp:  "Upload an Apple Ads brand image asset.",
		LongHelp:   longHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			outputFormat, err := validateAdsRawOutput(output)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError("--file")
			}
			brandValue := strings.TrimSpace(*brandID)
			if brandValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --brand is required")
				return shared.MissingRequiredUsageError("--brand")
			}

			file, size, fileName, contentType, err := openPlatformAssetUploadFile(fileValue)
			if err != nil {
				return fmt.Errorf("ads assets upload: %w", err)
			}
			defer file.Close()

			client, _, err := resolvePlatformClientAndAdAccountID(ctx, common, appleads.ContextAdAccount)
			if err != nil {
				return fmt.Errorf("ads assets upload: %w", err)
			}
			uploadCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()
			response, err := client.UploadPlatformAsset(uploadCtx, file, size, fileName, contentType, brandValue)
			if err != nil {
				return fmt.Errorf("ads assets upload: %w", err)
			}
			return shared.PrintOutput(response, outputFormat, *output.Pretty)
		},
	}
}

func openPlatformAssetUploadFile(path string) (*os.File, int64, string, string, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		switch {
		case errors.Is(err, rootfs.ErrSymlink):
			return nil, 0, "", "", fmt.Errorf("asset file must not be a symlink: %w", rootfs.ErrSymlink)
		case errors.Is(err, os.ErrNotExist):
			return nil, 0, "", "", fmt.Errorf("asset file could not be opened: %w", os.ErrNotExist)
		case strings.Contains(err.Error(), "not a regular file"):
			return nil, 0, "", "", fmt.Errorf("asset file must be a regular file")
		default:
			return nil, 0, "", "", fmt.Errorf("asset file could not be opened")
		}
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, "", "", fmt.Errorf("asset file could not be inspected")
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, 0, "", "", fmt.Errorf("asset file must be a regular file")
	}
	if info.Size() == 0 {
		_ = file.Close()
		return nil, 0, "", "", fmt.Errorf("asset file must not be empty")
	}

	fileName := filepath.Base(path)
	var contentType string
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".heic":
		contentType = "image/heic"
	default:
		_ = file.Close()
		return nil, 0, "", "", fmt.Errorf("asset file must be PNG, JPEG, or HEIC")
	}
	return file, info.Size(), fileName, contentType, nil
}
