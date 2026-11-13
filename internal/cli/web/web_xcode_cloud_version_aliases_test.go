package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebVersionAliasesHierarchy(t *testing.T) {
	settings := findSub(WebXcodeCloudCommand(), "settings")
	if settings == nil {
		t.Fatal("expected settings subcommand")
	}
	group := findSub(settings, "version-aliases")
	if group == nil {
		t.Fatal("expected version-aliases subcommand")
	}
	for _, name := range []string{"list", "view", "create", "update", "delete"} {
		command := findSub(group, name)
		if command == nil || command.UsageFunc == nil {
			t.Fatalf("expected %q subcommand with UsageFunc", name)
		}
	}
}

func TestWebVersionAliasesListJSONOmitsNestedPayloads(t *testing.T) {
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/ci/api/teams/team-uuid/products/product-uuid/configuration-options/version-aliases-v3" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("limit"); got != "100" {
			t.Fatalf("limit = %q, want 100", got)
		}
		body := `{"items":[{"id":"alias-1","name":"Release","type":"CUSTOM","locked":true,"build":{"signed_url":"https://example.invalid/?token=secret"},"build_name":"42","related_workflow_summaries":[{"id":"wf-1","name":"Deploy"}],"build_supported":true}]}`
		return nextBuildNumberResponse(req, http.StatusOK, body), nil
	})

	cmd := webVersionAliasesList()
	if err := cmd.FlagSet.Parse([]string{"--product-id", " product-uuid ", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; output=%q", err, stdout)
	}
	if result["productId"] != "product-uuid" {
		t.Fatalf("productId = %#v", result["productId"])
	}
	if strings.Contains(stdout, "signed_url") || strings.Contains(stdout, "token=secret") || strings.Contains(stdout, "relatedWorkflow") {
		t.Fatalf("nested payload leaked in output: %q", stdout)
	}
	for _, want := range []string{`"id":"alias-1"`, `"name":"Release"`, `"buildName":"42"`, `"buildSupported":true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("output missing %s: %q", want, stdout)
		}
	}
}

func TestWebVersionAliasesValidateBeforeSession(t *testing.T) {
	original := resolveSessionFn
	called := false
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })

	list := webVersionAliasesList()
	if err := list.Exec(context.Background(), nil); err == nil {
		t.Fatal("expected missing product error")
	}
	if called {
		t.Fatal("session resolution must not run for invalid input")
	}
}

func TestWebVersionAliasesRequirePublicProviderID(t *testing.T) {
	original := resolveSessionFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: http.DefaultClient}, "cache", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })

	list := webVersionAliasesList()
	if err := list.FlagSet.Parse([]string{"--product-id", "product"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := list.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "session has no public provider ID") {
		t.Fatalf("list error = %v", err)
	}
}

func TestWebVersionAliasViewPreservesRawDetailResponse(t *testing.T) {
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || !strings.HasSuffix(req.URL.Path, "/version-aliases-v3/alias-1") {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return nextBuildNumberResponse(req, http.StatusOK, `{"id":"alias-1","name":" Release ","type":"xcode_version","locked":true,"build":{"signed_url":"https://example.invalid/?token=secret"},"build_name":"42","related_workflow_summaries":[{"id":"wf-1","name":"Deploy"}],"build_supported":true,"future_field":"preserve-me"}`), nil
	})

	cmd := webVersionAliasView()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--id", "alias-1", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; output=%q", err, stdout)
	}
	if result["productId"] != nil || result["action"] != nil || result["id"] != "alias-1" || result["name"] != " Release " {
		t.Fatalf("unexpected output: %#v", result)
	}
	if result["future_field"] != "preserve-me" {
		t.Fatalf("unknown response field was not preserved: %#v", result)
	}
	build, ok := result["build"].(map[string]any)
	if !ok || build["signed_url"] != "https://example.invalid/?token=secret" {
		t.Fatalf("nested build response was not preserved: %#v", result["build"])
	}
}

func TestWebVersionAliasCreateRejectsInvalidNameAndBuildBeforeSession(t *testing.T) {
	original := resolveSessionFn
	called := false
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })

	overLimitEmojiName := strings.Repeat("😀", 21)
	tests := []struct {
		name     string
		args     []string
		wantText string
	}{
		{
			name:     "missing name",
			args:     []string{"--product-id", "product", "--type", "xcode_version", "--build", "build", "--confirm"},
			wantText: "--name is required",
		},
		{
			name:     "blank name",
			args:     []string{"--product-id", "product", "--name", "   ", "--type", "xcode_version", "--build", "build", "--confirm"},
			wantText: "--name is required",
		},
		{
			name:     "missing build",
			args:     []string{"--product-id", "product", "--name", "Stable", "--type", "xcode_version", "--confirm"},
			wantText: "--build is required",
		},
		{
			name:     "blank build",
			args:     []string{"--product-id", "product", "--name", "Stable", "--type", "xcode_version", "--build", "", "--confirm"},
			wantText: "--build is required",
		},
		{
			name:     "name over UTF-16 limit",
			args:     []string{"--product-id", "product", "--name", overLimitEmojiName, "--type", "xcode_version", "--build", "build", "--confirm"},
			wantText: "--name must be at most 40 UTF-16 code units",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webVersionAliasCreate()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if err == nil || !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v", err)
				}
			})
			if !strings.Contains(stderr, tt.wantText) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantText)
			}
			if called {
				t.Fatal("session resolution must not run for invalid create input")
			}
		})
	}
}

func TestWebVersionAliasCreateAccepts40UTF16Name(t *testing.T) {
	originalIDFn := newVersionAliasIDFn
	newVersionAliasIDFn = func() string { return "alias-emoji" }
	t.Cleanup(func() { newVersionAliasIDFn = originalIDFn })

	name := strings.Repeat("😀", 20)
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut || req.Method == http.MethodGet {
			if req.Method == http.MethodPut {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read PUT body: %v", err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Fatalf("decode PUT body %q: %v", body, err)
				}
				if string(fields["name"]) != `"`+name+`"` {
					t.Fatalf("name = %s, want 20 emoji name", fields["name"])
				}
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-emoji", name, false, "build", "")), nil
		}
		t.Fatalf("unexpected method %s", req.Method)
		return nil, nil
	})

	cmd := webVersionAliasCreate()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product", "--name", name, "--type", "xcode_version", "--build", "build", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	}); stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestWebVersionAliasUpdateRejectsExplicitBlankFieldsBeforeSession(t *testing.T) {
	original := resolveSessionFn
	called := false
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "blank name", args: []string{"--product-id", "product", "--id", "alias", "--name", "", "--confirm"}, want: "--name is required"},
		{name: "blank build", args: []string{"--product-id", "product", "--id", "alias", "--build", "", "--confirm"}, want: "--build is required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webVersionAliasUpdate()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if err == nil || !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v", err)
				}
			})
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.want)
			}
			if called {
				t.Fatal("session resolution must not run for invalid update input")
			}
		})
	}
}

func TestWebVersionAliasUpdateRejectsInvalidEffectiveValuesBeforePut(t *testing.T) {
	for _, tt := range []struct {
		name      string
		aliasJSON string
		wantError string
	}{
		{
			name:      "existing name blank",
			aliasJSON: versionAliasJSON("alias-1", "   ", false, "build", ""),
			wantError: "existing version alias name must not be blank",
		},
		{
			name:      "existing build blank",
			aliasJSON: versionAliasJSON("alias-1", "Stable", false, "", ""),
			wantError: "existing version alias build must be a nonempty string; pass --build to replace it",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var methods []string
			stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method %s", req.Method)
				}
				return nextBuildNumberResponse(req, http.StatusOK, tt.aliasJSON), nil
			})

			cmd := webVersionAliasUpdate()
			if err := cmd.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias-1", "--locked=true", "--confirm"}); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if err == nil || !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v", err)
				}
			})
			if !strings.Contains(stderr, tt.wantError) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantError)
			}
			if got := strings.Join(methods, ","); got != "GET" {
				t.Fatalf("methods = %q, want only pre-read GET", got)
			}
		})
	}
}

func TestWebVersionAliasUpdateRejectsUnsupportedStoredBuildBeforePut(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build string
	}{
		{name: "object", build: `{"id":"build-1"}`},
		{name: "number", build: `42`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var methods []string
			stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method %s", req.Method)
				}
				body := `{"id":"alias-1","name":"Stable","type":"xcode_version","locked":true,"build":` + tt.build + `,"build_name":"42","related_workflow_summaries":[],"build_supported":true}`
				return nextBuildNumberResponse(req, http.StatusOK, body), nil
			})

			cmd := webVersionAliasUpdate()
			if err := cmd.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias-1", "--locked=true", "--confirm"}); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if err == nil || !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v", err)
				}
			})
			if !strings.Contains(stderr, "existing version alias build must be a nonempty string; pass --build to replace it") {
				t.Fatalf("stderr = %q", stderr)
			}
			if got := strings.Join(methods, ","); got != "GET" {
				t.Fatalf("methods = %q, want only pre-read GET", got)
			}
		})
	}
}

func TestWebVersionAliasUpdateExplicitBuildReplacesUnsupportedStoredBuild(t *testing.T) {
	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			getCount++
			if getCount == 1 {
				return nextBuildNumberResponse(req, http.StatusOK, `{"id":"alias-1","name":"Stable","type":"xcode_version","locked":true,"build":{"id":"build-old"},"build_name":"42","related_workflow_summaries":[],"build_supported":true}`), nil
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "Stable", true, "build-new", "43")), nil
		case http.MethodPut:
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read PUT body: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatalf("decode PUT body %q: %v", body, err)
			}
			if len(fields) != 4 || string(fields["name"]) != `"Stable"` || string(fields["type"]) != `"xcode_version"` || string(fields["build"]) != `"build-new"` || string(fields["locked"]) != `true` {
				t.Fatalf("unexpected PUT body: %s", body)
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "Stable", true, "build-new", "43")), nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	})

	cmd := webVersionAliasUpdate()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias-1", "--build", "build-new", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if getCount != 2 {
		t.Fatalf("GET count = %d, want pre-read plus verification", getCount)
	}
	if !strings.Contains(stdout, `"action":"updated"`) || !strings.Contains(stdout, `"name":"Stable"`) {
		t.Fatalf("unexpected update output: %q", stdout)
	}
}

func TestWebVersionAliasCreateSendsFourFieldsAndVerifies(t *testing.T) {
	originalIDFn := newVersionAliasIDFn
	newVersionAliasIDFn = func() string { return "alias-new" }
	t.Cleanup(func() { newVersionAliasIDFn = originalIDFn })

	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPut:
			if !strings.HasSuffix(req.URL.Path, "/version-aliases-v3/alias-new") {
				t.Fatalf("PUT path = %q", req.URL.Path)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read PUT body: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatalf("decode PUT body %q: %v", body, err)
			}
			if len(fields) != 4 || string(fields["name"]) != `"Stable"` || string(fields["type"]) != `"xcode_version"` || string(fields["build"]) != `"latest:stable"` || string(fields["locked"]) != `false` {
				t.Fatalf("unexpected PUT body: %s", body)
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-new", "Stable", false, "latest:stable", "")), nil
		case http.MethodGet:
			getCount++
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-new", "Stable", false, "latest:stable", "")), nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	})

	cmd := webVersionAliasCreate()
	if err := cmd.FlagSet.Parse([]string{
		"--product-id", "product-uuid",
		"--name", " Stable ",
		"--type", "xcode_version",
		"--build", "latest:stable",
		"--confirm", "--output", "json",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if getCount != 1 {
		t.Fatalf("GET count = %d, want one verification read", getCount)
	}
	if !strings.Contains(stdout, `"action":"created"`) || !strings.Contains(stdout, `"id":"alias-new"`) || !strings.Contains(stdout, `"name":"Stable"`) {
		t.Fatalf("unexpected create output: %q", stdout)
	}
}

func TestWebVersionAliasUpdatePreservesUnspecifiedFields(t *testing.T) {
	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			getCount++
			if getCount == 1 {
				return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "Old", true, "build-old", "41")), nil
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "New", true, "build-old", "42")), nil
		case http.MethodPut:
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read PUT body: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatalf("decode PUT body %q: %v", body, err)
			}
			if string(fields["name"]) != `"New"` || string(fields["type"]) != `"xcode_version"` || string(fields["build"]) != `"build-old"` || string(fields["locked"]) != `true` {
				t.Fatalf("update did not preserve omitted fields: %s", body)
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "New", true, "build-old", "42")), nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	})

	cmd := webVersionAliasUpdate()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--id", "alias-1", "--name", " New ", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if getCount != 2 {
		t.Fatalf("GET count = %d, want pre-read plus verification", getCount)
	}
	if !strings.Contains(stdout, `"action":"updated"`) || !strings.Contains(stdout, `"name":"New"`) {
		t.Fatalf("unexpected update output: %q", stdout)
	}
}

func TestWebVersionAliasUpdateVerifiesAfterMalformedSuccessfulPutResponse(t *testing.T) {
	var calls []string
	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method)
		switch req.Method {
		case http.MethodGet:
			getCount++
			if getCount == 1 {
				return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "Old", true, "build-old", "41")), nil
			}
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-1", "New", true, "build-old", "42")), nil
		case http.MethodPut:
			return nextBuildNumberResponse(req, http.StatusOK, `{"id":"alias-1"`), nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	})

	cmd := webVersionAliasUpdate()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias-1", "--name", "New", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if got := strings.Join(calls, ","); got != "GET,PUT,GET" {
		t.Fatalf("call order = %q, want pre-read, write, and verification GET", got)
	}
	if getCount != 2 {
		t.Fatalf("GET count = %d, want pre-read plus verification", getCount)
	}
	if !strings.Contains(stdout, `"action":"updated"`) || !strings.Contains(stdout, `"name":"New"`) {
		t.Fatalf("unexpected update output: %q", stdout)
	}
}

func TestWebVersionAliasDeleteRequiresConfirmedNotFound(t *testing.T) {
	var calls []string
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method)
		if req.Method == http.MethodDelete {
			if !strings.HasSuffix(req.URL.Path, "/version-aliases-v3/alias-1") {
				t.Fatalf("DELETE path = %q", req.URL.Path)
			}
			return nextBuildNumberResponse(req, http.StatusNoContent, ""), nil
		}
		if req.Method == http.MethodGet {
			return nextBuildNumberResponse(req, http.StatusNotFound, `{"errors":[{"status":"404"}]}`), nil
		}
		t.Fatalf("unexpected method %s", req.Method)
		return nil, nil
	})

	cmd := webVersionAliasDelete()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--id", "alias-1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if got := strings.Join(calls, ","); got != "DELETE,GET" {
		t.Fatalf("call order = %q", got)
	}
	if !strings.Contains(stdout, `"deleted":true`) || !strings.Contains(stdout, `"id":"alias-1"`) {
		t.Fatalf("unexpected delete output: %q", stdout)
	}
}

func TestWebVersionAliasMutationsValidateBeforeSession(t *testing.T) {
	original := resolveSessionFn
	called := false
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })

	create := webVersionAliasCreate()
	if err := create.FlagSet.Parse([]string{"--product-id", "product", "--name", "Stable", "--type", "xcode_version", "--build", "build"}); err != nil {
		t.Fatalf("parse create: %v", err)
	}
	_, stderr := captureOutput(t, func() {
		err := create.Exec(context.Background(), nil)
		if err == nil || !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("create error = %v", err)
		}
	})
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("create stderr = %q", stderr)
	}

	update := webVersionAliasUpdate()
	if err := update.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias", "--confirm"}); err != nil {
		t.Fatalf("parse update: %v", err)
	}
	if err := update.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "at least one version alias field is required") {
		t.Fatalf("update error = %v", err)
	}

	delete := webVersionAliasDelete()
	if err := delete.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias"}); err != nil {
		t.Fatalf("parse delete: %v", err)
	}
	_, stderr = captureOutput(t, func() {
		err := delete.Exec(context.Background(), nil)
		if err == nil || !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("delete error = %v", err)
		}
	})
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("delete stderr = %q", stderr)
	}
	if called {
		t.Fatal("session resolution must not run for invalid mutation input")
	}
}

func TestWebVersionAliasCreateReconcilesAmbiguousWrite(t *testing.T) {
	originalIDFn := newVersionAliasIDFn
	newVersionAliasIDFn = func() string { return "alias-new" }
	t.Cleanup(func() { newVersionAliasIDFn = originalIDFn })

	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			return nil, &url.Error{Op: http.MethodPut, URL: req.URL.String(), Err: io.ErrUnexpectedEOF}
		}
		if req.Method == http.MethodGet {
			return nextBuildNumberResponse(req, http.StatusOK, versionAliasJSON("alias-new", "Stable", false, "latest:stable", "")), nil
		}
		t.Fatalf("unexpected method %s", req.Method)
		return nil, nil
	})

	cmd := webVersionAliasCreate()
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product", "--name", "Stable", "--type", "xcode_version", "--build", "latest:stable", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if !strings.Contains(stdout, `"action":"created"`) {
		t.Fatalf("unexpected reconciled output: %q", stdout)
	}
}

func versionAliasJSON(id, name string, locked bool, build, buildName string) string {
	return `{"id":"` + id + `","name":"` + name + `","type":"xcode_version","locked":` + map[bool]string{true: "true", false: "false"}[locked] + `,"build":"` + build + `","build_name":"` + buildName + `","related_workflow_summaries":[],"build_supported":true}`
}

func TestWebVersionAliasCreateReconcilesServerFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		writeStatus int
		readStatus  int
		wantError   string
		wantCalls   string
	}{
		{"applied", http.StatusServiceUnavailable, http.StatusOK, "", "PUT,GET"},
		{"timeout applied", http.StatusRequestTimeout, http.StatusOK, "", "PUT,GET"},
		{"readback failed", http.StatusServiceUnavailable, http.StatusServiceUnavailable, "may have succeeded but reconciliation failed", "PUT,GET"},
		{"definitive rejection", http.StatusBadRequest, http.StatusOK, "400", "PUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalIDFn := newVersionAliasIDFn
			newVersionAliasIDFn = func() string { return "alias-new" }
			t.Cleanup(func() { newVersionAliasIDFn = originalIDFn })
			var calls []string
			stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
				calls = append(calls, req.Method)
				if req.Method == http.MethodPut {
					return nextBuildNumberResponse(req, tc.writeStatus, `{}`), nil
				}
				if req.Method == http.MethodGet {
					return nextBuildNumberResponse(req, tc.readStatus, versionAliasJSON("alias-new", "Stable", false, "latest:stable", "")), nil
				}
				t.Fatalf("unexpected method %s", req.Method)
				return nil, nil
			})
			command := webVersionAliasCreate()
			if err := command.FlagSet.Parse([]string{"--product-id", "product", "--name", "Stable", "--type", "xcode_version", "--build", "latest:stable", "--confirm", "--output", "json"}); err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, _ := captureOutput(t, func() { runErr = command.Exec(context.Background(), nil) })
			if tc.wantError == "" {
				if runErr != nil || !strings.Contains(stdout, `"action":"created"`) {
					t.Fatalf("error=%v output=%q", runErr, stdout)
				}
			} else if runErr == nil || !strings.Contains(runErr.Error(), tc.wantError) || stdout != "" {
				t.Fatalf("error=%v output=%q, want %q", runErr, stdout, tc.wantError)
			}
			if got := strings.Join(calls, ","); got != tc.wantCalls {
				t.Fatalf("calls=%s, want %s", got, tc.wantCalls)
			}
		})
	}
}

func TestWebVersionAliasDeleteReconcilesServerFailure(t *testing.T) {
	var calls []string
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method)
		if req.Method == http.MethodDelete {
			return nextBuildNumberResponse(req, http.StatusServiceUnavailable, `{}`), nil
		}
		if req.Method == http.MethodGet {
			return nextBuildNumberResponse(req, http.StatusNotFound, `{}`), nil
		}
		t.Fatalf("unexpected method %s", req.Method)
		return nil, nil
	})
	command := webVersionAliasDelete()
	if err := command.FlagSet.Parse([]string{"--product-id", "product", "--id", "alias-1", "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureOutput(t, func() { runErr = command.Exec(context.Background(), nil) })
	if runErr != nil || !strings.Contains(stdout, `"deleted":true`) {
		t.Fatalf("error=%v output=%q", runErr, stdout)
	}
	if got := strings.Join(calls, ","); got != "DELETE,GET" {
		t.Fatalf("calls=%s, want DELETE,GET", got)
	}
}
