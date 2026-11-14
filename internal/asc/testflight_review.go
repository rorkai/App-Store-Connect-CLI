package asc

// BetaAppReviewDetailAttributes describes beta app review details.
type BetaAppReviewDetailAttributes struct {
	ContactFirstName    string `json:"contactFirstName,omitempty"`
	ContactLastName     string `json:"contactLastName,omitempty"`
	ContactPhone        string `json:"contactPhone,omitempty"`
	ContactEmail        string `json:"contactEmail,omitempty"`
	DemoAccountName     string `json:"demoAccountName,omitempty"`
	DemoAccountPassword string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired bool   `json:"demoAccountRequired,omitempty"`
	Notes               string `json:"notes,omitempty"`

	api apiAttributes
}

// BetaAppReviewSubmissionAttributes describes beta app review submission metadata.
type BetaAppReviewSubmissionAttributes struct {
	BetaReviewState string `json:"betaReviewState,omitempty"`
	SubmittedDate   string `json:"submittedDate,omitempty"`
}

// BetaAppReviewDetailsResponse is the response from beta app review details list.
type BetaAppReviewDetailsResponse = Response[BetaAppReviewDetailAttributes]

// BetaAppReviewDetailResponse is the response from beta app review detail endpoints.
type BetaAppReviewDetailResponse = SingleResponse[BetaAppReviewDetailAttributes]

// BetaAppReviewSubmissionsResponse is the response from beta app review submissions list.
type BetaAppReviewSubmissionsResponse = Response[BetaAppReviewSubmissionAttributes]

// BetaAppReviewSubmissionResponse is the response from beta app review submission detail/create.
type BetaAppReviewSubmissionResponse = SingleResponse[BetaAppReviewSubmissionAttributes]

// BetaAppReviewDetailUpdateAttributes describes updateable beta app review details.
type BetaAppReviewDetailUpdateAttributes struct {
	ContactFirstName    *string `json:"contactFirstName,omitempty"`
	ContactLastName     *string `json:"contactLastName,omitempty"`
	ContactPhone        *string `json:"contactPhone,omitempty"`
	ContactEmail        *string `json:"contactEmail,omitempty"`
	DemoAccountName     *string `json:"demoAccountName,omitempty"`
	DemoAccountPassword *string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired *bool   `json:"demoAccountRequired,omitempty"`
	Notes               *string `json:"notes,omitempty"`
}

// BetaAppReviewDetailUpdateData is the data portion of a beta app review detail update.
type BetaAppReviewDetailUpdateData struct {
	Type       ResourceType                         `json:"type"`
	ID         string                               `json:"id"`
	Attributes *BetaAppReviewDetailUpdateAttributes `json:"attributes,omitempty"`
}

// BetaAppReviewDetailUpdateRequest is a request to update beta app review details.
type BetaAppReviewDetailUpdateRequest struct {
	Data BetaAppReviewDetailUpdateData `json:"data"`
}

// BetaAppReviewSubmissionCreateRelationships describes relationships for submission creates.
type BetaAppReviewSubmissionCreateRelationships struct {
	Build *Relationship `json:"build"`
}

// BetaAppReviewSubmissionCreateData is the data portion of a beta app review submission create.
type BetaAppReviewSubmissionCreateData struct {
	Type          ResourceType                                `json:"type"`
	Relationships *BetaAppReviewSubmissionCreateRelationships `json:"relationships"`
}

// BetaAppReviewSubmissionCreateRequest is a request to create a beta app review submission.
type BetaAppReviewSubmissionCreateRequest struct {
	Data BetaAppReviewSubmissionCreateData `json:"data"`
}
