package asc

import (
	"net/url"
	"strconv"
	"strings"
)

// InAppPurchaseType represents an App Store Connect in-app purchase type.
type InAppPurchaseType string

const (
	InAppPurchaseTypeConsumable              InAppPurchaseType = "CONSUMABLE"
	InAppPurchaseTypeNonConsumable           InAppPurchaseType = "NON_CONSUMABLE"
	InAppPurchaseTypeNonRenewingSubscription InAppPurchaseType = "NON_RENEWING_SUBSCRIPTION"
)

// ValidIAPTypes lists supported in-app purchase types.
var ValidIAPTypes = []string{
	string(InAppPurchaseTypeConsumable),
	string(InAppPurchaseTypeNonConsumable),
	string(InAppPurchaseTypeNonRenewingSubscription),
}

// InAppPurchaseV2Attributes represents an in-app purchase resource.
type InAppPurchaseV2Attributes struct {
	Name                      string `json:"name"`
	ProductID                 string `json:"productId"`
	InAppPurchaseType         string `json:"inAppPurchaseType"`
	State                     string `json:"state,omitempty"`
	ReviewNote                string `json:"reviewNote,omitempty"`
	FamilySharable            bool   `json:"familySharable,omitempty"`
	ContentHosting            bool   `json:"contentHosting,omitempty"`
	AvailableInAllTerritories bool   `json:"availableInAllTerritories,omitempty"`
}

// InAppPurchaseAttributes represents a legacy in-app purchase resource.
type InAppPurchaseAttributes struct {
	ReferenceName     string `json:"referenceName"`
	ProductID         string `json:"productId"`
	InAppPurchaseType string `json:"inAppPurchaseType"`
	State             string `json:"state,omitempty"`
}

// InAppPurchaseV2CreateAttributes describes attributes for creating an IAP.
type InAppPurchaseV2CreateAttributes struct {
	Name                      string `json:"name"`
	ProductID                 string `json:"productId"`
	InAppPurchaseType         string `json:"inAppPurchaseType"`
	ReviewNote                string `json:"reviewNote,omitempty"`
	FamilySharable            bool   `json:"familySharable,omitempty"`
	ContentHosting            bool   `json:"contentHosting,omitempty"`
	AvailableInAllTerritories bool   `json:"availableInAllTerritories,omitempty"`
}

// InAppPurchaseV2UpdateAttributes describes attributes for updating an IAP.
type InAppPurchaseV2UpdateAttributes struct {
	Name                      *string `json:"name,omitempty"`
	ReviewNote                *string `json:"reviewNote,omitempty"`
	FamilySharable            *bool   `json:"familySharable,omitempty"`
	ContentHosting            *bool   `json:"contentHosting,omitempty"`
	AvailableInAllTerritories *bool   `json:"availableInAllTerritories,omitempty"`
}

// InAppPurchaseV2Relationships describes relationships for IAPs.
type InAppPurchaseV2Relationships struct {
	App *Relationship `json:"app"`
}

// InAppPurchaseV2CreateData is the data portion of an IAP create request.
type InAppPurchaseV2CreateData struct {
	Type          ResourceType                    `json:"type"`
	Attributes    InAppPurchaseV2CreateAttributes `json:"attributes"`
	Relationships *InAppPurchaseV2Relationships   `json:"relationships,omitempty"`
}

// InAppPurchaseV2CreateRequest is a request to create an IAP.
type InAppPurchaseV2CreateRequest struct {
	Data InAppPurchaseV2CreateData `json:"data"`
}

// InAppPurchaseV2UpdateData is the data portion of an IAP update request.
type InAppPurchaseV2UpdateData struct {
	Type       ResourceType                     `json:"type"`
	ID         string                           `json:"id"`
	Attributes *InAppPurchaseV2UpdateAttributes `json:"attributes,omitempty"`
}

// InAppPurchaseV2UpdateRequest is a request to update an IAP.
type InAppPurchaseV2UpdateRequest struct {
	Data InAppPurchaseV2UpdateData `json:"data"`
}

// InAppPurchaseLocalizationAttributes describes an IAP localization.
type InAppPurchaseLocalizationAttributes struct {
	Name        string `json:"name"`
	Locale      string `json:"locale"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
}

// InAppPurchasesV2Response is the response from in-app purchase list endpoints.
type InAppPurchasesV2Response = Response[InAppPurchaseV2Attributes]

// InAppPurchaseV2Response is the response from in-app purchase detail endpoints.
type InAppPurchaseV2Response = SingleResponse[InAppPurchaseV2Attributes]

// InAppPurchaseLocalizationsResponse is the response from localization endpoints.
type InAppPurchaseLocalizationsResponse = Response[InAppPurchaseLocalizationAttributes]

// InAppPurchasesResponse is the response from legacy in-app purchase list endpoints.
type InAppPurchasesResponse = Response[InAppPurchaseAttributes]

// InAppPurchaseResponse is the response from legacy in-app purchase detail endpoints.
type InAppPurchaseResponse = SingleResponse[InAppPurchaseAttributes]

// IAPOption is a functional option for GetInAppPurchasesV2.
type IAPOption func(*inAppPurchasesQuery)

// IAPGetOption is a functional option for GetInAppPurchaseV2.
type IAPGetOption func(*inAppPurchaseGetQuery)

// IAPLocalizationsOption is a functional option for GetInAppPurchaseLocalizations.
type IAPLocalizationsOption func(*iapLocalizationsQuery)

type inAppPurchasesQuery struct {
	listQuery
	productIDs          []string
	names               []string
	fields              []string
	include             []string
	versionFields       []string
	nestedVersionsLimit int
}

type inAppPurchaseGetQuery struct {
	fields              []string
	include             []string
	versionFields       []string
	nestedVersionsLimit int
}

type iapLocalizationsQuery struct {
	listQuery
}

// WithIAPLimit sets the max number of IAPs to return.
func WithIAPLimit(limit int) IAPOption {
	return func(q *inAppPurchasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithIAPNextURL uses a next page URL directly.
func WithIAPNextURL(next string) IAPOption {
	return func(q *inAppPurchasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithIAPProductIDs filters in-app purchases by product ID.
func WithIAPProductIDs(productIDs []string) IAPOption {
	return func(q *inAppPurchasesQuery) {
		q.productIDs = normalizeUniqueList(productIDs)
	}
}

// WithIAPNames filters in-app purchases by current name.
func WithIAPNames(names []string) IAPOption {
	return func(q *inAppPurchasesQuery) {
		q.names = normalizeUniqueList(names)
	}
}

// WithIAPFields sets fields for returned in-app purchases.
func WithIAPFields(fields []string) IAPOption {
	return func(q *inAppPurchasesQuery) { q.fields = normalizeUniqueList(fields) }
}

// WithIAPInclude sets related resources to include for IAP list requests.
func WithIAPInclude(include []string) IAPOption {
	return func(q *inAppPurchasesQuery) { q.include = normalizeUniqueList(include) }
}

// WithIAPVersionFields sets fields for included in-app purchase versions.
func WithIAPVersionFields(fields []string) IAPOption {
	return func(q *inAppPurchasesQuery) { q.versionFields = normalizeUniqueList(fields) }
}

// WithIAPNestedVersionsLimit limits included versions.
func WithIAPNestedVersionsLimit(limit int) IAPOption {
	return func(q *inAppPurchasesQuery) {
		if limit > 0 {
			q.nestedVersionsLimit = limit
		}
	}
}

// WithIAPGetFields sets fields for a returned in-app purchase.
func WithIAPGetFields(fields []string) IAPGetOption {
	return func(q *inAppPurchaseGetQuery) { q.fields = normalizeUniqueList(fields) }
}

// WithIAPGetInclude sets related resources to include for an IAP detail request.
func WithIAPGetInclude(include []string) IAPGetOption {
	return func(q *inAppPurchaseGetQuery) { q.include = normalizeUniqueList(include) }
}

// WithIAPGetVersionFields sets fields for included versions on an IAP detail request.
func WithIAPGetVersionFields(fields []string) IAPGetOption {
	return func(q *inAppPurchaseGetQuery) { q.versionFields = normalizeUniqueList(fields) }
}

// WithIAPGetNestedVersionsLimit limits included versions on an IAP detail request.
func WithIAPGetNestedVersionsLimit(limit int) IAPGetOption {
	return func(q *inAppPurchaseGetQuery) {
		if limit > 0 {
			q.nestedVersionsLimit = limit
		}
	}
}

// WithIAPLocalizationsLimit sets the max number of localizations to return.
func WithIAPLocalizationsLimit(limit int) IAPLocalizationsOption {
	return func(q *iapLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithIAPLocalizationsNextURL uses a next page URL directly.
func WithIAPLocalizationsNextURL(next string) IAPLocalizationsOption {
	return func(q *iapLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildInAppPurchasesQuery(query *inAppPurchasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	addCSV(values, "filter[productId]", query.productIDs)
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "fields[inAppPurchases]", query.fields)
	addCSV(values, "include", query.include)
	addCSV(values, "fields[inAppPurchaseVersions]", query.versionFields)
	if query.nestedVersionsLimit > 0 {
		values.Set("limit[versions]", strconv.Itoa(query.nestedVersionsLimit))
	}
	return values.Encode()
}

func buildInAppPurchaseGetQuery(query *inAppPurchaseGetQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchases]", query.fields)
	addCSV(values, "include", query.include)
	addCSV(values, "fields[inAppPurchaseVersions]", query.versionFields)
	if query.nestedVersionsLimit > 0 {
		values.Set("limit[versions]", strconv.Itoa(query.nestedVersionsLimit))
	}
	return values.Encode()
}

func buildIAPLocalizationsQuery(query *iapLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
