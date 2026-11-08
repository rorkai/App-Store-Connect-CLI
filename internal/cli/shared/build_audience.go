package shared

import (
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// RequireAppStoreBuildAudience rejects builds restricted to internal testing.
func RequireAppStoreBuildAudience(buildResp *asc.BuildResponse) error {
	if strings.EqualFold(strings.TrimSpace(string(buildResp.Data.Attributes.BuildAudienceType)), string(asc.BuildAudienceTypeInternalOnly)) {
		return WithDiagnostic(NewValidationError(fmt.Errorf("build %s (%s) is INTERNAL_ONLY and cannot be attached to an App Store version", buildResp.Data.Attributes.Version, buildResp.Data.ID)), DiagnosticStateNotReady, "--build-id")
	}
	return nil
}
