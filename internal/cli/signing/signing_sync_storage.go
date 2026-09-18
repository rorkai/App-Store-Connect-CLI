package signing

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

// Encrypted signing artifacts are always produced and consumed locally. The
// storage backend only transports the resulting ciphertext.
const (
	signingSyncStorageGit    = "git"
	signingSyncStorageGitLab = "gitlab-secure-files"
	signingSyncStorageAWS    = "aws-secrets-manager"
)

const signingSyncStorageValues = signingSyncStorageGit + ", " + signingSyncStorageGitLab + ", or " + signingSyncStorageAWS

// signingSyncStorageFlags holds the remote storage selectors.
type signingSyncStorageFlags struct {
	storage         *string
	prefix          *string
	region          *string
	gitlabHost      *string
	gitlabProject   *string
	gitlabTokenFile *string
}

func bindSigningSyncStorageFlags(fs *flag.FlagSet) *signingSyncStorageFlags {
	return &signingSyncStorageFlags{
		storage: fs.String("storage", signingSyncStorageGit,
			"Encrypted artifact storage ("+signingSyncStorageValues+")"),
		prefix: fs.String("prefix", "",
			"Short path prefix scoping artifacts in a remote store (required with --storage "+signingSyncStorageGitLab+" or --storage "+signingSyncStorageAWS+")"),
		region: fs.String("region", "",
			"AWS region (required with --storage "+signingSyncStorageAWS+")"),
		gitlabHost: fs.String("gitlab-host", "",
			"HTTPS GitLab base URL (default https://gitlab.com)"),
		gitlabProject: fs.String("gitlab-project", "",
			"Numeric GitLab project ID (required with --storage "+signingSyncStorageGitLab+")"),
		gitlabTokenFile: fs.String("gitlab-token-file", "",
			"Protected file containing the GitLab API token (required with --storage "+signingSyncStorageGitLab+")"),
	}
}

// signingSyncStorageSelection is the validated storage choice. Parsing it
// performs no file or network access so flag errors stay ordered ahead of
// secret reads and outbound requests.
type signingSyncStorageSelection struct {
	kind            string
	repoURL         string
	branch          string
	prefix          string
	region          string
	gitlabHost      string
	gitlabProject   string
	gitlabTokenFile string
}

func providedSigningSyncFlags(fs *flag.FlagSet) map[string]bool {
	provided := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})
	return provided
}

func parseSigningSyncStorage(fs *flag.FlagSet, flags *signingSyncStorageFlags, repoURL, branch string) (signingSyncStorageSelection, error) {
	kind := strings.ToLower(strings.TrimSpace(*flags.storage))
	if kind == "" {
		kind = signingSyncStorageGit
	}
	provided := providedSigningSyncFlags(fs)
	selection := signingSyncStorageSelection{
		kind:            kind,
		repoURL:         strings.TrimSpace(repoURL),
		branch:          branch,
		prefix:          strings.TrimSpace(*flags.prefix),
		region:          strings.TrimSpace(*flags.region),
		gitlabHost:      strings.TrimSpace(*flags.gitlabHost),
		gitlabProject:   strings.TrimSpace(*flags.gitlabProject),
		gitlabTokenFile: strings.TrimSpace(*flags.gitlabTokenFile),
	}

	switch kind {
	case signingSyncStorageGit:
		for _, name := range []string{"prefix", "region", "gitlab-host", "gitlab-project", "gitlab-token-file"} {
			if provided[name] {
				return signingSyncStorageSelection{}, shared.UsageErrorf("--%s requires --storage %s or --storage %s", name, signingSyncStorageGitLab, signingSyncStorageAWS)
			}
		}
		if selection.repoURL == "" {
			return signingSyncStorageSelection{}, shared.UsageError("--repo is required")
		}
	case signingSyncStorageGitLab:
		if err := rejectGitOnlySigningSyncFlags(provided, kind); err != nil {
			return signingSyncStorageSelection{}, err
		}
		if provided["region"] {
			return signingSyncStorageSelection{}, shared.UsageErrorf("--region requires --storage %s", signingSyncStorageAWS)
		}
		if selection.gitlabProject == "" {
			return signingSyncStorageSelection{}, shared.UsageErrorf("--gitlab-project is required with --storage %s", signingSyncStorageGitLab)
		}
		if selection.prefix == "" {
			return signingSyncStorageSelection{}, shared.UsageErrorf("--prefix is required with --storage %s", signingSyncStorageGitLab)
		}
		if selection.gitlabTokenFile == "" {
			return signingSyncStorageSelection{}, shared.UsageErrorf("--gitlab-token-file is required with --storage %s", signingSyncStorageGitLab)
		}
		if err := signingpkg.ValidateArtifactPrefix(selection.prefix); err != nil {
			return signingSyncStorageSelection{}, shared.UsageError(err.Error())
		}
	case signingSyncStorageAWS:
		if err := rejectGitOnlySigningSyncFlags(provided, kind); err != nil {
			return signingSyncStorageSelection{}, err
		}
		for _, name := range []string{"gitlab-host", "gitlab-project", "gitlab-token-file"} {
			if provided[name] {
				return signingSyncStorageSelection{}, shared.UsageErrorf("--%s requires --storage %s", name, signingSyncStorageGitLab)
			}
		}
		if selection.prefix == "" {
			return signingSyncStorageSelection{}, shared.UsageErrorf("--prefix is required with --storage %s", signingSyncStorageAWS)
		}
		if selection.region == "" {
			return signingSyncStorageSelection{}, shared.UsageErrorf("--region is required with --storage %s", signingSyncStorageAWS)
		}
		if err := signingpkg.ValidateArtifactPrefix(selection.prefix); err != nil {
			return signingSyncStorageSelection{}, shared.UsageError(err.Error())
		}
	default:
		return signingSyncStorageSelection{}, shared.UsageErrorf("unsupported --storage %q; use %s", kind, signingSyncStorageValues)
	}
	return selection, nil
}

func rejectGitOnlySigningSyncFlags(provided map[string]bool, kind string) error {
	for _, name := range []string{"repo", "branch"} {
		if provided[name] {
			return shared.UsageErrorf("--%s is only supported with --storage %s, not --storage %s", name, signingSyncStorageGit, kind)
		}
	}
	return nil
}

// signingSyncTransport moves encrypted artifacts between the local working
// tree and the selected storage backend. It never sees the sync password.
type signingSyncTransport interface {
	Fetch(ctx context.Context, store *signingpkg.GitStore, allowCreate bool) error
	Publish(ctx context.Context, store *signingpkg.GitStore, message string) error
	Locator() string
	Kind() string
}

// transport resolves the selection into a usable backend. It is the first step
// that reads a token file or contacts a remote service.
func (s signingSyncStorageSelection) transport(ctx context.Context) (signingSyncTransport, error) {
	switch s.kind {
	case signingSyncStorageGit:
		return signingSyncGitTransport{repoURL: s.repoURL, branch: s.branch}, nil
	case signingSyncStorageGitLab:
		tokenData, err := readProtectedSecretFile(s.gitlabTokenFile, "GitLab token")
		if err != nil {
			return nil, err
		}
		token := trimPasswordFileNewline(string(tokenData))
		if strings.TrimSpace(token) == "" {
			return nil, shared.UsageError("GitLab token file is empty")
		}
		backend, err := signingpkg.NewGitLabSecureFilesStore(signingpkg.GitLabSecureFilesOptions{
			Host:           s.gitlabHost,
			ProjectID:      s.gitlabProject,
			Prefix:         s.prefix,
			Token:          token,
			MaxArtifacts:   maxEncryptedSigningFiles,
			RequestContext: shared.ContextWithTimeout,
			UploadContext:  shared.ContextWithUploadTimeout,
		})
		if err != nil {
			return nil, shared.UsageError(err.Error())
		}
		return signingSyncRemoteTransport{kind: s.kind, label: "GitLab Secure Files", backend: backend}, nil
	case signingSyncStorageAWS:
		backend, err := signingpkg.NewAWSSecretsManagerStore(ctx, signingpkg.AWSSecretsManagerOptions{
			Region:         s.region,
			Prefix:         s.prefix,
			MaxArtifacts:   maxEncryptedSigningFiles,
			RequestContext: shared.ContextWithTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("aws secrets manager: %w", err)
		}
		return signingSyncRemoteTransport{kind: s.kind, label: "AWS Secrets Manager", backend: backend}, nil
	default:
		return nil, shared.UsageErrorf("unsupported --storage %q; use %s", s.kind, signingSyncStorageValues)
	}
}

type signingSyncGitTransport struct {
	repoURL string
	branch  string
}

func (t signingSyncGitTransport) Fetch(ctx context.Context, store *signingpkg.GitStore, allowCreate bool) error {
	fmt.Fprintln(os.Stderr, "Cloning signing repo...")
	return store.Clone(ctx, allowCreate)
}

func (t signingSyncGitTransport) Publish(ctx context.Context, store *signingpkg.GitStore, message string) error {
	fmt.Fprintln(os.Stderr, "Pushing to git...")
	return store.CommitAndPush(ctx, message)
}

func (t signingSyncGitTransport) Locator() string { return sanitizeRepoURLForOutput(t.repoURL) }

func (t signingSyncGitTransport) Kind() string { return signingSyncStorageGit }

// signingSyncRemoteBackend transports already-encrypted artifacts to and from
// a remote store.
type signingSyncRemoteBackend interface {
	Fetch(ctx context.Context, store signingpkg.ArtifactStore) error
	Publish(ctx context.Context, store signingpkg.ArtifactStore) error
	Locator() string
}

type signingSyncRemoteTransport struct {
	kind    string
	label   string
	backend signingSyncRemoteBackend
}

func (t signingSyncRemoteTransport) Fetch(ctx context.Context, store *signingpkg.GitStore, _ bool) error {
	fmt.Fprintf(os.Stderr, "Fetching encrypted signing artifacts from %s...\n", t.label)
	return t.backend.Fetch(ctx, store)
}

func (t signingSyncRemoteTransport) Publish(ctx context.Context, store *signingpkg.GitStore, _ string) error {
	fmt.Fprintf(os.Stderr, "Publishing encrypted signing artifacts to %s...\n", t.label)
	return t.backend.Publish(ctx, store)
}

func (t signingSyncRemoteTransport) Locator() string { return t.backend.Locator() }

func (t signingSyncRemoteTransport) Kind() string { return t.kind }

// newSigningSyncStore creates the local ciphertext working tree for a
// transport. Only git storage uses the repository URL and branch.
func newSigningSyncStore(transport signingSyncTransport, localDir, repoURL, branch string) *signingpkg.GitStore {
	if transport != nil && transport.Kind() != signingSyncStorageGit {
		return &signingpkg.GitStore{LocalDir: localDir}
	}
	return &signingpkg.GitStore{RepoURL: repoURL, LocalDir: localDir, Branch: branch}
}
