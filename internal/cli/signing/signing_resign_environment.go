package signing

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/bitrise-io/go-pkcs12"
)

var signingResignPlatformDepsFn = platformSigningRunDeps

// runSigningResignEnvironment performs the narrow keychain portion of an IPA
// re-signing run. It intentionally does not install provisioning profiles in
// any user-controlled Xcode directory: profiles are embedded in the staged
// bundle by the caller instead.
func runSigningResignEnvironment(ctx context.Context, identity *signingRunIdentity, operation func(context.Context, string) error) (resultErr error) {
	defer func() {
		if resultErr != nil && !signingResignOperationalErrorTree(resultErr) {
			resultErr = wrapSigningResignOperationalError(
				signingResignStageEnvironment,
				signingResignCodeEnvironment,
				resultErr,
			)
		}
	}()
	if identity == nil || identity.Certificate == nil || identity.PrivateKey == nil {
		return fmt.Errorf("signing identity is missing")
	}
	if operation == nil {
		return fmt.Errorf("signing resign operation is required")
	}
	deps := signingResignPlatformDepsFn()
	if deps.GOOS != "darwin" {
		return fmt.Errorf("signing resign is supported only on macOS")
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	if deps.RandomBytes == nil || deps.TempDir == nil || deps.RemoveTempDir == nil ||
		deps.AcquireLock == nil || deps.Recover == nil || deps.WriteJournal == nil ||
		deps.RemoveJournal == nil || deps.KeychainSearchList == nil ||
		deps.CreateKeychain == nil || deps.ImportIdentity == nil ||
		deps.SetKeychainSearchList == nil || deps.RemoveKeychainSearchEntry == nil ||
		deps.DeleteKeychain == nil {
		return fmt.Errorf("signing environment is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	unlock, err := deps.AcquireLock(ctx)
	if err != nil {
		return fmt.Errorf("acquire signing environment lock failed")
	}
	if unlock == nil {
		return fmt.Errorf("signing environment lock returned no release function")
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			cleanupErr := wrapSigningResignOperationalError(
				signingResignStageCleanup,
				signingResignCodeCleanup,
				fmt.Errorf("%w: release signing environment lock failed: %w", ErrSigningResignCleanupFailed, unlockErr),
			)
			resultErr = errors.Join(
				resultErr,
				cleanupErr,
			)
		}
	}()
	if err := deps.Recover(ctx); err != nil {
		return fmt.Errorf("recover prior signing environment failed")
	}

	tempDir, err := deps.TempDir()
	if err != nil {
		return fmt.Errorf("create private signing directory: %w", err)
	}
	keychainPath := filepath.Join(tempDir, "signing.keychain-db")
	cleanupTempOnly := func() error {
		if err := deps.RemoveTempDir(tempDir); err != nil {
			return wrapSigningResignOperationalError(
				signingResignStageCleanup,
				signingResignCodeCleanup,
				fmt.Errorf("%w: remove private signing directory: %w", ErrSigningResignCleanupFailed, err),
			)
		}
		return nil
	}
	finishEarly := func(primary error) error {
		primary = wrapSigningResignOperationalError(
			signingResignStageEnvironment,
			signingResignCodeEnvironment,
			primary,
		)
		cleanupErr := cleanupTempOnly()
		if cleanupErr == nil {
			return primary
		}
		return errors.Join(primary, cleanupErr)
	}
	finishJournalFailure := func(primary error) error {
		primary = wrapSigningResignOperationalError(
			signingResignStageEnvironment,
			signingResignCodeEnvironment,
			primary,
		)
		// WriteJournal may have created a partial record before returning an
		// error. Attempt both cleanup operations and retain every cause so the
		// recovery decision is never silently lost.
		var cleanupErr error
		if err := deps.RemoveJournal(); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				wrapSigningResignOperationalError(
					signingResignStageCleanup,
					signingResignCodeCleanup,
					fmt.Errorf("%w: remove signing environment recovery journal failed: %w", ErrSigningResignCleanupFailed, err),
				),
			)
		}
		if err := cleanupTempOnly(); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		if cleanupErr == nil {
			return primary
		}
		return errors.Join(primary, cleanupErr)
	}
	if _, err := deps.KeychainSearchList(ctx); err != nil {
		return finishEarly(fmt.Errorf("read user keychain search list failed: %w", err))
	}
	keychainPassword, err := deps.RandomBytes(32)
	if err != nil {
		return finishEarly(fmt.Errorf("generate keychain password: %w", err))
	}
	defer clear(keychainPassword)
	importPassword, err := deps.RandomBytes(32)
	if err != nil {
		return finishEarly(fmt.Errorf("generate identity import password: %w", err))
	}
	importPasswordText := []byte(fmt.Sprintf("%x", importPassword))
	clear(importPassword)
	defer clear(importPasswordText)
	normalizedIdentity, err := pkcs12.Encode(rand.Reader, identity.PrivateKey, identity.Certificate, nil, string(importPasswordText))
	if err != nil {
		return finishEarly(fmt.Errorf("normalize identity for temporary import: %w", err))
	}
	defer clear(normalizedIdentity)

	journal := signingRunJournal{SchemaVersion: 1, TempDir: tempDir, KeychainPath: keychainPath}
	if err := deps.WriteJournal(journal, false); err != nil {
		return finishJournalFailure(fmt.Errorf("write signing environment recovery journal failed: %w", err))
	}
	keychainAttempted := false
	cleanupDone := false
	cleanup := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if !keychainAttempted {
			if err := deps.RemoveTempDir(tempDir); err != nil {
				return fmt.Errorf("%w: remove private signing directory failed: %w", ErrSigningResignCleanupFailed, err)
			}
			if err := deps.RemoveJournal(); err != nil {
				return fmt.Errorf("%w: remove signing environment recovery journal failed: %w", ErrSigningResignCleanupFailed, err)
			}
			return nil
		}
		var cleanupErr error
		cleanupErr = errors.Join(cleanupErr, deps.RemoveKeychainSearchEntry(cleanupCtx, keychainPath))
		cleanupErr = errors.Join(cleanupErr, deps.DeleteKeychain(cleanupCtx, keychainPath))
		if cleanupErr != nil {
			return fmt.Errorf("%w: signing environment cleanup did not complete; recovery journal retained: %w", ErrSigningResignCleanupFailed, cleanupErr)
		}
		if err := deps.RemoveTempDir(tempDir); err != nil {
			return fmt.Errorf("%w: remove private signing directory failed: %w", ErrSigningResignCleanupFailed, err)
		}
		if err := deps.RemoveJournal(); err != nil {
			return fmt.Errorf("%w: remove signing environment recovery journal failed: %w", ErrSigningResignCleanupFailed, err)
		}
		return nil
	}
	defer func() {
		if cleanupDone {
			return
		}
		if cleanupErr := cleanup(); cleanupErr != nil {
			resultErr = errors.Join(
				resultErr,
				wrapSigningResignOperationalError(
					signingResignStageCleanup,
					signingResignCodeCleanup,
					cleanupErr,
				),
			)
		}
	}()
	finish := func(primary error) error {
		cleanupDone = true
		if primary != nil && !signingResignOperationalErrorTree(primary) {
			primary = wrapSigningResignOperationalError(
				signingResignStageEnvironment,
				signingResignCodeEnvironment,
				primary,
			)
		}
		cleanupErr := cleanup()
		if cleanupErr != nil {
			cleanupErr = wrapSigningResignOperationalError(
				signingResignStageCleanup,
				signingResignCodeCleanup,
				cleanupErr,
			)
		}
		if primary == nil {
			return cleanupErr
		}
		if cleanupErr == nil {
			return primary
		}
		return errors.Join(primary, cleanupErr)
	}

	keychainAttempted = true
	if err := deps.CreateKeychain(ctx, keychainPath, keychainPassword); err != nil {
		return finish(fmt.Errorf("create temporary keychain failed"))
	}
	if err := deps.RemoveKeychainSearchEntry(ctx, keychainPath); err != nil {
		return finish(fmt.Errorf("isolate temporary keychain failed"))
	}
	if err := deps.ImportIdentity(ctx, keychainPath, keychainPassword, normalizedIdentity, importPasswordText, identity.CertificateSHA1); err != nil {
		return finish(fmt.Errorf("import identity into temporary keychain failed"))
	}
	// Keychain access happens in two phases. Signing invocations always pass
	// the explicit `--keychain` argument, which selects where codesign looks
	// up the imported identity. Search-list activation is still required
	// because Security.framework resolves the signer's certificate chain
	// through the user's keychain search list, not through that argument.
	// The temporary keychain is visible on the search list only while
	// `operation` runs: cleanup removes the entry and deletes the keychain on
	// every path, and the recovery journal covers interrupted runs.
	currentSearchList, err := deps.KeychainSearchList(ctx)
	if err != nil {
		return finish(fmt.Errorf("refresh user keychain search list failed"))
	}
	expectedSearchList := []string{keychainPath}
	for _, existing := range currentSearchList {
		if existing != keychainPath {
			expectedSearchList = append(expectedSearchList, existing)
		}
	}
	if err := deps.SetKeychainSearchList(ctx, expectedSearchList); err != nil {
		return finish(fmt.Errorf("activate temporary keychain failed"))
	}
	if err := operation(ctx, keychainPath); err != nil {
		return finish(err)
	}
	return finish(nil)
}
