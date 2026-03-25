package sandbox

import (
	"context"
	"fmt"
	"net/mail"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func validateSandboxEmail(value string) error {
	address := strings.TrimSpace(value)
	if address == "" {
		return fmt.Errorf("--email is required")
	}
	if _, err := mail.ParseAddress(address); err != nil {
		return fmt.Errorf("--email must be a valid email address")
	}
	return nil
}

// NormalizeSandboxTerritoryCode validates and normalizes a 3-letter App Store
// territory code used by sandbox tester operations.
func NormalizeSandboxTerritoryCode(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--territory is required")
	}
	upper := strings.ToUpper(trimmed)
	if _, ok := sandboxTerritoryCodes[upper]; !ok {
		return "", fmt.Errorf("--territory must be a valid App Store territory code")
	}
	return upper, nil
}

func normalizeSandboxTerritory(value string) (string, error) {
	return NormalizeSandboxTerritoryCode(value)
}

func normalizeSandboxTerritoryFilter(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return NormalizeSandboxTerritoryCode(value)
}

var sandboxRenewalRates = map[string]asc.SandboxTesterSubscriptionRenewalRate{
	string(asc.SandboxTesterRenewalEveryOneHour):        asc.SandboxTesterRenewalEveryOneHour,
	string(asc.SandboxTesterRenewalEveryThirtyMinutes):  asc.SandboxTesterRenewalEveryThirtyMinutes,
	string(asc.SandboxTesterRenewalEveryFifteenMinutes): asc.SandboxTesterRenewalEveryFifteenMinutes,
	string(asc.SandboxTesterRenewalEveryFiveMinutes):    asc.SandboxTesterRenewalEveryFiveMinutes,
	string(asc.SandboxTesterRenewalEveryThreeMinutes):   asc.SandboxTesterRenewalEveryThreeMinutes,
}

func normalizeSandboxRenewalRate(value string) (asc.SandboxTesterSubscriptionRenewalRate, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	normalized := strings.ToUpper(trimmed)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if rate, ok := sandboxRenewalRates[normalized]; ok {
		return rate, nil
	}
	return "", fmt.Errorf("--subscription-renewal-rate must be one of: %s", strings.Join(sandboxRenewalRateValues(), ", "))
}

func sandboxRenewalRateValues() []string {
	values := make([]string, 0, len(sandboxRenewalRates))
	for key := range sandboxRenewalRates {
		values = append(values, key)
	}
	sort.Strings(values)
	return values
}

func findSandboxTesterByEmail(ctx context.Context, client *asc.Client, email string) (*asc.SandboxTesterResponse, error) {
	next := ""
	for {
		resp, err := client.GetSandboxTesters(ctx,
			asc.WithSandboxTestersEmail(email),
			asc.WithSandboxTestersLimit(200),
			asc.WithSandboxTestersNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		if len(resp.Data) > 1 {
			return nil, fmt.Errorf("multiple sandbox testers found for %q", strings.TrimSpace(email))
		}
		if len(resp.Data) == 1 {
			return &asc.SandboxTesterResponse{Data: resp.Data[0], Links: resp.Links}, nil
		}
		if strings.TrimSpace(resp.Links.Next) == "" {
			break
		}
		if err := shared.ValidateNextURL(resp.Links.Next); err != nil {
			return nil, err
		}
		next = resp.Links.Next
	}
	return nil, fmt.Errorf("no sandbox tester found for %q", strings.TrimSpace(email))
}

func findSandboxTesterIDByEmail(ctx context.Context, client *asc.Client, email string) (string, error) {
	response, err := findSandboxTesterByEmail(ctx, client, email)
	if err != nil {
		return "", err
	}
	return response.Data.ID, nil
}

var sandboxTerritoryCodes = map[string]struct{}{
	"ABW": {},
	"AFG": {},
	"AGO": {},
	"AIA": {},
	"ALB": {},
	"AND": {},
	"ANT": {},
	"ARE": {},
	"ARG": {},
	"ARM": {},
	"ASM": {},
	"ATG": {},
	"AUS": {},
	"AUT": {},
	"AZE": {},
	"BDI": {},
	"BEL": {},
	"BEN": {},
	"BES": {},
	"BFA": {},
	"BGD": {},
	"BGR": {},
	"BHR": {},
	"BHS": {},
	"BIH": {},
	"BLR": {},
	"BLZ": {},
	"BMU": {},
	"BOL": {},
	"BRA": {},
	"BRB": {},
	"BRN": {},
	"BTN": {},
	"BWA": {},
	"CAF": {},
	"CAN": {},
	"CHE": {},
	"CHL": {},
	"CHN": {},
	"CIV": {},
	"CMR": {},
	"COD": {},
	"COG": {},
	"COK": {},
	"COL": {},
	"COM": {},
	"CPV": {},
	"CRI": {},
	"CUB": {},
	"CUW": {},
	"CXR": {},
	"CYM": {},
	"CYP": {},
	"CZE": {},
	"DEU": {},
	"DJI": {},
	"DMA": {},
	"DNK": {},
	"DOM": {},
	"DZA": {},
	"ECU": {},
	"EGY": {},
	"ERI": {},
	"ESP": {},
	"EST": {},
	"ETH": {},
	"FIN": {},
	"FJI": {},
	"FLK": {},
	"FRA": {},
	"FRO": {},
	"FSM": {},
	"GAB": {},
	"GBR": {},
	"GEO": {},
	"GGY": {},
	"GHA": {},
	"GIB": {},
	"GIN": {},
	"GLP": {},
	"GMB": {},
	"GNB": {},
	"GNQ": {},
	"GRC": {},
	"GRD": {},
	"GRL": {},
	"GTM": {},
	"GUF": {},
	"GUM": {},
	"GUY": {},
	"HKG": {},
	"HND": {},
	"HRV": {},
	"HTI": {},
	"HUN": {},
	"IDN": {},
	"IMN": {},
	"IND": {},
	"IRL": {},
	"IRQ": {},
	"ISL": {},
	"ISR": {},
	"ITA": {},
	"JAM": {},
	"JEY": {},
	"JOR": {},
	"JPN": {},
	"KAZ": {},
	"KEN": {},
	"KGZ": {},
	"KHM": {},
	"KIR": {},
	"KNA": {},
	"KOR": {},
	"KWT": {},
	"LAO": {},
	"LBN": {},
	"LBR": {},
	"LBY": {},
	"LCA": {},
	"LIE": {},
	"LKA": {},
	"LSO": {},
	"LTU": {},
	"LUX": {},
	"LVA": {},
	"MAC": {},
	"MAR": {},
	"MCO": {},
	"MDA": {},
	"MDG": {},
	"MDV": {},
	"MEX": {},
	"MHL": {},
	"MKD": {},
	"MLI": {},
	"MLT": {},
	"MMR": {},
	"MNE": {},
	"MNG": {},
	"MNP": {},
	"MOZ": {},
	"MRT": {},
	"MSR": {},
	"MTQ": {},
	"MUS": {},
	"MWI": {},
	"MYS": {},
	"MYT": {},
	"NAM": {},
	"NCL": {},
	"NER": {},
	"NFK": {},
	"NGA": {},
	"NIC": {},
	"NIU": {},
	"NLD": {},
	"NOR": {},
	"NPL": {},
	"NRU": {},
	"NZL": {},
	"OMN": {},
	"PAK": {},
	"PAN": {},
	"PER": {},
	"PHL": {},
	"PLW": {},
	"PNG": {},
	"POL": {},
	"PRI": {},
	"PRT": {},
	"PRY": {},
	"PSE": {},
	"PYF": {},
	"QAT": {},
	"REU": {},
	"ROU": {},
	"RUS": {},
	"RWA": {},
	"SAU": {},
	"SEN": {},
	"SGP": {},
	"SHN": {},
	"SLB": {},
	"SLE": {},
	"SLV": {},
	"SMR": {},
	"SOM": {},
	"SPM": {},
	"SRB": {},
	"SSD": {},
	"STP": {},
	"SUR": {},
	"SVK": {},
	"SVN": {},
	"SWE": {},
	"SWZ": {},
	"SXM": {},
	"SYC": {},
	"TCA": {},
	"TCD": {},
	"TGO": {},
	"THA": {},
	"TJK": {},
	"TKM": {},
	"TLS": {},
	"TON": {},
	"TTO": {},
	"TUN": {},
	"TUR": {},
	"TUV": {},
	"TWN": {},
	"TZA": {},
	"UGA": {},
	"UKR": {},
	"UMI": {},
	"URY": {},
	"USA": {},
	"UZB": {},
	"VAT": {},
	"VCT": {},
	"VEN": {},
	"VGB": {},
	"VIR": {},
	"VNM": {},
	"VUT": {},
	"WLF": {},
	"WSM": {},
	"XKS": {},
	"YEM": {},
	"ZAF": {},
	"ZMB": {},
	"ZWE": {},
}
