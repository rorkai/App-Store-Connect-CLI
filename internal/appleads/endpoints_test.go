package appleads

import (
	"strings"
	"testing"
)

func TestPlatformEndpointSpecsCoverAccountAndAppSurface(t *testing.T) {
	specs := PlatformEndpointSpecs()

	wants := map[string]struct {
		method  string
		path    string
		context ContextKind
	}{
		"me view":                       {"GET", "v1/me", ContextNone},
		"acls list":                     {"GET", "v1/acls", ContextNone},
		"orgs view":                     {"GET", "v1/orgs/{id}", ContextNone},
		"ad-accounts create":            {"POST", "v1/ad-accounts", ContextNone},
		"ad-accounts view":              {"GET", "v1/ad-accounts/{id}", ContextAdAccount},
		"ad-accounts update":            {"PUT", "v1/ad-accounts/{id}", ContextAdAccount},
		"advertiser-resources list":     {"GET", "v1/advertiser-resources", ContextNone},
		"apps search":                   {"GET", "v1/search/apps", ContextAdAccount},
		"apps view":                     {"GET", "v1/apps/{adamId}", ContextAdAccount},
		"apps supported-languages find": {"POST", "v1/metadata/apps/supported-languages/query", ContextAdAccount},
		"apps eligibility find":         {"POST", "v1/eligibilities/apps/query", ContextAdAccount},
		"rejection-reasons apps find":   {"POST", "v1/rejection-reasons/apps/query", ContextAdAccount},
		"rejection-reasons apps view":   {"GET", "v1/rejection-reasons/apps/{rejectionReasonId}", ContextAdAccount},
	}

	for _, spec := range specs {
		key := strings.Join(spec.CommandPath, " ")
		want, ok := wants[key]
		if !ok {
			continue
		}
		if spec.Version != APIVersionPlatformV1 || spec.Method != want.method || spec.Path != want.path || spec.Context != want.context {
			t.Fatalf("%s contract = %s %s version=%q context=%v", key, spec.Method, spec.Path, spec.Version, spec.Context)
		}
		delete(wants, key)
	}
	if len(wants) != 0 {
		t.Fatalf("missing platform commands: %+v", wants)
	}

	create, _ := PlatformEndpointByCommandPath("ad-accounts", "create")
	update, _ := PlatformEndpointByCommandPath("ad-accounts", "update")
	if create.BodyOptional || update.BodyOptional || create.BodyKind != BodyObject || update.BodyKind != BodyObject {
		t.Fatal("ad-account create/update must require JSON object files")
	}
	if update.ConfirmBodyField != "delegations" {
		t.Fatalf("ad-account update confirm field = %q, want delegations", update.ConfirmBodyField)
	}
	if !create.RiskConfirm {
		t.Fatal("ad-account create must require risk confirmation")
	}
	if create.RiskConfirmBodyField != "" || create.RiskConfirmBodyValue != "" {
		t.Fatalf("ad-account create risk exemption = %q=%q, want unconditional confirmation", create.RiskConfirmBodyField, create.RiskConfirmBodyValue)
	}

	for _, path := range [][]string{{"apps", "supported-languages", "find"}, {"apps", "eligibility", "find"}, {"rejection-reasons", "apps", "find"}} {
		spec, ok := PlatformEndpointByCommandPath(path...)
		if !ok || !spec.BodyOptional || !spec.RetrySafe {
			t.Fatalf("%q query body must be optional and retry-safe", strings.Join(path, " "))
		}
	}
}

func findQueryParam(spec EndpointSpec, name string) ParamSpec {
	for _, param := range spec.QueryParams {
		if param.Name == name {
			return param
		}
	}
	return ParamSpec{}
}
