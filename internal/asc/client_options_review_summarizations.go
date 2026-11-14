package asc

// CustomerReviewSummarizationsOption is a functional option for review summarizations.
type CustomerReviewSummarizationsOption func(*customerReviewSummarizationsQuery)

// WithCustomerReviewSummarizationsPlatforms filters review summarizations by platform(s).
func WithCustomerReviewSummarizationsPlatforms(platforms []string) CustomerReviewSummarizationsOption {
	return func(q *customerReviewSummarizationsQuery) {
		q.platforms = normalizeList(platforms)
	}
}

// WithCustomerReviewSummarizationsTerritories filters review summarizations by territory(ies).
func WithCustomerReviewSummarizationsTerritories(territories []string) CustomerReviewSummarizationsOption {
	return func(q *customerReviewSummarizationsQuery) {
		q.territories = normalizeList(territories)
	}
}

// WithCustomerReviewSummarizationsFields sets fields[customerReviewSummarizations] for responses.
func WithCustomerReviewSummarizationsFields(fields []string) CustomerReviewSummarizationsOption {
	return func(q *customerReviewSummarizationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithCustomerReviewSummarizationsTerritoryFields sets fields[territories] for included territories.
func WithCustomerReviewSummarizationsTerritoryFields(fields []string) CustomerReviewSummarizationsOption {
	return func(q *customerReviewSummarizationsQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithCustomerReviewSummarizationsInclude sets include for review summarization responses.
func WithCustomerReviewSummarizationsInclude(include []string) CustomerReviewSummarizationsOption {
	return func(q *customerReviewSummarizationsQuery) {
		q.include = normalizeList(include)
	}
}
