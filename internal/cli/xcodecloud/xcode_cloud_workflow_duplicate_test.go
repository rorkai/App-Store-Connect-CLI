package xcodecloud

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const duplicateSourceEnvelope = `{
  "data": {
    "type": "ciWorkflows",
    "id": "wf-source",
    "attributes": {
      "name": "Deploy",
      "description": "Source description",
      "actions": [{"name": "Archive", "actionType": "ARCHIVE", "unknownFutureField": "keep"}],
      "isEnabled": true,
      "clean": true,
      "isLockedForEditing": true,
      "containerFilePath": "App.xcodeproj",
      "lastModifiedDate": "2026-03-14T06:25:37.578Z"
    },
    "relationships": {
      "product": {"data": {"type": "ciProducts", "id": "prod-1"}},
      "repository": {"data": {"type": "scmRepositories", "id": "repo-1"}},
      "xcodeVersion": {"data": {"type": "ciXcodeVersions", "id": "xcode-1"}},
      "macOsVersion": {"data": {"type": "ciMacOsVersions", "id": "macos-1"}}
    }
  }
}`

func TestBuildCiWorkflowDuplicatePayloadPreservesUnknownActionFields(t *testing.T) {
	payload, err := buildCiWorkflowDuplicatePayload(json.RawMessage(duplicateSourceEnvelope), ciWorkflowDuplicateOptions{
		name:             "Copy",
		sourceWorkflowID: "wf-source",
	})
	if err != nil {
		t.Fatalf("buildCiWorkflowDuplicatePayload error: %v", err)
	}
	if !strings.Contains(string(payload), "unknownFutureField") {
		t.Fatalf("expected unknown action fields to survive the copy, got %s", payload)
	}
	for _, dropped := range []string{"lastModifiedDate", "isLockedForEditing"} {
		if strings.Contains(string(payload), dropped) {
			t.Fatalf("expected %s to be stripped from the copy, got %s", dropped, payload)
		}
	}
}

func TestBuildCiWorkflowDuplicatePayloadOmitsNullFields(t *testing.T) {
	source := `{
  "data": {
    "type": "ciWorkflows",
    "id": "wf-source",
    "attributes": {
      "name": "Deploy",
      "description": "",
      "actions": [{"name": "Archive", "actionType": "ARCHIVE", "destination": null, "testConfiguration": null, "scheme": "App", "platform": "IOS", "isRequiredToPass": true}],
      "isEnabled": true,
      "clean": true,
      "containerFilePath": "App.xcodeproj",
      "tagStartCondition": null,
      "branchStartCondition": {"source": {"patterns": [{"pattern": "main"}]}, "filesAndFoldersRule": null, "autoCancel": true}
    },
    "relationships": {
      "product": {"data": {"type": "ciProducts", "id": "prod-1"}},
      "repository": {"data": {"type": "scmRepositories", "id": "repo-1"}},
      "xcodeVersion": {"data": {"type": "ciXcodeVersions", "id": "xcode-1"}},
      "macOsVersion": {"data": {"type": "ciMacOsVersions", "id": "macos-1"}}
    }
  }
}`

	payload, err := buildCiWorkflowDuplicatePayload(json.RawMessage(source), ciWorkflowDuplicateOptions{
		name:             "Copy",
		sourceWorkflowID: "wf-source",
	})
	if err != nil {
		t.Fatalf("buildCiWorkflowDuplicatePayload error: %v", err)
	}

	var decoded struct {
		Data struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := decoded.Data.Attributes["tagStartCondition"]; ok {
		t.Fatalf("expected null tagStartCondition to be omitted, got %s", payload)
	}
	if strings.Contains(string(decoded.Data.Attributes["actions"]), "null") {
		t.Fatalf("expected null action fields to be omitted, got %s", decoded.Data.Attributes["actions"])
	}
	if strings.Contains(string(decoded.Data.Attributes["branchStartCondition"]), "null") {
		t.Fatalf("expected null filesAndFoldersRule to be omitted, got %s", decoded.Data.Attributes["branchStartCondition"])
	}
	if string(decoded.Data.Attributes["description"]) != `""` {
		t.Fatalf("expected empty description to be preserved, got %s", decoded.Data.Attributes["description"])
	}
}

func TestBuildCiWorkflowDuplicatePayloadDescriptionHandling(t *testing.T) {
	tests := []struct {
		name          string
		opts          ciWorkflowDuplicateOptions
		wantDescr     string
		sourceReplace [2]string
	}{
		{
			name:      "copies source description",
			opts:      ciWorkflowDuplicateOptions{name: "Copy", sourceWorkflowID: "wf-source"},
			wantDescr: "Source description",
		},
		{
			name:      "explicit empty description overrides source",
			opts:      ciWorkflowDuplicateOptions{name: "Copy", description: "", overrideDescr: true, sourceWorkflowID: "wf-source"},
			wantDescr: "",
		},
		{
			name:          "absent source description becomes empty string",
			opts:          ciWorkflowDuplicateOptions{name: "Copy", sourceWorkflowID: "wf-source"},
			wantDescr:     "",
			sourceReplace: [2]string{`"description": "Source description",`, ""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := duplicateSourceEnvelope
			if test.sourceReplace[0] != "" {
				source = strings.Replace(source, test.sourceReplace[0], test.sourceReplace[1], 1)
			}

			payload, err := buildCiWorkflowDuplicatePayload(json.RawMessage(source), test.opts)
			if err != nil {
				t.Fatalf("buildCiWorkflowDuplicatePayload error: %v", err)
			}

			var decoded struct {
				Data struct {
					Attributes struct {
						Description *string `json:"description"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if decoded.Data.Attributes.Description == nil {
				t.Fatalf("expected description to be present in the create payload, got %s", payload)
			}
			if *decoded.Data.Attributes.Description != test.wantDescr {
				t.Fatalf("description = %q, want %q", *decoded.Data.Attributes.Description, test.wantDescr)
			}
		})
	}
}

func TestBuildCiWorkflowDuplicatePayloadRejectsIncompleteSource(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "missing actions",
			source:  strings.Replace(duplicateSourceEnvelope, `"actions": [{"name": "Archive", "actionType": "ARCHIVE", "unknownFutureField": "keep"}],`, "", 1),
			wantErr: `missing required attribute "actions"`,
		},
		{
			name:    "missing container file path",
			source:  strings.Replace(duplicateSourceEnvelope, `"containerFilePath": "App.xcodeproj",`, "", 1),
			wantErr: `missing required attribute "containerFilePath"`,
		},
		{
			name:    "missing relationship linkages",
			source:  strings.Replace(duplicateSourceEnvelope, `"xcodeVersion": {"data": {"type": "ciXcodeVersions", "id": "xcode-1"}},`, "", 1),
			wantErr: "xcodeVersion relationship linkage",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildCiWorkflowDuplicatePayload(json.RawMessage(test.source), ciWorkflowDuplicateOptions{
				name:             "Copy",
				sourceWorkflowID: "wf-source",
			})
			if err == nil {
				t.Fatalf("expected error for %s", test.name)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, test.wantErr)
			}
		})
	}
}

func TestXcodeCloudWorkflowsDuplicateHelpDisclosesPrivateAPIGaps(t *testing.T) {
	cmd := newXcodeCloudWorkflowsDuplicateCommand(&shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}})

	help := cmd.LongHelp
	for _, want := range []string{
		"clean setting",
		"TestFlight post-actions",
		"workflow environment variables",
		"asc web xcode-cloud env-vars",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("LongHelp missing %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "same start conditions, actions, environment,") {
		t.Fatalf("LongHelp still claims to copy a generic environment:\n%s", help)
	}

	for _, name := range []string{"id", "name", "description", "enabled"} {
		flag := cmd.FlagSet.Lookup(name)
		if flag == nil {
			t.Fatalf("missing --%s", name)
		}
	}
}
