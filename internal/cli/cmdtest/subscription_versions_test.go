package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	subscriptionscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
)

func TestSubscriptionVersionsListJSON(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/123456789/versions" {
			reportSubscriptionVersionHandlerError(t, w, "unexpected request: %s %s", req.Method, req.URL.Path)
			return
		}
		wantQuery := url.Values{
			"fields[subscriptionImages]":        {"fileName,fileSize"},
			"fields[subscriptionLocalizations]": {"name,locale"},
			"fields[subscriptions]":             {"name,productId"},
			"fields[subscriptionVersions]":      {"version,state"},
			"filter[state]":                     {"PREPARE_FOR_SUBMISSION"},
			"include":                           {"localizations,images"},
			"limit":                             {"7"},
			"limit[images]":                     {"5"},
			"limit[localizations]":              {"6"},
		}.Encode()
		if got := req.URL.Query().Encode(); got != wantQuery {
			reportSubscriptionVersionHandlerError(t, w, "query = %q, want %q", got, wantQuery)
			return
		}
		writeSubscriptionVersionJSON(w, http.StatusOK, `{"data":[{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1,"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "versions", "list", "--subscription-id", "123456789",
			"--state", "PREPARE_FOR_SUBMISSION", "--fields", "version,state",
			"--subscription-fields", "name,productId", "--image-fields", "fileName,fileSize",
			"--localization-fields", "name,locale", "--include", "localizations,images",
			"--limit", "7", "--image-limit", "5", "--localization-limit", "6", "--output", "json",
		}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	wantStderr := "Warning: `--image-limit` is deprecated. Use `--images-limit`.\n" +
		"Warning: `--localization-limit` is deprecated. Use `--localizations-limit`.\n"
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "ver-1" {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestSubscriptionVersionsValidationUsesUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "missing version ID", args: []string{"subscriptions", "versions", "view"}, message: "Error: --id is required"},
		{name: "invalid state", args: []string{"subscriptions", "versions", "list", "--subscription-id", "123456789", "--state", "UNKNOWN"}, message: "invalid --state"},
		{name: "invalid relationship limit", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--images-limit", "51"}, message: "--images-limit must be between 1 and 50"},
		{name: "next query conflict", args: []string{"subscriptions", "versions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/versions?cursor=next", "--state", "APPROVED"}, message: "--next cannot be combined with --state"},
		{name: "delete confirm", args: []string{"subscriptions", "versions", "images", "delete", "--id", "img-1"}, message: "Error: --confirm is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q", stdout)
			}
			if !strings.Contains(stderr, test.message) {
				t.Fatalf("stderr = %q, want %q", stderr, test.message)
			}
		})
	}
}

func TestSubscriptionsListRejectsVersionQueryFlagsForResolvedApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "6759231657")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "list", "--include", "versions"}); err != nil {
			t.Fatal(err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "require --group-id") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSubscriptionsListRejectsOpaqueNextOwnerAndLimitBeforeClient(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=next"
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "app owner", args: []string{"--app", "app-2"}, message: "--next cannot be combined with --app"},
		{name: "explicit empty app owner", args: []string{"--app", ""}, message: "--next cannot be combined with --app"},
		{name: "whitespace app owner", args: []string{"--app", "  "}, message: "--next cannot be combined with --app"},
		{name: "group owner", args: []string{"--group-id", "group-2"}, message: "--next cannot be combined with --group-id"},
		{name: "limit", args: []string{"--limit", "7"}, message: "--next cannot be combined with --limit"},
		{name: "explicit zero limit", args: []string{"--limit", "0"}, message: "--next cannot be combined with --limit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"subscriptions", "list", "--next", next}, test.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q", stdout)
			}
			if !strings.Contains(stderr, test.message) {
				t.Fatalf("stderr = %q, want %q", stderr, test.message)
			}
			if clientFactoryCalled {
				t.Fatal("client factory ran before opaque next validation")
			}
		})
	}
}

func TestSubscriptionVersionReadSelectionValidationBeforeAuth(t *testing.T) {
	clearSubscriptionVersionAuth(t)
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "version list fields", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--app", "6759231657", "--fields", "bogus"}, message: "--fields must be one of"},
		{name: "version list subscription fields", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--app", "6759231657", "--subscription-fields", "bogus"}, message: "--subscription-fields must be one of"},
		{name: "version list image fields", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--app", "6759231657", "--image-fields", "state"}, message: "--image-fields must be one of"},
		{name: "version list localization fields", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--app", "6759231657", "--localization-fields", "state"}, message: "--localization-fields must be one of"},
		{name: "version list include", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--app", "6759231657", "--include", "version"}, message: "--include must be one of"},
		{name: "version list explicitly empty fields", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--fields", ""}, message: "--fields must not be empty"},
		{name: "version list explicitly empty state", args: []string{"subscriptions", "versions", "list", "--subscription-id", "Monthly", "--state", ""}, message: "--state must not be empty"},
		{name: "version detail fields", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--fields", "name"}, message: "--fields must be one of"},
		{name: "version detail subscription fields", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--subscription-fields", "version"}, message: "--subscription-fields must be one of"},
		{name: "version detail image fields", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--image-fields", "state"}, message: "--image-fields must be one of"},
		{name: "version detail localization fields", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--localization-fields", "state"}, message: "--localization-fields must be one of"},
		{name: "version detail include", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--include", "version"}, message: "--include must be one of"},
		{name: "localization list fields", args: []string{"subscriptions", "versions", "localizations", "list", "--version-id", "ver-1", "--fields", "state"}, message: "--fields must be one of"},
		{name: "localization list version fields", args: []string{"subscriptions", "versions", "localizations", "list", "--version-id", "ver-1", "--version-fields", "name"}, message: "--version-fields must be one of"},
		{name: "localization list include", args: []string{"subscriptions", "versions", "localizations", "list", "--version-id", "ver-1", "--include", "localizations"}, message: "--include must be one of"},
		{name: "localization detail fields", args: []string{"subscriptions", "versions", "localizations", "view", "--id", "loc-1", "--fields", "state"}, message: "--fields must be one of"},
		{name: "localization detail version fields", args: []string{"subscriptions", "versions", "localizations", "view", "--id", "loc-1", "--version-fields", "name"}, message: "--version-fields must be one of"},
		{name: "localization detail include", args: []string{"subscriptions", "versions", "localizations", "view", "--id", "loc-1", "--include", "localizations"}, message: "--include must be one of"},
		{name: "image list fields", args: []string{"subscriptions", "versions", "images", "list", "--version-id", "ver-1", "--fields", "state"}, message: "--fields must be one of"},
		{name: "image primary fields", args: []string{"subscriptions", "versions", "images", "primary", "--version-id", "ver-1", "--fields", "state"}, message: "--fields must be one of"},
		{name: "image detail fields", args: []string{"subscriptions", "versions", "images", "view", "--id", "img-1", "--fields", "state"}, message: "--fields must be one of"},
		{name: "image detail explicitly empty fields", args: []string{"subscriptions", "versions", "images", "view", "--id", "img-1", "--fields", ""}, message: "--fields must not be empty"},
		{name: "subscription list fields", args: []string{"subscriptions", "list", "--group-id", "group-1", "--fields", "version"}, message: "--fields must be one of"},
		{name: "subscription list version fields", args: []string{"subscriptions", "list", "--group-id", "group-1", "--version-fields", "name"}, message: "--version-fields must be one of"},
		{name: "subscription list include", args: []string{"subscriptions", "list", "--group-id", "group-1", "--include", "subscription"}, message: "--include must be one of"},
		{name: "subscription list next conflict", args: []string{"subscriptions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=next", "--include", "versions"}, message: "--next cannot be combined with --include"},
		{name: "subscription detail fields", args: []string{"subscriptions", "view", "--id", "sub-1", "--fields", "version"}, message: "--fields must be one of"},
		{name: "subscription detail version fields", args: []string{"subscriptions", "view", "--id", "sub-1", "--version-fields", "name"}, message: "--version-fields must be one of"},
		{name: "subscription detail include", args: []string{"subscriptions", "view", "--id", "sub-1", "--include", "subscription"}, message: "--include must be one of"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q", stdout)
			}
			if !strings.Contains(stderr, test.message) {
				t.Fatalf("stderr = %q, want %q", stderr, test.message)
			}
			if strings.Contains(stderr, "authentication") || strings.Contains(stderr, "credentials") {
				t.Fatalf("selection validation ran after auth: %q", stderr)
			}
		})
	}
}

func TestSubscriptionVersionCommandsRejectUnexpectedArgsBeforeAuth(t *testing.T) {
	clearSubscriptionVersionAuth(t)
	tests := [][]string{
		{"subscriptions", "versions", "create", "unexpected"},
		{"subscriptions", "versions", "list", "unexpected"},
		{"subscriptions", "versions", "view", "unexpected"},
		{"subscriptions", "versions", "links", "unexpected"},
		{"subscriptions", "versions", "localizations", "list", "unexpected"},
		{"subscriptions", "versions", "localizations", "links", "unexpected"},
		{"subscriptions", "versions", "localizations", "view", "unexpected"},
		{"subscriptions", "versions", "localizations", "create", "unexpected"},
		{"subscriptions", "versions", "localizations", "update", "unexpected"},
		{"subscriptions", "versions", "localizations", "delete", "unexpected"},
		{"subscriptions", "versions", "images", "list", "unexpected"},
		{"subscriptions", "versions", "images", "primary", "unexpected"},
		{"subscriptions", "versions", "images", "links", "unexpected"},
		{"subscriptions", "versions", "images", "primary-link", "unexpected"},
		{"subscriptions", "versions", "images", "view", "unexpected"},
		{"subscriptions", "versions", "images", "upload", "unexpected"},
		{"subscriptions", "versions", "images", "update", "unexpected"},
		{"subscriptions", "versions", "images", "delete", "unexpected"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[2:], "_"), func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if !strings.Contains(stderr, "unexpected argument(s): unexpected") {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestSubscriptionVersionImageUploadLifecycle(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	imagePath := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(imagePath, []byte("test-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	step := 0
	httpClient := useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		step++
		switch step {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/subscriptionImages" {
				reportSubscriptionVersionHandlerError(t, w, "reservation request = %s %s", req.Method, req.URL.Path)
				return
			}
			writeSubscriptionVersionJSON(w, http.StatusCreated, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":10,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","offset":0,"length":10}]}}}`)
		case 2:
			if req.Method != http.MethodPut || req.Header.Get("X-Test-Original-Host") != "upload.example.com" || req.ContentLength != 10 {
				reportSubscriptionVersionHandlerError(t, w, "upload request = %s %s length=%d", req.Method, req.URL.String(), req.ContentLength)
				return
			}
			w.WriteHeader(http.StatusOK)
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v2/subscriptionImages/img-1" {
				reportSubscriptionVersionHandlerError(t, w, "commit request = %s %s", req.Method, req.URL.Path)
				return
			}
			var payload asc.SubscriptionImageV2UpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				reportSubscriptionVersionHandlerError(t, w, "decode commit body: %v", err)
				return
			}
			if payload.Data.ID != "img-1" || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
				reportSubscriptionVersionHandlerError(t, w, "commit payload = %+v", payload)
				return
			}
			writeSubscriptionVersionJSON(w, http.StatusOK, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","assetDeliveryState":{"state":"AWAITING_UPLOAD"}}}}`)
		default:
			reportSubscriptionVersionHandlerError(t, w, "unexpected request %d: %s %s", step, req.Method, req.URL.String())
		}
	}))
	restoreUploader := subscriptionscli.SetSubscriptionVersionImageUploaderForTesting(func(ctx context.Context, file *os.File, _ int64, operations []asc.UploadOperation) error {
		return asc.ExecuteUploadOperations(ctx, file.Name(), operations, asc.WithUploadHTTPClient(httpClient), asc.WithUploadConcurrency(1))
	})
	t.Cleanup(restoreUploader)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "versions", "images", "upload", "--version-id", "ver-1", "--file", imagePath, "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if step != 3 {
		t.Fatalf("upload lifecycle performed %d steps, want 3", step)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var payload asc.SubscriptionImageV2Response
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("invalid output JSON: %v\n%s", err, stdout)
	}
	if payload.Data.ID != "img-1" {
		t.Fatalf("output ID = %q", payload.Data.ID)
	}
}

func TestSubscriptionVersionImageUploadErrorsIncludeReservedID(t *testing.T) {
	tests := []struct {
		name       string
		failCommit bool
		noOps      bool
		message    string
	}{
		{name: "missing upload operations", noOps: true, message: "reserved image img-reserved returned no upload operations"},
		{name: "upload failure", message: "upload failed for reserved image img-reserved"},
		{name: "commit failure", failCommit: true, message: "failed to commit reserved image img-reserved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			imagePath := filepath.Join(t.TempDir(), "image.png")
			if err := os.WriteFile(imagePath, []byte("test-image"), 0o600); err != nil {
				t.Fatal(err)
			}
			step := 0
			httpClient := useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				step++
				switch step {
				case 1:
					if test.noOps {
						writeSubscriptionVersionJSON(w, http.StatusCreated, `{"data":{"type":"subscriptionImages","id":"img-reserved","attributes":{"fileName":"image.png","fileSize":10}}}`)
						return
					}
					writeSubscriptionVersionJSON(w, http.StatusCreated, `{"data":{"type":"subscriptionImages","id":"img-reserved","attributes":{"fileName":"image.png","fileSize":10,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","offset":0,"length":10}]}}}`)
				case 2:
					if test.failCommit {
						w.WriteHeader(http.StatusOK)
						return
					}
					w.WriteHeader(http.StatusInternalServerError)
				case 3:
					writeSubscriptionVersionJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","title":"Commit failed"}]}`)
				default:
					reportSubscriptionVersionHandlerError(t, w, "unexpected request %d: %s %s", step, req.Method, req.URL.Path)
				}
			}))
			restoreUploader := subscriptionscli.SetSubscriptionVersionImageUploaderForTesting(func(ctx context.Context, file *os.File, _ int64, operations []asc.UploadOperation) error {
				return asc.ExecuteUploadOperations(ctx, file.Name(), operations, asc.WithUploadHTTPClient(httpClient), asc.WithUploadConcurrency(1))
			})
			t.Cleanup(restoreUploader)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			captureOutput(t, func() {
				if err := root.Parse([]string{"subscriptions", "versions", "images", "upload", "--version-id", "ver-1", "--file", imagePath}); err != nil {
					t.Fatal(err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr == nil || !strings.Contains(runErr.Error(), test.message) {
				t.Fatalf("error = %v, want %q", runErr, test.message)
			}
		})
	}
}

func TestSubscriptionVersionLocalizationUpdateNullableFields(t *testing.T) {
	setupAuth(t)
	useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v2/subscriptionLocalizations/loc-1" {
			reportSubscriptionVersionHandlerError(t, w, "unexpected request: %s %s", req.Method, req.URL.Path)
			return
		}
		var payload struct {
			Data struct {
				Attributes map[string]json.RawMessage `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			reportSubscriptionVersionHandlerError(t, w, "decode update body: %v", err)
			return
		}
		if got := string(payload.Data.Attributes["name"]); got != "null" {
			reportSubscriptionVersionHandlerError(t, w, "name JSON = %s, want null", got)
			return
		}
		if got := string(payload.Data.Attributes["description"]); got != `""` {
			reportSubscriptionVersionHandlerError(t, w, "description JSON = %s, want empty string", got)
			return
		}
		writeSubscriptionVersionJSON(w, http.StatusOK, `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":""}}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "versions", "localizations", "update",
			"--id", "loc-1", "--clear-name", "--description", "", "--output", "json",
		}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var response asc.SubscriptionLocalizationV2Response
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if response.Data.ID != "loc-1" {
		t.Fatalf("output ID = %q", response.Data.ID)
	}
}

func TestSubscriptionVersionLocalizationUpdateRejectsSetAndClear(t *testing.T) {
	clearSubscriptionVersionAuth(t)
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "name", args: []string{"--name", "Pro", "--clear-name"}, message: "--name cannot be used with --clear-name"},
		{name: "description", args: []string{"--description", "Features", "--clear-description"}, message: "--description cannot be used with --clear-description"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := []string{"subscriptions", "versions", "localizations", "update", "--id", "loc-1"}
			args = append(args, test.args...)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if !strings.Contains(stderr, test.message) {
				t.Fatalf("stderr = %q, want %q", stderr, test.message)
			}
		})
	}
}

func clearSubscriptionVersionAuth(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY",
		"ASC_PRIVATE_KEY_B64", "ASC_PROFILE", "ASC_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
}

func useSubscriptionVersionServer(t *testing.T, handler http.Handler) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	targetHost := strings.TrimPrefix(server.URL, "http://")
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = "http"
		clone.URL.Host = targetHost
		clone.Host = targetHost
		clone.Header.Set("X-Test-Original-Host", req.URL.Host)
		return server.Client().Transport.RoundTrip(clone)
	})
	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH"),
		httpClient,
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restore)
	return httpClient
}

func reportSubscriptionVersionHandlerError(t *testing.T, w http.ResponseWriter, format string, args ...any) {
	t.Helper()
	t.Errorf(format, args...)
	http.Error(w, "test handler assertion failed", http.StatusInternalServerError)
}

func writeSubscriptionVersionJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
