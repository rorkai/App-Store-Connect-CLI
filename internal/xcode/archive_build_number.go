package xcode

import (
	"fmt"
	"strings"
)

// ArchiveBuildNumberFloor identifies the application configuration supplying a local floor.
type ArchiveBuildNumberFloor struct {
	Target        string
	Configuration string
	BundleID      string
	BuildNumber   string
}

// ReadArchiveBuildNumberFloor uses only structured project and xcconfig settings.
func ReadArchiveBuildNumberFloor(projectPath, target, configuration string, args []string) (*ArchiveBuildNumberFloor, error) {
	if reserved := firstReservedPassthroughArgument(args,
		[]string{"-project", "-workspace", "-scheme", "-target", "-configuration", "-xcconfig"},
		nil, []string{"current_project_version", "configuration"}); reserved != "" {
		return nil, fmt.Errorf("--archive-xcodebuild-flag cannot override %s with --local-build-number-floor", reserved)
	}
	project, err := openStructuredVersionProject(projectPath)
	if err != nil {
		return nil, err
	}
	selected, err := project.selectViewConfiguration(target, configuration)
	if err != nil {
		return nil, err
	}
	for _, candidate := range project.project.Proj.Targets {
		if candidate.Name == selected.target && !candidate.IsAppProduct() {
			return nil, fmt.Errorf("target %q is not an application; use --target for the app archived by --scheme", selected.target)
		}
	}
	build, _, err := project.resolveSetting(selected, currentProjectSetting)
	if err != nil {
		return nil, fmt.Errorf("resolve CURRENT_PROJECT_VERSION: %w", err)
	}
	bundle, _, err := project.resolveSetting(selected, "PRODUCT_BUNDLE_IDENTIFIER")
	if err != nil {
		return nil, fmt.Errorf("resolve PRODUCT_BUNDLE_IDENTIFIER: %w", err)
	}
	if strings.TrimSpace(bundle) == "" {
		return nil, fmt.Errorf("PRODUCT_BUNDLE_IDENTIFIER must not be empty")
	}
	return &ArchiveBuildNumberFloor{Target: selected.target, Configuration: selected.name, BundleID: strings.TrimSpace(bundle), BuildNumber: strings.TrimSpace(build)}, nil
}
