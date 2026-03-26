package signing

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const matchPasswordEnvVar = "ASC_MATCH_PASSWORD"

// SyncResult is the structured output for sync operations.
type SyncResult struct {
	Operation   string   `json:"operation"`
	RepoURL     string   `json:"repoUrl"`
	BundleID    string   `json:"bundleId"`
	ProfileType string   `json:"profileType"`
	Files       []string `json:"files"`
}

// SigningSyncCommand returns the signing sync command group.
func SigningSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc signing sync <subcommand> [flags]",
		ShortHelp:  "Sync signing assets with an encrypted git repo.",
		LongHelp: `Sync signing certificates and provisioning profiles with an encrypted git repository.

Lightweight alternative to fastlane match. Fetches signing assets from
App Store Connect, encrypts them, and stores in a shared git repo.
Team members pull and decrypt to get signing files.

Examples:
  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
    --repo git@github.com:team/certs.git --password "$MATCH_PASSWORD"

  asc signing sync pull --repo git@github.com:team/certs.git --password "$MATCH_PASSWORD" \
    --output-dir ./signing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			syncPushCommand(),
			syncPullCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func resolvePassword(flagValue string) (string, error) {
	password := strings.TrimSpace(flagValue)
	if password != "" {
		return password, nil
	}
	password = strings.TrimSpace(os.Getenv(matchPasswordEnvVar))
	if password != "" {
		return password, nil
	}
	return "", shared.UsageError("--password is required (or set ASC_MATCH_PASSWORD)")
}

func syncPushCommand() *ffcli.Command {
	fs := flag.NewFlagSet("push", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Bundle identifier (required)")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, etc. (required)")
	repoURL := fs.String("repo", "", "Git repo URL for encrypted storage (required)")
	password := fs.String("password", "", "Encryption password (or ASC_MATCH_PASSWORD env)")
	branch := fs.String("branch", "main", "Git branch")
	certType := fs.String("certificate-type", "", "Certificate type filter (optional)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (for development profiles)")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "push",
		ShortUsage: "asc signing sync push --bundle-id ID --profile-type TYPE --repo URL --password PASS",
		ShortHelp:  "Fetch signing assets from ASC, encrypt, and push to git.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			bundle := strings.TrimSpace(*bundleID)
			if bundle == "" {
				return shared.UsageError("--bundle-id is required")
			}
			profType := strings.ToUpper(strings.TrimSpace(*profileType))
			if profType == "" {
				return shared.UsageError("--profile-type is required")
			}
			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			if *createMissing && isDevelopmentProfile(profType) && strings.TrimSpace(*deviceIDs) == "" {
				return shared.UsageError("--device is required for development profiles with --create-missing")
			}

			pass, err := resolvePassword(*password)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Fetch signing assets from ASC.
			fmt.Fprintln(os.Stderr, "Fetching signing assets from App Store Connect...")

			bundleIDResp, err := findBundleID(requestCtx, client, bundle)
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			certs, err := findCertificates(requestCtx, client, profType, *certType)
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			profile, created, err := findOrCreateProfile(
				requestCtx, client,
				bundleIDResp.Data.ID, bundle, profType,
				extractIDs(certs.Data),
				shared.SplitCSV(*deviceIDs),
				*createMissing,
			)
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}
			if created {
				fmt.Fprintln(os.Stderr, "Created new profile")
			}

			// Clone git repo.
			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
			if err != nil {
				return fmt.Errorf("signing sync push: create temp dir: %w", err)
			}

			store := &signingpkg.GitStore{
				RepoURL:  repo,
				LocalDir: tmpDir,
				Branch:   *branch,
			}
			defer func() { _ = store.Cleanup() }()

			fmt.Fprintln(os.Stderr, "Cloning signing repo...")
			if err := store.Clone(ctx, true); err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			// Write encrypted files.
			var files []string

			certDir := certDirectoryName(profType)
			for _, cert := range certs.Data {
				certContent, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cert.Attributes.CertificateContent))
				if err != nil {
					return fmt.Errorf("signing sync push: decode cert: %w", err)
				}
				relPath := filepath.Join("certs", certDir, safeFileName(cert.Attributes.SerialNumber, cert.ID)+".cer")
				if err := store.WriteEncryptedFile(relPath, certContent, pass); err != nil {
					return fmt.Errorf("signing sync push: encrypt cert: %w", err)
				}
				files = append(files, relPath)
				fmt.Fprintf(os.Stderr, "  Encrypted %s\n", relPath)
			}

			profileContent, err := base64.StdEncoding.DecodeString(strings.TrimSpace(profile.Data.Attributes.ProfileContent))
			if err != nil {
				return fmt.Errorf("signing sync push: decode profile: %w", err)
			}
			profileDir := profileDirectoryName(profType)
			profileRelPath := filepath.Join("profiles", profileDir, safeFileName(profile.Data.Attributes.Name, profile.Data.ID)+".mobileprovision")
			if err := store.WriteEncryptedFile(profileRelPath, profileContent, pass); err != nil {
				return fmt.Errorf("signing sync push: encrypt profile: %w", err)
			}
			files = append(files, profileRelPath)
			fmt.Fprintf(os.Stderr, "  Encrypted %s\n", profileRelPath)

			// Commit and push.
			commitMsg := fmt.Sprintf("Update signing assets for %s (%s)", bundle, profType)
			fmt.Fprintln(os.Stderr, "Pushing to git...")
			if err := store.CommitAndPush(ctx, commitMsg); err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			fmt.Fprintln(os.Stderr, "Done")

			result := SyncResult{
				Operation:   "push",
				RepoURL:     sanitizeRepoURLForOutput(repo),
				BundleID:    bundle,
				ProfileType: profType,
				Files:       files,
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func syncPullCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)

	repoURL := fs.String("repo", "", "Git repo URL (required)")
	password := fs.String("password", "", "Decryption password (or ASC_MATCH_PASSWORD env)")
	branch := fs.String("branch", "main", "Git branch")
	outputDir := fs.String("output-dir", "./signing", "Output directory for decrypted files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc signing sync pull --repo URL --password PASS [--output-dir DIR]",
		ShortHelp:  "Pull and decrypt signing assets from git.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			pass, err := resolvePassword(*password)
			if err != nil {
				return err
			}

			outDir := strings.TrimSpace(*outputDir)
			if outDir == "" {
				outDir = "./signing"
			}

			// Clone git repo.
			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
			if err != nil {
				return fmt.Errorf("signing sync pull: create temp dir: %w", err)
			}

			store := &signingpkg.GitStore{
				RepoURL:  repo,
				LocalDir: tmpDir,
				Branch:   *branch,
			}
			defer func() { _ = store.Cleanup() }()

			fmt.Fprintln(os.Stderr, "Cloning signing repo...")
			if err := store.Clone(ctx, false); err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}

			// List and decrypt all files.
			encryptedFiles, err := store.ListEncryptedFiles()
			if err != nil {
				return fmt.Errorf("signing sync pull: list files: %w", err)
			}

			if len(encryptedFiles) == 0 {
				fmt.Fprintln(os.Stderr, "No encrypted signing files found in repo")
				result := SyncResult{
					Operation: "pull",
					RepoURL:   sanitizeRepoURLForOutput(repo),
					Files:     []string{},
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("signing sync pull: create output dir: %w", err)
			}

			var files []string
			for _, relPath := range encryptedFiles {
				plaintext, err := store.ReadEncryptedFile(relPath, pass)
				if err != nil {
					return fmt.Errorf("signing sync pull: decrypt %s: %w", relPath, err)
				}

				if err := writeDecryptedOutputFile(outDir, relPath, plaintext); err != nil {
					return fmt.Errorf("signing sync pull: %w", err)
				}

				files = append(files, relPath)
				fmt.Fprintf(os.Stderr, "  Decrypted %s\n", relPath)
			}

			fmt.Fprintf(os.Stderr, "Done — %d files written to %s\n", len(files), outDir)

			result := SyncResult{
				Operation: "pull",
				RepoURL:   sanitizeRepoURLForOutput(repo),
				Files:     files,
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func sanitizeRepoURLForOutput(raw string) string {
	return urlsanitize.SanitizeURLForLog(raw, urlsanitize.DefaultSignedQueryKeys, urlsanitize.DefaultSensitiveQueryKeys)
}

func writeDecryptedOutputFile(outDir, relPath string, plaintext []byte) error {
	destPath := filepath.Join(outDir, relPath)
	if err := signingpkg.EnsureInsideDir(outDir, destPath); err != nil {
		return fmt.Errorf("path escape in %s: %w", relPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := signingpkg.RejectSymlinkIfExists(destPath); err != nil {
		return fmt.Errorf("path escape in %s: %w", relPath, err)
	}
	if err := os.WriteFile(destPath, plaintext, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", relPath, err)
	}
	return nil
}

func certDirectoryName(profileType string) string {
	normalized := strings.ToUpper(profileType)
	if strings.Contains(normalized, "DEVELOPMENT") {
		return "development"
	}
	return "distribution"
}

func profileDirectoryName(profileType string) string {
	normalized := strings.ToUpper(profileType)
	switch {
	case strings.Contains(normalized, "STORE"):
		return "appstore"
	case strings.Contains(normalized, "ADHOC"), strings.Contains(normalized, "AD_HOC"):
		return "adhoc"
	case strings.Contains(normalized, "DEVELOPMENT"):
		return "development"
	case strings.Contains(normalized, "INHOUSE"), strings.Contains(normalized, "IN_HOUSE"):
		return "enterprise"
	default:
		return "other"
	}
}
