package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	integrationsAPIRefererURL            = appStoreBaseURL + "/access/integrations/api"
	integrationsIndividualKeysRefererURL = appStoreBaseURL + "/access/integrations/api/individual-keys"
)

var (
	ErrAPIKeyNotFound        = errors.New("api key not found")
	ErrAPIKeyNotVisible      = errors.New("api key not visible in accessible key lists")
	ErrAPIKeyRolesUnresolved = errors.New("api key roles could not be resolved")
)

func integrationsHeaders(referer string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.api+json, application/json, text/csv")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-CSRF-ITC", "[asc-ui]")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", referer)
	return headers
}

func olympusHeaders(referer string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Requested-With", "xsdr2$")
	if referer != "" {
		headers.Set("Referer", referer)
	}
	return headers
}

func (c *Client) doIrisV1Request(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, irisV1BaseURL, method, path, body, integrationsHeaders(integrationsAPIRefererURL))
}

func (c *Client) doIrisV2Request(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, irisV2BaseURL, method, path, body, integrationsHeaders(integrationsIndividualKeysRefererURL))
}

func (c *Client) doOlympusRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, olympusBaseURL, method, path, body, olympusHeaders(integrationsIndividualKeysRefererURL))
}

type KeyActor struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type teamAPIKey struct {
	KeyID       string
	Name        string
	Roles       []string
	Active      bool
	KeyType     string
	LastUsed    string
	GeneratedBy *KeyActor
	RevokedBy   *KeyActor
}

type individualAPIKey struct {
	KeyID            string
	Name             string
	Roles            []string
	Active           bool
	KeyType          string
	LastUsed         string
	CreatedByActorID string
	RevokedByActorID string
}

type olympusActor struct {
	ID    string
	Roles []string
	Name  string
}

type APIKeyRoleLookup struct {
	KeyID       string    `json:"keyId"`
	Name        string    `json:"name,omitempty"`
	Kind        string    `json:"kind"`
	Roles       []string  `json:"roles"`
	RoleSource  string    `json:"roleSource"`
	Active      bool      `json:"active"`
	KeyType     string    `json:"keyType,omitempty"`
	LastUsed    string    `json:"lastUsed,omitempty"`
	Lookup      string    `json:"lookup"`
	GeneratedBy *KeyActor `json:"generatedBy,omitempty"`
	RevokedBy   *KeyActor `json:"revokedBy,omitempty"`
}

func fullName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func nextLookupPagePath(links map[string]any, baseURL, responseName string) (string, error) {
	nextLink, err := extractNextLink(links)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s pagination links: %w", responseName, err)
	}
	if strings.TrimSpace(nextLink) == "" {
		return "", nil
	}
	nextPath, err := normalizeNextPath(nextLink, baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to normalize %s pagination link: %w", responseName, err)
	}
	return nextPath, nil
}

func (c *Client) listTeamKeys(ctx context.Context) ([]teamAPIKey, error) {
	type teamKeyPayload struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				LastUsed       string   `json:"lastUsed"`
				Roles          []string `json:"roles"`
				Nickname       string   `json:"nickname"`
				RevokingDate   string   `json:"revokingDate"`
				AllAppsVisible bool     `json:"allAppsVisible"`
				CanDownload    bool     `json:"canDownload"`
				IsActive       bool     `json:"isActive"`
				KeyType        string   `json:"keyType"`
			} `json:"attributes"`
			Relationships struct {
				CreatedBy struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"createdBy"`
				RevokedBy struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"revokedBy"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"attributes"`
		} `json:"included"`
		Links map[string]any `json:"links"`
	}

	nextPath := "/apiKeys?include=createdBy,revokedBy,provider&sort=-isActive,-revokingDate&limit=2000"
	visited := map[string]struct{}{}
	payloads := make([]teamKeyPayload, 0, 1)

	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return nil, fmt.Errorf("team keys pagination loop detected")
		}
		visited[nextPath] = struct{}{}

		body, err := c.doIrisV1Request(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return nil, err
		}

		var payload teamKeyPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse team keys response: %w", err)
		}
		payloads = append(payloads, payload)

		nextPath, err = nextLookupPagePath(payload.Links, irisV1BaseURL, "team keys")
		if err != nil {
			return nil, err
		}
	}

	totalIncluded := 0
	totalKeys := 0
	for _, payload := range payloads {
		totalIncluded += len(payload.Included)
		totalKeys += len(payload.Data)
	}

	users := make(map[string]string, totalIncluded)
	for _, payload := range payloads {
		for _, item := range payload.Included {
			if item.Type != "users" {
				continue
			}
			users[item.ID] = fullName(item.Attributes.FirstName, item.Attributes.LastName)
		}
	}

	keys := make([]teamAPIKey, 0, totalKeys)
	for _, payload := range payloads {
		for _, item := range payload.Data {
			key := teamAPIKey{
				KeyID:    strings.TrimSpace(item.ID),
				Name:     strings.TrimSpace(item.Attributes.Nickname),
				Roles:    append([]string(nil), item.Attributes.Roles...),
				Active:   item.Attributes.IsActive,
				KeyType:  strings.TrimSpace(item.Attributes.KeyType),
				LastUsed: strings.TrimSpace(item.Attributes.LastUsed),
			}
			if item.Relationships.CreatedBy.Data != nil {
				id := strings.TrimSpace(item.Relationships.CreatedBy.Data.ID)
				key.GeneratedBy = &KeyActor{ID: id, Name: strings.TrimSpace(users[id])}
			}
			if item.Relationships.RevokedBy.Data != nil {
				id := strings.TrimSpace(item.Relationships.RevokedBy.Data.ID)
				key.RevokedBy = &KeyActor{ID: id, Name: strings.TrimSpace(users[id])}
			}
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (c *Client) listIndividualKeys(ctx context.Context) ([]individualAPIKey, error) {
	type individualKeyPayload struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				LastUsed string   `json:"lastUsed"`
				Roles    []string `json:"roles"`
				Nickname string   `json:"nickname"`
				Name     string   `json:"name"`
				IsActive bool     `json:"isActive"`
				KeyType  string   `json:"keyType"`
			} `json:"attributes"`
			Relationships struct {
				CreatedByActor struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"createdByActor"`
				RevokedByActor struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"revokedByActor"`
			} `json:"relationships"`
		} `json:"data"`
		Links map[string]any `json:"links"`
	}

	nextPath := "/apiKeys?include=visibleApps,createdByActor,revokedByActor&limit[visibleApps]=3&limit=2000"
	visited := map[string]struct{}{}
	payloads := make([]individualKeyPayload, 0, 1)

	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return nil, fmt.Errorf("individual keys pagination loop detected")
		}
		visited[nextPath] = struct{}{}

		body, err := c.doIrisV2Request(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return nil, err
		}

		var payload individualKeyPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse individual keys response: %w", err)
		}
		payloads = append(payloads, payload)

		nextPath, err = nextLookupPagePath(payload.Links, irisV2BaseURL, "individual keys")
		if err != nil {
			return nil, err
		}
	}

	totalKeys := 0
	for _, payload := range payloads {
		totalKeys += len(payload.Data)
	}
	keys := make([]individualAPIKey, 0, totalKeys)
	for _, payload := range payloads {
		for _, item := range payload.Data {
			key := individualAPIKey{
				KeyID:    strings.TrimSpace(item.ID),
				Name:     strings.TrimSpace(item.Attributes.Nickname),
				Roles:    append([]string(nil), item.Attributes.Roles...),
				Active:   item.Attributes.IsActive,
				KeyType:  strings.TrimSpace(item.Attributes.KeyType),
				LastUsed: strings.TrimSpace(item.Attributes.LastUsed),
			}
			if key.Name == "" {
				key.Name = strings.TrimSpace(item.Attributes.Name)
			}
			if item.Relationships.CreatedByActor.Data != nil {
				key.CreatedByActorID = strings.TrimSpace(item.Relationships.CreatedByActor.Data.ID)
			}
			if item.Relationships.RevokedByActor.Data != nil {
				key.RevokedByActorID = strings.TrimSpace(item.Relationships.RevokedByActor.Data.ID)
			}
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (c *Client) getActor(ctx context.Context, actorID string) (*olympusActor, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, fmt.Errorf("actor id is required")
	}

	body, err := c.doOlympusRequest(ctx, http.MethodGet, "/actors/"+actorID+"?include=provider,person", nil)
	if err != nil {
		return nil, err
	}
	return decodeOlympusActor(body)
}

func (c *Client) listActors(ctx context.Context) ([]olympusActor, error) {
	type actorsPayload struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Roles []string `json:"roles"`
			} `json:"attributes"`
			Relationships struct {
				Person struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"person"`
				Provider struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"provider"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
				Name      string `json:"name"`
			} `json:"attributes"`
		} `json:"included"`
		Links map[string]any `json:"links"`
	}

	nextPath := "/actors?include=provider,person&limit=2000"
	visited := map[string]struct{}{}
	payloads := make([]actorsPayload, 0, 1)

	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return nil, fmt.Errorf("actors pagination loop detected")
		}
		visited[nextPath] = struct{}{}

		body, err := c.doOlympusRequest(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return nil, err
		}

		var payload actorsPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse actors response: %w", err)
		}
		payloads = append(payloads, payload)

		nextPath, err = nextLookupPagePath(payload.Links, olympusBaseURL, "actors")
		if err != nil {
			return nil, err
		}
	}

	totalIncluded := 0
	totalActors := 0
	for _, payload := range payloads {
		totalIncluded += len(payload.Included)
		totalActors += len(payload.Data)
	}

	names := make(map[string]string, totalIncluded)
	for _, payload := range payloads {
		for _, item := range payload.Included {
			switch item.Type {
			case "people":
				names[item.ID] = fullName(item.Attributes.FirstName, item.Attributes.LastName)
			case "providers":
				names[item.ID] = strings.TrimSpace(item.Attributes.Name)
			}
		}
	}

	actors := make([]olympusActor, 0, totalActors)
	for _, payload := range payloads {
		for _, item := range payload.Data {
			actor := olympusActor{
				ID:    strings.TrimSpace(item.ID),
				Roles: append([]string(nil), item.Attributes.Roles...),
			}
			if item.Relationships.Person.Data != nil {
				actor.Name = strings.TrimSpace(names[item.Relationships.Person.Data.ID])
			}
			if actor.Name == "" && item.Relationships.Provider.Data != nil {
				actor.Name = strings.TrimSpace(names[item.Relationships.Provider.Data.ID])
			}
			actors = append(actors, actor)
		}
	}
	return actors, nil
}

func decodeOlympusActor(body []byte) (*olympusActor, error) {
	var payload struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Roles []string `json:"roles"`
			} `json:"attributes"`
			Relationships struct {
				Person struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"person"`
				Provider struct {
					Data *struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"provider"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
				Name      string `json:"name"`
			} `json:"attributes"`
		} `json:"included"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse actor response: %w", err)
	}

	names := make(map[string]string, len(payload.Included))
	for _, item := range payload.Included {
		switch item.Type {
		case "people":
			names[item.ID] = fullName(item.Attributes.FirstName, item.Attributes.LastName)
		case "providers":
			names[item.ID] = strings.TrimSpace(item.Attributes.Name)
		}
	}

	actor := &olympusActor{
		ID:    strings.TrimSpace(payload.Data.ID),
		Roles: append([]string(nil), payload.Data.Attributes.Roles...),
	}
	if payload.Data.Relationships.Person.Data != nil {
		actor.Name = strings.TrimSpace(names[payload.Data.Relationships.Person.Data.ID])
	}
	if actor.Name == "" && payload.Data.Relationships.Provider.Data != nil {
		actor.Name = strings.TrimSpace(names[payload.Data.Relationships.Provider.Data.ID])
	}
	return actor, nil
}

func (c *Client) LookupAPIKeyRoles(ctx context.Context, keyID string) (*APIKeyRoleLookup, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}

	var teamErr error
	keys, err := c.listTeamKeys(ctx)
	if err != nil {
		if !shouldFallbackToIndividualKeys(err) {
			return nil, err
		}
		teamErr = err
	} else {
		for _, item := range keys {
			if item.KeyID != keyID {
				continue
			}
			return &APIKeyRoleLookup{
				KeyID:       item.KeyID,
				Name:        item.Name,
				Kind:        "team",
				Roles:       append([]string(nil), item.Roles...),
				RoleSource:  "key",
				Active:      item.Active,
				KeyType:     item.KeyType,
				LastUsed:    item.LastUsed,
				Lookup:      "team_keys",
				GeneratedBy: item.GeneratedBy,
				RevokedBy:   item.RevokedBy,
			}, nil
		}
	}

	individualKeys, err := c.listIndividualKeys(ctx)
	if err != nil {
		if shouldFallbackToIndividualKeys(err) {
			return nil, fmt.Errorf("%w: %s", ErrAPIKeyNotVisible, keyID)
		}
		return nil, err
	}
	for _, item := range individualKeys {
		if item.KeyID != keyID {
			continue
		}
		result := &APIKeyRoleLookup{
			KeyID:      item.KeyID,
			Name:       item.Name,
			Kind:       "individual",
			Roles:      append([]string(nil), item.Roles...),
			Active:     item.Active,
			KeyType:    item.KeyType,
			LastUsed:   item.LastUsed,
			Lookup:     "individual_keys",
			RoleSource: "key",
		}
		if item.CreatedByActorID != "" {
			result.GeneratedBy = &KeyActor{ID: item.CreatedByActorID}
		}
		if item.RevokedByActorID != "" {
			result.RevokedBy = &KeyActor{ID: item.RevokedByActorID}
		}
		if len(result.Roles) > 0 {
			return result, nil
		}

		actor, actors, err := c.resolveActor(ctx, item.CreatedByActorID)
		if err != nil {
			if errors.Is(err, ErrAPIKeyRolesUnresolved) {
				return nil, fmt.Errorf("%w: %s", ErrAPIKeyRolesUnresolved, keyID)
			}
			return nil, err
		}
		if actor == nil || len(actor.Roles) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrAPIKeyRolesUnresolved, keyID)
		}
		result.Roles = append([]string(nil), actor.Roles...)
		result.RoleSource = "actor"
		result.GeneratedBy = &KeyActor{ID: actor.ID, Name: actor.Name}
		if result.RevokedBy != nil {
			if revoked, ok := actors[result.RevokedBy.ID]; ok {
				result.RevokedBy.Name = revoked.Name
			}
		}
		return result, nil
	}

	if teamErr != nil {
		return nil, fmt.Errorf("%w: %s", ErrAPIKeyNotVisible, keyID)
	}
	return nil, fmt.Errorf("%w: %s", ErrAPIKeyNotFound, keyID)
}

func shouldFallbackToIndividualKeys(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Status {
	case http.StatusForbidden, http.StatusNotFound:
		return true
	default:
		return false
	}
}

func shouldFallbackToActorList(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Status {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed:
		return true
	default:
		return false
	}
}

func (c *Client) resolveActor(ctx context.Context, actorID string) (*olympusActor, map[string]olympusActor, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, nil, nil
	}

	actor, err := c.getActor(ctx, actorID)
	if err == nil {
		actors := map[string]olympusActor{
			actor.ID: *actor,
		}
		return actor, actors, nil
	}
	if !shouldFallbackToActorList(err) {
		return nil, nil, err
	}

	actors, listErr := c.listActors(ctx)
	if listErr != nil {
		if shouldFallbackToActorList(listErr) {
			return nil, nil, ErrAPIKeyRolesUnresolved
		}
		return nil, nil, listErr
	}
	byID := make(map[string]olympusActor, len(actors))
	for _, item := range actors {
		byID[item.ID] = item
	}
	match, ok := byID[actorID]
	if !ok {
		return nil, byID, nil
	}
	return &match, byID, nil
}
