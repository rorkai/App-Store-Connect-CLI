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
func parseSigningPullSourceFormat(provided map[string]bool, format, storage, repo, branch string) (bool, string, error) {
	if !provided["source-format"] {
		return false, branch, nil
	}
	if strings.ToLower(strings.TrimSpace(format)) != signingSyncSourceFormatOpenSSL {
		return false, "", shared.UsageError("--source-format must be openssl")
	}
	if strings.ToLower(strings.TrimSpace(storage)) != signingSyncStorageGit {
		return false, "", shared.UsageErrorf("--source-format openssl requires --storage %s", signingSyncStorageGit)
	}
	if !provided["branch"] {
		return true, "master", nil
	}
	if info, err := os.Stat(repo); err == nil && info.IsDir() {
		return false, "", shared.UsageError("--branch is only supported with a Git URL source")
	}
	if strings.TrimSpace(branch) == "" {
		return false, "", shared.UsageError("--branch must not be empty")
	}
	return true, strings.TrimSpace(branch), nil
}

// pullOpenSSLSigningSource reads an OpenSSL-encrypted signing repository laid
// out as certs/<type>/ and profiles/<type>/ without changing it, and writes
// the same files a pull of a signing sync store would.
func pullOpenSSLSigningSource(ctx context.Context, repo, branch, password, outDir string, bundleIDs []string, profileType string, batch bool) (SyncResult, error) {
	root, label, cleanup, err := openOpenSSLImportSource(ctx, repo, branch, (*signingpkg.GitStore).Clone)
	if err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
	}
	defer cleanup()
	decrypted, skipped, err := loadOpenSSLSigningPullFiles(root, password, time.Now())
	if err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
	}
	result := SyncResult{Operation: "pull", RepoURL: label, SourceFormat: signingSyncSourceFormatOpenSSL, Skipped: skipped}
	return writeSigningPullFiles(result, decrypted, outDir, bundleIDs, profileType, batch)
}

// loadOpenSSLSigningPullFiles decrypts the source into memory, re-encodes each
// private key as a PKCS#12 identity protected by password, and validates the
// result as a signing sync store. Identities that no valid profile links are
// reported and left out.
func loadOpenSSLSigningPullFiles(root rootfs.Root, password string, now time.Time) ([]decryptedSigningFile, []asc.SigningSyncSkippedEntry, error) {
	certificates, profiles, skipped, err := loadOpenSSLImportSource(root, password)
	if err != nil {
		return nil, nil, err
	}
	writes, err := planOpenSSLImportWrites(certificates, profiles, nil, now)
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
	activeIdentityPaths, err := validateDecryptedSigningFiles(files, password)
	if err != nil {
		return nil, nil, err
	}
	kept := files[:0]
	for _, file := range files {
		if _, active := activeIdentityPaths[file.RelativePath]; file.Identity && !active {
			source := keySources[file.RelativePath]
			fmt.Fprintf(os.Stderr, "Warning: skipping %s; no currently valid profile links this private key\n", source)
			skipped = append(skipped, asc.SigningSyncSkippedEntry{Path: source, Reason: "no currently valid profile links this private key; only the certificate is written"})
			continue
		}
		kept = append(kept, file)
	}
	return kept, skipped, nil
}
