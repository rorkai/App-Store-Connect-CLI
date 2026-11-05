package signing

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var extractSigningResignCertificateFn = extractSigningResignCertificate

func verifySigningResignCertificate(ctx context.Context, codePath, expectedSHA256 string) error {
	return extractSigningResignCertificateFn(ctx, codePath, expectedSHA256)
}

func extractSigningResignCertificate(ctx context.Context, codePath, expectedSHA256 string) (resultErr error) {
	if len(expectedSHA256) != sha256.Size*2 {
		return fmt.Errorf("expected signer certificate digest is invalid")
	}
	if _, err := hex.DecodeString(expectedSHA256); err != nil {
		return fmt.Errorf("expected signer certificate digest is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "asc-signing-resign-cert.")
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageCertificate,
			signingResignCodeCertificate,
			fmt.Errorf("create certificate inspection directory: %w", err),
		)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			resultErr = errors.Join(
				resultErr,
				wrapSigningResignOperationalError(
					signingResignStageCleanup,
					signingResignCodeCleanup,
					fmt.Errorf("remove certificate inspection directory failed: %w", cleanupErr),
				),
			)
		}
	}()
	if err := os.Chmod(directory, 0o700); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageCertificate,
			signingResignCodeCertificate,
			fmt.Errorf("secure certificate inspection directory: %w", err),
		)
	}
	prefix := filepath.Join(directory, "certificate-")
	if _, err := runSigningResignToolFn(ctx, "/usr/bin/codesign", "-d", "--extract-certificates="+prefix, codePath); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageCertificate,
			signingResignCodeCertificate,
			fmt.Errorf("extract signer certificate: %w", err),
		)
	}
	leafName := filepath.Base(prefix) + "0"
	leafInfo, err := os.Lstat(filepath.Join(directory, leafName))
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageCertificate,
			signingResignCodeCertificate,
			fmt.Errorf("read extracted leaf signer certificate: %w", err),
		)
	}
	if leafInfo.Mode()&os.ModeSymlink != 0 || !leafInfo.Mode().IsRegular() {
		return fmt.Errorf("extracted leaf signer certificate is not a regular file")
	}
	data, err := os.ReadFile(filepath.Join(directory, leafName))
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageCertificate,
			signingResignCodeCertificate,
			fmt.Errorf("read extracted leaf signer certificate: %w", err),
		)
	}
	certificate, err := x509.ParseCertificate(data)
	if err != nil {
		clear(data)
		return fmt.Errorf("parse extracted leaf signer certificate: %w", err)
	}
	digest := sha256.Sum256(certificate.Raw)
	clear(data)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), expectedSHA256) {
		return fmt.Errorf("signed code object certificate does not match the supplied identity")
	}
	return nil
}
