package schema

import (
	"context"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestParseInterspersedSchemaFlagsPreservesErrorPrecedence(t *testing.T) {
	t.Run("earlier unknown flag", func(t *testing.T) {
		fs := flag.NewFlagSet("schema", flag.ContinueOnError)
		fs.String("method", "", "")

		_, err := shared.ParseInterspersedFlags(fs, []string{
			"apps", "--bogus", "--profile", "staging",
		})
		if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -bogus") {
			t.Fatalf("ParseInterspersedFlags() error = %v, want the unknown-flag error", err)
		}
	})

	t.Run("misplaced root profile", func(t *testing.T) {
		fs := flag.NewFlagSet("schema", flag.ContinueOnError)
		fs.String("method", "", "")

		_, err := shared.ParseInterspersedFlags(fs, []string{"apps", "--profile", "staging"})
		if err == nil || err.Error() != "`--profile` must appear before positional arguments" {
			t.Fatalf("ParseInterspersedFlags() error = %v, want the profile-placement error", err)
		}
	})
}

func TestSchemaCommandExecRejectsMisplacedProfileAfterQuery(t *testing.T) {
	err := SchemaCommand().Exec(context.Background(), []string{"apps", "--profile", "work"})
	if err == nil || err.Error() != "`--profile` must appear before positional arguments" {
		t.Fatalf("SchemaCommand().Exec() error = %v, want the profile-placement usage error", err)
	}
}

func TestLoadIndex_ParsesEmbeddedData(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}
	if len(endpoints) < 1000 {
		t.Errorf("expected 1000+ endpoints, got %d", len(endpoints))
	}
}

func TestLoadIndex_HasExpectedEndpoints(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}

	expected := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/apps"},
		{"GET", "/v1/builds"},
		{"POST", "/v1/bundleIds"},
		{"DELETE", "/v1/profiles/{id}"},
	}

	index := make(map[string]bool)
	for _, e := range endpoints {
		index[e.Method+" "+e.Path] = true
	}

	for _, want := range expected {
		key := want.method + " " + want.path
		if !index[key] {
			t.Errorf("expected endpoint %s not found", key)
		}
	}
}

func TestMatchEndpoint_PathSubstring(t *testing.T) {
	e := Endpoint{Method: "GET", Path: "/v1/apps/{id}/builds"}
	if !matchEndpoint(e, "builds") {
		t.Error("expected match for 'builds'")
	}
	if matchEndpoint(e, "certificates") {
		t.Error("unexpected match for 'certificates'")
	}
}

func TestMatchEndpoint_MethodAndPath(t *testing.T) {
	e := Endpoint{Method: "POST", Path: "/v1/apps"}
	if !matchEndpoint(e, "POST /v1/apps") {
		t.Error("expected match for 'POST /v1/apps'")
	}
	if matchEndpoint(e, "DELETE /v1/apps") {
		t.Error("unexpected match for 'DELETE /v1/apps'")
	}
	prefix := Endpoint{Method: "POST", Path: "/v1/apps/{id}/appInfos"}
	if matchEndpoint(prefix, "POST /v1/apps") {
		t.Error("unexpected prefix match for exact method and path query")
	}
}

func TestMatchEndpoint_DotNotation(t *testing.T) {
	e := Endpoint{Method: "GET", Path: "/v1/apps/{id}/builds"}
	if !matchEndpoint(e, "apps.builds") {
		t.Error("expected match for dot notation 'apps.builds'")
	}
	collection := Endpoint{Method: "GET", Path: "/v1/apps"}
	if !matchEndpoint(collection, "apps.list") {
		t.Error("expected match for action dot notation 'apps.list'")
	}
	member := Endpoint{Method: "GET", Path: "/v1/apps/{id}"}
	if matchEndpoint(member, "apps.list") {
		t.Error("unexpected list match for member endpoint")
	}
	versioned := Endpoint{Method: "GET", Path: "/v2/gameCenterAchievements/{id}", getAction: "get"}
	if !matchEndpoint(versioned, "v2.gameCenterAchievements.get") {
		t.Error("expected exact version-qualified action dot notation match")
	}
	if matchEndpoint(versioned, "v1.gameCenterAchievements.get") {
		t.Error("unexpected match for a different API version")
	}
}

func TestMatchEndpoint_FuzzyQueryDoesNotMatchActionSuffix(t *testing.T) {
	tests := []struct {
		query    string
		endpoint Endpoint
	}{
		{query: "list", endpoint: Endpoint{Method: "GET", Path: "/v1/apps", getAction: "list"}},
		{query: "create", endpoint: Endpoint{Method: "POST", Path: "/v1/apps"}},
		{query: "update", endpoint: Endpoint{Method: "PATCH", Path: "/v1/apps/{id}"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if matchEndpoint(tt.endpoint, tt.query) {
				t.Fatalf("bare fuzzy query %q matched synthesized action", tt.query)
			}
		})
	}
}

func TestMatchEndpoint_CaseInsensitive(t *testing.T) {
	e := Endpoint{Method: "GET", Path: "/v1/apps"}
	if !matchEndpoint(e, "APPS") {
		t.Error("expected case-insensitive match")
	}
}

func TestPathToDotNotation(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/v1/apps", "apps"},
		{"GET", "/v1/apps/{id}/builds", "apps.builds"},
		{"POST", "/v1/builds", "post:builds"},
		{"DELETE", "/v1/profiles/{id}", "delete:profiles"},
		{"GET", "/v2/inAppPurchases/{id}/pricePoints", "inAppPurchases.pricePoints"},
	}

	for _, tt := range tests {
		got := pathToDotNotation(tt.method, tt.path)
		if got != tt.want {
			t.Errorf("pathToDotNotation(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestPathToActionDotNotation(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{method: "GET", path: "/v1/apps", want: "apps.list"},
		{method: "GET", path: "/v1/apps/{id}", want: "apps.get"},
		{method: "GET", path: "/v1/apps/{id}/builds", want: "apps.builds.list"},
		{method: "POST", path: "/v1/apps", want: "apps.create"},
		{method: "PATCH", path: "/v1/apps/{id}", want: "apps.update"},
		{method: "DELETE", path: "/v1/apps/{id}", want: "apps.delete"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if got := pathToActionDotNotation(Endpoint{Method: tt.method, Path: tt.path}); got != tt.want {
				t.Fatalf("pathToActionDotNotation(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

func TestPathToActionDotNotationUsesOperationCardinality(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		getAction string
		want      string
	}{
		{name: "to one related", path: "/v1/builds/{id}/appStoreVersion", getAction: "get", want: "builds.appStoreVersion.get"},
		{name: "to many related", path: "/v1/apps/{id}/builds", getAction: "list", want: "apps.builds.list"},
		{name: "to one relationship", path: "/v1/builds/{id}/relationships/appStoreVersion", getAction: "get", want: "builds.relationships.appStoreVersion.get"},
		{name: "to many relationship", path: "/v1/apps/{id}/relationships/builds", getAction: "list", want: "apps.relationships.builds.list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := Endpoint{Method: "GET", Path: tt.path, getAction: tt.getAction}
			if got := pathToActionDotNotation(endpoint); got != tt.want {
				t.Fatalf("pathToActionDotNotation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathToVersionedActionDotNotation(t *testing.T) {
	endpoint := Endpoint{
		Method:    "GET",
		Path:      "/v2/gameCenterAchievements/{id}",
		getAction: "get",
	}
	if got, want := pathToVersionedActionDotNotation(endpoint), "v2.gameCenterAchievements.get"; got != want {
		t.Fatalf("pathToVersionedActionDotNotation() = %q, want %q", got, want)
	}
}

func TestLoadIndex_HasParameters(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}

	for _, e := range endpoints {
		if e.Method == "GET" && e.Path == "/v1/apps" {
			if len(e.Parameters) == 0 {
				t.Error("GET /v1/apps should have parameters")
			}
			return
		}
	}
	t.Error("GET /v1/apps not found")
}

func TestLoadIndex_HasResponseSchema(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}

	for _, e := range endpoints {
		if e.Method == "GET" && e.Path == "/v1/apps" {
			if e.ResponseSchema == "" {
				t.Error("GET /v1/apps should have responseSchema")
			}
			return
		}
	}
	t.Error("GET /v1/apps not found")
}

func TestLoadIndex_HasVersionCreateRequestRelationships(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}

	expected := map[string]struct {
		name         string
		resourceType string
	}{
		"/v1/inAppPurchaseVersions": {
			name:         "inAppPurchase",
			resourceType: "inAppPurchases",
		},
		"/v1/subscriptionVersions": {
			name:         "subscription",
			resourceType: "subscriptions",
		},
		"/v1/subscriptionGroupVersions": {
			name:         "subscriptionGroup",
			resourceType: "subscriptionGroups",
		},
	}

	found := make(map[string]bool, len(expected))
	for _, endpoint := range endpoints {
		want, ok := expected[endpoint.Path]
		if endpoint.Method != "POST" || !ok {
			continue
		}

		relationship, ok := endpoint.RequestRelationships[want.name]
		if !ok {
			t.Errorf("POST %s missing request relationship %q", endpoint.Path, want.name)
			continue
		}
		if relationship.ResourceType != want.resourceType {
			t.Errorf("POST %s relationship resourceType = %q, want %q", endpoint.Path, relationship.ResourceType, want.resourceType)
		}
		if relationship.Cardinality != "one" {
			t.Errorf("POST %s relationship cardinality = %q, want one", endpoint.Path, relationship.Cardinality)
		}
		if !relationship.Required {
			t.Errorf("POST %s relationship should be required", endpoint.Path)
		}
		found[endpoint.Path] = true
	}

	for path := range expected {
		if !found[path] {
			t.Errorf("POST %s not found", path)
		}
	}
}

func TestLoadIndex_HasToManyRequestRelationship(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}

	for _, endpoint := range endpoints {
		if endpoint.Method != "POST" || endpoint.Path != "/v1/profiles" {
			continue
		}
		relationship, ok := endpoint.RequestRelationships["certificates"]
		if !ok {
			t.Fatal("POST /v1/profiles missing certificates request relationship")
		}
		if relationship.ResourceType != "certificates" {
			t.Errorf("resourceType = %q, want certificates", relationship.ResourceType)
		}
		if relationship.Cardinality != "many" {
			t.Errorf("cardinality = %q, want many", relationship.Cardinality)
		}
		if !relationship.Required {
			t.Error("certificates relationship should be required")
		}
		return
	}

	t.Error("POST /v1/profiles not found")
}

func TestLoadIndex_IncludesPathLevelIDParameter(t *testing.T) {
	endpoints, err := loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error: %v", err)
	}

	for _, e := range endpoints {
		if e.Method != "GET" || e.Path != "/v1/apps/{id}" {
			continue
		}
		for _, p := range e.Parameters {
			if p.Name == "id" && p.In == "path" {
				if !p.Required {
					t.Error("path parameter id should be required")
				}
				return
			}
		}
		t.Fatal("GET /v1/apps/{id} should include path parameter id")
	}

	t.Error("GET /v1/apps/{id} not found")
}

func TestNormalizeMethodFilter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "lowercase", input: "get", want: "GET"},
		{name: "surrounded whitespace", input: " post ", want: "POST"},
		{name: "invalid", input: "DELTE", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMethodFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeMethodFilter(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeMethodFilter(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeMethodFilter(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
