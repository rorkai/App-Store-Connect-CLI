package versions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// attachBuildEditableStates are the appVersionState values in which App Store
// Connect lets a version's build change.
var attachBuildEditableStates = []string{
	"PREPARE_FOR_SUBMISSION",
	"DEVELOPER_REJECTED",
	"REJECTED",
	"METADATA_REJECTED",
	"INVALID_BINARY",
}

// withAttachBuildConflictHint explains Apple's 409 "The specified pre-release
// build could not be added." on PATCH /v1/appStoreVersions/{id}/relationships/build.
// Apple sends it when the version is in review or live, and when the build
// already ships with another version, without saying which. The hint names the
// version's state or the version that holds the build. Reads are best effort;
// when one fails, or neither case applies, attachErr is returned unchanged.
func withAttachBuildConflictHint(ctx context.Context, client *asc.Client, versionID, buildID string, attachErr error) error {
	apiErr, ok := errors.AsType[*asc.APIError](attachErr)
	if !ok || apiErr.StatusCode != http.StatusConflict {
		return attachErr
	}

	version, err := client.GetAppStoreVersion(ctx, versionID)
	if err != nil {
		return attachErr
	}
	attrs := version.Data.Attributes
	state := strings.TrimSpace(attrs.AppVersionState)
	if state == "" {
		state = strings.TrimSpace(attrs.AppStoreState)
	}
	if state != "" && !slices.Contains(attachBuildEditableStates, state) {
		return &shared.HintedError{Err: attachErr, Hint: fmt.Sprintf(
			"Version %s (%s) is %s, and App Store Connect changes a version's build only while it is %s. To change the build of a version in review, cancel its review submission first (find it with `asc review submissions-list --app APP_ID`, then `asc review submissions-cancel --id SUBMISSION_ID --confirm`).",
			attrs.VersionString, attrs.Platform, state, strings.Join(attachBuildEditableStates, ", "),
		)}
	}

	holder, err := client.GetBuildAppStoreVersion(ctx, buildID)
	if err != nil {
		return attachErr
	}
	holderID := strings.TrimSpace(holder.Data.ID)
	if holderID == "" || holderID == strings.TrimSpace(versionID) {
		return attachErr
	}
	holderState := strings.TrimSpace(holder.Data.Attributes.AppVersionState)
	if holderState == "" {
		holderState = strings.TrimSpace(holder.Data.Attributes.AppStoreState)
	}
	// versionString is Apple data; quote it before it goes into a command
	// someone may paste into a shell, and fall back to a placeholder when no
	// rendering is safe in every shell.
	versionFlag, ok := shared.ShellQuote(attrs.VersionString)
	if !ok {
		versionFlag = "VERSION"
	}
	return &shared.HintedError{Err: attachErr, Hint: fmt.Sprintf(
		"Build %s is already the build of version %s (%s, id %s), and a build ships with one version only. Attach a build no version uses yet (`asc builds list --app APP_ID --version %s`), or upload a new build for %s.",
		buildID, holder.Data.Attributes.VersionString, holderState, holderID, versionFlag, attrs.VersionString,
	)}
}
