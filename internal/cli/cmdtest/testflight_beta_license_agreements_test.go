package cmdtest

import (
	"net/http"
	"net/url"
	"testing"
)

func TestTestFlightBetaLicenseAgreementsUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "view missing id and app",
			args:    []string{"testflight", "agreements", "view"},
			wantErr: "Error: --id or --app is required",
		},
		{
			name:    "view id and app both set",
			args:    []string{"testflight", "agreements", "view", "--id", "AGREEMENT_ID", "--app", "APP_ID"},
			wantErr: "Error: --id and --app are mutually exclusive",
		},
		{
			name:    "view app with include",
			args:    []string{"testflight", "agreements", "view", "--app", "APP_ID", "--include", "app"},
			wantErr: "Error: --app-fields and --include are only valid with --id",
		},
		{
			name:    "edit missing id",
			args:    []string{"testflight", "agreements", "edit", "--agreement-text", "Updated"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "edit missing agreement text",
			args:    []string{"testflight", "agreements", "edit", "--id", "AGREEMENT_ID"},
			wantErr: "Error: --agreement-text is required",
		},
	})
}

func TestTestFlightBetaLicenseAgreementsSendRequest(t *testing.T) {
	runTestFlightRequestCases(t, []testFlightRequestCase{
		{
			name:       "list",
			args:       []string{"testflight", "agreements", "list", "--app", "app-1", "--limit", "2"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaLicenseAgreements",
			wantQuery:  url.Values{"filter[app]": {"app-1"}, "limit": {"2"}},
			respBody:   `{"data":[{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Terms"}}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"agree-1"`},
		},
		{
			name:       "view by id",
			args:       []string{"testflight", "agreements", "view", "--id", "agree-1", "--fields", "agreementText"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaLicenseAgreements/agree-1",
			wantQuery:  url.Values{"fields[betaLicenseAgreements]": {"agreementText"}},
			respBody:   `{"data":{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Terms"}}}`,
			wantStdout: []string{`"id":"agree-1"`},
		},
		{
			name:       "view by app",
			args:       []string{"testflight", "agreements", "view", "--app", "app-1", "--fields", "agreementText"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/apps/app-1/betaLicenseAgreement",
			wantQuery:  url.Values{"fields[betaLicenseAgreements]": {"agreementText"}},
			respBody:   `{"data":{"type":"betaLicenseAgreements","id":"agree-2","attributes":{"agreementText":"Terms"}}}`,
			wantStdout: []string{`"id":"agree-2"`},
		},
		{
			name:       "edit",
			args:       []string{"testflight", "agreements", "edit", "--id", "agree-1", "--agreement-text", "Updated terms"},
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/betaLicenseAgreements/agree-1",
			wantBody:   `{"data":{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Updated terms"}}}`,
			respBody:   `{"data":{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Updated terms"}}}`,
			wantStdout: []string{`"id":"agree-1"`},
		},
	})
}
