package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPOfferCodesCodeCreateValidationErrors(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "custom-codes missing offer code id",
			args:    []string{"iap", "offer-codes", "custom-codes", "create", "--custom-code", "SUMMER26", "--quantity", "100"},
			wantErr: "--offer-code-id is required",
		},
		{
			name:    "custom-codes missing custom code",
			args:    []string{"iap", "offer-codes", "custom-codes", "create", "--offer-code-id", "offer-1", "--quantity", "100"},
			wantErr: "--custom-code is required",
		},
		{
			name:    "custom-codes missing quantity",
			args:    []string{"iap", "offer-codes", "custom-codes", "create", "--offer-code-id", "offer-1", "--custom-code", "SUMMER26"},
			wantErr: "--quantity must be a positive integer",
		},
		{
			name:    "custom-codes negative quantity",
			args:    []string{"iap", "offer-codes", "custom-codes", "create", "--offer-code-id", "offer-1", "--custom-code", "SUMMER26", "--quantity", "-5"},
			wantErr: "--quantity must be a positive integer",
		},
		{
			name:    "custom-codes invalid expiration date",
			args:    []string{"iap", "offer-codes", "custom-codes", "create", "--offer-code-id", "offer-1", "--custom-code", "SUMMER26", "--quantity", "100", "--expiration-date", "not-a-date"},
			wantErr: "--expiration-date must be in YYYY-MM-DD format",
		},
		{
			name:    "one-time-codes missing offer code id",
			args:    []string{"iap", "offer-codes", "one-time-codes", "create", "--quantity", "100", "--expiration-date", "2026-12-31"},
			wantErr: "--offer-code-id is required",
		},
		{
			name:    "one-time-codes zero quantity",
			args:    []string{"iap", "offer-codes", "one-time-codes", "create", "--offer-code-id", "offer-1", "--quantity", "0", "--expiration-date", "2026-12-31"},
			wantErr: "--quantity must be a positive integer",
		},
		{
			name:    "one-time-codes negative quantity",
			args:    []string{"iap", "offer-codes", "one-time-codes", "create", "--offer-code-id", "offer-1", "--quantity", "-5", "--expiration-date", "2026-12-31"},
			wantErr: "--quantity must be a positive integer",
		},
		{
			name:    "one-time-codes invalid expiration date",
			args:    []string{"iap", "offer-codes", "one-time-codes", "create", "--offer-code-id", "offer-1", "--quantity", "100", "--expiration-date", "not-a-date"},
			wantErr: "--expiration-date must be in YYYY-MM-DD format",
		},
		{
			name:    "one-time-codes missing expiration date",
			args:    []string{"iap", "offer-codes", "one-time-codes", "create", "--offer-code-id", "offer-1", "--quantity", "100"},
			wantErr: "--expiration-date is required",
		},
		{
			name:    "one-time-codes invalid environment",
			args:    []string{"iap", "offer-codes", "one-time-codes", "create", "--offer-code-id", "offer-1", "--quantity", "100", "--expiration-date", "2026-12-31", "--environment", "beta"},
			wantErr: "--environment must be one of: PRODUCTION, SANDBOX",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitCode(t, test.args, test.wantErr)
		})
	}
}

// --- custom-codes create ---

func TestIAPOfferCodesCustomCodesCreateSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseOfferCodeCustomCodes").JSONBody(`{"data":{"type":"inAppPurchaseOfferCodeCustomCodes","attributes":{"customCode":"SUMMER26","numberOfCodes":100},"relationships":{"offerCode":{"data":{"type":"inAppPurchaseOfferCodes","id":"offer-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeCustomCodes","id":"cc-1","attributes":{"customCode":"SUMMER26","numberOfCodes":100,"active":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "offer-codes", "custom-codes", "create",
		"--offer-code-id", "offer-1",
		"--custom-code", "SUMMER26",
		"--quantity", "100",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				CustomCode    string `json:"customCode"`
				NumberOfCodes int    `json:"numberOfCodes"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "cc-1" {
		t.Fatalf("expected id cc-1, got %q", out.Data.ID)
	}
	if out.Data.Attributes.CustomCode != "SUMMER26" {
		t.Fatalf("expected customCode SUMMER26, got %q", out.Data.Attributes.CustomCode)
	}
}

// --- one-time-codes create ---

func TestIAPOfferCodesOneTimeCodesCreateSandboxEnvironmentSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseOfferCodeOneTimeUseCodes").JSONBody(`{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","attributes":{"numberOfCodes":100,"expirationDate":"2026-12-31","environment":"SANDBOX"},"relationships":{"offerCode":{"data":{"type":"inAppPurchaseOfferCodes","id":"offer-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-2","attributes":{"numberOfCodes":100,"expirationDate":"2026-12-31","active":true,"environment":"SANDBOX"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "offer-codes", "one-time-codes", "create",
		"--offer-code-id", "offer-1",
		"--quantity", "100",
		"--expiration-date", "2026-12-31",
		"--environment", "sandbox",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Data struct {
			Attributes struct {
				Environment string `json:"environment"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.Attributes.Environment != "SANDBOX" {
		t.Fatalf("expected response environment SANDBOX, got %q", out.Data.Attributes.Environment)
	}
}

func TestIAPOfferCodesOneTimeCodesCreateSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseOfferCodeOneTimeUseCodes").JSONBody(`{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","attributes":{"numberOfCodes":500,"expirationDate":"2026-09-30"},"relationships":{"offerCode":{"data":{"type":"inAppPurchaseOfferCodes","id":"offer-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-1","attributes":{"numberOfCodes":500,"expirationDate":"2026-09-30","active":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "offer-codes", "one-time-codes", "create",
		"--offer-code-id", "offer-1",
		"--quantity", "500",
		"--expiration-date", "2026-09-30",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				NumberOfCodes  int    `json:"numberOfCodes"`
				ExpirationDate string `json:"expirationDate"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "otuc-1" {
		t.Fatalf("expected id otuc-1, got %q", out.Data.ID)
	}
	if out.Data.Attributes.NumberOfCodes != 500 {
		t.Fatalf("expected numberOfCodes 500, got %d", out.Data.Attributes.NumberOfCodes)
	}
}
