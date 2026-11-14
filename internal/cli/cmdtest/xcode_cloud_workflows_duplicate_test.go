package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const duplicateSourceWorkflow = `{
  "data": {
    "type": "ciWorkflows",
    "id": "wf-source",
    "attributes": {
      "name": "TestFlight Deploy",
      "description": "Build and archive",
      "branchStartCondition": {"source": {"patterns": [{"pattern": "main"}]}, "autoCancel": true},
      "actions": [{"name": "Archive", "actionType": "ARCHIVE", "scheme": "MyApp", "platform": "IOS", "isRequiredToPass": true}],
      "isEnabled": true,
      "isLockedForEditing": true,
      "tagStartCondition": null,
      "clean": true,
      "containerFilePath": "MyApp.xcodeproj",
      "lastModifiedDate": "2026-03-14T06:25:37.578Z"
    },
    "relationships": {
      "product": {"data": {"type": "ciProducts", "id": "prod-1"}},
      "repository": {"links": {"self": "https://example.com"}, "data": {"type": "scmRepositories", "id": "repo-1"}},
      "xcodeVersion": {"data": {"type": "ciXcodeVersions", "id": "xcode-1"}},
      "macOsVersion": {"data": {"type": "ciMacOsVersions", "id": "macos-1"}},
      "buildRuns": {"links": {"related": "https://example.com/buildRuns"}}
    }
  }
}`

func TestXcodeCloudWorkflowsDuplicateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing id",
			args:    []string{"xcode-cloud", "workflows", "duplicate", "--name", "Copy"},
			wantErr: "--id is required",
		},
		{
			name:    "missing name",
			args:    []string{"xcode-cloud", "workflows", "duplicate", "--id", "wf-source"},
			wantErr: "--name is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestXcodeCloudWorkflowsDuplicateCopiesSourceConfiguration(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciWorkflows/wf-source", "include=product,repository,xcodeVersion,macOsVersion"), asctest.JSON(http.StatusOK, duplicateSourceWorkflow))
	reg.Register(asctest.POST("/v1/ciWorkflows").JSONBody(`{"data":{"attributes":{"actions":[{"actionType":"ARCHIVE","isRequiredToPass":true,"name":"Archive","platform":"IOS","scheme":"MyApp"}],"branchStartCondition":{"autoCancel":true,"source":{"patterns":[{"pattern":"main"}]}},"clean":true,"containerFilePath":"MyApp.xcodeproj","description":"Build and archive","isEnabled":false,"name":"TestFlight Deploy Copy"},"relationships":{"macOsVersion":{"data":{"id":"macos-1","type":"ciMacOsVersions"}},"product":{"data":{"id":"prod-1","type":"ciProducts"}},"repository":{"data":{"id":"repo-1","type":"scmRepositories"}},"xcodeVersion":{"data":{"id":"xcode-1","type":"ciXcodeVersions"}}},"type":"ciWorkflows"}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciWorkflows","id":"wf-copy","attributes":{"name":"TestFlight Deploy Copy","isEnabled":false,"unknownFutureField":true}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "workflows", "duplicate", "--id", "wf-source", "--name", "TestFlight Deploy Copy"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				IsEnabled          *bool `json:"isEnabled"`
				UnknownFutureField *bool `json:"unknownFutureField"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode stdout %q: %v", stdout, err)
	}
	if response.Data.ID != "wf-copy" {
		t.Fatalf("expected created workflow id wf-copy, got %q", response.Data.ID)
	}
	if response.Data.Attributes.IsEnabled == nil || *response.Data.Attributes.IsEnabled {
		t.Fatalf("expected stdout to preserve isEnabled=false from the create envelope, got %q", stdout)
	}
	if response.Data.Attributes.UnknownFutureField == nil || !*response.Data.Attributes.UnknownFutureField {
		t.Fatalf("expected stdout to preserve unmodeled create-response fields, got %q", stdout)
	}

	var payload struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Name                 string          `json:"name"`
				Description          string          `json:"description"`
				IsEnabled            *bool           `json:"isEnabled"`
				Clean                *bool           `json:"clean"`
				ContainerFilePath    string          `json:"containerFilePath"`
				Actions              json.RawMessage `json:"actions"`
				BranchStartCondition json.RawMessage `json:"branchStartCondition"`
				LastModifiedDate     *string         `json:"lastModifiedDate"`
				IsLockedForEditing   *bool           `json:"isLockedForEditing"`
			} `json:"attributes"`
			Relationships map[string]struct {
				Data *struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			} `json:"relationships"`
		} `json:"data"`
	}
	created := json.RawMessage(reg.Requests()[1].Body)
	if err := json.Unmarshal(created, &payload); err != nil {
		t.Fatalf("decode create payload %q: %v", created, err)
	}

	if payload.Data.Type != "ciWorkflows" {
		t.Fatalf("expected type ciWorkflows, got %q", payload.Data.Type)
	}
	if payload.Data.ID != "" {
		t.Fatalf("expected no id in create payload, got %q", payload.Data.ID)
	}
	if payload.Data.Attributes.Name != "TestFlight Deploy Copy" {
		t.Fatalf("expected copied name, got %q", payload.Data.Attributes.Name)
	}
	if payload.Data.Attributes.Description != "Build and archive" {
		t.Fatalf("expected source description, got %q", payload.Data.Attributes.Description)
	}
	if payload.Data.Attributes.IsEnabled == nil || *payload.Data.Attributes.IsEnabled {
		t.Fatalf("expected isEnabled=false in create payload, got %v", payload.Data.Attributes.IsEnabled)
	}
	if payload.Data.Attributes.Clean == nil || !*payload.Data.Attributes.Clean {
		t.Fatalf("expected clean=true copied from source, got %v", payload.Data.Attributes.Clean)
	}
	if payload.Data.Attributes.ContainerFilePath != "MyApp.xcodeproj" {
		t.Fatalf("expected copied containerFilePath, got %q", payload.Data.Attributes.ContainerFilePath)
	}
	if len(payload.Data.Attributes.Actions) == 0 || !strings.Contains(string(payload.Data.Attributes.Actions), "ARCHIVE") {
		t.Fatalf("expected copied actions, got %q", payload.Data.Attributes.Actions)
	}
	if !strings.Contains(string(payload.Data.Attributes.BranchStartCondition), "main") {
		t.Fatalf("expected copied start condition, got %q", payload.Data.Attributes.BranchStartCondition)
	}
	if payload.Data.Attributes.LastModifiedDate != nil {
		t.Fatalf("expected lastModifiedDate to be stripped, got %v", *payload.Data.Attributes.LastModifiedDate)
	}
	if payload.Data.Attributes.IsLockedForEditing != nil {
		t.Fatalf("expected the copy to be created unlocked, got isLockedForEditing=%v", *payload.Data.Attributes.IsLockedForEditing)
	}

	wantRelationships := map[string]string{
		"product":      "prod-1",
		"repository":   "repo-1",
		"xcodeVersion": "xcode-1",
		"macOsVersion": "macos-1",
	}
	if len(payload.Data.Relationships) != len(wantRelationships) {
		t.Fatalf("expected only %d relationships, got %v", len(wantRelationships), payload.Data.Relationships)
	}
	for name, wantID := range wantRelationships {
		rel, ok := payload.Data.Relationships[name]
		if !ok || rel.Data == nil {
			t.Fatalf("expected relationship %q linkage, got %v", name, payload.Data.Relationships)
		}
		if rel.Data.ID != wantID {
			t.Fatalf("relationship %q id = %q, want %q", name, rel.Data.ID, wantID)
		}
	}
}

func TestXcodeCloudWorkflowsDuplicateEnabledAndDescriptionOverrides(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciWorkflows/wf-source", "include=product,repository,xcodeVersion,macOsVersion"), asctest.JSON(http.StatusOK, duplicateSourceWorkflow))
	reg.Register(asctest.POST("/v1/ciWorkflows").JSONBody(`{"data":{"attributes":{"actions":[{"actionType":"ARCHIVE","isRequiredToPass":true,"name":"Archive","platform":"IOS","scheme":"MyApp"}],"branchStartCondition":{"autoCancel":true,"source":{"patterns":[{"pattern":"main"}]}},"clean":true,"containerFilePath":"MyApp.xcodeproj","description":"Nightly copy","isEnabled":true,"name":"Nightly"},"relationships":{"macOsVersion":{"data":{"id":"macos-1","type":"ciMacOsVersions"}},"product":{"data":{"id":"prod-1","type":"ciProducts"}},"repository":{"data":{"id":"repo-1","type":"scmRepositories"}},"xcodeVersion":{"data":{"id":"xcode-1","type":"ciXcodeVersions"}}},"type":"ciWorkflows"}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciWorkflows","id":"wf-copy"}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		args := []string{"xcode-cloud", "workflows", "duplicate", "--id", "wf-source", "--name", "Nightly", "--description", "Nightly copy", "--enabled"}
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Data struct {
			Attributes struct {
				Description string `json:"description"`
				IsEnabled   *bool  `json:"isEnabled"`
			} `json:"attributes"`
		} `json:"data"`
	}
	created := json.RawMessage(reg.Requests()[1].Body)
	if err := json.Unmarshal(created, &payload); err != nil {
		t.Fatalf("decode create payload %q: %v", created, err)
	}
	if payload.Data.Attributes.Description != "Nightly copy" {
		t.Fatalf("expected overridden description, got %q", payload.Data.Attributes.Description)
	}
	if payload.Data.Attributes.IsEnabled == nil || !*payload.Data.Attributes.IsEnabled {
		t.Fatalf("expected isEnabled=true, got %v", payload.Data.Attributes.IsEnabled)
	}
}

func TestXcodeCloudWorkflowsDuplicateMissingRelationshipFails(t *testing.T) {
	setupAuth(t)

	// Apple answers GET /v1/ciWorkflows/{id} without include with links-only relationships,
	// which cannot satisfy the required linkages of CiWorkflowCreateRequest.
	source := `{"data":{"type":"ciWorkflows","id":"wf-source","attributes":{"name":"A","description":"","actions":[],"isEnabled":true,"clean":false,"containerFilePath":"App.xcodeproj"},"relationships":{"repository":{"links":{"related":"https://example.com/repository"}},"buildRuns":{"links":{"related":"https://example.com/buildRuns"}}}}}`

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciWorkflows/wf-source", "include=product,repository,xcodeVersion,macOsVersion"), asctest.JSON(http.StatusOK, source))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "workflows", "duplicate", "--id", "wf-source", "--name", "Copy"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error when source workflow lacks required relationships")
	}
	for _, want := range []string{"macOsVersion", "product", "repository", "xcodeVersion"} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected missing %s relationship in error, got %v", want, runErr)
		}
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}
