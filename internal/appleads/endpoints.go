package appleads

import "strings"

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
	Aliases      []string
	Type         ParamType
	Required     bool
	Repeated     bool
	ContextValue bool
	Max          int
	Allowed      []string
	Description  string
	Default      int
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
	RequiresOrg      bool
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
	DefaultListAlias             bool
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

const maxAppleAdsPageLimit = 1000

var (
	adamIDParam          = ParamSpec{Name: "adamId", Flag: "adam-id", Type: ParamInt, Required: true}
	adGroupParam         = ParamSpec{Name: "adgroupId", Flag: "ad-group", Type: ParamInt, Required: true}
	adParam              = ParamSpec{Name: "adId", Flag: "ad", Type: ParamInt, Required: true}
	budgetOrderParam     = ParamSpec{Name: "boId", Flag: "budget-order", Type: ParamInt, Required: true}
	campaignParam        = ParamSpec{Name: "campaignId", Flag: "campaign", Type: ParamInt, Required: true}
	creativeParam        = ParamSpec{Name: "creativeId", Flag: "creative", Type: ParamInt, Required: true}
	keywordParam         = ParamSpec{Name: "keywordId", Flag: "keyword", Type: ParamInt, Required: true}
	negativeKeywordParam = ParamSpec{Name: "keywordId", Flag: "negative-keyword", Aliases: []string{"keyword"}, Type: ParamInt, Required: true}
	productPageParam     = ParamSpec{Name: "productPageId", Flag: "product-page", Type: ParamString, Required: true}
	reasonParam          = ParamSpec{Name: "productPageReasonId", Flag: "reason", Type: ParamInt, Required: true}
	reportParam          = ParamSpec{Name: "reportId", Flag: "report", Type: ParamInt, Required: true}

	limitParam  = ParamSpec{Name: "limit", Flag: "limit", Type: ParamInt, Max: maxAppleAdsPageLimit}
	offsetParam = ParamSpec{Name: "offset", Flag: "offset", Type: ParamInt}
)

func q(name, flag string, typ ParamType, required bool) ParamSpec {
	return ParamSpec{Name: name, Flag: flag, Type: typ, Required: required}
}

func qAllowed(name, flag string, allowed ...string) ParamSpec {
	return ParamSpec{Name: name, Flag: flag, Type: ParamString, Allowed: append([]string(nil), allowed...)}
}

func qLimitOffset() []ParamSpec {
	return []ParamSpec{limitParam, offsetParam}
}

func endpoint(name, method, path string, commandPath []string, bodyKind BodyKind, bodyType, responseType string, pathParams []ParamSpec, queryParams []ParamSpec) EndpointSpec {
	return EndpointSpec{
		Name:         name,
		Method:       method,
		Path:         path,
		CommandPath:  append([]string(nil), commandPath...),
		BodyKind:     bodyKind,
		BodyType:     bodyType,
		ResponseType: responseType,
		RequiresOrg:  true,
		PathParams:   append([]ParamSpec(nil), pathParams...),
		QueryParams:  append([]ParamSpec(nil), queryParams...),
	}
}

// EndpointSpecs returns the current Apple Ads Campaign Management API v5 surface.
func EndpointSpecs() []EndpointSpec {
	specs := []EndpointSpec{
		endpoint("get-user-acl", "GET", "v5/acls", []string{"acls", "list"}, BodyNone, "", "UserAclListResponse", nil, nil),
		endpoint("get-me-details", "GET", "v5/me", []string{"me", "view"}, BodyNone, "", "MeDetailResponse", nil, nil),

		endpoint("search-for-ios-apps", "GET", "v5/search/apps", []string{"apps", "search"}, BodyNone, "", "AppInfoListResponse", nil, []ParamSpec{limitParam, offsetParam, q("query", "query", ParamString, true), q("returnOwnedApps", "return-owned-apps", ParamBool, false)}),
		endpoint("get-app-details", "GET", "v5/apps/{adamId}", []string{"apps", "view"}, BodyNone, "", "MediaDetailResponse", []ParamSpec{adamIDParam}, nil),
		endpoint("get-localized-app-details", "GET", "v5/apps/{adamId}/locale-details", []string{"apps", "localized-details"}, BodyNone, "", "MediaLocaleDetailResponse", []ParamSpec{adamIDParam}, nil),
		endpoint("find-app-eligibility-records", "POST", "v5/apps/{adamId}/eligibilities/find", []string{"apps", "eligibility", "find"}, BodyObject, "Selector", "EligibilityRecordListResponse", []ParamSpec{adamIDParam}, nil),
		endpoint("find-app-assets", "POST", "v5/apps/{adamId}/assets/find", []string{"apps", "assets", "find"}, BodyObject, "Selector", "AppAssetListResponse", []ParamSpec{adamIDParam}, nil),

		endpoint("get-product-pages", "GET", "v5/apps/{adamId}/product-pages", []string{"product-pages", "list"}, BodyNone, "", "ProductPageDetailListResponse", []ParamSpec{adamIDParam}, []ParamSpec{q("name", "name", ParamString, false), qAllowed("states", "states", "HIDDEN", "VISIBLE")}),
		endpoint("get-product-pages-by-identifier", "GET", "v5/apps/{adamId}/product-pages/{productPageId}", []string{"product-pages", "view"}, BodyNone, "", "ProductPageDetailResponse", []ParamSpec{adamIDParam, productPageParam}, nil),
		endpoint("get-product-page-locales", "GET", "v5/apps/{adamId}/product-pages/{productPageId}/locale-details", []string{"product-pages", "locales", "list"}, BodyNone, "", "ProductPageLocaleDetailListResponse", []ParamSpec{adamIDParam, productPageParam}, []ParamSpec{qAllowed("deviceClasses", "device-classes", "IPAD", "IPHONE"), q("expand", "expand", ParamBool, false), q("languageCodes", "language-codes", ParamString, false), q("languages", "languages", ParamString, false)}),
		endpoint("get-supported-countries-or-regions", "GET", "v5/countries-or-regions", []string{"product-pages", "countries", "list"}, BodyNone, "", "CountriesOrRegionsListResponse", nil, []ParamSpec{q("countriesOrRegions", "countries-or-regions", ParamString, false)}),
		endpoint("get-app-preview-device-sizes", "GET", "v5/creativeappmappings/devices", []string{"product-pages", "devices", "list"}, BodyNone, "", "AppPreviewDevicesMappingResponse", nil, nil),

		endpoint("get-all-budget-orders", "GET", "v5/budgetorders", []string{"budget-orders", "list"}, BodyNone, "", "BudgetOrderInfoListResponse", nil, qLimitOffset()),
		endpoint("create-a-budget-order", "POST", "v5/budgetorders", []string{"budget-orders", "create"}, BodyObject, "BudgetOrderCreate", "BudgetOrderInfoResponse", nil, nil),
		endpoint("get-a-budget-order", "GET", "v5/budgetorders/{boId}", []string{"budget-orders", "view"}, BodyNone, "", "BudgetOrderInfoResponse", []ParamSpec{budgetOrderParam}, nil),
		endpoint("update-a-budget-order", "PUT", "v5/budgetorders/{boId}", []string{"budget-orders", "update"}, BodyObject, "BudgetOrderUpdate", "BudgetOrderInfoResponse", []ParamSpec{budgetOrderParam}, nil),

		endpoint("get-all-campaigns", "GET", "v5/campaigns", []string{"campaigns", "list"}, BodyNone, "", "CampaignListResponse", nil, qLimitOffset()),
		endpoint("create-a-campaign", "POST", "v5/campaigns", []string{"campaigns", "create"}, BodyObject, "Campaign", "CampaignResponse", nil, nil),
		endpoint("find-campaigns", "POST", "v5/campaigns/find", []string{"campaigns", "find"}, BodyObject, "Selector", "CampaignListResponse", nil, nil),
		endpoint("delete-a-campaign", "DELETE", "v5/campaigns/{campaignId}", []string{"campaigns", "delete"}, BodyNone, "", "VoidResponse", []ParamSpec{campaignParam}, nil),
		endpoint("get-a-campaign", "GET", "v5/campaigns/{campaignId}", []string{"campaigns", "view"}, BodyNone, "", "CampaignResponse", []ParamSpec{campaignParam}, nil),
		endpoint("update-a-campaign", "PUT", "v5/campaigns/{campaignId}", []string{"campaigns", "update"}, BodyObject, "UpdateCampaignRequest", "CampaignResponse", []ParamSpec{campaignParam}, nil),

		endpoint("get-all-ad-groups", "GET", "v5/campaigns/{campaignId}/adgroups", []string{"ad-groups", "list"}, BodyNone, "", "AdGroupListResponse", []ParamSpec{campaignParam}, qLimitOffset()),
		endpoint("create-an-ad-group", "POST", "v5/campaigns/{campaignId}/adgroups", []string{"ad-groups", "create"}, BodyObject, "AdGroup", "AdGroupResponse", []ParamSpec{campaignParam}, nil),
		endpoint("find-ad-groups", "POST", "v5/campaigns/{campaignId}/adgroups/find", []string{"ad-groups", "find"}, BodyObject, "Selector", "AdGroupListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("find-ad-groups-across-organization", "POST", "v5/adgroups/find", []string{"ad-groups", "find-org"}, BodyObject, "Selector", "AdGroupListResponse", nil, nil),
		endpoint("delete-an-ad-group", "DELETE", "v5/campaigns/{campaignId}/adgroups/{adgroupId}", []string{"ad-groups", "delete"}, BodyNone, "", "VoidResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("get-an-ad-group", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}", []string{"ad-groups", "view"}, BodyNone, "", "AdGroupResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("update-an-ad-group", "PUT", "v5/campaigns/{campaignId}/adgroups/{adgroupId}", []string{"ad-groups", "update"}, BodyObject, "AdGroupUpdate", "AdGroupResponse", []ParamSpec{campaignParam, adGroupParam}, nil),

		endpoint("get-all-ads", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads", []string{"ads", "list"}, BodyNone, "", "AdListResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("create-an-ad", "POST", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads", []string{"ads", "create"}, BodyObject, "AdCreate", "AdResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("delete-an-ad", "DELETE", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads/{adId}", []string{"ads", "delete"}, BodyNone, "", "VoidResponse", []ParamSpec{campaignParam, adGroupParam, adParam}, nil),
		endpoint("get-an-ad", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads/{adId}", []string{"ads", "view"}, BodyNone, "", "AdResponse", []ParamSpec{campaignParam, adGroupParam, adParam}, nil),
		endpoint("update-an-ad", "PUT", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads/{adId}", []string{"ads", "update"}, BodyObject, "AdUpdate", "AdResponse", []ParamSpec{campaignParam, adGroupParam, adParam}, nil),
		endpoint("find-ads", "POST", "v5/campaigns/{campaignId}/ads/find", []string{"ads", "find"}, BodyObject, "Selector", "AdListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("find-ads-across-organization", "POST", "v5/ads/find", []string{"ads", "find-org"}, BodyObject, "Selector", "AdListResponse", nil, nil),

		endpoint("get-all-targeting-keywords-in-an-ad-group", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords", []string{"targeting-keywords", "list"}, BodyNone, "", "KeywordListResponse", []ParamSpec{campaignParam, adGroupParam}, qLimitOffset()),
		endpoint("find-targeting-keywords-in-a-campaign", "POST", "v5/campaigns/{campaignId}/adgroups/targetingkeywords/find", []string{"targeting-keywords", "find"}, BodyObject, "Selector", "KeywordListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("get-a-targeting-keyword-in-an-ad-group", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/{keywordId}", []string{"targeting-keywords", "view"}, BodyNone, "", "KeywordResponse", []ParamSpec{campaignParam, adGroupParam, keywordParam}, nil),
		endpoint("create-targeting-keywords", "POST", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/bulk", []string{"targeting-keywords", "create-bulk"}, BodyArray, "[Keyword]", "KeywordListResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("update-targeting-keywords", "PUT", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/bulk", []string{"targeting-keywords", "update-bulk"}, BodyArray, "[KeywordUpdateRequest]", "KeywordListResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("delete-a-targeting-keyword", "DELETE", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/{keywordId}", []string{"targeting-keywords", "delete"}, BodyNone, "", "VoidResponse", []ParamSpec{campaignParam, adGroupParam, keywordParam}, nil),
		endpoint("delete-targeting-keywords", "POST", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/delete/bulk", []string{"targeting-keywords", "delete-bulk"}, BodyArray, "[int64]", "IntegerResponse", []ParamSpec{campaignParam, adGroupParam}, nil),

		endpoint("get-all-campaign-negative-keywords", "GET", "v5/campaigns/{campaignId}/negativekeywords", []string{"campaign-negative-keywords", "list"}, BodyNone, "", "NegativeKeywordListResponse", []ParamSpec{campaignParam}, qLimitOffset()),
		endpoint("find-campaign-negative-keywords", "POST", "v5/campaigns/{campaignId}/negativekeywords/find", []string{"campaign-negative-keywords", "find"}, BodyObject, "Selector", "NegativeKeywordListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("get-a-campaign-negative-keyword", "GET", "v5/campaigns/{campaignId}/negativekeywords/{keywordId}", []string{"campaign-negative-keywords", "view"}, BodyNone, "", "NegativeKeywordResponse", []ParamSpec{campaignParam, negativeKeywordParam}, nil),
		endpoint("create-campaign-negative-keywords", "POST", "v5/campaigns/{campaignId}/negativekeywords/bulk", []string{"campaign-negative-keywords", "create-bulk"}, BodyArray, "[NegativeKeyword]", "NegativeKeywordListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("update-campaign-negative-keywords", "PUT", "v5/campaigns/{campaignId}/negativekeywords/bulk", []string{"campaign-negative-keywords", "update-bulk"}, BodyArray, "[NegativeKeyword]", "NegativeKeywordListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("delete-campaign-negative-keywords", "POST", "v5/campaigns/{campaignId}/negativekeywords/delete/bulk", []string{"campaign-negative-keywords", "delete-bulk"}, BodyArray, "[int64]", "IntegerResponse", []ParamSpec{campaignParam}, nil),

		endpoint("get-all-ad-group-negative-keywords", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords", []string{"ad-group-negative-keywords", "list"}, BodyNone, "", "NegativeKeywordListResponse", []ParamSpec{campaignParam, adGroupParam}, qLimitOffset()),
		endpoint("find-ad-group-negative-keywords", "POST", "v5/campaigns/{campaignId}/adgroups/negativekeywords/find", []string{"ad-group-negative-keywords", "find"}, BodyObject, "Selector", "NegativeKeywordListResponse", []ParamSpec{campaignParam}, nil),
		endpoint("get-an-ad-group-negative-keyword", "GET", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/{keywordId}", []string{"ad-group-negative-keywords", "view"}, BodyNone, "", "NegativeKeywordResponse", []ParamSpec{campaignParam, adGroupParam, negativeKeywordParam}, nil),
		endpoint("create-ad-group-negative-keywords", "POST", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/bulk", []string{"ad-group-negative-keywords", "create-bulk"}, BodyArray, "[NegativeKeyword]", "NegativeKeywordListResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("update-ad-group-negative-keywords", "PUT", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/bulk", []string{"ad-group-negative-keywords", "update-bulk"}, BodyArray, "[NegativeKeyword]", "NegativeKeywordListResponse", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("delete-ad-group-negative-keywords", "POST", "v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/delete/bulk", []string{"ad-group-negative-keywords", "delete-bulk"}, BodyArray, "[int64]", "IntegerResponse", []ParamSpec{campaignParam, adGroupParam}, nil),

		endpoint("search-for-geolocations", "GET", "v5/search/geo", []string{"geo", "search"}, BodyNone, "", "SearchEntityListResponse", nil, []ParamSpec{q("countrycode", "country-code", ParamString, false), q("entity", "entity", ParamString, false), limitParam, offsetParam, q("query", "query", ParamString, false)}),
		endpoint("get-a-list-of-geo-locations", "POST", "v5/search/geo", []string{"geo", "resolve"}, BodyArray, "[GeoRequest]", "SearchEntityListResponse", nil, qLimitOffset()),

		endpoint("get-all-creatives", "GET", "v5/creatives", []string{"creatives", "list"}, BodyNone, "", "CreativeListResponse", nil, qLimitOffset()),
		endpoint("create-a-creative", "POST", "v5/creatives", []string{"creatives", "create"}, BodyObject, "(CustomProductPageCreative | DefaultProductPageCreative)", "CreativeResponse", nil, nil),
		endpoint("find-creatives", "POST", "v5/creatives/find", []string{"creatives", "find"}, BodyObject, "Selector", "CreativeListResponse", nil, nil),
		endpoint("get-a-creative", "GET", "v5/creatives/{creativeId}", []string{"creatives", "view"}, BodyNone, "", "CreativeResponse", []ParamSpec{creativeParam}, []ParamSpec{q("includeDeletedCreativeSetAssets", "include-deleted-creative-set-assets", ParamBool, false)}),

		endpoint("find-ad-creative-rejection-reasons", "POST", "v5/product-page-reasons/find", []string{"rejection-reasons", "find"}, BodyObject, "Selector", "ProductPageReasonListResponse", nil, nil),
		endpoint("gets-a-product-page-reason", "GET", "v5/product-page-reasons/{productPageReasonId}", []string{"rejection-reasons", "view"}, BodyNone, "", "ProductPageReasonResponse", []ParamSpec{reasonParam}, nil),

		endpoint("get-campaign-level-reports", "POST", "v5/reports/campaigns", []string{"reports", "campaigns"}, BodyObject, "ReportingRequest", "ReportingResponseBody", nil, nil),
		endpoint("get-ad-group-level-reports", "POST", "v5/reports/campaigns/{campaignId}/adgroups", []string{"reports", "ad-groups"}, BodyObject, "ReportingRequest", "ReportingResponseBody", []ParamSpec{campaignParam}, nil),
		endpoint("get-keyword-level-reports", "POST", "v5/reports/campaigns/{campaignId}/keywords", []string{"reports", "keywords"}, BodyObject, "ReportingRequest", "ReportingResponseBody", []ParamSpec{campaignParam}, nil),
		endpoint("get-search-term-level-reports", "POST", "v5/reports/campaigns/{campaignId}/searchterms", []string{"reports", "search-terms"}, BodyObject, "ReportingRequest", "ReportingResponseBody", []ParamSpec{campaignParam}, nil),
		endpoint("get-ad-level-reports", "POST", "v5/reports/campaigns/{campaignId}/ads", []string{"reports", "ads"}, BodyObject, "ReportingRequest", "ReportingResponseBody", []ParamSpec{campaignParam}, nil),
		endpoint("get-keyword-level-within-ad-group-reports", "POST", "v5/reports/campaigns/{campaignId}/adgroups/{adgroupId}/keywords", []string{"reports", "ad-group-keywords"}, BodyObject, "ReportingRequest", "ReportingResponseBody", []ParamSpec{campaignParam, adGroupParam}, nil),
		endpoint("get-search-term-level-within-ad-group-reports", "POST", "v5/reports/campaigns/{campaignId}/adgroups/{adgroupId}/searchterms", []string{"reports", "ad-group-search-terms"}, BodyObject, "ReportingRequest", "ReportingResponseBody", []ParamSpec{campaignParam, adGroupParam}, nil),

		endpoint("get-all-impression-share-reports", "GET", "v5/custom-reports", []string{"impression-share-reports", "list"}, BodyNone, "", "CustomReportResponseBody", nil, []ParamSpec{q("field", "field", ParamString, false), {Name: "limit", Flag: "limit", Type: ParamInt, Max: 50}, offsetParam, q("sortOrder", "sort-order", ParamString, false)}),
		endpoint("impression-share-report", "POST", "v5/custom-reports", []string{"impression-share-reports", "create"}, BodyObject, "CustomReportRequest", "CustomReportResponseBody", nil, nil),
		endpoint("get-a-single-impression-share-report", "GET", "v5/custom-reports/{reportId}", []string{"impression-share-reports", "view"}, BodyNone, "", "CustomReportResponseBody", []ParamSpec{reportParam}, nil),
	}
	riskConfirmNames := map[string]struct{}{
		"create-a-budget-order":             {},
		"update-a-budget-order":             {},
		"create-a-campaign":                 {},
		"update-a-campaign":                 {},
		"create-an-ad-group":                {},
		"update-an-ad-group":                {},
		"create-an-ad":                      {},
		"update-an-ad":                      {},
		"create-targeting-keywords":         {},
		"update-targeting-keywords":         {},
		"create-campaign-negative-keywords": {},
		"update-campaign-negative-keywords": {},
		"create-ad-group-negative-keywords": {},
		"update-ad-group-negative-keywords": {},
	}

	for i := range specs {
		specs[i].RequiresConfirm = specs[i].Method == "DELETE" || strings.Contains(specs[i].Path, "/delete/bulk")
		_, specs[i].RiskConfirm = riskConfirmNames[specs[i].Name]
		specs[i].SupportsPaginate = hasLimitOffset(specs[i].QueryParams)
		specs[i].DefaultListAlias = len(specs[i].CommandPath) == 2 && (specs[i].CommandPath[1] == "list" || specs[i].Name == "get-me-details")
		if specs[i].Name == "get-user-acl" || specs[i].Name == "get-me-details" {
			specs[i].RequiresOrg = false
		}
	}
	return specs
}

func hasLimitOffset(params []ParamSpec) bool {
	hasLimit := false
	hasOffset := false
	for _, param := range params {
		switch param.Name {
		case "limit":
			hasLimit = true
		case "offset":
			hasOffset = true
		}
	}
	return hasLimit && hasOffset
}

// EndpointByCommandPath returns a spec by command path.
func EndpointByCommandPath(path ...string) (EndpointSpec, bool) {
	joined := strings.Join(path, " ")
	for _, spec := range EndpointSpecs() {
		if strings.Join(spec.CommandPath, " ") == joined {
			return spec, true
		}
	}
	return EndpointSpec{}, false
}
