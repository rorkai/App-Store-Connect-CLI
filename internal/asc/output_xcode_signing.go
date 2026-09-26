package asc

import (
	"fmt"
	"strings"
)

// XcodeSigningPlanOutput is the outward JSON and human-readable output for a
// local Xcode signing plan. It is intentionally separate from the plan
// artifact type so rendering changes cannot alter the artifact hash contract.
type XcodeSigningPlanOutput struct {
	SchemaVersion           int                               `json:"schemaVersion"`
	Command                 string                            `json:"command"`
	GeneratedAt             string                            `json:"generatedAt"`
	PlanHash                string                            `json:"planHash"`
	Ready                   bool                              `json:"ready"`
	ProjectPath             string                            `json:"projectPath"`
	SettingsFilePath        string                            `json:"settingsFilePath"`
	PlanPath                string                            `json:"planPath"`
	ReceiptPath             string                            `json:"receiptPath"`
	AllowExternalXCConfig   bool                              `json:"allowExternalXCConfig"`
	Desired                 []XcodeSigningPlanTargetOutput    `json:"desired"`
	Files                   []XcodeSigningPlanFileOutput      `json:"files"`
	Changes                 []XcodeSigningSettingChangeOutput `json:"changes"`
	MissingOptionalIncludes []string                          `json:"missingOptionalIncludes,omitempty"`
	ProfilePaths            []string                          `json:"profilePaths,omitempty"`
	Configuration           string                            `json:"configuration,omitempty"`
	ExportMethod            string                            `json:"exportMethod,omitempty"`
	SkipTargets             []string                          `json:"skipTargets,omitempty"`
	Inferences              []XcodeSigningPlanInferenceOutput `json:"inferences,omitempty"`
	ExportOptions           *XcodeSigningPlanExportOptions    `json:"exportOptions,omitempty"`
	Blockers                []string                          `json:"blockers"`
	Warnings                []string                          `json:"warnings"`
}

// XcodeSigningPlanInferenceOutput records the profile chosen for one target.
type XcodeSigningPlanInferenceOutput struct {
	Target            string `json:"target"`
	Configuration     string `json:"configuration"`
	BundleID          string `json:"bundleId"`
	ProfilePath       string `json:"profilePath"`
	ProfileUUID       string `json:"profileUuid"`
	CertificateSHA256 string `json:"certificateSha256"`
	Match             string `json:"match"`
}

// XcodeSigningPlanExportOptions is the export-options payload in plan output.
type XcodeSigningPlanExportOptions struct {
	Method               string            `json:"method,omitempty"`
	SigningStyle         string            `json:"signingStyle,omitempty"`
	TeamID               string            `json:"teamID,omitempty"`
	ProvisioningProfiles map[string]string `json:"provisioningProfiles,omitempty"`
}

// XcodeSigningPlanTargetOutput describes one target in a signing plan.
type XcodeSigningPlanTargetOutput struct {
	Target         string                                `json:"target"`
	Configurations []XcodeSigningPlanConfigurationOutput `json:"configurations"`
}

// XcodeSigningPlanConfigurationOutput describes one build configuration in a
// signing plan.
type XcodeSigningPlanConfigurationOutput struct {
	Name     string                          `json:"name"`
	Settings []XcodeSigningPlanSettingOutput `json:"settings"`
}

// XcodeSigningPlanSettingOutput describes one normalized desired signing
// setting.
type XcodeSigningPlanSettingOutput struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// XcodeSigningPlanFileOutput records a source file digest bound into a plan.
type XcodeSigningPlanFileOutput struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
}

// XcodeSigningSettingChangeOutput records one concrete signing-setting
// operation in a plan or receipt.
type XcodeSigningSettingChangeOutput struct {
	Target        string  `json:"target"`
	Configuration string  `json:"configuration"`
	Setting       string  `json:"setting"`
	Operation     string  `json:"operation"`
	Resolution    string  `json:"resolution"`
	OldValue      *string `json:"oldValue"`
	NewValue      *string `json:"newValue"`
	Path          string  `json:"path"`
	Source        string  `json:"source"`
}

// XcodeSigningApplyOutput is the outward JSON and human-readable output for a
// completed local Xcode signing apply. The receipt artifact remains owned by
// the xcode package and is encoded from its domain type before this view is
// rendered.
type XcodeSigningApplyOutput struct {
	SchemaVersion int                               `json:"schemaVersion"`
	AppliedAt     string                            `json:"appliedAt"`
	Completed     bool                              `json:"completed"`
	PlanHash      string                            `json:"planHash"`
	PlanPath      string                            `json:"planPath"`
	ReceiptPath   string                            `json:"receiptPath"`
	ChangedFiles  []string                          `json:"changedFiles"`
	Files         []XcodeSigningFileChangeOutput    `json:"files"`
	Changes       []XcodeSigningSettingChangeOutput `json:"changes"`
}

// XcodeSigningFileChangeOutput binds a written file to its before and after
// digest in an apply receipt.
type XcodeSigningFileChangeOutput struct {
	Path         string `json:"path"`
	Source       string `json:"source"`
	BeforeSHA256 string `json:"beforeSha256"`
	AfterSHA256  string `json:"afterSha256"`
}

func xcodeSigningPlanOutputRows(plan *XcodeSigningPlanOutput) ([]string, [][]string) {
	if plan == nil {
		plan = &XcodeSigningPlanOutput{}
	}
	return []string{"field", "value"}, [][]string{
		{"ready", fmt.Sprintf("%t", plan.Ready)},
		{"plan", plan.PlanPath},
		{"plan hash", plan.PlanHash},
		{"changes", fmt.Sprintf("%d", len(plan.Changes))},
		{"blockers", strings.Join(plan.Blockers, "; ")},
		{"warnings", strings.Join(plan.Warnings, "; ")},
	}
}

func xcodeSigningApplyOutputRows(result *XcodeSigningApplyOutput) ([]string, [][]string) {
	if result == nil {
		result = &XcodeSigningApplyOutput{}
	}
	return []string{"field", "value"}, [][]string{
		{"completed", fmt.Sprintf("%t", result.Completed)},
		{"plan", result.PlanPath},
		{"receipt", result.ReceiptPath},
		{"plan hash", result.PlanHash},
		{"changed files", strings.Join(result.ChangedFiles, ", ")},
	}
}
