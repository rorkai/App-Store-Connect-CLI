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

// BetaAppReviewSubmissionCreateRelationships describes relationships for submission creates.
type BetaAppReviewSubmissionCreateRelationships struct {
	Build *Relationship `json:"build"`
}

// BetaAppReviewSubmissionCreateData is the data portion of a beta app review submission create.
type BetaAppReviewSubmissionCreateData struct {
	Type          ResourceType                                `json:"type"`
	Relationships *BetaAppReviewSubmissionCreateRelationships `json:"relationships"`
}
