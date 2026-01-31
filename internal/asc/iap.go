package asc

import (
	"net/url"
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
	ReferenceName    string `json:"referenceName"`
	ProductID        string `json:"productId"`
	InAppPurchaseType string `json:"inAppPurchaseType"`
	State            string `json:"state,omitempty"`
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

// IAPLocalizationsOption is a functional option for GetInAppPurchaseLocalizations.
type IAPLocalizationsOption func(*iapLocalizationsQuery)

type inAppPurchasesQuery struct {
	listQuery
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
	return values.Encode()
}

func buildIAPLocalizationsQuery(query *iapLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
