package distribute

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

func TestInspectCommandPropagatesCancellation(t *testing.T) {
	ipaPath := filepath.Join(t.TempDir(), "App.ipa")
	if err := os.WriteFile(ipaPath, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := inspectCommand(newDistributeTestDeps())
	if err := command.Parse([]string{"--ipa", ipaPath, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := command.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("inspect command error = %v, want context.Canceled", err)
	}
}

func TestRenderInspectionDeviceDisclosure(t *testing.T) {
	result := distribution.Inspection{
		App:      distribution.App{BundleID: "com.example.demo", Title: "Demo", Version: "1.0", BuildNumber: "7"},
		Artifact: distribution.Artifact{SHA256: "artifact-sha256"},
		Signing: distribution.Signing{
			ProfileClass:                 distribution.ProfileClassAdHoc,
			ProfileUUID:                  "profile-uuid",
			TeamID:                       "TEAM123",
			DeviceCount:                  2,
			Devices:                      []string{"device-0001", "device-0002"},
			CodeSignatureVerification:    distribution.CodeSignatureVerification{Status: distribution.CodeSignatureVerified},
			ProfileIntegrityVerification: distribution.CodeSignatureVerification{Status: distribution.CodeSignatureVerified},
			ProfileTrustVerification:     distribution.CodeSignatureVerification{Status: distribution.CodeSignatureVerified},
		},
		Preparation: distribution.Preparation{MetadataEligible: true, Issues: []string{}},
	}

	for _, test := range []struct {
		name        string
		markdown    bool
		wantPublic  string
		wantPrivate string
	}{
		{
			name: "table",
			wantPublic: `┌───────────────────┬──────────────────────────┐
│       Field       │          Value           │
├───────────────────┼──────────────────────────┤
│ Metadata Eligible │ true                     │
│ Code Signature    │ verified                 │
│ Profile Integrity │ verified                 │
│ Profile Trust     │ verified                 │
│ Bundle ID         │ com.example.demo         │
│ Title             │ Demo                     │
│ Version           │ 1.0                      │
│ Build             │ 7                        │
│ Profile Class     │ ad-hoc                   │
│ Profile UUID      │ profile-uuid             │
│ Team ID           │ TEAM123                  │
│ Devices           │ 2                        │
│ Device UDIDs      │ device-0001, device-0002 │
│ IPA SHA-256       │ artifact-sha256          │
│ Issues            │                          │
└───────────────────┴──────────────────────────┘
`,
			wantPrivate: `┌───────────────────┬──────────────────┐
│       Field       │      Value       │
├───────────────────┼──────────────────┤
│ Metadata Eligible │ true             │
│ Code Signature    │ verified         │
│ Profile Integrity │ verified         │
│ Profile Trust     │ verified         │
│ Bundle ID         │ com.example.demo │
│ Title             │ Demo             │
│ Version           │ 1.0              │
│ Build             │ 7                │
│ Profile Class     │ ad-hoc           │
│ Profile UUID      │ profile-uuid     │
│ Team ID           │ TEAM123          │
│ Devices           │ 2                │
│ IPA SHA-256       │ artifact-sha256  │
│ Issues            │                  │
└───────────────────┴──────────────────┘
`,
		},
		{
			name:     "markdown",
			markdown: true,
			wantPublic: `| Field             | Value                    |
|:------------------|:-------------------------|
| Metadata Eligible | true                     |
| Code Signature    | verified                 |
| Profile Integrity | verified                 |
| Profile Trust     | verified                 |
| Bundle ID         | com.example.demo         |
| Title             | Demo                     |
| Version           | 1.0                      |
| Build             | 7                        |
| Profile Class     | ad-hoc                   |
| Profile UUID      | profile-uuid             |
| Team ID           | TEAM123                  |
| Devices           | 2                        |
| Device UDIDs      | device-0001, device-0002 |
| IPA SHA-256       | artifact-sha256          |
| Issues            |                          |
`,
			wantPrivate: `| Field             | Value            |
|:------------------|:-----------------|
| Metadata Eligible | true             |
| Code Signature    | verified         |
| Profile Integrity | verified         |
| Profile Trust     | verified         |
| Bundle ID         | com.example.demo |
| Title             | Demo             |
| Version           | 1.0              |
| Build             | 7                |
| Profile Class     | ad-hoc           |
| Profile UUID      | profile-uuid     |
| Team ID           | TEAM123          |
| Devices           | 2                |
| IPA SHA-256       | artifact-sha256  |
| Issues            |                  |
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := renderInspection(&out, result, test.markdown); err != nil {
				t.Fatal(err)
			}
			output := out.String()
			if output != test.wantPublic {
				t.Fatalf("public %s renderer output:\n%s\nwant:\n%s", test.name, output, test.wantPublic)
			}

			privateResult := result
			privateResult.Signing.Devices = nil
			out.Reset()
			if err := renderInspection(&out, privateResult, test.markdown); err != nil {
				t.Fatal(err)
			}
			output = out.String()
			if output != test.wantPrivate {
				t.Fatalf("private %s renderer output:\n%s\nwant:\n%s", test.name, output, test.wantPrivate)
			}
		})
	}
}

func TestDistributionCommandsHonorAlreadyCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, command := range []struct {
		name string
		cmd  *ffcli.Command
	}{
		{name: "inspect", cmd: inspectCommand(newDistributeTestDeps())},
		{name: "prepare", cmd: prepareCommand(newDistributeTestDeps())},
	} {
		t.Run(command.name, func(t *testing.T) {
			if err := command.cmd.Exec(ctx, nil); !errors.Is(err, context.Canceled) {
				t.Fatalf("Exec() error = %v, want context.Canceled", err)
			}
		})
	}
}
