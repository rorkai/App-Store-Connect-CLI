package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateSandboxAccountMirrorsWebFlow(t *testing.T) {
	type requestRecord struct {
		Method  string
		Path    string
		Accept  string
		Origin  string
		Referer string
		Body    map[string]string
	}

	records := make([]requestRecord, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		records = append(records, requestRecord{
			Method:  r.Method,
			Path:    r.URL.Path,
			Accept:  r.Header.Get("Accept"),
			Origin:  r.Header.Get("Origin"),
			Referer: r.Header.Get("Referer"),
			Body:    body,
		})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL + "/iris/v1",
	}

	err := client.CreateSandboxAccount(context.Background(), SandboxAccountCreateAttributes{
		FirstName:       "Asc",
		LastName:        "Probe",
		AccountName:     "asc-probe@example.com",
		AccountPassword: "Passwordtest1",
		StoreFront:      "usa",
	})
	if err != nil {
		t.Fatalf("CreateSandboxAccount() error = %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(records))
	}

	if records[0].Method != http.MethodPost || records[0].Path != "/sandbox/v2/account/validateFields" {
		t.Fatalf("unexpected first request: %+v", records[0])
	}
	if got := records[0].Body["acAccountPassword"]; got != "" {
		t.Fatalf("expected first validation request without password, got %q", got)
	}

	if records[1].Method != http.MethodPost || records[1].Path != "/sandbox/v2/account/validateFields" {
		t.Fatalf("unexpected second request: %+v", records[1])
	}
	if got := records[1].Body["acAccountPassword"]; got != "Passwordtest1" {
		t.Fatalf("expected second validation request with password, got %q", got)
	}

	if records[2].Method != http.MethodPost || records[2].Path != "/sandbox/v2/account/create" {
		t.Fatalf("unexpected third request: %+v", records[2])
	}
	if got := records[2].Body["storeFront"]; got != "USA" {
		t.Fatalf("expected create request storefront USA, got %q", got)
	}

	for i, record := range records {
		if record.Accept != "*/*" {
			t.Fatalf("request %d expected Accept */*, got %q", i, record.Accept)
		}
		if record.Origin != server.URL {
			t.Fatalf("request %d expected Origin %q, got %q", i, server.URL, record.Origin)
		}
		if record.Referer != server.URL+"/access/users/sandbox" {
			t.Fatalf("request %d expected Referer %q, got %q", i, server.URL+"/access/users/sandbox", record.Referer)
		}
	}
}

func TestCreateSandboxAccountRejectsMissingFields(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "https://example.test/iris/v1"}

	err := client.CreateSandboxAccount(context.Background(), SandboxAccountCreateAttributes{
		FirstName:       "Asc",
		LastName:        "",
		AccountName:     "not-an-email",
		AccountPassword: "",
		StoreFront:      "",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "last name is required") {
		t.Fatalf("expected last-name validation error, got %v", err)
	}
}

func TestCreateSandboxAccountRejectsDisplayNameEmail(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "https://example.test/iris/v1"}

	err := client.CreateSandboxAccount(context.Background(), SandboxAccountCreateAttributes{
		FirstName:       "Asc",
		LastName:        "Probe",
		AccountName:     "Asc Probe <asc-probe@example.com>",
		AccountPassword: "Passwordtest1",
		StoreFront:      "USA",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "account name must be a valid email address") {
		t.Fatalf("expected account-name validation error, got %v", err)
	}
}
