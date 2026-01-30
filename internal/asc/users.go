package asc

// UserAttributes describes an App Store Connect user.
type UserAttributes struct {
	Username            string   `json:"username"`
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	Email               string   `json:"email,omitempty"`
	Roles               []string `json:"roles"`
	AllAppsVisible      bool     `json:"allAppsVisible"`
	ProvisioningAllowed bool     `json:"provisioningAllowed"`
}

// UserInvitationAttributes describes an App Store Connect user invitation.
type UserInvitationAttributes struct {
	Email               string   `json:"email"`
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	Roles               []string `json:"roles"`
	AllAppsVisible      bool     `json:"allAppsVisible"`
	ProvisioningAllowed bool     `json:"provisioningAllowed"`
	ExpirationDate      string   `json:"expirationDate,omitempty"`
}

// UsersResponse is the response from users endpoint.
type UsersResponse = Response[UserAttributes]

// UserResponse is the response from user detail endpoint.
type UserResponse = SingleResponse[UserAttributes]

// UserVisibleAppsLinkagesResponse is the response from user visible apps linkage endpoint.
type UserVisibleAppsLinkagesResponse = LinkagesResponse

// UserInvitationsResponse is the response from user invitations endpoint.
type UserInvitationsResponse = Response[UserInvitationAttributes]

// UserInvitationResponse is the response from user invitation detail endpoint.
type UserInvitationResponse = SingleResponse[UserInvitationAttributes]

// UserUpdateAttributes describes attributes for updating a user.
type UserUpdateAttributes struct {
	Roles               []string `json:"roles,omitempty"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
}

// UserUpdateData is the data portion of a user update request.
type UserUpdateData struct {
	Type       ResourceType          `json:"type"`
	ID         string                `json:"id"`
	Attributes *UserUpdateAttributes `json:"attributes,omitempty"`
}

// UserUpdateRequest is a request to update a user.
type UserUpdateRequest struct {
	Data UserUpdateData `json:"data"`
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

// UserInvitationCreateRelationships describes relationships for creating a user invitation.
type UserInvitationCreateRelationships struct {
	VisibleApps *RelationshipList `json:"visibleApps,omitempty"`
}

// UserInvitationCreateData is the data portion of a user invitation create request.
type UserInvitationCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    UserInvitationCreateAttributes     `json:"attributes"`
	Relationships *UserInvitationCreateRelationships `json:"relationships,omitempty"`
}

// UserInvitationCreateRequest is a request to create a user invitation.
type UserInvitationCreateRequest struct {
	Data UserInvitationCreateData `json:"data"`
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
