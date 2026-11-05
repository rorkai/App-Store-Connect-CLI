package distribute

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

type publicationStore struct {
	delegate core.ObjectStore
}

func (store publicationStore) Ensure(ctx context.Context, object core.PutObject) (core.StoredObject, error) {
	requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
	defer cancel()
	return store.delegate.Ensure(requestCtx, object)
}

func (store publicationStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
	defer cancel()
	return store.delegate.PresignGet(requestCtx, key, ttl)
}

// publishExecutionResult is safe to return from an in-process workflow. Exact
// bearer links remain exclusively in the protected link artifact.
type publishExecutionResult struct {
	Receipt   core.PublishReceipt `json:"receipt"`
	Recovered bool                `json:"recovered"`
}

type privatePublishVerificationRequest struct {
	BundleDir     string
	ReceiptPath   string
	LinkPath      string
	VerifyTimeout time.Duration
}

// reverifyPrivatePublish verifies an existing private publication without
// creating directories, repairing receipts, refreshing links, or contacting
// the object store through any mutating operation.
func reverifyPrivatePublish(ctx context.Context, request privatePublishVerificationRequest) (publishExecutionResult, error) {
	if strings.TrimSpace(request.BundleDir) == "" || strings.TrimSpace(request.ReceiptPath) == "" || strings.TrimSpace(request.LinkPath) == "" {
		return publishExecutionResult{}, shared.UsageError("bundle directory, receipt, and link paths are required")
	}
	if request.VerifyTimeout <= 0 {
		return publishExecutionResult{}, shared.UsageError("verify timeout must be positive")
	}
	bundleDir := strings.TrimSpace(request.BundleDir)
	receiptPath := strings.TrimSpace(request.ReceiptPath)
	linkPath := strings.TrimSpace(request.LinkPath)
	bundleRoot, err := rootfs.New(bundleDir)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("open prepared bundle: %w", err)
	}
	defer bundleRoot.Close()
	if err := rejectBundleContainedArtifacts(bundleRoot, receiptPath, linkPath); err != nil {
		return publishExecutionResult{}, shared.UsageErrorf("publish artifacts: %v", err)
	}
	artifacts, err := openExistingArtifactPaths(receiptPath, linkPath)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("open publication artifacts: %w", err)
	}
	defer artifacts.close()
	state, found, err := artifacts.loadState()
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("load sensitive publication state: %w", err)
	}
	if !found {
		return publishExecutionResult{}, fmt.Errorf("sensitive publication state does not exist")
	}
	bundle, err := loadPreparedBundle(ctx, bundleRoot)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("load prepared bundle: %w", err)
	}
	defer bundle.IPA.Close()
	if err := validateStoredPrivateState(state, bundle, artifacts.receiptPath, artifacts.linkPath); err != nil {
		return publishExecutionResult{}, fmt.Errorf("validate private publication state: %w", err)
	}
	if err := artifacts.verifyExactReceipt(state.Receipt); err != nil {
		return publishExecutionResult{}, fmt.Errorf("verify publication receipt: %w", err)
	}
	verifier, err := newPublicationVerifier(request.VerifyTimeout)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("configure publication verifier: %w", err)
	}
	if err := reverifyPublication(ctx, verifier, state.Receipt, state.Links, time.Now().UTC()); err != nil {
		return publishExecutionResult{}, fmt.Errorf("reverify publication: %w", err)
	}
	return publishExecutionResult{Receipt: state.Receipt, Recovered: true}, nil
}

func openExistingArtifactPaths(receiptPath, linkPath string) (artifactPaths, error) {
	paths, err := anchorArtifactPaths(strings.TrimSpace(receiptPath), strings.TrimSpace(linkPath))
	if err != nil {
		return artifactPaths{}, err
	}

	var infos [2]os.FileInfo
	for index, item := range []struct {
		path  artifactPath
		label string
	}{{paths.receipt, "receipt"}, {paths.link, "sensitive link artifact"}} {
		info, found, err := inspectExistingProtectedPublishArtifact(item.path.root, item.path.relative, item.label)
		if err != nil {
			paths.close()
			return artifactPaths{}, err
		}
		if !found {
			paths.close()
			return artifactPaths{}, fmt.Errorf("%s does not exist", filepath.Join(item.path.rootPath, item.path.relative))
		}
		infos[index] = info
	}
	if os.SameFile(infos[0], infos[1]) {
		paths.close()
		return artifactPaths{}, fmt.Errorf("receipt and link paths resolve to the same physical destination")
	}
	paths.receiptExists = true
	paths.linkExists = true
	return paths, nil
}

func inspectExistingProtectedPublishArtifact(root *os.Root, name, label string) (os.FileInfo, bool, error) {
	file, err := secureopen.OpenExistingNoFollowInRoot(root, name)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	if err := validateProtectedPublishArtifact(file, info, label); err != nil {
		return nil, false, err
	}
	return info, true, nil
}

func validateStoredPrivateState(state publishState, bundle *core.PreparedBundle, receiptPath, linkPath string) error {
	receipt := state.Receipt
	if receipt.Access != core.AccessPrivate || strings.TrimSpace(receipt.PublicBaseURL) != "" {
		return fmt.Errorf("publication is not private")
	}
	endpoint, err := core.ValidateEndpoint(receipt.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid saved endpoint")
	}
	downloadEndpoint, err := core.ValidateEndpoint(receipt.DownloadEndpoint)
	if err != nil {
		return fmt.Errorf("invalid saved download endpoint")
	}
	if receipt.AddressingStyle != "path" && receipt.AddressingStyle != "virtual" {
		return fmt.Errorf("invalid saved addressing style")
	}
	if !regionPattern.MatchString(receipt.Region) || !validPublishBucket(receipt.Bucket) {
		return fmt.Errorf("invalid saved object-store destination")
	}
	if _, err := core.NormalizePrefix(receipt.Prefix); err != nil {
		return fmt.Errorf("invalid saved object key prefix")
	}
	urlTTL, err := time.ParseDuration(receipt.URLTTL)
	if err != nil || urlTTL <= 0 {
		return fmt.Errorf("invalid saved URL lifetime")
	}
	downloadGrace, err := time.ParseDuration(receipt.DownloadGrace)
	if err != nil || !validPrivatePublishLifetime(urlTTL, downloadGrace) {
		return fmt.Errorf("invalid saved download grace")
	}
	return validateRecoveredState(
		state, bundle, endpoint.String(), downloadEndpoint.String(), "", receipt.Region, receipt.AddressingStyle,
		receipt.Bucket, receipt.Prefix, core.AccessPrivate, urlTTL, downloadGrace, receiptPath, linkPath,
	)
}

func validPublishBucket(bucket string) bool {
	return core.ValidateBucket(bucket) == nil
}

func validPrivatePublishLifetime(urlTTL, downloadGrace time.Duration) bool {
	const maximumPrivateLifetime = 7 * 24 * time.Hour
	return urlTTL > 0 &&
		downloadGrace >= 0 &&
		urlTTL <= maximumPrivateLifetime &&
		downloadGrace <= maximumPrivateLifetime-urlTTL
}
