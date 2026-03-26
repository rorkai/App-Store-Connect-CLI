package shared

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

// PublishDefaultPollInterval is the default polling interval for build discovery.
const PublishDefaultPollInterval = 30 * time.Second

type buildUploadFailureDiagnosticsFunc func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error)

var (
	buildUploadFailureDiagnosticsFn buildUploadFailureDiagnosticsFunc = diagnoseBuildUploadFailure
	buildStatusBundleIDSupportedFn                                    = xcode.SupportsBuildStatusBundleID
)

// ContextWithTimeoutDuration creates a context with a specific timeout.
func ContextWithTimeoutDuration(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, timeout)
}

// WaitForBuildByNumberOrUploadFailure waits for a build matching version/build
// number to appear while also watching the originating build upload for early
// failure states. This prevents long hangs when App Store Connect rejects the
// uploaded artifact before a build record is created.
func WaitForBuildByNumberOrUploadFailure(ctx context.Context, client *asc.Client, appID, uploadID, version, buildNumber, platform string, pollInterval time.Duration) (*asc.BuildResponse, error) {
	if pollInterval <= 0 {
		pollInterval = PublishDefaultPollInterval
	}
	buildNumber = strings.TrimSpace(buildNumber)
	if buildNumber == "" {
		return nil, fmt.Errorf("build number is required to resolve build")
	}
	uploadID = strings.TrimSpace(uploadID)

	return asc.PollUntil(ctx, pollInterval, func(ctx context.Context) (*asc.BuildResponse, bool, error) {
		if uploadID != "" {
			upload, err := client.GetBuildUpload(ctx, uploadID)
			if err != nil {
				if !shouldIgnoreBuildWaitLookupError(err) {
					return nil, false, err
				}
			} else {
				if err := buildUploadFailureError(upload); err != nil {
					return nil, false, enrichBuildUploadFailure(ctx, client, appID, upload, err)
				}
				buildID, err := buildIDForUpload(upload)
				if err != nil {
					return nil, false, err
				}
				if buildID != "" {
					// Keep upload-status probing best-effort only for linked-build
					// lookups that legitimately have not materialized yet.
					build, err := client.GetBuild(ctx, buildID)
					if err != nil {
						if !shouldIgnoreBuildWaitLookupError(err) {
							return nil, false, err
						}
					} else {
						return build, true, nil
					}
				}
			}
		}
		build, err := findBuildByNumber(ctx, client, appID, version, buildNumber, platform, uploadID)
		if err != nil {
			return nil, false, err
		}
		if build != nil {
			return build, true, nil
		}
		return nil, false, nil
	})
}

// VerifyBuildUploadAfterCommit briefly watches a newly committed upload for
// immediate App Store Connect failures. It returns nil on timeout so the caller
// can keep the default asynchronous success behavior when no failure is
// observed during the bounded verification window.
func VerifyBuildUploadAfterCommit(ctx context.Context, client *asc.Client, appID, uploadID string, pollInterval, verifyTimeout time.Duration) error {
	if client == nil {
		return nil
	}
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" || verifyTimeout <= 0 {
		return nil
	}

	verifyCtx, cancel := ContextWithTimeoutDuration(ctx, verifyTimeout)
	defer cancel()

	effectiveInterval := pollInterval
	switch {
	case effectiveInterval <= 0:
		effectiveInterval = 5 * time.Second
	case effectiveInterval > 5*time.Second:
		effectiveInterval = 5 * time.Second
	}
	if effectiveInterval > verifyTimeout {
		effectiveInterval = verifyTimeout
	}
	if effectiveInterval <= 0 {
		effectiveInterval = time.Millisecond
	}

	_, err := asc.PollUntil(verifyCtx, effectiveInterval, func(ctx context.Context) (*asc.BuildUploadResponse, bool, error) {
		upload, err := client.GetBuildUpload(ctx, uploadID)
		if err != nil {
			if shouldIgnoreBuildWaitLookupError(err) || shouldIgnorePostCommitBuildUploadLookupError(ctx, err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		if err := buildUploadFailureError(upload); err != nil {
			return nil, false, enrichBuildUploadFailure(ctx, client, appID, upload, err)
		}
		buildID, err := buildIDForUpload(upload)
		if err != nil {
			return nil, false, err
		}
		if buildID != "" {
			return upload, true, nil
		}
		return nil, false, nil
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return nil
	}
	return err
}

func shouldIgnorePostCommitBuildUploadLookupError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if asc.IsRetryable(err) {
		return true
	}

	var apiErr *asc.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 500 {
		return true
	}

	if errors.Is(err, context.DeadlineExceeded) && ctx != nil && ctx.Err() == nil {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || isTemporaryNetError(netErr)) {
		return true
	}

	return false
}

type temporaryNetError interface {
	Temporary() bool
}

func isTemporaryNetError(err net.Error) bool {
	tempErr, ok := err.(temporaryNetError)
	return ok && tempErr.Temporary()
}

func findBuildByNumber(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform, uploadID string) (*asc.BuildResponse, error) {
	preReleaseResp, err := client.GetPreReleaseVersions(ctx, appID,
		asc.WithPreReleaseVersionsVersion(version),
		asc.WithPreReleaseVersionsPlatform(platform),
		asc.WithPreReleaseVersionsLimit(10),
	)
	if err != nil {
		return nil, err
	}
	if len(preReleaseResp.Data) == 0 {
		return nil, nil
	}
	if len(preReleaseResp.Data) > 1 {
		return nil, fmt.Errorf("multiple pre-release versions found for version %q and platform %q", version, platform)
	}

	preReleaseID := preReleaseResp.Data[0].ID
	buildOpts := []asc.BuildsOption{
		asc.WithBuildsPreReleaseVersion(preReleaseID),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(200),
	}
	if uploadID != "" {
		buildOpts = append(buildOpts, asc.WithBuildsInclude([]string{"buildUpload"}))
	}
	buildsResp, err := client.GetBuilds(ctx, appID, buildOpts...)
	if err != nil {
		return nil, err
	}
	for _, build := range buildsResp.Data {
		if strings.TrimSpace(build.Attributes.Version) != buildNumber {
			continue
		}
		if uploadID != "" {
			buildUploadID, err := buildUploadIDForBuild(build)
			if err != nil {
				return nil, err
			}
			if buildUploadID != uploadID {
				continue
			}
		}
		return &asc.BuildResponse{Data: build}, nil
	}
	return nil, nil
}

type buildRelationships struct {
	BuildUpload *asc.Relationship `json:"buildUpload,omitempty"`
}

func buildUploadIDForBuild(build asc.Resource[asc.BuildAttributes]) (string, error) {
	if len(build.Relationships) == 0 {
		return "", nil
	}

	var relationships buildRelationships
	if err := json.Unmarshal(build.Relationships, &relationships); err != nil {
		return "", fmt.Errorf("parse build %q relationships: %w", strings.TrimSpace(build.ID), err)
	}
	if relationships.BuildUpload == nil {
		return "", nil
	}
	return strings.TrimSpace(relationships.BuildUpload.Data.ID), nil
}

type buildUploadRelationships struct {
	Build *asc.Relationship `json:"build,omitempty"`
}

func buildIDForUpload(upload *asc.BuildUploadResponse) (string, error) {
	if upload == nil || len(upload.Data.Relationships) == 0 {
		return "", nil
	}

	var relationships buildUploadRelationships
	if err := json.Unmarshal(upload.Data.Relationships, &relationships); err != nil {
		return "", fmt.Errorf("parse build upload %q relationships: %w", strings.TrimSpace(upload.Data.ID), err)
	}
	if relationships.Build == nil {
		return "", nil
	}
	return strings.TrimSpace(relationships.Build.Data.ID), nil
}

func buildUploadFailureError(upload *asc.BuildUploadResponse) error {
	if upload == nil || upload.Data.Attributes.State == nil || upload.Data.Attributes.State.State == nil {
		return nil
	}

	state := strings.ToUpper(strings.TrimSpace(*upload.Data.Attributes.State.State))
	if state != "FAILED" {
		return nil
	}

	details := buildUploadStateDetails(upload.Data.Attributes.State.Errors)
	if details == "" {
		return fmt.Errorf("build upload %q failed with state %s", upload.Data.ID, state)
	}
	return fmt.Errorf("build upload %q failed with state %s: %s", upload.Data.ID, state, details)
}

func enrichBuildUploadFailure(ctx context.Context, client *asc.Client, appID string, upload *asc.BuildUploadResponse, baseErr error) error {
	if baseErr == nil {
		return nil
	}
	details, err := buildUploadFailureDiagnosticsFn(ctx, client, appID, upload)
	if err != nil {
		return baseErr
	}
	details = strings.TrimSpace(details)
	if details == "" || strings.Contains(baseErr.Error(), details) {
		return baseErr
	}
	return fmt.Errorf("%w; App Store Connect processing details: %s", baseErr, details)
}

func diagnoseBuildUploadFailure(ctx context.Context, client *asc.Client, appID string, upload *asc.BuildUploadResponse) (string, error) {
	if upload == nil {
		return "", nil
	}

	appID = strings.TrimSpace(appID)
	buildNumber := strings.TrimSpace(upload.Data.Attributes.CFBundleVersion)
	if appID == "" || buildNumber == "" {
		return "", nil
	}

	creds, err := ResolveAuthCredentials("")
	if err != nil {
		return "", err
	}
	keyPath, err := buildStatusPrivateKeyPath(creds)
	if err != nil {
		return "", err
	}

	bundleID := resolveBuildStatusBundleID(ctx, client, appID)
	result, err := xcode.BuildStatus(ctx, xcode.BuildStatusOptions{
		AppleID:            appID,
		BundleID:           bundleID,
		BundleVersion:      buildNumber,
		BundleShortVersion: strings.TrimSpace(upload.Data.Attributes.CFBundleShortVersionString),
		Platform:           string(upload.Data.Attributes.Platform),
		APIKey:             strings.TrimSpace(creds.KeyID),
		APIIssuer:          strings.TrimSpace(creds.IssuerID),
		P8FilePath:         keyPath,
	})
	if err != nil {
		return "", err
	}
	return joinDiagnosticDetails(result.ProcessingErrors), nil
}

func resolveBuildStatusBundleID(ctx context.Context, client *asc.Client, appID string) string {
	if client == nil || !buildStatusBundleIDSupportedFn(ctx) {
		return ""
	}

	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}

	app, err := client.GetApp(ctx, appID)
	if err != nil || app == nil {
		return ""
	}
	return strings.TrimSpace(app.Data.Attributes.BundleID)
}

func buildStatusPrivateKeyPath(creds ResolvedAuthCredentials) (string, error) {
	if pem := strings.TrimSpace(creds.KeyPEM); pem != "" {
		if decoded, cacheKey, ok := decodeBuildStatusPrivateKeyPEMBase64(pem); ok {
			if path := cachedTempPrivateKeyPath(cacheKey); path != "" {
				return path, nil
			}
			return writeTempPrivateKey(decoded, cacheKey)
		}
		normalized := normalizePrivateKeyValue(pem)
		cacheKey := tempPrivateKeyCacheKey("raw", normalized)
		if path := cachedTempPrivateKeyPath(cacheKey); path != "" {
			return path, nil
		}
		return writeTempPrivateKey([]byte(normalized), cacheKey)
	}
	if path := strings.TrimSpace(creds.KeyPath); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", nil
}

func decodeBuildStatusPrivateKeyPEMBase64(value string) ([]byte, string, bool) {
	compact := strings.Join(strings.Fields(value), "")
	if compact == "" {
		return nil, "", false
	}
	decoded, err := decodeBase64Secret(value)
	if err != nil {
		return nil, "", false
	}
	normalized := normalizePrivateKeyValue(string(decoded))
	if !looksLikePrivateKeyPEM(normalized) {
		return nil, "", false
	}
	return []byte(normalized), tempPrivateKeyCacheKey("b64", compact), true
}

func looksLikePrivateKeyPEM(value string) bool {
	normalized := normalizePrivateKeyValue(value)
	return strings.Contains(normalized, "BEGIN ") && strings.Contains(normalized, "PRIVATE KEY")
}

func joinDiagnosticDetails(values []string) string {
	return strings.Join(xcode.UniqueDiagnosticDetails(values), "; ")
}

func shouldIgnoreBuildWaitLookupError(err error) bool {
	return asc.IsNotFound(err)
}

// SetBuildUploadFailureDiagnosticsForTesting overrides build failure enrichment.
// Tests only.
func SetBuildUploadFailureDiagnosticsForTesting(fn func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error)) func() {
	previous := buildUploadFailureDiagnosticsFn
	if fn == nil {
		buildUploadFailureDiagnosticsFn = diagnoseBuildUploadFailure
	} else {
		buildUploadFailureDiagnosticsFn = fn
	}
	return func() {
		buildUploadFailureDiagnosticsFn = previous
	}
}

func buildUploadStateDetails(details []asc.StateDetail) string {
	if len(details) == 0 {
		return ""
	}

	parts := make([]string, 0, len(details))
	for _, detail := range details {
		code := strings.TrimSpace(detail.Code)
		message := strings.TrimSpace(detail.Message)
		switch {
		case code != "" && message != "":
			parts = append(parts, fmt.Sprintf("%s (%s)", code, message))
		case code != "":
			parts = append(parts, code)
		case message != "":
			parts = append(parts, message)
		}
	}

	return strings.Join(parts, ", ")
}
