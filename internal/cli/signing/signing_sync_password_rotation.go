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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func syncRotatePasswordCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rotate-password", flag.ExitOnError)

	repoURL := fs.String("repo", "", "Git repo URL (required with --storage git)")
	passwordFile := fs.String("password-file", "", "Protected file containing the current repository encryption password (required)")
	newPasswordFile := fs.String("new-password-file", "", "Protected file containing the new repository encryption password (required)")
	branch := fs.String("branch", "main", "Git branch")
	storage := fs.String("storage", signingSyncStorageGit, "Encrypted artifact storage ("+signingSyncStorageGit+" or "+signingSyncStorageObject+")")
	objectFlags := bindSigningSyncObjectFlags(fs)
	confirm := fs.Bool("confirm", false, "Confirm that the previous password will no longer decrypt the stored artifacts")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "rotate-password",
		ShortUsage: "asc signing sync rotate-password (--repo URL | --storage object --object-bucket NAME) --password-file PATH --new-password-file PATH --confirm",
		ShortHelp:  "Re-encrypt every signing asset with a new repository password.",
		LongHelp: `Re-encrypt every artifact in an encrypted signing Git repository or
object storage prefix.

The complete store is authenticated and validated with the current password
before any local rewrite. Private PKCS#12 identities are rewrapped with the
new password, and the rewritten artifacts are validated again before anything
is published.

Git storage publishes all artifacts in one Git commit.

Object storage cannot replace several objects atomically, so rotation writes
new objects first and then swaps them in:
  1. Every new ciphertext is written to a staged key,
     <key>.asc-rotation-<id>, which pulls ignore. A failure here removes
     the staged objects and changes no live artifact.
  2. Every live object must still match the version that was fetched. A
     concurrent push aborts the rotation before any live write.
  3. Each live object is replaced only if it still matches the fetched
     version. If one replacement fails, the artifacts already replaced are
     restored, so every artifact keeps the current password. If restoring
     also fails, the error lists the artifacts that use the new password,
     and the staged objects are kept so you can finish the rotation by
     copying each one over its live key.

Staged objects are deleted after a successful rotation; any that cannot be
deleted are reported and are safe to remove. A process killed during step 3
can leave a mixed store; the staged objects hold the complete new set.

Both password files must be protected regular files with mode 0600 or more
restrictive. Distribute the new secret before dependent jobs pull again.

Examples:
  asc signing sync rotate-password --repo git@github.com:team/certs.git \
    --password-file ~/.config/asc/signing-sync-password \
    --new-password-file ~/.config/asc/signing-sync-password-next --confirm

  asc signing sync rotate-password --storage object --object-bucket team-certs --object-prefix asc/ \
    --password-file ~/.config/asc/signing-sync-password \
    --new-password-file ~/.config/asc/signing-sync-password-next --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			selectedStorage := strings.ToLower(strings.TrimSpace(*storage))
			provided := providedSigningSyncFlags(fs)
			repo := strings.TrimSpace(*repoURL)
			selectedBranch := strings.TrimSpace(*branch)
			var objectOptions signingpkg.ObjectStorageOptions
			switch selectedStorage {
			case signingSyncStorageGit:
				if err := rejectSigningSyncObjectFlags(provided); err != nil {
					return err
				}
				if repo == "" {
					return shared.UsageError("--repo is required")
				}
			case signingSyncStorageObject:
				if err := rejectGitOnlySigningSyncFlags(provided, selectedStorage); err != nil {
					return err
				}
				var err error
				objectOptions, err = parseSigningSyncObjectStorage(objectFlags)
				if err != nil {
					return err
				}
			default:
				return shared.UsageErrorf("signing sync rotate-password supports only --storage %s or --storage %s", signingSyncStorageGit, signingSyncStorageObject)
			}
			currentPath := strings.TrimSpace(*passwordFile)
			if currentPath == "" {
				return shared.UsageError("--password-file is required")
			}
			newPath := strings.TrimSpace(*newPasswordFile)
			if newPath == "" {
				return shared.UsageError("--new-password-file is required")
			}
			if selectedStorage == signingSyncStorageGit && selectedBranch == "" {
				return shared.UsageError("--branch must not be empty")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required to rotate the signing sync password")
			}

			currentPassword, nextPassword, err := readSigningSyncRotationPasswords(currentPath, newPath)
			if err != nil {
				return err
			}

			var target signingSyncRotationTarget
			if selectedStorage == signingSyncStorageObject {
				backend, err := newSigningSyncObjectStore(ctx, objectOptions)
				if err != nil {
					return fmt.Errorf("signing sync rotate-password: %w", err)
				}
				target = &signingSyncObjectRotationTarget{backend: backend}
			} else {
				target = signingSyncGitRotationTarget{repoURL: repo, branch: selectedBranch}
			}

			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-rotate-*")
			if err != nil {
				return fmt.Errorf("signing sync rotate-password: create temp dir: %w", err)
			}
			store := &signingpkg.GitStore{LocalDir: tmpDir}
			if selectedStorage == signingSyncStorageGit {
				store.RepoURL = repo
				store.Branch = selectedBranch
			}
			defer func() { _ = store.Cleanup() }()

			if err := target.Fetch(ctx, store); err != nil {
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
					RepoURL:   target.Locator(),
					Storage:   target.Storage(),
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
			if err := target.Publish(ctx, store); err != nil {
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
				RepoURL:         target.Locator(),
				Storage:         target.Storage(),
				Files:           encryptedFiles,
				IdentityPresent: identityPresent,
				SensitiveFiles:  sensitiveFiles,
			}, *output.Output, *output.Pretty)
		},
	}
}

// signingSyncRotationTarget fetches the complete encrypted store for a
// rotation and publishes the rewritten artifacts.
type signingSyncRotationTarget interface {
	Fetch(ctx context.Context, store *signingpkg.GitStore) error
	Publish(ctx context.Context, store *signingpkg.GitStore) error
	Locator() string
	Storage() *asc.SigningSyncStorage
}

type signingSyncGitRotationTarget struct {
	repoURL string
	branch  string
}

func (t signingSyncGitRotationTarget) Fetch(ctx context.Context, store *signingpkg.GitStore) error {
	fmt.Fprintln(os.Stderr, "Cloning signing repo...")
	return store.Clone(ctx, false)
}

func (t signingSyncGitRotationTarget) Publish(ctx context.Context, store *signingpkg.GitStore) error {
	return store.CommitAndPush(ctx, "Rotate signing sync password")
}

func (t signingSyncGitRotationTarget) Locator() string { return sanitizeRepoURLForOutput(t.repoURL) }

func (t signingSyncGitRotationTarget) Storage() *asc.SigningSyncStorage {
	return gitSigningSyncStorage(t.repoURL, t.branch)
}

// signingSyncObjectRotationTarget keeps the fetched ciphertext so a failed
// swap can restore the artifacts that were already replaced.
type signingSyncObjectRotationTarget struct {
	backend  *signingpkg.ObjectStorageStore
	previous map[string][]byte
}

func (t *signingSyncObjectRotationTarget) Fetch(ctx context.Context, store *signingpkg.GitStore) error {
	fmt.Fprintln(os.Stderr, "Fetching encrypted signing artifacts from object storage...")
	if err := t.backend.Fetch(ctx, store); err != nil {
		return err
	}
	files, err := store.ListEncryptedFiles()
	if err != nil {
		return err
	}
	t.previous = make(map[string][]byte, len(files))
	for _, relPath := range files {
		ciphertext, err := store.ReadEncryptedArtifact(relPath)
		if err != nil {
			return fmt.Errorf("read fetched %s: %w", relPath, err)
		}
		t.previous[relPath] = ciphertext
	}
	return nil
}

func (t *signingSyncObjectRotationTarget) Publish(ctx context.Context, store *signingpkg.GitStore) error {
	report, err := t.backend.PublishRotation(ctx, store, t.previous)
	if err != nil {
		return err
	}
	for _, key := range report.LeftoverStagedKeys {
		fmt.Fprintf(os.Stderr, "Warning: could not delete staged rotation object %s; it is ignored by pulls and safe to delete\n", key)
	}
	return nil
}

func (t *signingSyncObjectRotationTarget) Locator() string { return t.backend.Locator() }

func (t *signingSyncObjectRotationTarget) Storage() *asc.SigningSyncStorage {
	return &asc.SigningSyncStorage{Kind: signingSyncStorageObject, Location: t.backend.Locator()}
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
