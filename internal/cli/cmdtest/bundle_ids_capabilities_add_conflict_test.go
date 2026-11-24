package cmdtest

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestBundleIDsCapabilitiesAddConflictHintNamesTheNextStep pins the hint for
// each 409 Apple returns on POST /v1/bundleIdCapabilities that is not an
// existence conflict. Apple's bodies were captured live against the disposable
// app's bundle ID on 2026-11-23 (the Expected-one-of list is abbreviated). The
// call keeps failing with exit 5; only the hint is new.
func TestBundleIDsCapabilitiesAddConflictHintNamesTheNextStep(t *testing.T) {
	const notInEnum = `{"errors":[{"id":"3283fe9f-de32-491a-a500-d62d50a71869","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.TYPE","title":"An attribute in the provided entity has the wrong type","detail":"'%s' is not a valid value for the attribute 'capabilityType'. Expected one of: 'ICLOUD', 'APP_GROUPS', 'APPLE_ID_AUTH'","source":{"pointer":"/data/attributes/capabilityType"}}]}`
	const wrongType = "Error: bundle-ids capabilities add: failed to create: An attribute in the provided entity has the wrong type: '%s' is not a valid value for the attribute 'capabilityType'. Expected one of: 'ICLOUD', 'APP_GROUPS', 'APPLE_ID_AUTH'\n"
	const signInSettings = `'[{"key":"APPLE_ID_AUTH_APP_CONSENT","options":[{"key":"PRIMARY_APP_CONSENT","enabled":true}]}]'`

	tests := []struct {
		name       string
		bundle     string
		capability string
		body       string
		want       string
	}{
		{
			name:       "type the API cannot enable",
			bundle:     "BUNDLE1234",
			capability: "FAMILY_CONTROLS",
			body:       fmt.Sprintf(notInEnum, "FAMILY_CONTROLS"),
			want: fmt.Sprintf(wrongType, "FAMILY_CONTROLS") +
				"Hint: The App Store Connect API does not accept FAMILY_CONTROLS (Apple lists the capability types it accepts above), and retrying with other flags or --if-exists does not change that. If FAMILY_CONTROLS is a capability the Apple Developer portal offers, enable it on this App ID there (Certificates, Identifiers & Profiles > Identifiers), then regenerate the provisioning profiles that use the App ID.\n",
		},
		{
			name:       "misspelling of a type the API accepts",
			bundle:     "BUNDLE1234",
			capability: "HEALTH_KIT",
			body:       fmt.Sprintf(notInEnum, "HEALTH_KIT"),
			want: fmt.Sprintf(wrongType, "HEALTH_KIT") +
				`Hint: HEALTH_KIT is not a capability type the App Store Connect API recognizes; the closest one it accepts is HEALTHKIT: asc bundle-ids capabilities add --bundle "BUNDLE1234" --capability HEALTHKIT` + "\n",
		},
		{
			name:       "longer name that starts with a type the API accepts",
			bundle:     "BUNDLE1234",
			capability: "HEALTHKIT_RECALIBRATE_ESTIMATES",
			body:       fmt.Sprintf(notInEnum, "HEALTHKIT_RECALIBRATE_ESTIMATES"),
			want: fmt.Sprintf(wrongType, "HEALTHKIT_RECALIBRATE_ESTIMATES") +
				"Hint: The App Store Connect API does not accept HEALTHKIT_RECALIBRATE_ESTIMATES (Apple lists the capability types it accepts above), and retrying with other flags or --if-exists does not change that. If HEALTHKIT_RECALIBRATE_ESTIMATES is a capability the Apple Developer portal offers, enable it on this App ID there (Certificates, Identifiers & Profiles > Identifiers), then regenerate the provisioning profiles that use the App ID.\n",
		},
		{
			name:       "longer lower-case hyphenated name that starts with a type the API accepts",
			bundle:     "BUNDLE1234",
			capability: "healthkit-recalibrate-estimates",
			body:       fmt.Sprintf(notInEnum, "healthkit-recalibrate-estimates"),
			want: fmt.Sprintf(wrongType, "healthkit-recalibrate-estimates") +
				"Hint: The App Store Connect API does not accept HEALTHKIT-RECALIBRATE-ESTIMATES (Apple lists the capability types it accepts above), and retrying with other flags or --if-exists does not change that. If HEALTHKIT-RECALIBRATE-ESTIMATES is a capability the Apple Developer portal offers, enable it on this App ID there (Certificates, Identifiers & Profiles > Identifiers), then regenerate the provisioning profiles that use the App ID.\n",
		},
		{
			name:       "longer hyphenated name that starts with a two-word type the API accepts",
			bundle:     "BUNDLE1234",
			capability: "APPLE-PAY-LATER",
			body:       fmt.Sprintf(notInEnum, "APPLE-PAY-LATER"),
			want: fmt.Sprintf(wrongType, "APPLE-PAY-LATER") +
				"Hint: The App Store Connect API does not accept APPLE-PAY-LATER (Apple lists the capability types it accepts above), and retrying with other flags or --if-exists does not change that. If APPLE-PAY-LATER is a capability the Apple Developer portal offers, enable it on this App ID there (Certificates, Identifiers & Profiles > Identifiers), then regenerate the provisioning profiles that use the App ID.\n",
		},
		{
			name:       "type rejection reported after another errors entry",
			bundle:     "BUNDLE1234",
			capability: "GAMECENTER",
			body:       `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"There is a problem with the request entity","detail":"The request entity is invalid"},{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.TYPE","title":"An attribute in the provided entity has the wrong type","detail":"'GAMECENTER' is not a valid value for the attribute 'capabilityType'. Expected one of: 'ICLOUD', 'GAME_CENTER'"}]}`,
			want: "Error: bundle-ids capabilities add: failed to create: There is a problem with the request entity: The request entity is invalid\n\nAdditional errors:\n  - 'GAMECENTER' is not a valid value for the attribute 'capabilityType'. Expected one of: 'ICLOUD', 'GAME_CENTER'\n" +
				`Hint: GAMECENTER is not a capability type the App Store Connect API recognizes; the closest one it accepts is GAME_CENTER: asc bundle-ids capabilities add --bundle "BUNDLE1234" --capability GAME_CENTER` + "\n",
		},
		{
			name:       "Developer Portal-only type with a web command",
			bundle:     "BUNDLE1234",
			capability: "PRIVATE_CLOUD_COMPUTE",
			body:       fmt.Sprintf(notInEnum, "PRIVATE_CLOUD_COMPUTE"),
			want: fmt.Sprintf(wrongType, "PRIVATE_CLOUD_COMPUTE") +
				`Hint: PRIVATE_CLOUD_COMPUTE is a Developer Portal-only capability. Enable it through a web session with asc web bundle-ids capabilities enable --bundle-id "BUNDLE1234" --capability PRIVATE_CLOUD_COMPUTE --confirm` + "\n",
		},
		{
			name:       "entitlement-style name for an API type",
			bundle:     "BUNDLE1234",
			capability: "APPLICATION_GROUPS",
			body:       fmt.Sprintf(notInEnum, "APPLICATION_GROUPS"),
			want: fmt.Sprintf(wrongType, "APPLICATION_GROUPS") +
				`Hint: App Store Connect calls this capability APP_GROUPS: asc bundle-ids capabilities add --bundle "BUNDLE1234" --capability APP_GROUPS` + "\n",
		},
		{
			name:       "Sign in with Apple under its portal name",
			bundle:     "BUNDLE1234",
			capability: "SIGN_IN_WITH_APPLE",
			body:       fmt.Sprintf(notInEnum, "SIGN_IN_WITH_APPLE"),
			want: fmt.Sprintf(wrongType, "SIGN_IN_WITH_APPLE") +
				`Hint: App Store Connect calls this capability APPLE_ID_AUTH: asc bundle-ids capabilities add --bundle "BUNDLE1234" --capability APPLE_ID_AUTH --settings ` + signInSettings + "\n",
		},
		{
			name:       "Sign in with Apple without a configuration",
			bundle:     "BUNDLE1234",
			capability: "APPLE_ID_AUTH",
			body:       `{"errors":[{"id":"0fe8d6cb-e96a-40d8-ba4b-77634026ca30","status":"409","code":"ENTITY_ERROR","title":"There is a problem with the request entity","detail":"Please select at least one configuration for Sign In with Apple."}]}`,
			want: "Error: bundle-ids capabilities add: failed to create: There is a problem with the request entity: Please select at least one configuration for Sign In with Apple.\n" +
				`Hint: Sign in with Apple needs a configuration. To enable it as a primary App ID: asc bundle-ids capabilities add --bundle "BUNDLE1234" --capability APPLE_ID_AUTH --settings ` + signInSettings + "\n",
		},
		{
			name:       "bundle identifier instead of the resource ID",
			bundle:     "com.example.app",
			capability: "PUSH_NOTIFICATIONS",
			body:       `{"errors":[{"id":"ee5ac3f3-1d0d-40d4-86c7-2a4592a36f09","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The provided entity includes a relationship with an invalid value","source":{"pointer":"data/relationships/bundleIds"}}]}`,
			want: "Error: bundle-ids capabilities add: failed to create: The provided entity includes a relationship with an invalid value: The provided entity includes a relationship with an invalid value\n" +
				`Hint: --bundle takes the bundle ID's resource ID (for example ABCDE12345), not its identifier "com.example.app". Look the ID up with asc bundle-ids list --identifier "com.example.app", then rerun with --bundle set to it.` + "\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.POST("/v1/bundleIdCapabilities"), asctest.JSON(http.StatusConflict, test.body))

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "bundle-ids", "capabilities", "add",
				"--bundle", test.bundle, "--capability", test.capability, "--output", "json")

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.want {
				t.Fatalf("stderr = %q\nwant     %q", stderr, test.want)
			}
		})
	}
}
