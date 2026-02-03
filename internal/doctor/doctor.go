package doctor

import (
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// Check is a single diagnostic check.
// OK indicates pass/fail.
type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Report is the full doctor output.
type Report struct {
	ConfigPath string  `json:"config_path"`
	Profile    string  `json:"profile"`
	OK         bool    `json:"ok"`
	Checks     []Check `json:"checks"`
}

// ResolveProfile trims and prefers the flag profile over config default.
func ResolveProfile(flagProfile, cfgDefault string) string {
	flagProfile = strings.TrimSpace(flagProfile)
	if flagProfile != "" {
		return flagProfile
	}
	return strings.TrimSpace(cfgDefault)
}

// SelectCredential returns the credential fields for a given profile.
// If profile is non-empty, it first searches cfg.Keys for a matching name.
// If no match is found (or profile is empty), it falls back to top-level cfg fields
// if any are present.
func SelectCredential(cfg *config.Config, profile string) (keyID, issuerID, privateKeyPath string, found bool) {
	if cfg == nil {
		return "", "", "", false
	}

	profile = strings.TrimSpace(profile)
	if profile != "" {
		for _, cred := range cfg.Keys {
			if strings.TrimSpace(cred.Name) == profile {
				return strings.TrimSpace(cred.KeyID), strings.TrimSpace(cred.IssuerID), strings.TrimSpace(cred.PrivateKeyPath), true
			}
		}
	}

	keyID = strings.TrimSpace(cfg.KeyID)
	issuerID = strings.TrimSpace(cfg.IssuerID)
	privateKeyPath = strings.TrimSpace(cfg.PrivateKeyPath)
	if keyID != "" || issuerID != "" || privateKeyPath != "" {
		return keyID, issuerID, privateKeyPath, true
	}
	return "", "", "", false
}

// BuildReport builds a structured report including pass/fail checks.
func BuildReport(cfgPath string, cfg *config.Config, cfgLoadErr error, profile string) Report {
	report := Report{ConfigPath: cfgPath, Profile: profile}
	checks := make([]Check, 0, 9)

	// config.path
	cfgPathTrim := strings.TrimSpace(cfgPath)
	checks = append(checks, Check{Name: "config.path", OK: cfgPathTrim != "", Message: cfgPathTrim})

	// config.exists
	cfgExistsOK := false
	cfgExistsMsg := ""
	if cfgPathTrim == "" {
		cfgExistsMsg = "no config path"
	} else {
		if _, err := os.Stat(cfgPathTrim); err == nil {
			cfgExistsOK = true
			cfgExistsMsg = "found"
		} else {
			cfgExistsMsg = err.Error()
		}
	}
	checks = append(checks, Check{Name: "config.exists", OK: cfgExistsOK, Message: cfgExistsMsg})

	// config.load
	loadOK := cfgLoadErr == nil
	loadMsg := "loaded"
	if !loadOK {
		loadMsg = cfgLoadErr.Error()
	}
	checks = append(checks, Check{Name: "config.load", OK: loadOK, Message: loadMsg})

	// profile.selected
	profileMsg := strings.TrimSpace(profile)
	if profileMsg == "" {
		profileMsg = "(default)"
	}
	checks = append(checks, Check{Name: "profile.selected", OK: true, Message: profileMsg})

	keyID, issuerID, privateKeyPath, credFound := SelectCredential(cfg, profile)

	// credentials.present
	credMsg := "found"
	if !credFound {
		credMsg = "no credential found in config"
	}
	checks = append(checks, Check{Name: "credentials.present", OK: credFound, Message: credMsg})

	// key_id.present
	checks = append(checks, Check{Name: "key_id.present", OK: strings.TrimSpace(keyID) != "", Message: maskEmpty(keyID)})

	// issuer_id.present
	checks = append(checks, Check{Name: "issuer_id.present", OK: strings.TrimSpace(issuerID) != "", Message: maskEmpty(issuerID)})

	// private_key_path.present
	checks = append(checks, Check{Name: "private_key_path.present", OK: strings.TrimSpace(privateKeyPath) != "", Message: maskEmpty(privateKeyPath)})

	// private_key_path.readable
	readOK := false
	readMsg := ""
	if strings.TrimSpace(privateKeyPath) == "" {
		readMsg = "no private_key_path"
	} else {
		f, err := os.Open(privateKeyPath)
		if err != nil {
			readMsg = err.Error()
		} else {
			_ = f.Close()
			readOK = true
			readMsg = "readable"
		}
	}
	checks = append(checks, Check{Name: "private_key_path.readable", OK: readOK, Message: readMsg})

	report.Checks = checks
	report.OK = true
	for _, c := range checks {
		if !c.OK {
			report.OK = false
			break
		}
	}
	return report
}

func maskEmpty(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "(empty)"
	}
	return v
}
