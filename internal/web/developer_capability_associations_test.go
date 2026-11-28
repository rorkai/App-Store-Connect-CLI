package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyDeveloperCapabilityAssociations(t *testing.T) {
	var metadata developerCapabilityMetadataAttributes
	if err := json.Unmarshal([]byte(`{"associations":[{"relationshipName":"associatedBundleIds","relationshipType":"bundleIds","minInstances":1,"maxLimit":1,"allowedInstances":"SINGLE"},{"relationshipName":"mediaSharingProtocolIds","relationshipType":"mediaSharingProtocolIds","minInstances":0,"maxLimit":2,"allowedInstances":"MULTIPLE"}]}`), &metadata); err != nil {
		t.Fatal(err)
	}
	current := developerResource{Relationships: map[string]json.RawMessage{
		"associatedBundleIds": json.RawMessage(`{"data":[{"type":"bundleIds","id":"old-parent"}]}`),
		"capability":          json.RawMessage(`{"data":{"type":"capabilities","id":"DATA_CONTAINER_ACCESS"}}`),
	}}
	for _, test := range []struct {
		name      string
		requested map[string][]string
		wantError string
	}{
		{name: "preserve", requested: nil},
		{name: "replace parent", requested: map[string][]string{"associatedBundleIds": {"new-parent"}}},
		{name: "empty required", requested: map[string][]string{"associatedBundleIds": {}}, wantError: `association "associatedBundleIds" requires at least 1 resource`},
		{name: "single cardinality", requested: map[string][]string{"associatedBundleIds": {"one", "two"}}, wantError: `association "associatedBundleIds" allows at most one resource`},
		{name: "max limit", requested: map[string][]string{"mediaSharingProtocolIds": {"one", "two", "three"}}, wantError: `association "mediaSharingProtocolIds" allows at most 2 resources`},
		{name: "unknown name", requested: map[string][]string{"typo": {"one"}}, wantError: `unknown capability association "typo"`},
		{name: "null ids", requested: map[string][]string{"associatedBundleIds": nil}, wantError: `association "associatedBundleIds" must be an array of resource ids, got null`},
		{name: "blank id", requested: map[string][]string{"associatedBundleIds": {" "}}, wantError: `association "associatedBundleIds" has an empty resource id`},
		{name: "duplicate id", requested: map[string][]string{"mediaSharingProtocolIds": {"one", "one"}}, wantError: `association "mediaSharingProtocolIds" repeats resource "one"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := applyDeveloperCapabilityAssociations(metadata, current, test.requested)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(result.Relationships["capability"]) != string(current.Relationships["capability"]) {
				t.Fatal("unrelated capability relationship changed")
			}
			var relationship struct {
				Data []developerResource `json:"data"`
			}
			if err := json.Unmarshal(result.Relationships["associatedBundleIds"], &relationship); err != nil {
				t.Fatal(err)
			}
			wantID := "old-parent"
			if test.requested != nil {
				wantID = "new-parent"
			}
			if len(relationship.Data) != 1 || relationship.Data[0].ID != wantID || relationship.Data[0].Type != "bundleIds" {
				t.Fatalf("parent relationship = %+v, want bundleIds/%s", relationship.Data, wantID)
			}
			if string(current.Relationships["associatedBundleIds"]) != `{"data":[{"type":"bundleIds","id":"old-parent"}]}` {
				t.Fatal("input resource was mutated")
			}
		})
	}
	if err := validateDeveloperCapabilityAssociations(metadata, developerResource{}); err == nil || !strings.Contains(err.Error(), `association "associatedBundleIds" requires at least 1 resource`) {
		t.Fatalf("missing required association error = %v", err)
	}
}

func TestDeveloperCapabilityAssociationsRespectRelationshipShape(t *testing.T) {
	var metadata developerCapabilityMetadataAttributes
	if err := json.Unmarshal([]byte(`{"associations":[{"relationshipName":"futureParentBundleId","relationshipType":"bundleIds","minInstances":1,"allowedInstances":"SINGLE"}]}`), &metadata); err != nil {
		t.Fatal(err)
	}
	current := developerResource{Relationships: map[string]json.RawMessage{
		"futureParentBundleId": json.RawMessage(`{"data":{"type":"bundleIds","id":"old-parent"}}`),
	}}
	result, err := applyDeveloperCapabilityAssociations(metadata, current, map[string][]string{"futureParentBundleId": {"new-parent"}})
	if err != nil {
		t.Fatal(err)
	}
	var relationship struct {
		Data developerResource `json:"data"`
	}
	if err := json.Unmarshal(result.Relationships["futureParentBundleId"], &relationship); err != nil {
		t.Fatal(err)
	}
	if relationship.Data.Type != "bundleIds" || relationship.Data.ID != "new-parent" {
		t.Fatalf("parent relationship = %+v", relationship.Data)
	}
	if _, err := applyDeveloperCapabilityAssociations(metadata, developerResource{}, map[string][]string{"futureParentBundleId": {"new-parent"}}); err == nil || !strings.Contains(err.Error(), "unknown relationship shape") {
		t.Fatalf("unknown relationship shape error = %v", err)
	}
	current.Relationships["futureParentBundleId"] = json.RawMessage(`{"data":null}`)
	if _, err := applyDeveloperCapabilityAssociations(metadata, current, map[string][]string{"futureParentBundleId": {"new-parent"}}); err == nil || !strings.Contains(err.Error(), "unknown relationship shape") {
		t.Fatalf("null relationship shape error = %v", err)
	}
	current.Relationships["futureParentBundleId"] = json.RawMessage(`{"data":{"type":"wrong","id":"old-parent"}}`)
	if err := validateDeveloperCapabilityAssociations(metadata, current); err == nil || !strings.Contains(err.Error(), `expected resource type "bundleIds"`) {
		t.Fatalf("wrong association type error = %v", err)
	}
}

func TestDeveloperCapabilityAssociationsCreateKnownRelationshipShapes(t *testing.T) {
	for _, test := range []struct {
		name         string
		resourceType string
		many         bool
	}{
		{name: "appGroups", resourceType: "appGroups", many: true},
		{name: "cloudContainers", resourceType: "cloudContainers", many: true},
		{name: "certificates", resourceType: "certificates", many: true},
		{name: "merchantIds", resourceType: "merchantIds", many: true},
		{name: "relatedAppConsentBundleIds", resourceType: "bundleIds", many: true},
		{name: "parentBundleId", resourceType: "bundleIds"},
		{name: "appConsentBundleId", resourceType: "bundleIds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			definition, err := json.Marshal([]developerCapabilityAssociation{{RelationshipName: test.name, RelationshipType: test.resourceType, AllowedInstances: "SINGLE", MinInstances: 1}})
			if err != nil {
				t.Fatal(err)
			}
			metadata := developerCapabilityMetadataAttributes{Associations: definition}
			result, err := applyDeveloperCapabilityAssociations(metadata, developerResource{}, map[string][]string{test.name: {"first-resource"}})
			if err != nil {
				t.Fatal(err)
			}
			var relationship struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(result.Relationships[test.name], &relationship); err != nil {
				t.Fatal(err)
			}
			if (relationship.Data[0] == '[') != test.many {
				t.Fatalf("relationship %s has wrong cardinality: %s", test.name, relationship.Data)
			}
			resources, _, err := developerCapabilityAssociationResources(result.Relationships[test.name])
			if err != nil || len(resources) != 1 || resources[0].Type != test.resourceType || resources[0].ID != "first-resource" {
				t.Fatalf("relationship %s resources = %+v, error = %v", test.name, resources, err)
			}
			if _, err := applyDeveloperCapabilityAssociations(developerCapabilityMetadataAttributes{}, developerResource{}, map[string][]string{test.name: {"first-resource"}}); err == nil || !strings.Contains(err.Error(), "unknown capability association") {
				t.Fatalf("undeclared %s association error = %v", test.name, err)
			}
		})
	}
}
