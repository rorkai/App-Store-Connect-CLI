package signing

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// EncryptedArtifactSuffix is the on-disk suffix of an encrypted signing
// artifact. Remote stores keep it so a stored object is never mistaken for
// plaintext signing material.
const EncryptedArtifactSuffix = ".enc"

// MaxEncryptedArtifactBytes is the per-artifact ciphertext limit shared by the
// encrypted repository and every remote store.
const MaxEncryptedArtifactBytes = maxEncryptedFileSize

// RequestContextFunc bounds one outbound request to a remote store. Commands
// pass the shared CLI timeout helpers so remote stores obey the same request
// budget as App Store Connect calls.
type RequestContextFunc func(context.Context) (context.Context, context.CancelFunc)

// defaultRemoteRequestTimeout bounds a request when no helper is supplied.
const defaultRemoteRequestTimeout = 2 * time.Minute

func boundedRequestContext(request RequestContextFunc, ctx context.Context) (context.Context, context.CancelFunc) {
	if request != nil {
		return request(ctx)
	}
	return context.WithTimeout(ctx, defaultRemoteRequestTimeout)
}

// ArtifactStore is the ciphertext-only view of a local encrypted signing tree.
// Remote stores transport artifacts through this interface and never see the
// sync password or any decrypted byte.
type ArtifactStore interface {
	ListEncryptedFiles() ([]string, error)
	ReadEncryptedArtifact(relPath string) ([]byte, error)
	WriteEncryptedArtifact(relPath string, ciphertext []byte) error
}

// ReadEncryptedArtifact returns the raw ciphertext of an encrypted artifact
// without decrypting it.
func (g *GitStore) ReadEncryptedArtifact(relPath string) ([]byte, error) {
	if err := ValidateEncryptedRepositoryPaths([]string{relPath}); err != nil {
		return nil, err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return nil, err
	}
	return root.ReadFileLimited(relPath+EncryptedArtifactSuffix, MaxEncryptedArtifactBytes)
}

// WriteEncryptedArtifact stores raw ciphertext fetched from a remote store.
// The bytes are written unchanged so the existing envelope, metadata, and
// password checks decide whether they are usable.
func (g *GitStore) WriteEncryptedArtifact(relPath string, ciphertext []byte) error {
	if err := ValidateEncryptedRepositoryPaths([]string{relPath}); err != nil {
		return err
	}
	if len(ciphertext) == 0 {
		return fmt.Errorf("encrypted artifact %s is empty", relPath)
	}
	if len(ciphertext) > MaxEncryptedArtifactBytes {
		return fmt.Errorf("encrypted artifact %s exceeds the %d-byte size limit", relPath, MaxEncryptedArtifactBytes)
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.WriteFile(relPath+EncryptedArtifactSuffix, ciphertext, 0o600)
}

var artifactPrefixComponentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,39}$`)

const (
	maxArtifactPrefixLength     = 64
	maxArtifactPrefixComponents = 3
)

// ValidateArtifactPrefix checks the short path prefix that scopes signing
// artifacts inside a shared remote store.
func ValidateArtifactPrefix(prefix string) error {
	if strings.TrimSpace(prefix) != prefix {
		return errors.New("prefix must not have leading or trailing whitespace")
	}
	if prefix == "" {
		return errors.New("prefix must not be empty")
	}
	if len(prefix) > maxArtifactPrefixLength {
		return fmt.Errorf("prefix must be %d characters or fewer", maxArtifactPrefixLength)
	}
	components := strings.Split(prefix, "/")
	if len(components) > maxArtifactPrefixComponents {
		return fmt.Errorf("prefix must have %d or fewer path components", maxArtifactPrefixComponents)
	}
	for _, component := range components {
		if !artifactPrefixComponentPattern.MatchString(component) {
			return fmt.Errorf("prefix component %q must start with a letter or digit and use only letters, digits, dot, underscore, or hyphen", component)
		}
	}
	return nil
}

// validateRemoteArtifactRelativePath applies the encrypted repository path
// rules and additionally rejects the traversal components that a rooted
// filesystem would otherwise have to catch during the write.
func validateRemoteArtifactRelativePath(relPath string) error {
	if err := ValidateEncryptedRepositoryPaths([]string{relPath}); err != nil {
		return err
	}
	if strings.HasPrefix(relPath, "/") {
		return fmt.Errorf("encrypted artifact path %q must be relative", relPath)
	}
	for _, component := range strings.Split(relPath, "/") {
		if component == "." || component == ".." {
			return fmt.Errorf("encrypted artifact path %q contains a traversal component", relPath)
		}
	}
	return nil
}

// remoteArtifactName maps a validated encrypted relative path to its stored
// object name inside a remote store.
func remoteArtifactName(prefix, relPath string) (string, error) {
	if err := validateRemoteArtifactRelativePath(relPath); err != nil {
		return "", err
	}
	return prefix + "/" + relPath + EncryptedArtifactSuffix, nil
}

// remoteArtifactRelativePath reverses remoteArtifactName for objects that
// belong to the prefix, reporting whether the name is in scope.
func remoteArtifactRelativePath(prefix, name string) (string, bool) {
	scope := prefix + "/"
	if !strings.HasPrefix(name, scope) || !strings.HasSuffix(name, EncryptedArtifactSuffix) {
		return "", false
	}
	relPath := strings.TrimSuffix(strings.TrimPrefix(name, scope), EncryptedArtifactSuffix)
	if relPath == "" {
		return "", false
	}
	return relPath, true
}
