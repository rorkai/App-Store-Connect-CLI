package ads

import (
	"context"
	"errors"
	"flag"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestAdsCommandRegistersEveryEndpointSpec(t *testing.T) {
	root := AdsCommand()
	for _, spec := range appleads.EndpointSpecs() {
		cmd := findCommand(root, spec.CommandPath...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(spec.CommandPath, " "))
		}
		if cmd.Exec == nil {
			t.Fatalf("command asc ads %s has no Exec", strings.Join(spec.CommandPath, " "))
		}
		assertSpecFlags(t, cmd, spec)

		if spec.DefaultListAlias {
			alias := findCommand(root, spec.CommandPath[0])
			if alias == nil {
				t.Fatalf("missing default list alias asc ads %s", spec.CommandPath[0])
			}
			if alias.Exec == nil {
				t.Fatalf("default list alias asc ads %s has no Exec", spec.CommandPath[0])
			}
			assertSpecFlags(t, alias, spec)
		}
	}
}

func TestAdsCampaignsHelpReadsAsManagementSurface(t *testing.T) {
	root := AdsCommand()
	campaigns := findCommand(root, "campaigns")
	if campaigns == nil {
		t.Fatal("missing campaigns command")
	}
	if campaigns.ShortHelp != "Manage Apple Ads campaigns." {
		t.Fatalf("campaigns ShortHelp = %q, want management surface", campaigns.ShortHelp)
	}
	if campaigns.FlagSet.Lookup("campaign") != nil {
		t.Fatal("campaigns list alias should not expose workflow-only --campaign flag")
	}

	resume := findCommand(root, "campaigns", "resume")
	if resume == nil {
		t.Fatal("missing campaigns resume command")
	}
	campaignFlag := resume.FlagSet.Lookup("campaign")
	if campaignFlag == nil {
		t.Fatal("resume command missing --campaign flag")
	}
	if got := campaignFlag.Usage; got != "Apple Ads campaign ID (required)" {
		t.Fatalf("resume --campaign usage = %q, want operator-friendly wording", got)
	}
	if !strings.Contains(resume.LongHelp, "--campaign CAMPAIGN_ID --confirm --org ORG_ID") {
		t.Fatalf("resume LongHelp = %q, want campaign ID example", resume.LongHelp)
	}
}

func TestAdsCampaignUpdateHelpDocumentsRequiredEnvelope(t *testing.T) {
	root := AdsCommand()
	update := findCommand(root, "campaigns", "update")
	if update == nil {
		t.Fatal("missing campaigns update command")
	}

	for _, want := range []string{
		`Apple requires a "campaign" envelope for campaign updates.`,
		`{"campaign":{"status":"PAUSED"}}`,
	} {
		if !strings.Contains(update.LongHelp, want) {
			t.Fatalf("campaigns update LongHelp = %q, want %q", update.LongHelp, want)
		}
	}
}

func TestCollectQueryValidatesEndpointSpecificLimitsAndEnums(t *testing.T) {
	customReports, _ := appleads.EndpointByCommandPath("impression-share-reports", "list")
	fs, flags := bindEndpointFlags(customReports, "test")
	if err := fs.Parse([]string{"--limit", "0"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if _, err := collectQuery(customReports, flags); err == nil || !strings.Contains(err.Error(), "--limit must be between 1 and 50") {
		t.Fatalf("custom reports explicit zero limit error = %v, want min 1 error", err)
	}

	_, flags = bindEndpointFlags(customReports, "test")
	*flags.queryInts["limit"] = 51
	if _, err := collectQuery(customReports, flags); err == nil || !strings.Contains(err.Error(), "--limit must be between 1 and 50") {
		t.Fatalf("custom reports limit error = %v, want max 50 error", err)
	}

	productPages, _ := appleads.EndpointByCommandPath("product-pages", "list")
	_, flags = bindEndpointFlags(productPages, "test")
	*flags.pathStrings["adamId"] = "123456789"
	*flags.queryStrings["states"] = "VISIBLE,PAUSED"
	if _, err := collectQuery(productPages, flags); err == nil || !strings.Contains(err.Error(), "--states must be one of: HIDDEN, VISIBLE") {
		t.Fatalf("states error = %v, want enum validation", err)
	}
}

func TestCollectPathParamsRequiresDocumentedIdentifiers(t *testing.T) {
	campaign, _ := appleads.EndpointByCommandPath("campaigns", "view")
	_, flags := bindEndpointFlags(campaign, "test")
	if _, err := collectPathParams(campaign, flags); err == nil || !strings.Contains(err.Error(), "--campaign is required") {
		t.Fatalf("path error = %v, want campaign required", err)
	}

	*flags.pathStrings["campaignId"] = "123"
	params, err := collectPathParams(campaign, flags)
	if err != nil {
		t.Fatalf("collectPathParams() error: %v", err)
	}
	if params["campaignId"] != "123" {
		t.Fatalf("campaignId = %q, want 123", params["campaignId"])
	}

	*flags.pathStrings["campaignId"] = "not-a-number"
	if _, err := collectPathParams(campaign, flags); err == nil || !strings.Contains(err.Error(), "--campaign must be an integer") {
		t.Fatalf("path error = %v, want integer validation", err)
	}
}

func TestRawRequestRequiresOrgGuardrails(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requiresOrg bool
		wantErr     string
	}{
		{name: "me does not need org", path: "v5/me", requiresOrg: false},
		{name: "me with query does not need org", path: "v5/me?fields=id", requiresOrg: false},
		{name: "acls does not need org", path: "https://api.searchads.apple.com/api/v5/acls", requiresOrg: false},
		{name: "absolute me with query does not need org", path: "https://api.searchads.apple.com/api/v5/me?fields=id", requiresOrg: false},
		{name: "campaigns needs org", path: "v5/campaigns", requiresOrg: true},
		{name: "reject non apple host", path: "https://example.com/api/v5/campaigns", wantErr: "Apple Ads v5 URL"},
		{name: "reject path traversal", path: "v5/../campaigns", wantErr: "path traversal"},
		{name: "reject wrong version", path: "v4/campaigns", wantErr: "start with v5/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requiresOrg, err := rawRequestRequiresOrg(tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("rawRequestRequiresOrg() error: %v", err)
			}
			if requiresOrg != tt.requiresOrg {
				t.Fatalf("requiresOrg = %t, want %t", requiresOrg, tt.requiresOrg)
			}
		})
	}
}

func TestResolveCredentialsPrefersExplicitProfileAndStrictRejectsMixedSources(t *testing.T) {
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	if err := appleads.StoreCredentialsConfigAt("profile-a", appleads.Credentials{
		ClientID:       "CLIENT",
		TeamID:         "TEAM",
		KeyID:          "KEY",
		PrivateKeyPath: "private-key.pem",
		OrgID:          "ORG",
	}, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	profileName := "profile-a"
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	credentials, err := resolveCredentials(commonFlags{AdsProfile: &profileName})
	if err != nil {
		t.Fatalf("resolveCredentials() error: %v", err)
	}
	if credentials.Profile != "profile-a" || credentials.AccessToken != "" || credentials.ClientID != "CLIENT" {
		t.Fatalf("credentials = %+v, want stored profile over access token", credentials)
	}

	t.Setenv("ASC_ADS_STRICT_AUTH", "1")
	_, err = resolveCredentials(commonFlags{AdsProfile: &profileName})
	if err == nil || !strings.Contains(err.Error(), "mixed Apple Ads authentication sources") {
		t.Fatalf("strict mixed source error = %v", err)
	}
}

func TestResolveClientRequiresOrgForOrgScopedEndpoints(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	_, err := resolveClient(context.Background(), commonFlags{}, true)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("resolveClient() error = %v, want usage error", err)
	}

	org := "123456"
	client, err := resolveClient(context.Background(), commonFlags{Org: &org}, true)
	if err != nil {
		t.Fatalf("resolveClient() with org error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestResolveClientUsesStoredAdsOrgWithAccessToken(t *testing.T) {
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	if err := config.SaveAt(configPath, &config.Config{
		Ads: config.AdsConfig{OrgID: "CONFIG_ORG"},
	}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	client, err := resolveClient(context.Background(), commonFlags{}, true)
	if err != nil {
		t.Fatalf("resolveClient() error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestEnvCredentialsRejectsInvalidPrivateKeyBase64(t *testing.T) {
	t.Setenv("ASC_ADS_CLIENT_ID", "CLIENT")
	t.Setenv("ASC_ADS_TEAM_ID", "TEAM")
	t.Setenv("ASC_ADS_KEY_ID", "KEY")
	t.Setenv("ASC_ADS_PRIVATE_KEY_B64", "not-base64")

	_, _, err := envCredentials()
	if err == nil || !strings.Contains(err.Error(), "ASC_ADS_PRIVATE_KEY_B64 is not valid base64") {
		t.Fatalf("envCredentials() error = %v, want invalid base64 error", err)
	}
}

func TestResolveCredentialsStrictRejectsAccessTokenAndKeyEnv(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_STRICT_AUTH", "1")
	t.Setenv("ASC_ADS_CLIENT_ID", "CLIENT")
	t.Setenv("ASC_ADS_TEAM_ID", "TEAM")
	t.Setenv("ASC_ADS_KEY_ID", "KEY")
	t.Setenv("ASC_ADS_PRIVATE_KEY_PATH", "private-key.pem")

	_, err := resolveCredentials(commonFlags{})
	if err == nil || !strings.Contains(err.Error(), "mixed Apple Ads authentication sources") {
		t.Fatalf("resolveCredentials() error = %v, want mixed source error", err)
	}
}

func TestResolveCredentialsRejectsPartialEnvBeforeStoredFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_CLIENT_ID", "CLIENT")
	if err := appleads.StoreCredentialsConfigAt("profile-a", appleads.Credentials{
		ClientID:       "STORED_CLIENT",
		TeamID:         "STORED_TEAM",
		KeyID:          "STORED_KEY",
		PrivateKeyPath: "stored-private-key.pem",
		OrgID:          "ORG",
	}, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	_, err := resolveCredentials(commonFlags{})
	if err == nil || !strings.Contains(err.Error(), "incomplete Apple Ads environment credentials") {
		t.Fatalf("resolveCredentials() error = %v, want incomplete env error", err)
	}
}

func TestCollectQueryIncludesAllowedValidValues(t *testing.T) {
	productPages, _ := appleads.EndpointByCommandPath("product-pages", "list")
	_, flags := bindEndpointFlags(productPages, "test")
	*flags.queryStrings["states"] = "HIDDEN,VISIBLE"
	query, err := collectQuery(productPages, flags)
	if err != nil {
		t.Fatalf("collectQuery() error: %v", err)
	}
	want := url.Values{"states": {"HIDDEN,VISIBLE"}}
	if query.Encode() != want.Encode() {
		t.Fatalf("query = %s, want %s", query.Encode(), want.Encode())
	}
}

func TestEndpointHelpUsesOperatorFriendlyAuthDiscoveryNames(t *testing.T) {
	root := AdsCommand()
	tests := []struct {
		path []string
		want string
	}{
		{path: []string{"me"}, want: "View the current Apple Ads user."},
		{path: []string{"me", "view"}, want: "View the current Apple Ads user."},
		{path: []string{"acls"}, want: "List Apple Ads account ACLs."},
		{path: []string{"acls", "list"}, want: "List Apple Ads account ACLs."},
	}
	for _, test := range tests {
		cmd := findCommand(root, test.path...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(test.path, " "))
		}
		if cmd.ShortHelp != test.want {
			t.Fatalf("asc ads %s ShortHelp = %q, want %q", strings.Join(test.path, " "), cmd.ShortHelp, test.want)
		}
	}
}

func findCommand(root *ffcli.Command, path ...string) *ffcli.Command {
	current := root
	for _, part := range path {
		var next *ffcli.Command
		for _, sub := range current.Subcommands {
			if sub.Name == part {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

func assertSpecFlags(t *testing.T, cmd *ffcli.Command, spec appleads.EndpointSpec) {
	t.Helper()
	for _, name := range []string{"ads-profile", "output"} {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("asc ads %s missing --%s", strings.Join(spec.CommandPath, " "), name)
		}
	}
	if spec.RequiresOrg && cmd.FlagSet.Lookup("org") == nil {
		t.Fatalf("asc ads %s missing --org", strings.Join(spec.CommandPath, " "))
	}
	for _, param := range spec.PathParams {
		if cmd.FlagSet.Lookup(param.Flag) == nil {
			t.Fatalf("asc ads %s missing --%s", strings.Join(spec.CommandPath, " "), param.Flag)
		}
	}
	for _, param := range spec.QueryParams {
		if cmd.FlagSet.Lookup(param.Flag) == nil {
			t.Fatalf("asc ads %s missing --%s", strings.Join(spec.CommandPath, " "), param.Flag)
		}
	}
	if spec.BodyKind != appleads.BodyNone && cmd.FlagSet.Lookup("file") == nil {
		t.Fatalf("asc ads %s missing --file", strings.Join(spec.CommandPath, " "))
	}
	if spec.RequiresConfirm && cmd.FlagSet.Lookup("confirm") == nil {
		t.Fatalf("asc ads %s missing --confirm", strings.Join(spec.CommandPath, " "))
	}
	if spec.SupportsPaginate && cmd.FlagSet.Lookup("paginate") == nil {
		t.Fatalf("asc ads %s missing --paginate", strings.Join(spec.CommandPath, " "))
	}
}
