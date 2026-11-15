package asc

import "fmt"

// SigningResignInputResult identifies the source artifact without exposing
// its local path.
type SigningResignInputResult struct {
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

// SigningResignArtifactResult identifies a generated artifact and its digest.
type SigningResignArtifactResult struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

// SigningResignIdentityResult reports the certificate binding used for the
// local signing operation.
type SigningResignIdentityResult struct {
	CertificateSHA256 string `json:"certificateSha256"`
	TeamID            string `json:"teamId"`
}

// SigningResignTargetResult reports one validated app-like target.
type SigningResignTargetResult struct {
	Kind           string `json:"kind"`
	RelativePath   string `json:"relativePath"`
	BundleID       string `json:"bundleId"`
	SourceBundleID string `json:"sourceBundleId,omitempty"`
	ProfileClass   string `json:"profileClass"`
	ProfileUUID    string `json:"profileUuid"`
	ProfileSHA256  string `json:"profileSha256"`
	Status         string `json:"status"`
}

// SigningResignEntitlementRewrite records one explicitly requested,
// profile-authorized claim transformation. Values are emitted as the
// entitlement plist values, not as profile wildcards.
type SigningResignEntitlementRewrite struct {
	TargetRelativePath string `json:"targetRelativePath"`
	BundleID           string `json:"bundleId"`
	Key                string `json:"key"`
	ElementIndex       *int   `json:"elementIndex,omitempty"`
	From               any    `json:"from"`
	To                 any    `json:"to"`
}

// SigningResignVerification reports the scope and result of post-signing
// verification.
type SigningResignVerification struct {
	Status string `json:"status"`
	Scope  string `json:"scope"`
}

// SigningResignResult is the stable computed output contract for
// `asc signing resign`.
type SigningResignResult struct {
	SchemaVersion       int                                `json:"schemaVersion"`
	Command             string                             `json:"command"`
	Input               SigningResignInputResult           `json:"input"`
	Output              SigningResignArtifactResult        `json:"output"`
	Identity            SigningResignIdentityResult        `json:"identity"`
	Targets             []SigningResignTargetResult        `json:"targets"`
	EntitlementRewrites *[]SigningResignEntitlementRewrite `json:"entitlementRewrites,omitempty"`
	Verification        SigningResignVerification          `json:"verification"`
}

func signingResignResultRows(result *SigningResignResult) ([]string, [][]string) {
	rows := [][]string{
		{"schemaVersion", fmt.Sprintf("%d", result.SchemaVersion)},
		{"command", result.Command},
		{"input.sizeBytes", fmt.Sprintf("%d", result.Input.SizeBytes)},
		{"input.sha256", result.Input.SHA256},
		{"output.path", result.Output.Path},
		{"output.sizeBytes", fmt.Sprintf("%d", result.Output.SizeBytes)},
		{"output.sha256", result.Output.SHA256},
		{"identity.certificateSha256", result.Identity.CertificateSHA256},
		{"identity.teamId", result.Identity.TeamID},
		{"verification.status", result.Verification.Status},
		{"verification.scope", result.Verification.Scope},
	}
	for _, target := range result.Targets {
		prefix := "target." + target.RelativePath
		rows = append(rows, []string{prefix + ".kind", target.Kind}, []string{prefix + ".bundleId", target.BundleID})
		if target.SourceBundleID != "" {
			rows = append(rows, []string{prefix + ".sourceBundleId", target.SourceBundleID})
		}
		rows = append(
			rows,
			[]string{prefix + ".profileClass", target.ProfileClass},
			[]string{prefix + ".profileUuid", target.ProfileUUID},
			[]string{prefix + ".profileSha256", target.ProfileSHA256},
			[]string{prefix + ".status", target.Status},
		)
	}
	if result.EntitlementRewrites == nil {
		return []string{"field", "value"}, rows
	}
	for index, rewrite := range *result.EntitlementRewrites {
		prefix := fmt.Sprintf("entitlementRewrite.%03d", index)
		rows = append(
			rows,
			[]string{prefix + ".targetRelativePath", rewrite.TargetRelativePath},
			[]string{prefix + ".bundleId", rewrite.BundleID},
			[]string{prefix + ".key", rewrite.Key},
		)
		if rewrite.ElementIndex != nil {
			rows = append(rows, []string{prefix + ".elementIndex", fmt.Sprintf("%d", *rewrite.ElementIndex)})
		}
		rows = append(
			rows,
			[]string{prefix + ".from", fmt.Sprintf("%v", rewrite.From)},
			[]string{prefix + ".to", fmt.Sprintf("%v", rewrite.To)},
		)
	}
	return []string{"field", "value"}, rows
}
