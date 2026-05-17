package xcodecloud

import "testing"

func TestXcodeCloudCommandConstructors(t *testing.T) {
	top := XcodeCloudCommand()
	if top == nil {
		t.Fatal("expected xcode-cloud command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := XcodeCloudCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return XcodeCloudRunCommand() },
		func() any { return XcodeCloudStatusCommand() },
		func() any { return XcodeCloudWorkflowsCommand() },
		func() any { return XcodeCloudBuildRunsCommand() },
		func() any { return XcodeCloudActionsCommand() },
		func() any { return XcodeCloudArtifactsCommand() },
		func() any { return XcodeCloudTestResultsCommand() },
		func() any { return XcodeCloudIssuesCommand() },
		func() any { return XcodeCloudScmCommand() },
		func() any { return XcodeCloudProductsCommand() },
		func() any { return XcodeCloudMacOSVersionsCommand() },
		func() any { return XcodeCloudXcodeVersionsCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
