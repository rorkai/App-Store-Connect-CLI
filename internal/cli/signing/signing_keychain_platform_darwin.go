//go:build darwin

package signing

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type persistentSigningUtilityRunner func(context.Context, []byte, ...string) ([]byte, []byte, error)

func platformSigningKeychainInstallDeps() signingKeychainInstallDeps {
	runDeps := platformSigningRunDeps()
	return signingKeychainInstallDeps{
		GOOS:                      "darwin",
		SecurityAvailable:         signingRunSecurityAvailable(),
		AcquireLock:               runDeps.AcquireLock,
		CreateKeychain:            createPersistentSigningKeychain,
		ImportIdentity:            importPersistentSigningIdentity,
		KeychainSearchList:        runDeps.KeychainSearchList,
		SetKeychainSearchList:     runDeps.SetKeychainSearchList,
		RemoveKeychainSearchEntry: runDeps.RemoveKeychainSearchEntry,
		DeleteKeychain:            runDeps.DeleteKeychain,
		AppleIntermediates:        artifacts.AppleIntermediateCertificates,
		ImportCertificate: func(ctx context.Context, keychainPath string, certificate []byte) error {
			return importPersistentSigningCertificate(ctx, keychainPath, certificate, createSigningRunTempDir, removeSigningRunTempDir, runSigningUtility)
		},
	}
}

func importPersistentSigningCertificate(
	ctx context.Context,
	keychainPath string,
	certificate []byte,
	createTempDir func() (string, error),
	removeTempDir func(string) error,
	runUtility persistentSigningUtilityRunner,
) (resultErr error) {
	tempDir, err := createTempDir()
	if err != nil {
		return fmt.Errorf("create private certificate directory: %w", err)
	}
	defer func() {
		if err := removeTempDir(tempDir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove private certificate directory: %w", err))
		}
	}()
	tempRoot, err := rootfs.New(tempDir)
	if err != nil {
		return err
	}
	defer tempRoot.Close()
	const certificateName = "intermediate.cer"
	if err := tempRoot.CreateNewFile(certificateName, certificate, 0o600); err != nil {
		return fmt.Errorf("write certificate: %w", err)
	}
	_, stderr, err := runUtility(ctx, nil, "add-certificates", "-k", keychainPath, filepath.Join(tempDir, certificateName))
	if err != nil {
		return utilityFailure("add certificate", stderr, err)
	}
	return nil
}

func createPersistentSigningKeychain(ctx context.Context, keychainPath string, password []byte) (bool, error) {
	if len(password) == 0 {
		return false, fmt.Errorf("keychain password is empty")
	}
	created, err := createPersistentKeychainWithSecurityFramework(keychainPath, password)
	if err != nil {
		return created, err
	}
	if err := configurePersistentSigningKeychain(ctx, keychainPath, runSigningUtility); err != nil {
		return true, err
	}
	return true, nil
}

func configurePersistentSigningKeychain(
	ctx context.Context,
	keychainPath string,
	runUtility persistentSigningUtilityRunner,
) error {
	_, stderr, err := runUtility(ctx, nil, "set-keychain-settings", "-l", keychainPath)
	if err != nil {
		return utilityFailure("configure persistent keychain", stderr, err)
	}
	return nil
}

func importPersistentSigningIdentity(ctx context.Context, keychainPath string, keychainPassword, identityData, importPassword []byte, expectedSHA1 string) error {
	if err := importPKCS12WithSecurityFramework(keychainPath, identityData, importPassword); err != nil {
		return err
	}
	if err := withPersistentSigningKeychainPasswordInput(keychainPassword, func(stdin []byte) error {
		_, stderr, err := runSigningUtility(ctx, stdin, "set-key-partition-list", "-S", "apple-tool:,apple:", "-s", "-t", "private", keychainPath)
		if err != nil {
			return utilityFailure("restrict key partition list", stderr, err, keychainPassword)
		}
		return nil
	}); err != nil {
		return err
	}
	stdout, stderr, err := runSigningUtility(ctx, nil, "find-certificate", "-a", "-Z", keychainPath)
	if err != nil {
		return utilityFailure("verify imported certificate", stderr, err)
	}
	certificates := parseSigningRunCertificateFingerprints(stdout)
	if err := validatePersistentSigningCertificateFingerprints(certificates, expectedSHA1); err != nil {
		return err
	}
	_, stderr, err = runSigningUtility(ctx, nil, "find-key", "-s", "-t", "private", keychainPath)
	if err != nil {
		return utilityFailure("verify imported private key", stderr, err)
	}
	return verifyPersistentSigningIdentityUsable(ctx, keychainPath, expectedSHA1)
}

func verifyPersistentSigningIdentityUsable(ctx context.Context, keychainPath, expectedSHA1 string) (resultErr error) {
	return withPersistentSigningProbe(
		ctx,
		keychainPath,
		expectedSHA1,
		createSigningRunTempDir,
		removeSigningRunTempDir,
		verifySigningRunIdentityUsable,
	)
}

func withPersistentSigningProbe(
	ctx context.Context,
	keychainPath, expectedSHA1 string,
	createTempDir func() (string, error),
	removeTempDir func(string) error,
	verify func(context.Context, string, string, string) error,
) (resultErr error) {
	probeDir, err := createTempDir()
	if err != nil {
		return fmt.Errorf("create private codesign probe directory: %w", err)
	}
	defer func() {
		if err := removeTempDir(probeDir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove private codesign probe directory: %w", err))
		}
	}()
	return verify(ctx, probeDir, keychainPath, expectedSHA1)
}

func validatePersistentSigningCertificateFingerprints(certificates []string, expectedSHA1 string) error {
	matches := 0
	for _, fingerprint := range certificates {
		if strings.EqualFold(fingerprint, expectedSHA1) {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("verify imported certificate: expected certificate %s exactly once, found %v", expectedSHA1, certificates)
	}
	return nil
}

func withPersistentSigningKeychainPasswordInput(password []byte, operation func([]byte) error) error {
	stdin := make([]byte, len(password)+1)
	copy(stdin, password)
	stdin[len(stdin)-1] = '\n'
	defer clear(stdin)
	return operation(stdin)
}
