package sandbox

import (
	"context"
	"fmt"
	"net/mail"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
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
	normalized, err := ascterritory.Normalize(trimmed)
	if err != nil {
		return "", fmt.Errorf("--territory must be a valid App Store territory code")
	}
	return normalized, nil
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
		resp, err := client.GetSandboxTesters(
			ctx,
			asc.WithSandboxTestersEmail(email),
			asc.WithSandboxTestersLimit(200),
			asc.WithSandboxTestersNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		if len(resp.Data) > 1 {
			candidates := make([]shared.AmbiguousCandidate, 0, len(resp.Data))
			for _, tester := range resp.Data {
				candidates = append(candidates, shared.AmbiguousCandidate{
					ID:    strings.TrimSpace(tester.ID),
					Label: strings.TrimSpace(tester.Attributes.Email),
					Extra: strings.TrimSpace(strings.TrimSpace(tester.Attributes.FirstName) + " " + strings.TrimSpace(tester.Attributes.LastName)),
				})
			}
			return nil, &shared.AmbiguousSelectionError{
				Kind:        "sandbox tester",
				Description: fmt.Sprintf("email %q", strings.TrimSpace(email)),
				Flag:        "--id",
				Candidates:  candidates,
			}
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
