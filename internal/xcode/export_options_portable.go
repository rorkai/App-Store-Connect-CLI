//go:build !darwin

package xcode

import (
	"context"
	"fmt"

	"github.com/bitrise-io/go-xcode/exportoptions"
)

// buildPlatformExportOptionsPayload uses the stable v1 typed model on hosts
// without Xcode. This keeps automatic ExportOptions generation portable.
func buildPlatformExportOptionsPayload(opts ExportOptionsGenerateOptions, teamID string, manual manualExportOptions) map[string]any {
	switch opts.Method {
	case exportOptionsMethodReleaseTesting, exportOptionsMethodDebugging, exportOptionsMethodEnterprise:
		model := exportoptions.NewNonAppStoreOptions(exportoptions.Method(opts.Method))
		model.TeamID = teamID
		model.Destination = exportoptions.Destination(opts.Destination)
		model.SigningStyle = exportoptions.SigningStyle(opts.SigningStyle)
		if opts.SigningStyle == exportOptionsSigningStyleManual {
			model.SigningCertificate = manual.SigningCertificate
			model.BundleIDProvisioningProfileMapping = cloneProvisioningProfiles(manual.ProvisioningProfiles)
			model.ICloudContainerEnvironment = exportoptions.ICloudContainerEnvironment(manual.ICloudContainerEnvironment)
		}
		return model.Hash()
	}
	model := exportoptions.NewAppStoreConnectOptions(exportoptions.MethodAppStoreConnect)
	model.TeamID = teamID
	model.Destination = exportoptions.Destination(opts.Destination)
	model.SigningStyle = exportoptions.SigningStyle(opts.SigningStyle)
	if opts.SigningStyle == exportOptionsSigningStyleManual {
		model.SigningCertificate = manual.SigningCertificate
		model.BundleIDProvisioningProfileMapping = cloneProvisioningProfiles(manual.ProvisioningProfiles)
		model.ICloudContainerEnvironment = exportoptions.ICloudContainerEnvironment(manual.ICloudContainerEnvironment)
	}
	return model.Hash()
}

func generateManualExportOptions(context.Context, string, string, string) (manualExportOptions, error) {
	return manualExportOptions{}, fmt.Errorf("manual signing export options generation is only supported on macOS because it requires Xcode signing assets")
}
