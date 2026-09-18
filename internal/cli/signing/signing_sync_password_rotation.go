package signing

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func syncRotatePasswordCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rotate-password", flag.ExitOnError)

	repoURL := fs.String("repo", "", "Git repo URL (required)")
	passwordFile := fs.String("password-file", "", "Protected file containing the current repository encryption password (required)")
	newPasswordFile := fs.String("new-password-file", "", "Protected file containing the new repository encryption password (required)")
	branch := fs.String("branch", "main", "Git branch")
	storage := fs.String("storage", signingSyncStorageGit, "Encrypted artifact storage (only "+signingSyncStorageGit+" supports password rotation)")
	confirm := fs.Bool("confirm", false, "Confirm that the previous password will no longer decrypt the branch head")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "rotate-password",
		ShortUsage: "asc signing sync rotate-password --repo URL --password-file PATH --new-password-file PATH --confirm",
		ShortHelp:  "Re-encrypt every signing asset with a new repository password.",
		LongHelp: `Re-encrypt every artifact in an encrypted signing Git repository.

The complete repository is authenticated and validated with the current
password before any local rewrite. Private PKCS#12 identities are rewrapped
with the new password, the rewritten repository is validated again, and all
artifacts are published in one Git commit.

Both password files must be protected regular files with mode 0600 or more
restrictive. Distribute the new secret before dependent jobs pull again.

Example:
  asc signing sync rotate-password --repo git@github.com:team/certs.git \
    --password-file ~/.config/asc/signing-sync-password \
    --new-password-file ~/.config/asc/signing-sync-password-next --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			selectedStorage := strings.ToLower(strings.TrimSpace(*storage))
			if selectedStorage == "" {
				selectedStorage = signingSyncStorageGit
			}
			if selectedStorage != signingSyncStorageGit {
				return shared.UsageErrorf("signing sync rotate-password supports only --storage %s", signingSyncStorageGit)
			}
			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			currentPath := strings.TrimSpace(*passwordFile)
			if currentPath == "" {
				return shared.UsageError("--password-file is required")
			}
			newPath := strings.TrimSpace(*newPasswordFile)
			if newPath == "" {
				return shared.UsageError("--new-password-file is required")
			}
			selectedBranch := strings.TrimSpace(*branch)
			if selectedBranch == "" {
				return shared.UsageError("--branch must not be empty")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required to rotate the signing sync password")
			}

			currentPassword, nextPassword, err := readSigningSyncRotationPasswords(currentPath, newPath)
			if err != nil {
				return err
			}

			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-rotate-*")
			if err != nil {
				return fmt.Errorf("signing sync rotate-password: create temp dir: %w", err)
			}
			store := &signingpkg.GitStore{RepoURL: repo, LocalDir: tmpDir, Branch: selectedBranch}
			defer func() { _ = store.Cleanup() }()

			fmt.Fprintln(os.Stderr, "Cloning signing repo...")
			if err := store.Clone(ctx, false); err != nil {
				return fmt.Errorf("signing sync rotate-password: %w", err)
			}
			encryptedFiles, err := store.ListEncryptedFiles()
			if err != nil {
				return fmt.Errorf("signing sync rotate-password: list files: %w", err)
			}
			slices.Sort(encryptedFiles)
			if len(encryptedFiles) == 0 {
				fmt.Fprintln(os.Stderr, "No encrypted signing files found in repo")
				return shared.PrintOutput(&SyncResult{
					Operation: "rotate-password",
					RepoURL:   sanitizeRepoURLForOutput(repo),
					Files:     []string{},
				}, *output.Output, *output.Pretty)
			}

			decrypted, _, err := loadAndValidateSigningFiles(store, encryptedFiles, currentPassword)
			if err != nil {
				return fmt.Errorf("signing sync rotate-password: %w", err)
			}
			if err := rewriteSigningFilesWithPassword(store, decrypted, currentPassword, nextPassword); err != nil {
				return fmt.Errorf("signing sync rotate-password: %w", err)
			}
			rewritten, _, err := loadAndValidateSigningFiles(store, encryptedFiles, nextPassword)
			if err != nil {
				return fmt.Errorf("signing sync rotate-password: validate rewritten repository: %w", err)
			}

			fmt.Fprintln(os.Stderr, "Publishing rotated signing assets...")
			if err := store.CommitAndPush(ctx, "Rotate signing sync password"); err != nil {
				return fmt.Errorf("signing sync rotate-password: %w", err)
			}

			sensitiveFiles := make([]string, 0)
			identityPresent := false
			for _, file := range rewritten {
				if file.Sensitive {
					sensitiveFiles = append(sensitiveFiles, file.RelativePath)
				}
				identityPresent = identityPresent || file.Identity
			}
			fmt.Fprintf(os.Stderr, "Done — %d encrypted signing files rotated\n", len(encryptedFiles))
			return shared.PrintOutput(&SyncResult{
				Operation:       "rotate-password",
				RepoURL:         sanitizeRepoURLForOutput(repo),
				Files:           encryptedFiles,
				IdentityPresent: identityPresent,
				SensitiveFiles:  sensitiveFiles,
			}, *output.Output, *output.Pretty)
		},
	}
}

func readSigningSyncRotationPasswords(currentPath, newPath string) (string, string, error) {
	currentInfo, err := os.Stat(currentPath)
	if err == nil {
		if nextInfo, nextErr := os.Stat(newPath); nextErr == nil && os.SameFile(currentInfo, nextInfo) {
			return "", "", shared.UsageError("--password-file and --new-password-file must identify different files")
		}
	}
	currentData, err := readProtectedSecretFile(currentPath, "current signing sync password")
	if err != nil {
		return "", "", err
	}
	nextData, err := readProtectedSecretFile(newPath, "new signing sync password")
	if err != nil {
		return "", "", err
	}
	currentPassword := trimPasswordFileNewline(string(currentData))
	if currentPassword == "" {
		return "", "", shared.UsageError("current signing sync password file is empty")
	}
	nextPassword := trimPasswordFileNewline(string(nextData))
	if nextPassword == "" {
		return "", "", shared.UsageError("new signing sync password file is empty")
	}
	if subtle.ConstantTimeCompare([]byte(currentPassword), []byte(nextPassword)) == 1 {
		return "", "", shared.UsageError("current and new signing sync passwords must differ")
	}
	return currentPassword, nextPassword, nil
}

func rewriteSigningFilesWithPassword(store *signingpkg.GitStore, files []decryptedSigningFile, currentPassword, newPassword string) error {
	for _, file := range files {
		plaintext := file.Plaintext
		if file.Identity {
			privateKey, certificate, err := modernpkcs12.Decode(plaintext, currentPassword)
			if err != nil || certificate == nil {
				return fmt.Errorf("rewrap %s: identity is not decodable with the current password", file.RelativePath)
			}
			plaintext, err = normalizeSigningIdentity(&signingIdentity{
				PrivateKey:        privateKey,
				Certificate:       certificate,
				CertificateSHA256: signingCertificateSHA256(certificate),
			}, newPassword)
			if err != nil {
				return fmt.Errorf("rewrap %s: %w", file.RelativePath, err)
			}
		}

		var err error
		if file.Metadata.Version == 0 {
			err = store.ReplaceEncryptedFile(file.RelativePath, plaintext, newPassword)
		} else {
			err = store.ReplaceEncryptedFileWithMetadata(file.RelativePath, plaintext, newPassword, file.Metadata)
		}
		if err != nil {
			return fmt.Errorf("reencrypt %s: %w", file.RelativePath, err)
		}
	}
	return nil
}
