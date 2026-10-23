package appleads

import (
	"net/http"
	"strings"
)

// BodyKind describes the JSON body shape accepted by an Apple Ads endpoint.
type BodyKind string

const (
	BodyNone      BodyKind = ""
	BodyObject    BodyKind = "object"
	BodyArray     BodyKind = "array"
	BodyMultipart BodyKind = "multipart"
)

// ParamType describes the primitive type of a path or query parameter.
type ParamType string

const (
	ParamString ParamType = "string"
	ParamInt    ParamType = "int"
	ParamBool   ParamType = "bool"
)

// ParamSpec describes a documented Apple Ads path or query parameter.
type ParamSpec struct {
	Name         string
	Flag         string
	Type         ParamType
	Required     bool
	Repeated     bool
	ContextValue bool
	Max          int
	Allowed      []string
	Description  string
	Default      int
}

// ReadOnlyRequest reports whether the endpoint only reads Apple Ads data.
// Apple transports selector lookups (/find, /query), reporting, and search as
// POST requests, so read-only mode consults this instead of the method alone.
func (spec EndpointSpec) ReadOnlyRequest() bool {
	switch spec.Method {
	case http.MethodGet, http.MethodHead:
		return true
	case http.MethodPost:
	default:
		return false
	}
	path := strings.TrimPrefix(strings.TrimSpace(spec.Path), "/")
	return strings.HasSuffix(path, "/find") ||
		strings.HasSuffix(path, "/query") ||
		// Geolocation resolution in the search namespace is a POST read.
		strings.HasPrefix(path, "v1/search/")
}

// EndpointSpec is the single source of truth for the Apple Ads command and
// client surface.
type EndpointSpec struct {
	Name         string
	Method       string
	Path         string
	Version      APIVersion
	Context      ContextKind
	CommandPath  []string
	BodyKind     BodyKind
	BodyOptional bool
	// CLIRequiresBody records a client-side requirement that is stricter than
	// the SDK/OpenAPI request contract. Apple accepts an empty selector body on
	// some query endpoints, but the Platform API requires a selector filter for
	// the keyword queries exposed by asc.
	CLIRequiresBody bool
	BodyType        string
	BodyHint        string
	BodyFileExample string
	// BodyExample is a minimal valid request payload rendered in command help
	// so callers can start from a working body instead of external schema docs.
	BodyExample      string
	ResponseType     string
	RequiresConfirm  bool
	ConfirmBodyField string
	// RiskConfirm separates potential spend, billing, delivery, targeting, or
	// access impact acknowledgement from destructive confirmation. A body
	// field/value pair can exempt a documented safe payload from the
	// acknowledgement. When RiskConfirmAllowedBodyFields is set, every field
	// in the safe payload object must be listed before acknowledgement can be
	// skipped.
	RiskConfirm                  bool
	RiskConfirmBodyField         string
	RiskConfirmBodyValue         string
	RiskConfirmAllowedBodyFields []string
	RetrySafe                    bool
	PathParams                   []ParamSpec
	QueryParams                  []ParamSpec
	SupportsPaginate             bool
}

// PlatformEndpointSpecs returns the implemented Apple Ads Platform API v1
// resource surface.
func PlatformEndpointSpecs() []EndpointSpec {
	id := ParamSpec{Name: "id", Flag: "ad-account", Type: ParamString, Required: true, ContextValue: true}
	orgID := ParamSpec{Name: "id", Flag: "org-id", Type: ParamString, Required: true}
	rejectionReasonID := ParamSpec{Name: "rejectionReasonId", Flag: "reason", Type: ParamInt, Required: true}
	searchLimit := ParamSpec{
		Name:        "limit",
		Flag:        "limit",
		Type:        ParamInt,
		Description: "Maximum results to return",
		Default:     20,
	}
	searchOffset := ParamSpec{
		Name:        "offset",
		Flag:        "offset",
		Type:        ParamInt,
		Description: "Zero-based result offset for pagination",
	}
	searchQuery := q("query", "query", ParamString, false)
	searchQuery.Description = "Free-text app name or developer-name search"
	searchCPIDs := q("cpids", "cpids", ParamString, false)
	searchCPIDs.Description = "Comma-separated iTunes content provider IDs"
	searchOwned := q("returnOwnedApps", "return-owned-apps", ParamBool, false)
	searchOwned.Description = "Return apps owned by this organization"
	searchStorefronts := ParamSpec{
		Name:        "storeFronts",
		Flag:        "store-fronts",
		Type:        ParamString,
		Repeated:    true,
		Description: "Comma-separated ISO 3166-1 alpha-2 storefront codes",
	}

	specs := []EndpointSpec{
		platformEndpoint("platform-get-me-details", "GET", "v1/me", []string{"me", "view"}, ContextNone, BodyNone, false, "", "MeResponse", nil, nil),
		platformEndpoint("platform-get-user-acls", "GET", "v1/acls", []string{"acls", "list"}, ContextNone, BodyNone, false, "", "UserAclListResponse", nil, nil),
		platformEndpoint("platform-get-org", "GET", "v1/orgs/{id}", []string{"orgs", "view"}, ContextNone, BodyNone, false, "", "OrgResponse", []ParamSpec{orgID}, nil),
		platformEndpoint("platform-create-ad-account", "POST", "v1/ad-accounts", []string{"ad-accounts", "create"}, ContextNone, BodyObject, false, "AdAccountCreate", "AdAccountResponse", nil, nil),
		platformEndpoint("platform-get-ad-account", "GET", "v1/ad-accounts/{id}", []string{"ad-accounts", "view"}, ContextAdAccount, BodyNone, false, "", "AdAccountResponse", []ParamSpec{id}, nil),
		platformEndpoint("platform-update-ad-account", "PUT", "v1/ad-accounts/{id}", []string{"ad-accounts", "update"}, ContextAdAccount, BodyObject, false, "AdAccountUpdate", "AdAccountResponse", []ParamSpec{id}, nil),
		platformEndpoint("platform-get-advertiser-resources", "GET", "v1/advertiser-resources", []string{"advertiser-resources", "list"}, ContextNone, BodyNone, false, "", "AdvertiserResourceListResponse", nil, []ParamSpec{{Name: "resourceType", Flag: "resource-type", Type: ParamString, Required: true, Allowed: []string{"CONTENT_PROVIDER", "BUSINESS_BRAND"}}}),
		platformEndpoint("platform-search-apps", "GET", "v1/search/apps", []string{"apps", "search"}, ContextAdAccount, BodyNone, false, "", "AppsSearchResponse", nil, []ParamSpec{
			searchQuery,
			searchOwned,
			searchCPIDs,
			searchStorefronts,
			searchOffset,
			searchLimit,
		}),
		platformEndpoint("platform-get-app", "GET", "v1/apps/{adamId}", []string{"apps", "view"}, ContextAdAccount, BodyNone, false, "", "AppDetailsResponse", []ParamSpec{adamIDParam}, nil),
		retrySafePlatformEndpoint(platformEndpoint("platform-query-supported-app-languages", "POST", "v1/metadata/apps/supported-languages/query", []string{"apps", "supported-languages", "find"}, ContextAdAccount, BodyObject, true, "QueryRequest", "AppSupportedLanguagesQueryResponse", nil, nil)),
		retrySafePlatformEndpoint(platformEndpoint("platform-find-app-eligibilities", "POST", "v1/eligibilities/apps/query", []string{"apps", "eligibility", "find"}, ContextAdAccount, BodyObject, true, "EligibilityQueryRequest", "EligibilityQueryResponse", nil, nil)),
		retrySafePlatformEndpoint(platformEndpoint("platform-find-app-rejection-reasons", "POST", "v1/rejection-reasons/apps/query", []string{"rejection-reasons", "apps", "find"}, ContextAdAccount, BodyObject, true, "CreativeRejectionReasonQueryRequest", "CreativeRejectionReasonQueryResponse", nil, nil)),
		platformEndpoint("platform-get-app-rejection-reason", "GET", "v1/rejection-reasons/apps/{rejectionReasonId}", []string{"rejection-reasons", "apps", "view"}, ContextAdAccount, BodyNone, false, "", "RejectionReasonResponse", []ParamSpec{rejectionReasonID}, nil),
	}
	specs = append(specs, platformMapsEndpointSpecs()...)
	specs = append(specs, platformCampaignEndpointSpecs()...)

	for i := range specs {
		if specs[i].Name == "platform-search-apps" || specs[i].Name == "platform-search-geo-locations" {
			specs[i].SupportsPaginate = true
		}
		if specs[i].Name == "platform-resolve-geo-locations" {
			specs[i].RetrySafe = true
		}
		if specs[i].Name == "platform-update-ad-account" {
			specs[i].ConfirmBodyField = "delegations"
		}
		if specs[i].Name == "platform-create-ad-account" {
			specs[i].RiskConfirm = true
		}
	}
	return append(specs, platformReportsOptimizationEndpointSpecs()...)
}

func platformEndpoint(name, method, path string, commandPath []string, context ContextKind, bodyKind BodyKind, bodyOptional bool, bodyType, responseType string, pathParams, queryParams []ParamSpec) EndpointSpec {
	return EndpointSpec{
		Name:         name,
		Method:       method,
		Path:         path,
		Version:      APIVersionPlatformV1,
		Context:      context,
		CommandPath:  append([]string(nil), commandPath...),
		BodyKind:     bodyKind,
		BodyOptional: bodyOptional,
		BodyType:     bodyType,
		ResponseType: responseType,
		PathParams:   append([]ParamSpec(nil), pathParams...),
		QueryParams:  append([]ParamSpec(nil), queryParams...),
	}
}

func retrySafePlatformEndpoint(spec EndpointSpec) EndpointSpec {
	spec.RetrySafe = true
	return spec
}

// PlatformEndpointByCommandPath returns a Platform API v1 spec by command path.
func PlatformEndpointByCommandPath(path ...string) (EndpointSpec, bool) {
	joined := strings.Join(path, " ")
	for _, spec := range PlatformEndpointSpecs() {
		if strings.Join(spec.CommandPath, " ") == joined {
			return spec, true
		}
	}
	return EndpointSpec{}, false
}

var (
	adamIDParam      = ParamSpec{Name: "adamId", Flag: "adam-id", Type: ParamInt, Required: true}
	productPageParam = ParamSpec{Name: "productPageId", Flag: "product-page", Type: ParamString, Required: true}
)

func q(name, flag string, typ ParamType, required bool) ParamSpec {
	return ParamSpec{Name: name, Flag: flag, Type: typ, Required: required}
}
