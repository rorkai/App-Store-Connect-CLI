package shared

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// PublishDefaultPollInterval is the default polling interval for build discovery.
const PublishDefaultPollInterval = 30 * time.Second

// ContextWithTimeoutDuration creates a context with a specific timeout.
func ContextWithTimeoutDuration(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, timeout)
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
		build, err := findBuildByNumber(ctx, client, appID, version, buildNumber, platform)
		if err != nil {
			return nil, false, err
		}
		if build != nil {
			return build, true, nil
		}
		if uploadID != "" {
			upload, err := client.GetBuildUpload(ctx, uploadID)
			if err != nil {
				return nil, false, err
			}
			if err := buildUploadFailureError(upload); err != nil {
				return nil, false, err
			}
		}
		return nil, false, nil
	})
}

func findBuildByNumber(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string) (*asc.BuildResponse, error) {
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
	buildsResp, err := client.GetBuilds(ctx, appID,
		asc.WithBuildsPreReleaseVersion(preReleaseID),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(200),
	)
	if err != nil {
		return nil, err
	}
	for _, build := range buildsResp.Data {
		if strings.TrimSpace(build.Attributes.Version) == buildNumber {
			return &asc.BuildResponse{Data: build}, nil
		}
	}
	return nil, nil
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
