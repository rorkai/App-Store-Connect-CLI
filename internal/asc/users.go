package asc

import (
	"encoding/json"
	"fmt"
)

// UserAttributes describes an App Store Connect user.
type UserAttributes struct {
	Username            string   `json:"username"`
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	Email               string   `json:"email,omitempty"`
	Roles               []string `json:"roles"`
	AllAppsVisible      bool     `json:"allAppsVisible"`
	ProvisioningAllowed bool     `json:"provisioningAllowed"`
	sparseFields        map[string]struct{}
	sparseExtras        map[string]json.RawMessage
}

// UnmarshalJSON records which user attributes App Store Connect supplied.
// Sparse fieldsets intentionally omit attributes, so a plain struct round-trip
// would otherwise reintroduce zero-valued fields into JSON output.
func (a *UserAttributes) UnmarshalJSON(data []byte) error {
	type userAttributesAlias UserAttributes
	var decoded userAttributesAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawFields); err != nil {
		return err
	}

	*a = UserAttributes(decoded)
	a.sparseFields = make(map[string]struct{}, len(rawFields))
	for name := range rawFields {
		a.sparseFields[name] = struct{}{}
	}
	for name, raw := range rawFields {
		switch name {
		case "username", "firstName", "lastName", "email", "roles", "allAppsVisible", "provisioningAllowed":
			continue
		default:
			if a.sparseExtras == nil {
				a.sparseExtras = make(map[string]json.RawMessage)
			}
			a.sparseExtras[name] = append(json.RawMessage(nil), raw...)
		}
	}
	return nil
}

// hasField reports whether a decoded response carried the named attribute.
// Attributes built in code count as present.
func (a UserAttributes) hasField(name string) bool {
	if a.sparseFields == nil {
		return true
	}
	_, ok := a.sparseFields[name]
	return ok
}

// MarshalJSON keeps sparse user responses sparse while retaining the existing
// full struct shape for callers that construct UserAttributes directly.
func (a UserAttributes) MarshalJSON() ([]byte, error) {
	if a.sparseFields == nil {
		type userAttributesAlias UserAttributes
		return json.Marshal(userAttributesAlias(a))
	}

	fields := make(map[string]json.RawMessage, len(a.sparseFields)+len(a.sparseExtras))
	for name, raw := range a.sparseExtras {
		fields[name] = append(json.RawMessage(nil), raw...)
	}
	add := func(name string, value any) error {
		if _, ok := a.sparseFields[name]; !ok {
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal user attribute %q: %w", name, err)
		}
		fields[name] = raw
		return nil
	}
	if err := add("username", a.Username); err != nil {
		return nil, err
	}
	if err := add("firstName", a.FirstName); err != nil {
		return nil, err
	}
	if err := add("lastName", a.LastName); err != nil {
		return nil, err
	}
	if err := add("email", a.Email); err != nil {
		return nil, err
	}
	if err := add("roles", a.Roles); err != nil {
		return nil, err
	}
	if err := add("allAppsVisible", a.AllAppsVisible); err != nil {
		return nil, err
	}
	if err := add("provisioningAllowed", a.ProvisioningAllowed); err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

// UserInvitationAttributes describes an App Store Connect user invitation.
type UserInvitationAttributes struct {
	Email               string   `json:"email"`
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	Roles               []string `json:"roles"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
	ExpirationDate      string   `json:"expirationDate,omitempty"`
}

// UserVisibleAppsLinkagesResponse is the response from user visible apps linkage endpoint.
type UserVisibleAppsLinkagesResponse = LinkagesResponse

// UserUpdateAttributes describes attributes for updating a user.
type UserUpdateAttributes struct {
	Roles               []string `json:"roles,omitempty"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
}

// UserUpdateRelationships describes relationships for updating a user.
type UserUpdateRelationships struct {
	VisibleApps *RelationshipList `json:"visibleApps,omitempty"`
}

// UserUpdateData is the data portion of a user update request.
type UserUpdateData struct {
	Type          ResourceType             `json:"type"`
	ID            string                   `json:"id"`
	Attributes    *UserUpdateAttributes    `json:"attributes,omitempty"`
	Relationships *UserUpdateRelationships `json:"relationships,omitempty"`
}

// UserInvitationCreateAttributes describes attributes for creating a user invitation.
type UserInvitationCreateAttributes struct {
	Email               string   `json:"email"`
	FirstName           string   `json:"firstName,omitempty"`
	LastName            string   `json:"lastName,omitempty"`
	Roles               []string `json:"roles"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
}

// UserDeleteResult represents CLI output for user deletion.
type UserDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// UserInvitationRevokeResult represents CLI output for invitation revocation.
type UserInvitationRevokeResult struct {
	ID      string `json:"id"`
	Revoked bool   `json:"revoked"`
}
