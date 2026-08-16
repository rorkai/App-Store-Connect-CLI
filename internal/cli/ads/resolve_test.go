package ads

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestNamedAdsProfileDoesNotInheritRootOrg(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	setAdsResolverTestEnv(t)
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{
		DefaultKeyName: "profile-a",
		OrgID:          "ORG_A",
		Keys: []config.AdsCredential{
			{Name: "profile-a", ClientID: "A", TeamID: "T", KeyID: "K", PrivateKeyPath: "a.pem", OrgID: "ORG_A"},
			{Name: "profile-b", ClientID: "B", TeamID: "T", KeyID: "K", PrivateKeyPath: "b.pem"},
		},
	}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	profile := "profile-b"
	credentials, err := resolveCredentials(commonFlags{AdsProfile: &profile})
	if err != nil {
		t.Fatalf("resolveCredentials() error: %v", err)
	}
	got, source, err := resolveOrgIDWithSource(commonFlags{}, credentials)
	if err != nil || got != "" || source != "" {
		t.Fatalf("profile-b org = %q source=%q error=%v, want no inherited root org", got, source, err)
	}
}

func TestNamedCredentialsDoNotUseRootOrgFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	setAdsResolverTestEnv(t)
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{OrgID: "ROOT_ORG"}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	got, source, err := resolveOrgIDWithSource(commonFlags{}, appleads.Credentials{Profile: "named-profile"})
	if err != nil || got != "" || source != "" {
		t.Fatalf("named profile org = %q source=%q error=%v, want no inherited root org", got, source, err)
	}
}

func TestProfilelessCredentialsCanUseRootOrgContext(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	setAdsResolverTestEnv(t)
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{OrgID: "ROOT_ORG"}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	got, source, err := resolveOrgIDWithSource(commonFlags{}, appleads.Credentials{AccessToken: "ACCESS"})
	if err != nil || got != "ROOT_ORG" || source != "ads.org_id" {
		t.Fatalf("profileless org = %q source=%q error=%v, want root context", got, source, err)
	}
}

func TestResolveAdAccountIDRejectsUnsafeValuesFromEverySource(t *testing.T) {
	tests := []struct {
		name        string
		adAccount   string
		credentials appleads.Credentials
		configure   func(*testing.T)
	}{
		{
			name:      "flag",
			adAccount: "123;orgId=999",
		},
		{
			name: "environment",
			configure: func(t *testing.T) {
				t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "123\n456")
			},
		},
		{
			name:        "profile",
			credentials: appleads.Credentials{Profile: "named", AdAccountID: "123\t456"},
		},
		{
			name: "config",
			configure: func(t *testing.T) {
				configPath := filepath.Join(t.TempDir(), "config.json")
				t.Setenv("ASC_CONFIG_PATH", configPath)
				if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{AdAccountID: "123\r456"}}); err != nil {
					t.Fatalf("SaveAt() error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setAdsResolverTestEnv(t)
			if tt.configure != nil {
				tt.configure(t)
			}
			flags := commonFlags{}
			if tt.adAccount != "" {
				flags.AdAccount = &tt.adAccount
			}
			_, _, err := resolveAdAccountIDWithSource(flags, tt.credentials)
			if err == nil || !strings.Contains(err.Error(), "invalid ad account ID") {
				t.Fatalf("resolveAdAccountIDWithSource() error = %v, want invalid ad account ID", err)
			}
		})
	}
}

func TestResolveOrgIDRejectsUnsafeValuesFromEverySource(t *testing.T) {
	tests := []struct {
		name        string
		org         string
		credentials appleads.Credentials
		configure   func(*testing.T)
	}{
		{
			name: "flag",
			org:  "123;adAccountId=999",
		},
		{
			name: "environment",
			configure: func(t *testing.T) {
				t.Setenv("ASC_ADS_ORG_ID", "123\n456")
			},
		},
		{
			name:        "profile",
			credentials: appleads.Credentials{Profile: "named", OrgID: "123\t456"},
		},
		{
			name: "config",
			configure: func(t *testing.T) {
				configPath := filepath.Join(t.TempDir(), "config.json")
				t.Setenv("ASC_CONFIG_PATH", configPath)
				if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{OrgID: "123\r456"}}); err != nil {
					t.Fatalf("SaveAt() error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setAdsResolverTestEnv(t)
			if tt.configure != nil {
				tt.configure(t)
			}
			flags := commonFlags{}
			if tt.org != "" {
				flags.Org = &tt.org
			}
			_, _, err := resolveOrgIDWithSource(flags, tt.credentials)
			if err == nil || !strings.Contains(err.Error(), "invalid organization ID") {
				t.Fatalf("resolveOrgIDWithSource() error = %v, want invalid organization ID", err)
			}
		})
	}
}

func TestResolveCredentialsMissingDefaultExplainsRemediation(t *testing.T) {
	tests := []struct {
		name       string
		saveConfig bool
		wantBase   string
		wantNoCred bool
	}{
		{name: "missing config file", saveConfig: false, wantBase: "configuration not found"},
		{name: "config without ads credentials", saveConfig: true, wantBase: "default credentials not found", wantNoCred: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asc.ResetConfigCacheForTest()
			t.Cleanup(asc.ResetConfigCacheForTest)
			configPath := filepath.Join(t.TempDir(), "config.json")
			t.Setenv("ASC_CONFIG_PATH", configPath)
			setAdsResolverTestEnv(t)
			if tt.saveConfig {
				if err := config.SaveAt(configPath, &config.Config{}); err != nil {
					t.Fatalf("SaveAt() error: %v", err)
				}
			}

			_, err := resolveCredentials(commonFlags{})
			if err == nil {
				t.Fatal("resolveCredentials() error = nil, want missing-credentials error")
			}
			if !strings.Contains(err.Error(), tt.wantBase) {
				t.Fatalf("resolveCredentials() error = %v, want %q", err, tt.wantBase)
			}
			for _, want := range []string{"asc ads auth login", "ASC_ADS_", "--ads-profile"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("resolveCredentials() error = %q, want remediation mentioning %q", err.Error(), want)
				}
			}
			if got := isNoAdsCredentialError(err); got != tt.wantNoCred {
				t.Fatalf("isNoAdsCredentialError(%v) = %v, want %v", err, got, tt.wantNoCred)
			}
		})
	}
}

func setAdsResolverTestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ASC_ADS_ACCESS_TOKEN",
		"ASC_ADS_CLIENT_ID",
		"ASC_ADS_TEAM_ID",
		"ASC_ADS_KEY_ID",
		"ASC_ADS_PRIVATE_KEY_PATH",
		"ASC_ADS_PRIVATE_KEY",
		"ASC_ADS_PRIVATE_KEY_B64",
		"ASC_ADS_ORG_ID",
		"ASC_ADS_AD_ACCOUNT_ID",
		"ASC_ADS_PROFILE",
		"ASC_ADS_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
}
