package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// --- custom-codes create ---

func TestIAPOfferCodesCustomCodesCreateMissingOfferCodeID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--custom-code", "SUMMER26",
			"--quantity", "100",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --offer-code-id is required") {
		t.Fatalf("expected stderr to contain --offer-code-id required, got %q", stderr)
	}
}

func TestIAPOfferCodesCustomCodesCreateMissingCustomCode(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "100",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --custom-code is required") {
		t.Fatalf("expected stderr to contain --custom-code required, got %q", stderr)
	}
}

func TestIAPOfferCodesCustomCodesCreateMissingQuantity(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--offer-code-id", "offer-1",
			"--custom-code", "SUMMER26",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --quantity must be a positive integer") {
		t.Fatalf("expected stderr to contain --quantity error, got %q", stderr)
	}
}

func TestIAPOfferCodesCustomCodesCreateInvalidQuantity(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--offer-code-id", "offer-1",
			"--custom-code", "SUMMER26",
			"--quantity", "-5",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --quantity must be a positive integer") {
		t.Fatalf("expected stderr to contain --quantity error, got %q", stderr)
	}
}

func TestIAPOfferCodesCustomCodesCreateInvalidExpirationDate(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--offer-code-id", "offer-1",
			"--custom-code", "SUMMER26",
			"--quantity", "100",
			"--expiration-date", "not-a-date",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --expiration-date must be in YYYY-MM-DD format") {
		t.Fatalf("expected stderr to contain expiration-date format error, got %q", stderr)
	}
}

func TestIAPOfferCodesCustomCodesCreateSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodeCustomCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodeCustomCodes, got %s", req.URL.Path)
		}

		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}

		data := payload["data"].(map[string]any)
		if data["type"] != "inAppPurchaseOfferCodeCustomCodes" {
			t.Fatalf("expected type inAppPurchaseOfferCodeCustomCodes, got %v", data["type"])
		}
		attrs := data["attributes"].(map[string]any)
		if attrs["customCode"] != "SUMMER26" {
			t.Fatalf("expected customCode SUMMER26, got %v", attrs["customCode"])
		}
		if attrs["numberOfCodes"].(float64) != 100 {
			t.Fatalf("expected numberOfCodes 100, got %v", attrs["numberOfCodes"])
		}

		relationships := data["relationships"].(map[string]any)
		offerCode := relationships["offerCode"].(map[string]any)["data"].(map[string]any)
		if offerCode["type"] != "inAppPurchaseOfferCodes" {
			t.Fatalf("expected relationship type inAppPurchaseOfferCodes, got %v", offerCode["type"])
		}
		if offerCode["id"] != "offer-1" {
			t.Fatalf("expected offerCode id offer-1, got %v", offerCode["id"])
		}

		body := `{"data":{"type":"inAppPurchaseOfferCodeCustomCodes","id":"cc-1","attributes":{"customCode":"SUMMER26","numberOfCodes":100,"active":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--offer-code-id", "offer-1",
			"--custom-code", "SUMMER26",
			"--quantity", "100",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestIAPOfferCodesCustomCodesCreatePrettyJSON(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":{"type":"inAppPurchaseOfferCodeCustomCodes","id":"cc-1","attributes":{"customCode":"SUMMER26","numberOfCodes":100}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "custom-codes", "create",
			"--offer-code-id", "offer-1",
			"--custom-code", "SUMMER26",
			"--quantity", "100",
			"--pretty",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	// Pretty-printed JSON should contain indentation
	if !strings.Contains(stdout, "\n") {
		t.Fatalf("expected pretty-printed JSON with newlines, got %q", stdout)
	}
	// Verify it parses as valid JSON
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nstdout: %s", err, stdout)
	}
}

// --- one-time-codes create ---

func TestIAPOfferCodesOneTimeCodesCreateMissingOfferCodeID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--quantity", "100",
			"--expiration-date", "2026-12-31",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --offer-code-id is required") {
		t.Fatalf("expected stderr to contain --offer-code-id required, got %q", stderr)
	}
}

func TestIAPOfferCodesOneTimeCodesCreateInvalidQuantity(t *testing.T) {
	tests := []struct {
		name     string
		quantity string
	}{
		{name: "zero", quantity: "0"},
		{name: "negative", quantity: "-5"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"iap", "offer-codes", "one-time-codes", "create",
					"--offer-code-id", "offer-1",
					"--quantity", test.quantity,
					"--expiration-date", "2026-12-31",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --quantity must be a positive integer") {
				t.Fatalf("expected stderr to contain --quantity error, got %q", stderr)
			}
		})
	}
}

func TestIAPOfferCodesOneTimeCodesCreateInvalidExpirationDate(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "100",
			"--expiration-date", "not-a-date",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --expiration-date must be in YYYY-MM-DD format") {
		t.Fatalf("expected stderr to contain expiration-date format error, got %q", stderr)
	}
}

func TestIAPOfferCodesOneTimeCodesCreateMissingExpirationDate(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "100",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --expiration-date is required") {
		t.Fatalf("expected stderr to contain expiration-date required error, got %q", stderr)
	}
}

func TestIAPOfferCodesOneTimeCodesCreateInvalidEnvironment(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "100",
			"--expiration-date", "2026-12-31",
			"--environment", "beta",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --environment must be one of: PRODUCTION, SANDBOX") {
		t.Fatalf("expected stderr to contain environment validation error, got %q", stderr)
	}
}

func TestIAPOfferCodesOneTimeCodesCreateSandboxEnvironmentSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}

		data := payload["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)
		if attrs["environment"] != "SANDBOX" {
			t.Fatalf("expected environment SANDBOX, got %v", attrs["environment"])
		}

		body := `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-2","attributes":{"numberOfCodes":100,"expirationDate":"2026-12-31","active":true,"environment":"SANDBOX"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "100",
			"--expiration-date", "2026-12-31",
			"--environment", "sandbox",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestIAPOfferCodesOneTimeCodesCreateHelpShowsEnvironmentFlag(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "iap", "offer-codes", "one-time-codes", "create")
	if cmd == nil {
		t.Fatal("expected iap offer-codes one-time-codes create command")
	}

	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "--environment") {
		t.Fatalf("expected usage to mention --environment, got %q", usage)
	}
	if !strings.Contains(usage, "SANDBOX") {
		t.Fatalf("expected usage to mention SANDBOX, got %q", usage)
	}
}

func TestIAPOfferCodesOneTimeCodesCreateSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodeOneTimeUseCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodeOneTimeUseCodes, got %s", req.URL.Path)
		}

		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}

		data := payload["data"].(map[string]any)
		if data["type"] != "inAppPurchaseOfferCodeOneTimeUseCodes" {
			t.Fatalf("expected type inAppPurchaseOfferCodeOneTimeUseCodes, got %v", data["type"])
		}
		attrs := data["attributes"].(map[string]any)
		if attrs["numberOfCodes"].(float64) != 500 {
			t.Fatalf("expected numberOfCodes 500, got %v", attrs["numberOfCodes"])
		}
		if attrs["expirationDate"] != "2026-09-30" {
			t.Fatalf("expected expirationDate 2026-09-30, got %v", attrs["expirationDate"])
		}

		relationships := data["relationships"].(map[string]any)
		offerCode := relationships["offerCode"].(map[string]any)["data"].(map[string]any)
		if offerCode["type"] != "inAppPurchaseOfferCodes" {
			t.Fatalf("expected relationship type inAppPurchaseOfferCodes, got %v", offerCode["type"])
		}
		if offerCode["id"] != "offer-1" {
			t.Fatalf("expected offerCode id offer-1, got %v", offerCode["id"])
		}

		body := `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-1","attributes":{"numberOfCodes":500,"expirationDate":"2026-09-30","active":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "500",
			"--expiration-date", "2026-09-30",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestIAPOfferCodesOneTimeCodesCreatePrettyJSON(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-1","attributes":{"numberOfCodes":100,"expirationDate":"2026-12-31"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "one-time-codes", "create",
			"--offer-code-id", "offer-1",
			"--quantity", "100",
			"--expiration-date", "2026-12-31",
			"--pretty",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	// Pretty-printed JSON should contain indentation
	if !strings.Contains(stdout, "\n") {
		t.Fatalf("expected pretty-printed JSON with newlines, got %q", stdout)
	}
	// Verify it parses as valid JSON
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nstdout: %s", err, stdout)
	}
}
