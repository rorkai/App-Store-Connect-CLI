package asc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// SandboxTesterAttributes describes a sandbox tester resource.
type SandboxTesterAttributes struct {
	FirstName               string `json:"firstName,omitempty"`
	LastName                string `json:"lastName,omitempty"`
	Email                   string `json:"email,omitempty"`
	AccountName             string `json:"acAccountName,omitempty"`
	Password                string `json:"password,omitempty"`
	ConfirmPassword         string `json:"confirmPassword,omitempty"`
	SecretQuestion          string `json:"secretQuestion,omitempty"`
	SecretAnswer            string `json:"secretAnswer,omitempty"`
	BirthDate               string `json:"birthDate,omitempty"`
	AppStoreTerritory       string `json:"appStoreTerritory,omitempty"`
	Territory               string `json:"territory,omitempty"`
	ApplePayCompatible      *bool  `json:"applePayCompatible,omitempty"`
	InterruptPurchases      *bool  `json:"interruptPurchases,omitempty"`
	SubscriptionRenewalRate string `json:"subscriptionRenewalRate,omitempty"`
}

// SandboxTestersResponse is the response from sandbox testers endpoints.
type SandboxTestersResponse = Response[SandboxTesterAttributes]

// SandboxTesterResponse is the response from sandbox tester detail/creates.
type SandboxTesterResponse = SingleResponse[SandboxTesterAttributes]

// SandboxTesterSubscriptionRenewalRate represents renewal rate settings.
type SandboxTesterSubscriptionRenewalRate string

const (
	SandboxTesterRenewalEveryOneHour        SandboxTesterSubscriptionRenewalRate = "MONTHLY_RENEWAL_EVERY_ONE_HOUR"
	SandboxTesterRenewalEveryThirtyMinutes  SandboxTesterSubscriptionRenewalRate = "MONTHLY_RENEWAL_EVERY_THIRTY_MINUTES"
	SandboxTesterRenewalEveryFifteenMinutes SandboxTesterSubscriptionRenewalRate = "MONTHLY_RENEWAL_EVERY_FIFTEEN_MINUTES"
	SandboxTesterRenewalEveryFiveMinutes    SandboxTesterSubscriptionRenewalRate = "MONTHLY_RENEWAL_EVERY_FIVE_MINUTES"
	SandboxTesterRenewalEveryThreeMinutes   SandboxTesterSubscriptionRenewalRate = "MONTHLY_RENEWAL_EVERY_THREE_MINUTES"
)

// SandboxTesterUpdateAttributes describes attributes for updating a sandbox tester.
type SandboxTesterUpdateAttributes struct {
	Territory               *string                               `json:"territory,omitempty"`
	InterruptPurchases      *bool                                 `json:"interruptPurchases,omitempty"`
	SubscriptionRenewalRate *SandboxTesterSubscriptionRenewalRate `json:"subscriptionRenewalRate,omitempty"`
}

// SandboxTesterUpdateData is the data portion of a sandbox tester update request.
type SandboxTesterUpdateData struct {
	Type       ResourceType                  `json:"type"`
	ID         string                        `json:"id"`
	Attributes SandboxTesterUpdateAttributes `json:"attributes"`
}

// SandboxTesterUpdateRequest is a request to update a sandbox tester.
type SandboxTesterUpdateRequest struct {
	Data SandboxTesterUpdateData `json:"data"`
}

// SandboxTesterClearHistoryRelationships describes relationships for clear history requests.
type SandboxTesterClearHistoryRelationships struct {
	SandboxTesters RelationshipList `json:"sandboxTesters"`
}

// SandboxTesterClearHistoryData is the data portion of a clear history request.
type SandboxTesterClearHistoryData struct {
	Type          ResourceType                           `json:"type"`
	Relationships SandboxTesterClearHistoryRelationships `json:"relationships"`
}

// SandboxTesterClearHistoryRequest is a request to clear purchase history.
type SandboxTesterClearHistoryRequest struct {
	Data SandboxTesterClearHistoryData `json:"data"`
}

// SandboxTesterClearHistoryResponse is the response from clear history requests.
type SandboxTesterClearHistoryResponse = SingleResponse[struct{}]

type sandboxTestersQuery struct {
	listQuery
	email     string
	territory string
}

// SandboxTestersOption is a functional option for GetSandboxTesters.
type SandboxTestersOption func(*sandboxTestersQuery)

// WithSandboxTestersLimit sets the max number of sandbox testers to return.
func WithSandboxTestersLimit(limit int) SandboxTestersOption {
	return func(q *sandboxTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSandboxTestersNextURL uses a next page URL directly.
func WithSandboxTestersNextURL(next string) SandboxTestersOption {
	return func(q *sandboxTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSandboxTestersEmail filters sandbox testers by email address.
func WithSandboxTestersEmail(email string) SandboxTestersOption {
	return func(q *sandboxTestersQuery) {
		if strings.TrimSpace(email) != "" {
			q.email = strings.TrimSpace(email)
		}
	}
}

// WithSandboxTestersTerritory filters sandbox testers by App Store territory code (e.g., USA).
func WithSandboxTestersTerritory(territory string) SandboxTestersOption {
	return func(q *sandboxTestersQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
	}
}

func buildSandboxTestersQuery(query *sandboxTestersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func sandboxTesterEmail(attr SandboxTesterAttributes) string {
	if strings.TrimSpace(attr.Email) != "" {
		return strings.TrimSpace(attr.Email)
	}
	if strings.TrimSpace(attr.AccountName) != "" {
		return strings.TrimSpace(attr.AccountName)
	}
	return ""
}

func sandboxTesterTerritory(attr SandboxTesterAttributes) string {
	if strings.TrimSpace(attr.AppStoreTerritory) != "" {
		return strings.ToUpper(strings.TrimSpace(attr.AppStoreTerritory))
	}
	if strings.TrimSpace(attr.Territory) != "" {
		return strings.ToUpper(strings.TrimSpace(attr.Territory))
	}
	return ""
}

func filterSandboxTesters(items []Resource[SandboxTesterAttributes], email, territory string) []Resource[SandboxTesterAttributes] {
	normalizedEmail := strings.TrimSpace(email)
	normalizedTerritory := strings.ToUpper(strings.TrimSpace(territory))
	if normalizedEmail == "" && normalizedTerritory == "" {
		return items
	}
	filtered := make([]Resource[SandboxTesterAttributes], 0, len(items))
	for _, item := range items {
		if normalizedEmail != "" && !strings.EqualFold(sandboxTesterEmail(item.Attributes), normalizedEmail) {
			continue
		}
		if normalizedTerritory != "" && sandboxTesterTerritory(item.Attributes) != normalizedTerritory {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// GetSandboxTesters retrieves sandbox testers with optional filters.
func (c *Client) GetSandboxTesters(ctx context.Context, opts ...SandboxTestersOption) (*SandboxTestersResponse, error) {
	query := applyOptions(opts)
	resp, err := getJSON[SandboxTestersResponse](ctx, c, readRequest{
		Path:        "/v2/sandboxTesters",
		Query:       buildSandboxTestersQuery(query),
		Next:        query.nextURL,
		NextContext: "sandboxTesters",
		ParseErr:    "failed to parse sandbox testers response",
	})
	if err != nil {
		return nil, err
	}

	resp.Data = filterSandboxTesters(resp.Data, query.email, query.territory)
	return resp, nil
}

// errSandboxTesterFound signals early pagination exit once the tester is located.
var errSandboxTesterFound = errors.New("sandbox tester found")

// GetSandboxTester retrieves a sandbox tester by ID.
func (c *Client) GetSandboxTester(ctx context.Context, testerID string) (*SandboxTesterResponse, error) {
	firstPage, err := c.GetSandboxTesters(ctx, WithSandboxTestersLimit(200))
	if err != nil {
		return nil, err
	}

	var found *SandboxTesterResponse
	err = PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return c.GetSandboxTesters(ctx, WithSandboxTestersNextURL(nextURL))
	}, func(page PaginatedResponse) error {
		resp, ok := page.(*SandboxTestersResponse)
		if !ok {
			return fmt.Errorf("unexpected page type %T", page)
		}
		for _, item := range resp.Data {
			if item.ID == testerID {
				found = &SandboxTesterResponse{Data: item, Links: resp.Links}
				return errSandboxTesterFound
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSandboxTesterFound) {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("sandbox tester not found: %s", testerID)
	}
	return found, nil
}

// UpdateSandboxTester updates a sandbox tester by ID.
func (c *Client) UpdateSandboxTester(ctx context.Context, testerID string, attributes SandboxTesterUpdateAttributes) (*SandboxTesterResponse, error) {
	payload := SandboxTesterUpdateRequest{
		Data: SandboxTesterUpdateData{
			Type:       ResourceTypeSandboxTesters,
			ID:         testerID,
			Attributes: attributes,
		},
	}
	return sendJSON[SandboxTesterResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v2/sandboxTesters/%s", testerID), Payload: payload, ParseErr: "failed to parse sandbox tester response"})
}

// ClearSandboxTesterPurchaseHistory clears purchase history for a sandbox tester.
func (c *Client) ClearSandboxTesterPurchaseHistory(ctx context.Context, testerID string) (*SandboxTesterClearHistoryResponse, error) {
	payload := SandboxTesterClearHistoryRequest{
		Data: SandboxTesterClearHistoryData{
			Type: ResourceTypeSandboxTestersClearHistory,
			Relationships: SandboxTesterClearHistoryRelationships{
				SandboxTesters: RelationshipList{
					Data: []ResourceData{
						{Type: ResourceTypeSandboxTesters, ID: testerID},
					},
				},
			},
		},
	}
	return sendJSON[SandboxTesterClearHistoryResponse](ctx, c, writeRequest{Method: "POST", Path: "/v2/sandboxTestersClearPurchaseHistoryRequest", Payload: payload, ParseErr: "failed to parse sandbox clear history response"})
}
