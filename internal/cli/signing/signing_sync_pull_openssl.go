package signing

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

const signingSyncSourceFormatOpenSSL = "openssl"

// parseSigningPullSourceFormat validates --source-format before any secret or
// repository access and returns the branch to read the source from.
func parseSigningPullSourceFormat(d *shared.Deps, provided map[string]bool, format, storage, repo, branch string) (bool, string, error) {
	if !provided["source-format"] {
		return false, branch, nil
	}
	if strings.ToLower(strings.TrimSpace(format)) != signingSyncSourceFormatOpenSSL {
		return false, "", d.UsageError("--source-format must be openssl")
	}
	if strings.ToLower(strings.TrimSpace(storage)) != signingSyncStorageGit {
		return false, "", d.UsageErrorf("--source-format openssl requires --storage %s", signingSyncStorageGit)
	}
	if !provided["branch"] {
		return true, "master", nil
	}
	if info, err := os.Stat(repo); err == nil && info.IsDir() {
		return false, "", d.UsageError("--branch is only supported with a Git URL source")
	}
	if strings.TrimSpace(branch) == "" {
		return false, "", d.UsageError("--branch must not be empty")
	}
	return true, strings.TrimSpace(branch), nil
}

// pullOpenSSLSigningSource reads an OpenSSL-encrypted signing repository laid
// out as certs/<type>/ and profiles/<type>/ without changing it, and writes
// the same files a pull of a signing sync store would.
func pullOpenSSLSigningSource(ctx context.Context, d *shared.Deps, repo, branch, password, outDir string, bundleIDs []string, profileType string, batch bool) (SyncResult, error) {
	root, label, cleanup, err := openOpenSSLImportSource(ctx, d, repo, branch, (*signingpkg.GitStore).Clone)
	if err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
	}
	defer cleanup()
	decrypted, skipped, err := loadOpenSSLSigningPullFiles(d, root, password, d.Now())
	if err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
	}
	result := SyncResult{Operation: "pull", RepoURL: label, SourceFormat: signingSyncSourceFormatOpenSSL, Skipped: skipped}
	return writeSigningPullFiles(d, result, decrypted, password, outDir, bundleIDs, profileType, batch)
}

// loadOpenSSLSigningPullFiles decrypts the source into memory, re-encodes each
// private key as a PKCS#12 identity protected by password, and validates the
// result as a signing sync store. Identities that no valid profile links are
// reported and left out. Installer identities require a currently valid certificate.
func loadOpenSSLSigningPullFiles(d *shared.Deps, root rootfs.Root, password string, now time.Time) ([]decryptedSigningFile, []asc.SigningSyncSkippedEntry, error) {
	certificates, profiles, skipped, err := loadOpenSSLImportSource(d, root, password, true)
	if err != nil {
		return nil, nil, err
	}
	writes, err := planOpenSSLImportWrites(d, certificates, profiles, nil, now)
	if err != nil {
		return nil, nil, err
	}
	files := make([]decryptedSigningFile, 0, len(writes))
	keySources := make(map[string]string)
	for _, write := range writes {
		file := decryptedSigningFile{RelativePath: write.relPath, Plaintext: write.plaintext}
		if write.metadata != nil {
			file.Metadata = *write.metadata
			file.Metadata.Version = 1
		}
		if write.identity != nil {
			file.Plaintext, err = normalizeSigningIdentity(&signingIdentity{
				PrivateKey:        write.identity.privateKey,
				Certificate:       write.identity.cert,
				CertificateSHA256: write.identity.sha256,
			}, password)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", write.identity.sourcePath, err)
			}
			keySources[write.relPath] = strings.TrimSuffix(write.identity.sourcePath, path.Ext(write.identity.sourcePath)) + ".p12"
		}
		file.Sensitive, file.Identity, err = classifySigningFile(file.RelativePath, file.Plaintext, file.Metadata, password)
		if err != nil {
			return nil, nil, fmt.Errorf("validate %s: %w", file.RelativePath, err)
		}
		files = append(files, file)
	}
	activeIdentityPaths, err := validateDecryptedSigningFiles(d, files, password)
	if err != nil {
		return nil, nil, err
	}
	kept := files[:0]
	for _, file := range files {
		if _, active := activeIdentityPaths[file.RelativePath]; file.Identity && !active {
			source := keySources[file.RelativePath]
			reason := "no currently valid profile links this private key; only the certificate is written"
			if file.Metadata.Kind == installerIdentityKind {
				reason = "installer certificate is not currently valid"
			}
			fmt.Fprintf(d.IO.ErrOut, "Warning: skipping %s; %s\n", source, reason)
			skipped = append(skipped, asc.SigningSyncSkippedEntry{Path: source, Reason: reason})
			continue
		}
		kept = append(kept, file)
	}
	return kept, skipped, nil
}
