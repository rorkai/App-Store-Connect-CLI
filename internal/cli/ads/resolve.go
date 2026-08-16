package ads

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

type commonFlags struct {
	AdsProfile *string
	Org        *string
	AdAccount  *string
}

func resolveClient(ctx context.Context, flags commonFlags, requiresOrg bool) (*appleads.Client, error) {
	credentials, err := resolveCredentials(flags)
	if err != nil {
		return nil, err
	}
	if requiresOrg {
		orgID, err := resolveOrgID(flags, credentials)
		if err != nil {
			return nil, err
		}
		if orgID == "" {
			return nil, shared.UsageError("--org is required (or set ASC_ADS_ORG_ID or an Ads profile org_id)")
		}
		credentials.OrgID = orgID
	}
	_ = ctx
	return appleads.NewClient(credentials)
}

func resolvePlatformClientAndAdAccountID(ctx context.Context, flags commonFlags, contextKind appleads.ContextKind) (*appleads.Client, string, error) {
	credentials, err := resolveCredentials(flags)
	if err != nil {
		return nil, "", err
	}
	adAccountID := ""
	if contextKind == appleads.ContextAdAccount || contextKind == appleads.ContextAdAccountOptional {
		adAccountID, err = resolveAdAccountID(flags, credentials)
		if err != nil {
			return nil, "", err
		}
		if adAccountID == "" && contextKind == appleads.ContextAdAccount {
			return nil, "", shared.UsageError("--ad-account is required (or set ASC_ADS_AD_ACCOUNT_ID or an Ads profile ad_account_id)")
		}
		credentials.AdAccountID = adAccountID
	}
	_ = ctx
	client, err := appleads.NewClient(credentials)
	return client, adAccountID, err
}

func resolveCredentials(flags commonFlags) (appleads.Credentials, error) {
	credentials, _, err := resolveCredentialsWithSource(flags)
	return credentials, err
}

func resolveCredentialsWithSource(flags commonFlags) (appleads.Credentials, string, error) {
	profile := strings.TrimSpace(value(flags.AdsProfile))
	profileSource := "--ads-profile"
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("ASC_ADS_PROFILE"))
		profileSource = "ASC_ADS_PROFILE"
	}
	accessToken := strings.TrimSpace(os.Getenv("ASC_ADS_ACCESS_TOKEN"))
	strict := parseBoolEnv("ASC_ADS_STRICT_AUTH")
	if profile != "" {
		if strict && accessToken != "" {
			return appleads.Credentials{}, "", fmt.Errorf("mixed Apple Ads authentication sources detected: profile and ASC_ADS_ACCESS_TOKEN")
		}
		if strict {
			if _, ok, err := envCredentials(); err != nil {
				return appleads.Credentials{}, "", err
			} else if ok {
				return appleads.Credentials{}, "", fmt.Errorf("mixed Apple Ads authentication sources detected: profile and ASC_ADS_* key credentials")
			}
		}
		credentials, _, err := appleads.GetCredentialsWithSource(profile)
		if err != nil {
			return appleads.Credentials{}, "", err
		}
		return credentials, profileSource, nil
	}
	if accessToken != "" {
		if strict {
			if _, ok, err := envCredentials(); err != nil {
				return appleads.Credentials{}, "", err
			} else if ok {
				return appleads.Credentials{}, "", fmt.Errorf("mixed Apple Ads authentication sources detected: ASC_ADS_ACCESS_TOKEN and ASC_ADS_* key credentials")
			}
		}
		return appleads.Credentials{AccessToken: accessToken}, "ASC_ADS_ACCESS_TOKEN", nil
	}

	env, ok, err := envCredentials()
	if err != nil {
		return appleads.Credentials{}, "", err
	}
	if ok {
		return env, "ASC_ADS_* key credentials", nil
	}

	credentials, _, err := appleads.GetCredentialsWithSource("")
	if err != nil {
		if errors.Is(err, appleads.ErrDefaultCredentialsNotFound) || errors.Is(err, config.ErrNotFound) {
			return appleads.Credentials{}, "", fmt.Errorf("%w; %s", err, adsCredentialsRemediation)
		}
		return appleads.Credentials{}, "", err
	}
	return credentials, "default Ads profile", nil
}

// adsCredentialsRemediation tells a first-run caller how to authenticate when
// no Apple Ads credential source is configured.
const adsCredentialsRemediation = "run 'asc ads auth login' to store Apple Ads credentials, set ASC_ADS_* environment credentials, or pass --ads-profile"

func envCredentials() (appleads.Credentials, bool, error) {
	rawOrgID := os.Getenv("ASC_ADS_ORG_ID")
	if err := appleads.ValidateOrgID(rawOrgID); err != nil {
		return appleads.Credentials{}, false, fmt.Errorf("ASC_ADS_ORG_ID: %w", err)
	}
	rawAdAccountID := os.Getenv("ASC_ADS_AD_ACCOUNT_ID")
	if err := appleads.ValidateAdAccountID(rawAdAccountID); err != nil {
		return appleads.Credentials{}, false, fmt.Errorf("ASC_ADS_AD_ACCOUNT_ID: %w", err)
	}
	credentials := appleads.Credentials{
		ClientID:       strings.TrimSpace(os.Getenv("ASC_ADS_CLIENT_ID")),
		TeamID:         strings.TrimSpace(os.Getenv("ASC_ADS_TEAM_ID")),
		KeyID:          strings.TrimSpace(os.Getenv("ASC_ADS_KEY_ID")),
		PrivateKeyPath: strings.TrimSpace(os.Getenv("ASC_ADS_PRIVATE_KEY_PATH")),
		PrivateKeyPEM:  strings.TrimSpace(os.Getenv("ASC_ADS_PRIVATE_KEY")),
		OrgID:          strings.TrimSpace(rawOrgID),
		AdAccountID:    strings.TrimSpace(rawAdAccountID),
	}
	privateKeyB64 := strings.TrimSpace(os.Getenv("ASC_ADS_PRIVATE_KEY_B64"))
	if privateKeyB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(privateKeyB64)
		if err != nil {
			return appleads.Credentials{}, false, fmt.Errorf("ASC_ADS_PRIVATE_KEY_B64 is not valid base64: %w", err)
		}
		if credentials.PrivateKeyPEM == "" {
			credentials.PrivateKeyPEM = string(decoded)
		}
	}
	complete := credentials.ClientID != "" &&
		credentials.TeamID != "" &&
		credentials.KeyID != "" &&
		(credentials.PrivateKeyPath != "" || credentials.PrivateKeyPEM != "")
	keyEnvSet := credentials.ClientID != "" ||
		credentials.TeamID != "" ||
		credentials.KeyID != "" ||
		credentials.PrivateKeyPath != "" ||
		credentials.PrivateKeyPEM != ""
	if !complete && keyEnvSet {
		return appleads.Credentials{}, false, fmt.Errorf("incomplete Apple Ads environment credentials: set ASC_ADS_CLIENT_ID, ASC_ADS_TEAM_ID, ASC_ADS_KEY_ID, and one of ASC_ADS_PRIVATE_KEY_PATH, ASC_ADS_PRIVATE_KEY, or ASC_ADS_PRIVATE_KEY_B64")
	}
	return credentials, complete, nil
}

func resolveAdAccountID(flags commonFlags, credentials appleads.Credentials) (string, error) {
	adAccountID, _, err := resolveAdAccountIDWithSource(flags, credentials)
	return adAccountID, err
}

func resolveAdAccountIDWithSource(flags commonFlags, credentials appleads.Credentials) (string, string, error) {
	if flags.AdAccount != nil {
		adAccountID, source, err := normalizeAdAccountIDWithSource(*flags.AdAccount, "--ad-account")
		if err != nil || adAccountID != "" {
			return adAccountID, source, err
		}
	}
	if adAccountID, source, err := normalizeAdAccountIDWithSource(os.Getenv("ASC_ADS_AD_ACCOUNT_ID"), "ASC_ADS_AD_ACCOUNT_ID"); err != nil || adAccountID != "" {
		return adAccountID, source, err
	}
	profileSource := "credential ad_account_id"
	if strings.TrimSpace(credentials.Profile) != "" {
		profileSource = "Ads profile ad_account_id"
	}
	if adAccountID, source, err := normalizeAdAccountIDWithSource(credentials.AdAccountID, profileSource); err != nil || adAccountID != "" {
		return adAccountID, source, err
	}
	if strings.TrimSpace(credentials.Profile) != "" {
		return "", "", nil
	}
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return "", "", nil
		}
		return "", "", err
	}
	return normalizeAdAccountIDWithSource(cfg.Ads.AdAccountID, "ads.ad_account_id")
}

func normalizeAdAccountIDWithSource(raw, source string) (string, string, error) {
	if err := appleads.ValidateAdAccountID(raw); err != nil {
		return "", "", fmt.Errorf("%s: %w", source, err)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", nil
	}
	return trimmed, source, nil
}

func resolveOrgID(flags commonFlags, credentials appleads.Credentials) (string, error) {
	orgID, _, err := resolveOrgIDWithSource(flags, credentials)
	return orgID, err
}

func resolveOrgIDWithSource(flags commonFlags, credentials appleads.Credentials) (string, string, error) {
	if orgID, source, err := normalizeOrgIDWithSource(value(flags.Org), "--org"); err != nil || orgID != "" {
		return orgID, source, err
	}
	if orgID, source, err := normalizeOrgIDWithSource(os.Getenv("ASC_ADS_ORG_ID"), "ASC_ADS_ORG_ID"); err != nil || orgID != "" {
		return orgID, source, err
	}
	profileSource := "credential org_id"
	if strings.TrimSpace(credentials.Profile) != "" {
		profileSource = "Ads profile org_id"
	}
	if orgID, source, err := normalizeOrgIDWithSource(credentials.OrgID, profileSource); err != nil || orgID != "" {
		if strings.TrimSpace(credentials.Profile) != "" {
			return orgID, source, err
		}
		return orgID, source, err
	}
	if strings.TrimSpace(credentials.Profile) != "" {
		return "", "", nil
	}
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return "", "", nil
		}
		return "", "", err
	}
	return normalizeOrgIDWithSource(cfg.Ads.OrgID, "ads.org_id")
}

func normalizeOrgIDWithSource(raw, source string) (string, string, error) {
	if err := appleads.ValidateOrgID(raw); err != nil {
		return "", "", fmt.Errorf("%s: %w", source, err)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", nil
	}
	return trimmed, source, nil
}

func requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(ctx)
}

func parseBoolEnv(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return strings.TrimSpace(*ptr)
}
