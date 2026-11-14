package asc

// BetaLicenseAgreementsOption is a functional option for beta license agreements.
type BetaLicenseAgreementsOption func(*betaLicenseAgreementsQuery)

// BetaLicenseAgreementOption is a functional option for beta license agreement detail.
type BetaLicenseAgreementOption func(*betaLicenseAgreementQuery)

// WithBetaLicenseAgreementsAppIDs filters beta license agreements by app ID(s).
func WithBetaLicenseAgreementsAppIDs(appIDs []string) BetaLicenseAgreementsOption {
	return func(q *betaLicenseAgreementsQuery) {
		q.appIDs = normalizeList(appIDs)
	}
}

// WithBetaLicenseAgreementsFields sets fields[betaLicenseAgreements] for responses.
func WithBetaLicenseAgreementsFields(fields []string) BetaLicenseAgreementsOption {
	return func(q *betaLicenseAgreementsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithBetaLicenseAgreementsAppFields sets fields[apps] for included app responses.
func WithBetaLicenseAgreementsAppFields(fields []string) BetaLicenseAgreementsOption {
	return func(q *betaLicenseAgreementsQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithBetaLicenseAgreementsInclude sets include for beta license agreements responses.
func WithBetaLicenseAgreementsInclude(include []string) BetaLicenseAgreementsOption {
	return func(q *betaLicenseAgreementsQuery) {
		q.include = normalizeList(include)
	}
}

// WithBetaLicenseAgreementFields sets fields[betaLicenseAgreements] for detail responses.
func WithBetaLicenseAgreementFields(fields []string) BetaLicenseAgreementOption {
	return func(q *betaLicenseAgreementQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithBetaLicenseAgreementAppFields sets fields[apps] for detail responses.
func WithBetaLicenseAgreementAppFields(fields []string) BetaLicenseAgreementOption {
	return func(q *betaLicenseAgreementQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithBetaLicenseAgreementInclude sets include for beta license agreement detail responses.
func WithBetaLicenseAgreementInclude(include []string) BetaLicenseAgreementOption {
	return func(q *betaLicenseAgreementQuery) {
		q.include = normalizeList(include)
	}
}
