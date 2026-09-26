package xcode

import (
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

// The signing output structs and their renderers live in internal/asc so the
// JSON contract stays registered centrally. The conversion from the domain
// artifact lives here instead of in internal/asc because internal/xcode
// imports internal/asc for its own registered output types, so an
// internal/asc -> internal/xcode import would create a cycle.

// newXcodeSigningPlanOutput converts the domain artifact into the registered
// outward output type without sharing mutable slices or pointer fields.
func newXcodeSigningPlanOutput(plan *localxcode.SigningPlan) *asc.XcodeSigningPlanOutput {
	if plan == nil {
		return nil
	}
	return &asc.XcodeSigningPlanOutput{
		SchemaVersion:           plan.SchemaVersion,
		Command:                 plan.Command,
		GeneratedAt:             plan.GeneratedAt,
		PlanHash:                plan.PlanHash,
		Ready:                   plan.Ready,
		ProjectPath:             plan.ProjectPath,
		SettingsFilePath:        plan.SettingsFilePath,
		PlanPath:                plan.PlanPath,
		ReceiptPath:             plan.ReceiptPath,
		AllowExternalXCConfig:   plan.AllowExternalXCConfig,
		Desired:                 cloneXcodeSigningPlanTargets(plan.Desired),
		Files:                   cloneXcodeSigningPlanFiles(plan.Files),
		Changes:                 cloneXcodeSigningSettingChanges(plan.Changes),
		MissingOptionalIncludes: cloneSigningStrings(plan.MissingOptionalIncludes),
		ProfilePaths:            cloneSigningStrings(plan.ProfilePaths),
		Configuration:           plan.Configuration,
		ExportMethod:            plan.ExportMethod,
		SkipTargets:             cloneSigningStrings(plan.SkipTargets),
		Inferences:              cloneXcodeSigningPlanInferences(plan.Inferences),
		ExportOptions:           cloneXcodeSigningPlanExportOptions(plan.ExportOptions),
		Blockers:                cloneSigningStrings(plan.Blockers),
		Warnings:                cloneSigningStrings(plan.Warnings),
	}
}

// newXcodeSigningApplyOutput converts the domain receipt into the registered
// outward output type without sharing mutable slices or pointer fields.
func newXcodeSigningApplyOutput(result *localxcode.SigningApplyResult) *asc.XcodeSigningApplyOutput {
	if result == nil {
		return nil
	}
	return &asc.XcodeSigningApplyOutput{
		SchemaVersion: result.SchemaVersion,
		AppliedAt:     result.AppliedAt,
		Completed:     result.Completed,
		PlanHash:      result.PlanHash,
		PlanPath:      result.PlanPath,
		ReceiptPath:   result.ReceiptPath,
		ChangedFiles:  cloneSigningStrings(result.ChangedFiles),
		Files:         cloneXcodeSigningFileChanges(result.Files),
		Changes:       cloneXcodeSigningSettingChanges(result.Changes),
	}
}

func cloneXcodeSigningPlanTargets(values []localxcode.SigningPlanTarget) []asc.XcodeSigningPlanTargetOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningPlanTargetOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningPlanTargetOutput{
			Target:         value.Target,
			Configurations: cloneXcodeSigningPlanConfigurations(value.Configurations),
		}
	}
	return cloned
}

func cloneXcodeSigningPlanConfigurations(values []localxcode.SigningPlanConfiguration) []asc.XcodeSigningPlanConfigurationOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningPlanConfigurationOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningPlanConfigurationOutput{
			Name:     value.Name,
			Settings: cloneXcodeSigningPlanSettings(value.Settings),
		}
	}
	return cloned
}

func cloneXcodeSigningPlanSettings(values []localxcode.SigningPlanSetting) []asc.XcodeSigningPlanSettingOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningPlanSettingOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningPlanSettingOutput{
			Key:   value.Key,
			Value: cloneSigningString(value.Value),
		}
	}
	return cloned
}

func cloneXcodeSigningPlanFiles(values []localxcode.SigningPlanFile) []asc.XcodeSigningPlanFileOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningPlanFileOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningPlanFileOutput{
			Path:   value.Path,
			SHA256: value.SHA256,
			Source: value.Source,
		}
	}
	return cloned
}

func cloneXcodeSigningSettingChanges(values []localxcode.SigningSettingChange) []asc.XcodeSigningSettingChangeOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningSettingChangeOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningSettingChangeOutput{
			Target:        value.Target,
			Configuration: value.Configuration,
			Setting:       value.Setting,
			Operation:     value.Operation,
			Resolution:    value.Resolution,
			OldValue:      cloneSigningString(value.OldValue),
			NewValue:      cloneSigningString(value.NewValue),
			Path:          value.Path,
			Source:        value.Source,
		}
	}
	return cloned
}

func cloneXcodeSigningFileChanges(values []localxcode.SigningFileChange) []asc.XcodeSigningFileChangeOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningFileChangeOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningFileChangeOutput{
			Path:         value.Path,
			Source:       value.Source,
			BeforeSHA256: value.BeforeSHA256,
			AfterSHA256:  value.AfterSHA256,
		}
	}
	return cloned
}

func cloneXcodeSigningPlanInferences(values []localxcode.SigningPlanInference) []asc.XcodeSigningPlanInferenceOutput {
	if values == nil {
		return nil
	}
	cloned := make([]asc.XcodeSigningPlanInferenceOutput, len(values))
	for index, value := range values {
		cloned[index] = asc.XcodeSigningPlanInferenceOutput{
			Target:            value.Target,
			Configuration:     value.Configuration,
			BundleID:          value.BundleID,
			ProfilePath:       value.ProfilePath,
			ProfileUUID:       value.ProfileUUID,
			CertificateSHA256: value.CertificateSHA256,
			Match:             value.Match,
		}
	}
	return cloned
}

func cloneXcodeSigningPlanExportOptions(options *localxcode.SigningPlanExportOptions) *asc.XcodeSigningPlanExportOptions {
	if options == nil {
		return nil
	}
	cloned := &asc.XcodeSigningPlanExportOptions{
		Method:       options.Method,
		SigningStyle: options.SigningStyle,
		TeamID:       options.TeamID,
	}
	if options.ProvisioningProfiles != nil {
		cloned.ProvisioningProfiles = make(map[string]string, len(options.ProvisioningProfiles))
		for key, value := range options.ProvisioningProfiles {
			cloned.ProvisioningProfiles[key] = value
		}
	}
	return cloned
}

func cloneSigningStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneSigningString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
