package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebICloudContainersCommandHierarchy(t *testing.T) {
	command := WebICloudContainersCommand()
	if command.Name != "icloud-containers" || command.UsageFunc == nil {
		t.Fatalf("unexpected command: %+v", command)
	}
	if len(command.Subcommands) != 2 || command.Subcommands[0].Name != "list" || command.Subcommands[1].Name != "create" {
		t.Fatalf("subcommands = %+v, want list and create", command.Subcommands)
	}
	if command.Subcommands[0].UsageFunc == nil || command.Subcommands[1].UsageFunc == nil {
		t.Fatal("iCloud container subcommands must set UsageFunc")
	}
	if command.Subcommands[0].FlagSet.Lookup("paginate") != nil {
		t.Fatal("iCloud container list must not advertise --paginate")
	}

	root := WebCommand()
	var found bool
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "icloud-containers" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("web command did not register icloud-containers")
	}
}

func stubWebICloudContainerCreateDependencies(t *testing.T) (*int, *int) {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origCreate := createDeveloperICloudContainerFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		createDeveloperICloudContainerFn = origCreate
		persistWebSessionFn = origPersist
	})
	var resolveCalls, persistCalls int
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}
	createDeveloperICloudContainerFn = func(context.Context, *webcore.Client, webcore.DeveloperICloudContainerCreateRequest) (*asc.WebICloudContainerCreateResult, error) {
		t.Fatal("create must not be called")
		return nil, nil
	}
	return &resolveCalls, &persistCalls
}

func TestWebICloudContainersCreatePrintsVerifiedPermanentReceipt(t *testing.T) {
	_, persistCalls := stubWebICloudContainerCreateDependencies(t)
	var got webcore.DeveloperICloudContainerCreateRequest
	createDeveloperICloudContainerFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperICloudContainerCreateRequest) (*asc.WebICloudContainerCreateResult, error) {
		got = request
		return &asc.WebICloudContainerCreateResult{
			Operation: "create", ContainerID: "cloud-1", Identifier: request.Identifier, Name: request.Name,
			Prefix: "TEAM123456", Changed: true, Verified: true, Permanent: true, Status: "created",
		}, nil
	}

	command := WebICloudContainersCreateCommand()
	if err := command.FlagSet.Parse([]string{
		"--identifier", " iCloud.com.example.app ",
		"--name", " Example Container ",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), command.FlagSet.Args()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if got.Identifier != "iCloud.com.example.app" || got.Name != "Example Container" {
		t.Fatalf("create request = %+v", got)
	}
	var receipt map[string]any
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, stdout)
	}
	for key, want := range map[string]any{
		"operation": "create", "containerId": "cloud-1", "identifier": "iCloud.com.example.app",
		"name": "Example Container", "prefix": "TEAM123456", "verified": true, "permanent": true, "status": "created",
	} {
		if receipt[key] != want {
			t.Errorf("receipt[%q] = %v, want %v", key, receipt[key], want)
		}
	}
	if *persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1", *persistCalls)
	}
}

func TestWebICloudContainersCreateRejectsInvalidIdentifierBeforeSession(t *testing.T) {
	resolveCalls, _ := stubWebICloudContainerCreateDependencies(t)
	for _, tc := range []struct{ identifier, want string }{
		{"com.example.app", `must start with "iCloud."`},
		{"icloud.com.example.app", `use "iCloud.com.example.app"`},
		{"iCloud.", "reverse-DNS string"},
	} {
		t.Run(tc.identifier, func(t *testing.T) {
			command := WebICloudContainersCreateCommand()
			if err := command.FlagSet.Parse([]string{"--identifier", tc.identifier, "--name", "Example", "--confirm"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				if err := command.Exec(context.Background(), command.FlagSet.Args()); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("stdout = %q, stderr = %q, want %q", stdout, stderr, tc.want)
			}
		})
	}
	if *resolveCalls != 0 {
		t.Fatalf("session resolved %d times before validation", *resolveCalls)
	}
}

func TestWebICloudContainersCreateReportsUnverifiedOutcome(t *testing.T) {
	_, persistCalls := stubWebICloudContainerCreateDependencies(t)
	createDeveloperICloudContainerFn = func(context.Context, *webcore.Client, webcore.DeveloperICloudContainerCreateRequest) (*asc.WebICloudContainerCreateResult, error) {
		return nil, &webcore.DeveloperICloudContainerUnverifiedError{Err: errors.New("developer portal accepted the iCloud container create but the read-back failed; run asc web icloud-containers list before retrying")}
	}
	command := WebICloudContainersCreateCommand()
	if err := command.FlagSet.Parse([]string{"--identifier", "iCloud.com.example.app", "--name", "Example", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if err == nil || errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "run asc web icloud-containers list before retrying") {
			t.Fatalf("error = %v, want unverified outcome", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if *persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1 even on failure", *persistCalls)
	}
}

func TestWebICloudContainersCreateRequiresConfirm(t *testing.T) {
	command := WebICloudContainersCreateCommand()
	if err := command.FlagSet.Parse([]string{
		"--identifier", "iCloud.com.example.app",
		"--name", "Example",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestWebICloudContainersListPrintsJSONAndPassesHidden(t *testing.T) {
	restore := stubWebICloudContainerReadDependencies(t)
	defer restore()

	var gotHidden bool
	listDeveloperICloudContainersFn = func(_ context.Context, _ *webcore.Client, hidden bool) (*webcore.DeveloperICloudContainersListResult, error) {
		gotHidden = hidden
		return &webcore.DeveloperICloudContainersListResult{
			Data: []webcore.DeveloperICloudContainer{{
				ID:   "cloud-1",
				Type: "cloudContainers",
				Attributes: webcore.DeveloperICloudContainerAttributes{
					Identifier: "iCloud.com.example.app",
					Name:       "Example Container",
					Prefix:     "TEAM123456",
					CanEdit:    true,
					CanDelete:  false,
				},
			}},
			Raw: json.RawMessage(`{"data":[{"type":"cloudContainers","id":"cloud-1","attributes":{"identifier":"iCloud.com.example.app","name":"Example Container"}}],"links":{},"meta":{},"unknownTopLevel":{"keep":true}}`),
		}, nil
	}

	command := WebICloudContainersListCommand()
	if err := command.FlagSet.Parse([]string{"--hidden", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !gotHidden {
		t.Fatal("--hidden was not passed to list operation")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	if got := string(envelope["unknownTopLevel"]); got != `{"keep":true}` {
		t.Fatalf("unknown top-level member = %s, want {\"keep\":true}", got)
	}
}

func TestWebICloudContainersListPrintsMeaningfulTable(t *testing.T) {
	restore := stubWebICloudContainerReadDependencies(t)
	defer restore()

	listDeveloperICloudContainersFn = func(_ context.Context, _ *webcore.Client, hidden bool) (*webcore.DeveloperICloudContainersListResult, error) {
		if hidden {
			t.Fatal("default hidden flag = true, want false")
		}
		return &webcore.DeveloperICloudContainersListResult{Data: []webcore.DeveloperICloudContainer{{
			ID:   "cloud-1",
			Type: "cloudContainers",
			Attributes: webcore.DeveloperICloudContainerAttributes{
				Identifier: "iCloud.com.example.app",
				Name:       "Example Container",
				Prefix:     "TEAM123456",
				Hidden:     false,
				CanEdit:    true,
				CanDelete:  false,
				ResponseID: "response-1",
			},
		}}}, nil
	}

	command := WebICloudContainersListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"ID", "Name", "Identifier", "Prefix", "Hidden", "Can Edit", "Can Delete", "Response ID", "cloud-1", "Example Container", "iCloud.com.example.app", "TEAM123456", "response-1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output %q does not contain %q", stdout, want)
		}
	}
}

func stubWebICloudContainerReadDependencies(t *testing.T) func() {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listDeveloperICloudContainersFn
	origPersist := persistWebSessionFn

	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	return func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listDeveloperICloudContainersFn = origList
		persistWebSessionFn = origPersist
	}
}

func TestWebICloudContainersListWarnsAboutIncompletePages(t *testing.T) {
	for _, format := range []string{"table", "markdown", "json"} {
		for _, next := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/next=%t", format, next), func(t *testing.T) {
				restore := stubWebICloudContainerReadDependencies(t)
				defer restore()
				listDeveloperICloudContainersFn = func(context.Context, *webcore.Client, bool) (*webcore.DeveloperICloudContainersListResult, error) {
					links := map[string]any{}
					if next {
						links["next"] = map[string]any{"href": "https://developer.apple.com/next"}
					}
					return &webcore.DeveloperICloudContainersListResult{
						Data:  []webcore.DeveloperICloudContainer{{ID: "cloud-1", Type: "cloudContainers"}},
						Links: links, Meta: map[string]any{"paging": map[string]any{"total": 1001, "limit": 1000}},
					}, nil
				}
				command := WebICloudContainersListCommand()
				if err := command.FlagSet.Parse([]string{"--output", format}); err != nil {
					t.Fatal(err)
				}
				_, stderr := captureWebCommandOutput(t, func() {
					if err := command.Exec(context.Background(), nil); err != nil {
						t.Fatal(err)
					}
				})
				if strings.Count(stderr, "Warning:") != 1 || !strings.Contains(stderr, "showing 1 of 1001 results") {
					t.Fatalf("stderr = %q, want one incomplete-page warning", stderr)
				}
			})
		}
	}
}
