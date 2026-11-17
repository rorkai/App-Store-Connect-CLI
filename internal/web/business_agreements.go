package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

// BusinessAgreement identifies an App Store Connect Business contract and its
// available replacement configuration, which is distinct from the current one.
type BusinessAgreement struct {
	ContractID                   string   `json:"contractId"`
	ContractDisplayName          string   `json:"contractDisplayName"`
	ConfigID                     string   `json:"configId"`
	ConfigVersion                *int     `json:"configVersion"`
	ConfigIsLatestVersion        *bool    `json:"configIsLatestVersion"`
	IsInEffect                   *bool    `json:"isInEffect"`
	Status                       string   `json:"status"`
	AvailableNewContractConfigID *string  `json:"availableNewContractConfigId"`
	AvailableNewConfigLocales    []string `json:"availableNewConfigLocales"`
	LegalEntity                  *struct {
		LegalEntityID         string `json:"legalEntityId"`
		ReadyForSignAgreement bool   `json:"readyForSignAgreement"`
	} `json:"legalEntity"`
	ApplicableActions struct {
		View []string `json:"view"`
	} `json:"applicableActions"`
}

// BusinessAgreementsResponse retains Apple's complete envelope for JSON output.
type BusinessAgreementsResponse struct {
	Raw        json.RawMessage
	Agreements []BusinessAgreement
}

// ListBusinessAgreements reads the selected App Store Connect provider's
// Business agreements, including updates to the Paid Applications agreement.
func (c *Client) ListBusinessAgreements(ctx context.Context) (*BusinessAgreementsResponse, error) {
	if c == nil || c.httpClient == nil {
		return nil, errors.New("business agreements web client is not configured")
	}
	providerID := strings.TrimSpace(c.publicProviderID)
	if providerID == "" {
		return nil, errors.New("business agreements require a selected public provider ID")
	}
	path := "/WebObjects/iTunesConnect.woa/ra/v1/contentProviders/" + url.PathEscape(providerID) + "/agreements"
	body, err := c.doRequestBase(ctx, appStoreBaseURL, http.MethodGet, path, nil, c.businessAgreementHeaders())
	if err != nil {
		return nil, err
	}
	var response struct {
		Data *struct {
			Agreements *[]BusinessAgreement `json:"agreements"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, errors.New("business agreements returned a malformed response")
	}
	if response.Data == nil || response.Data.Agreements == nil {
		return nil, errors.New("business agreements response is missing agreements")
	}
	return &BusinessAgreementsResponse{Raw: body, Agreements: *response.Data.Agreements}, nil
}

func (c *Client) businessAgreementHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("X-CSRF-ITC", "agreements-ui")
	headers.Set("X-Connect-Team-Type", "PURPLESOFTWARE")
	headers.Set("X-Connect-Team-ID", c.publicProviderID)
	headers.Set("Referer", appStoreBaseURL+"/business")
	return headers
}

// AcceptBusinessAgreement validates a single outstanding configuration and
// submits the same two consent fields as the Business web form, exactly once.
func (c *Client) AcceptBusinessAgreement(ctx context.Context, configID string) (*asc.WebBusinessAgreementAcceptResult, error) {
	configID = strings.TrimSpace(configID)
	if configID == "" {
		return nil, errors.New("contract configuration ID is required")
	}
	response, err := c.ListBusinessAgreements(ctx)
	if err != nil {
		return nil, err
	}
	var target *BusinessAgreement
	for i := range response.Agreements {
		outstandingID := response.Agreements[i].AvailableNewContractConfigID
		if outstandingID == nil || *outstandingID != configID {
			continue
		}
		if target != nil {
			return nil, errors.New("contract configuration matches multiple agreements; review in App Store Connect Business")
		}
		target = &response.Agreements[i]
	}
	if target == nil {
		return nil, errors.New("contract configuration is not an outstanding configuration")
	}
	if target.LegalEntity == nil || strings.TrimSpace(target.LegalEntity.LegalEntityID) == "" || target.ContractID == "" {
		return nil, errors.New("agreement has no assigned legal entity or existing contract")
	}
	if !target.LegalEntity.ReadyForSignAgreement || target.ConfigIsLatestVersion == nil || *target.ConfigIsLatestVersion || target.ConfigID == "" || target.ConfigID == configID {
		return nil, errors.New("agreement is not ready for acceptance")
	}
	if !slices.Contains(target.ApplicableActions.View, "ITC.ATB.Agmnt.Action.view.AgreeToTerms") && !slices.Contains(target.ApplicableActions.View, "ITC.ATB.Agmnt.Action.view.RenewAgreement") {
		return nil, errors.New("agreement does not offer acceptance")
	}
	locale := "en-US"
	if len(target.AvailableNewConfigLocales) > 0 && target.AvailableNewConfigLocales[0] != "" {
		locale = target.AvailableNewConfigLocales[0]
	}
	providerPath := "/WebObjects/iTunesConnect.woa/ra/v1/contentProviders/" + url.PathEscape(c.publicProviderID)
	termsPath := "/agreementTerms/" + url.PathEscape(configID) + "/locales/" + url.PathEscape(locale)
	body, err := c.doRequestBase(ctx, appStoreBaseURL, http.MethodGet, providerPath+termsPath, nil, c.businessAgreementHeaders())
	if err != nil {
		return nil, err
	}
	var terms struct {
		Constraints map[string]struct {
			AttributeName string `json:"attributeName"`
			Required      bool   `json:"required"`
			IsEditable    bool   `json:"isEditable"`
		} `json:"constraints"`
		Messages []json.RawMessage `json:"messages"`
		Data     *struct {
			ContractConfigID   string `json:"contractConfigId"`
			ProviderPublicID   string `json:"providerPublicId"`
			SelectedLocale     string `json:"selectedLocaleCode"`
			AgreeToTerms       *bool  `json:"agreeToTerms"`
			AgreeToFamilyShare *bool  `json:"agreeToFamilyShare"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &terms); err != nil || terms.Data == nil {
		return nil, errors.New("business agreement terms response is malformed")
	}
	if terms.Data.ContractConfigID != configID || terms.Data.ProviderPublicID != c.publicProviderID || terms.Data.SelectedLocale != locale {
		return nil, errors.New("business agreement terms do not match the selected provider, configuration, and locale")
	}
	if len(terms.Messages) != 0 || terms.Data.AgreeToTerms == nil || terms.Data.AgreeToFamilyShare == nil || *terms.Data.AgreeToTerms {
		return nil, errors.New("business agreement terms require review in App Store Connect Business")
	}
	for key, constraint := range terms.Constraints {
		if (key != "$[0].agreeToTerms" && key != "$[0].agreeToFamilyShare") || key != "$[0]."+constraint.AttributeName || !constraint.IsEditable {
			return nil, errors.New("business agreement terms contain an unsupported consent constraint")
		}
	}
	if !terms.Constraints["$[0].agreeToTerms"].Required {
		return nil, errors.New("business agreement terms contain an unsupported consent constraint")
	}
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("business agreement acceptance does not follow redirects")
	}
	path := providerPath + "/legalEntities/" + url.PathEscape(target.LegalEntity.LegalEntityID) + termsPath
	headers := c.businessAgreementHeaders()
	headers.Set("Origin", appStoreBaseURL)
	_, err = c.doRequestBaseWithHTTPClient(&client, ctx, appStoreBaseURL, http.MethodPost, path,
		map[string]bool{"agreeToTerms": true, "agreeToFamilyShare": true}, headers)
	if err != nil {
		if errors.Is(err, readonly.ErrRefused) {
			return nil, err
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.Status == http.StatusUnauthorized && slices.Contains(extractServiceErrorCodes(apiErr.rawResponseBody()), "NOT_AUTHORIZED.FRESH_2FA_REQUIRED") {
				freshSignIn := "for the same account and provider, run 'asc web auth logout --apple-id EMAIL', then 'asc web auth login --apple-id EMAIL --public-provider-id PROVIDER_ID' and complete 2FA"
				if _, brokered, _ := BrokerFromEnv(); brokered {
					freshSignIn = "sign in to Apple again through the broker's owner and complete 2FA there"
				}
				return nil, fmt.Errorf("web business agreements accept failed: Apple requires a fresh 2FA challenge; %s; run 'asc web business agreements list' before retrying acceptance: %w", freshSignIn, &APIError{Status: apiErr.Status})
			}
			err = &APIError{Status: apiErr.Status}
		} else if errors.Is(err, context.Canceled) {
			err = context.Canceled
		} else if errors.Is(err, context.DeadlineExceeded) {
			err = context.DeadlineExceeded
		} else {
			err = errors.New("request failed")
		}
		return nil, fmt.Errorf("business agreement acceptance outcome is unknown; no automatic retry was attempted; run 'asc web business agreements list' before retrying: %w", err)
	}
	state, err := c.ListBusinessAgreements(ctx)
	if err != nil {
		return nil, fmt.Errorf("business agreement acceptance is unverified: the request completed but the agreement re-read failed; run 'asc web business agreements list' before retrying: %w", err)
	}
	return verifyBusinessAgreementAccepted(c.publicProviderID, target.LegalEntity.LegalEntityID, configID, state)
}

func verifyBusinessAgreementAccepted(providerID, entityID, configID string, state *BusinessAgreementsResponse) (*asc.WebBusinessAgreementAcceptResult, error) {
	unverified := errors.New("business agreement acceptance is unverified; run 'asc web business agreements list' before retrying")
	var accepted *BusinessAgreement
	for i := range state.Agreements {
		agreement := &state.Agreements[i]
		if agreement.ConfigID != configID || agreement.LegalEntity == nil || agreement.LegalEntity.LegalEntityID != entityID {
			continue
		}
		if accepted != nil {
			return nil, unverified
		}
		accepted = agreement
	}
	if accepted == nil || accepted.ConfigIsLatestVersion == nil || !*accepted.ConfigIsLatestVersion ||
		accepted.IsInEffect == nil || !*accepted.IsInEffect || accepted.AvailableNewContractConfigID == nil || *accepted.AvailableNewContractConfigID != "" ||
		accepted.ContractID == "" || accepted.ConfigVersion == nil || accepted.Status == "" {
		return nil, unverified
	}
	return &asc.WebBusinessAgreementAcceptResult{
		PublicProviderID: providerID,
		LegalEntityID:    entityID,
		ContractID:       accepted.ContractID,
		ContractConfigID: accepted.ConfigID,
		ConfigVersion:    *accepted.ConfigVersion,
		Status:           accepted.Status,
		Verified:         true,
	}, nil
}
