package signing

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SigningFetchCommand returns the signing fetch subcommand.
func SigningFetchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (optional, or ASC_APP_ID env)")
	bundleID := fs.String("bundle-id", "", "Bundle identifier (e.g., com.example.app) - required")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, MAC_APP_STORE, etc. (required)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (required for development profiles)")
	certType := fs.String("certificate-type", "", "Certificate type filter (optional)")
	outputPath := fs.String("output", "./signing", "Output directory for signing files")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	output := shared.BindOutputFlagsWith(fs, "format", "json", "Output format for metadata: json (default), table, markdown")

	return &ffcli.Command{
		Name:       "fetch",
		ShortUsage: "asc signing fetch [flags]",
		ShortHelp:  "Fetch signing files (certificates + profiles) for an app.",
		LongHelp: `Fetch signing certificates and provisioning profiles for an app.

This command resolves the bundle ID, finds matching certificates and profiles,
and writes them to the output directory.

With --create-missing, it will create a new profile if none exist for the
specified configuration.

Examples:
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --output ./signing
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_DEVELOPMENT --device "DEVICE1,DEVICE2"
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --create-missing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			bundle := strings.TrimSpace(*bundleID)
			if bundle == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
				return flag.ErrHelp
			}

			profType := strings.TrimSpace(*profileType)
			if profType == "" {
				fmt.Fprintln(os.Stderr, "Error: --profile-type is required")
				return flag.ErrHelp
			}
			profType = strings.ToUpper(profType)
			if *createMissing && isDevelopmentProfile(profType) && strings.TrimSpace(*deviceIDs) == "" {
				fmt.Fprintln(os.Stderr, "Error: --device is required for development profiles")
				return flag.ErrHelp
			}

			outputDir := strings.TrimSpace(*outputPath)
			if outputDir == "" {
				outputDir = "./signing"
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID != "" {
				if err := validateBundleIDMatchesApp(requestCtx, client, resolvedAppID, bundle); err != nil {
					return fmt.Errorf("signing fetch: %w", err)
				}
			}

			result := &asc.SigningFetchResult{
				BundleID:    bundle,
				ProfileType: profType,
				OutputPath:  outputDir,
			}

			bundleIDResp, err := findBundleID(requestCtx, client, bundle)
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}
			result.BundleIDResource = bundleIDResp.Data.ID

			certs, err := findCertificates(requestCtx, client, profType, *certType)
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}
			result.CertificateIDs = extractIDs(certs.Data)

			profile, created, err := findOrCreateProfile(
				requestCtx,
				client,
				bundleIDResp.Data.ID,
				bundle,
				profType,
				result.CertificateIDs,
				shared.SplitCSV(*deviceIDs),
				*createMissing,
			)
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}
			result.ProfileID = profile.Data.ID
			result.Created = created

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("signing fetch: create output dir: %w", err)
			}

			profileName := safeFileName(profile.Data.Attributes.Name, profile.Data.ID)
			profilePath := filepath.Join(outputDir, profileName+".mobileprovision")
			profileContent, err := decodeBase64Content("profile", profile.Data.Attributes.ProfileContent)
			if err != nil {
				return fmt.Errorf("signing fetch: decode profile: %w", err)
			}
			if err := shared.WriteProfileFile(profilePath, profileContent); err != nil {
				return fmt.Errorf("signing fetch: write profile: %w", err)
			}
			result.ProfileFile = profilePath

			for _, cert := range certs.Data {
				certName := safeFileName(cert.Attributes.SerialNumber, cert.ID)
				certPath := filepath.Join(outputDir, certName+".cer")
				certContent, err := decodeBase64Content("certificate", cert.Attributes.CertificateContent)
				if err != nil {
					return fmt.Errorf("signing fetch: decode certificate: %w", err)
				}
				if err := writeBinaryFile(certPath, certContent); err != nil {
					return fmt.Errorf("signing fetch: write certificate: %w", err)
				}
				result.CertificateFiles = append(result.CertificateFiles, certPath)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func validateBundleIDMatchesApp(ctx context.Context, client *asc.Client, appID, bundleID string) error {
	app, err := client.GetApp(ctx, appID)
	if err != nil {
		return fmt.Errorf("fetch app: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(app.Data.Attributes.BundleID), strings.TrimSpace(bundleID)) {
		return fmt.Errorf("bundle ID %s does not match app %s (expected %s)", bundleID, appID, app.Data.Attributes.BundleID)
	}
	return nil
}

func findBundleID(ctx context.Context, client *asc.Client, identifier string) (*asc.BundleIDResponse, error) {
	resp, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(identifier))
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("bundle ID not found: %s", identifier)
	}
	return &asc.BundleIDResponse{Data: resp.Data[0]}, nil
}

func findCertificates(ctx context.Context, client *asc.Client, profileType, certType string) (*asc.CertificatesResponse, error) {
	certType = strings.TrimSpace(certType)
	if certType == "" {
		inferred, err := inferCertificateType(profileType)
		if err != nil {
			return nil, err
		}
		certType = inferred
	}

	var (
		all   []asc.Resource[asc.CertificateAttributes]
		links asc.Links
		next  string
	)
	for {
		resp, err := client.GetCertificates(ctx,
			asc.WithCertificatesFilterType(certType),
			asc.WithCertificatesNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
		links = resp.Links
		if strings.TrimSpace(resp.Links.Next) == "" {
			break
		}
		next = resp.Links.Next
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no certificates found for type %s", certType)
	}
	return &asc.CertificatesResponse{Data: all, Links: links}, nil
}

func findOrCreateProfile(ctx context.Context, client *asc.Client, bundleIDResourceID, bundleIdentifier, profileType string, certIDs, deviceIDs []string, createMissing bool) (*asc.ProfileResponse, bool, error) {
	next := ""
	for {
		profiles, err := client.GetProfiles(ctx,
			asc.WithProfilesFilterType(profileType),
			asc.WithProfilesNextURL(next),
		)
		if err != nil {
			return nil, false, err
		}

		for _, profile := range profiles.Data {
			if profile.Attributes.ProfileState != asc.ProfileStateActive {
				continue
			}
			content := strings.TrimSpace(profile.Attributes.ProfileContent)
			if content == "" {
				continue
			}
			decoded, err := decodeBase64Content("profile", content)
			if err != nil {
				return nil, false, err
			}
			if strings.Contains(string(decoded), bundleIdentifier) {
				return &asc.ProfileResponse{Data: profile}, false, nil
			}
		}

		if strings.TrimSpace(profiles.Links.Next) == "" {
			break
		}
		next = profiles.Links.Next
	}

	if !createMissing {
		return nil, false, fmt.Errorf("no active profile found for bundle ID; use --create-missing to create one")
	}
	if len(certIDs) == 0 {
		return nil, false, fmt.Errorf("no certificates available to create profile")
	}
	name := fmt.Sprintf("%s-%s", profileType, time.Now().Format("20060102"))
	profile, err := client.CreateProfile(ctx, asc.ProfileCreateAttributes{
		Name:        name,
		ProfileType: profileType,
	}, bundleIDResourceID, certIDs, deviceIDs)
	if err != nil {
		return nil, false, err
	}
	return profile, true, nil
}

func isDevelopmentProfile(profileType string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(profileType))
	return strings.Contains(normalized, "DEVELOPMENT") ||
		strings.Contains(normalized, "ADHOC") ||
		strings.Contains(normalized, "AD_HOC")
}

func inferCertificateType(profileType string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(profileType))

	switch {
	case strings.Contains(normalized, "IOS_APP_DEVELOPMENT"):
		return "IOS_DEVELOPMENT", nil
	case strings.Contains(normalized, "IOS_APP_STORE"),
		strings.Contains(normalized, "IOS_APP_ADHOC"),
		strings.Contains(normalized, "IOS_APP_INHOUSE"):
		return "IOS_DISTRIBUTION", nil
	case strings.Contains(normalized, "TVOS_APP_DEVELOPMENT"):
		return "TVOS_DEVELOPMENT", nil
	case strings.Contains(normalized, "TVOS_APP_STORE"),
		strings.Contains(normalized, "TVOS_APP_ADHOC"),
		strings.Contains(normalized, "TVOS_APP_INHOUSE"):
		return "TVOS_DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_CATALYST_APP_DEVELOPMENT"):
		return "IOS_DEVELOPMENT", nil
	case strings.Contains(normalized, "MAC_CATALYST_APP_STORE"):
		return "MAC_APP_DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_CATALYST_APP_DIRECT"):
		return "DEVELOPER_ID_APPLICATION", nil
	case strings.Contains(normalized, "MAC_APP_DEVELOPMENT"):
		return "MAC_APP_DEVELOPMENT", nil
	case strings.Contains(normalized, "MAC_APP_STORE"):
		return "MAC_APP_DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_APP_DIRECT"):
		return "DEVELOPER_ID_APPLICATION", nil
	default:
		return "", fmt.Errorf("unable to infer certificate type for profile type %s; use --certificate-type", profileType)
	}
}

func decodeBase64Content(label, content string) ([]byte, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("%s content is empty", label)
	}
	data, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return data, nil
}

func writeBinaryFile(path string, data []byte) error {
	file, err := shared.OpenNewFileNoFollow(path, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file already exists: %w", err)
		}
		return err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func extractIDs[T any](items []asc.Resource[T]) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func safeFileName(value, fallback string) string {
	sanitize := func(input string) string {
		clean := strings.TrimSpace(input)
		clean = strings.ReplaceAll(clean, "/", "_")
		clean = strings.ReplaceAll(clean, "\\", "_")
		return strings.Trim(clean, ". ")
	}

	clean := sanitize(value)
	if clean == "" || clean == "." || clean == ".." {
		clean = sanitize(fallback)
	}
	return clean
}
