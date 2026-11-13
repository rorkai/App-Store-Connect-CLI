package cmdtest

import (
	"net/http"
	"net/url"
	"testing"
)

func TestTestFlightReviewCommandsSendRequest(t *testing.T) {
	runTestFlightRequestCases(t, []testFlightRequestCase{
		{
			name:       "review view",
			args:       []string{"testflight", "review", "view", "--app", "app-1", "--limit", "2"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaAppReviewDetails",
			wantQuery:  url.Values{"filter[app]": {"app-1"}, "limit": {"2"}},
			respBody:   `{"data":[{"type":"betaAppReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"detail-1"`},
		},
		{
			name:       "review edit",
			args:       []string{"testflight", "review", "edit", "--id", "detail-1", "--contact-email", "dev@example.com"},
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/betaAppReviewDetails/detail-1",
			wantBody:   `{"data":{"type":"betaAppReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`,
			respBody:   `{"data":{"type":"betaAppReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`,
			wantStdout: []string{`"id":"detail-1"`},
		},
		{
			name:       "review submit",
			args:       []string{"testflight", "review", "submit", "--build-id", "build-1", "--confirm"},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/betaAppReviewSubmissions",
			wantBody:   `{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`,
			respStatus: http.StatusCreated,
			respBody:   `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1"}}`,
			wantStdout: []string{`"id":"submission-1"`},
		},
		{
			name:       "review app view",
			args:       []string{"testflight", "review", "app", "view", "--id", "detail-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaAppReviewDetails/detail-1/app",
			respBody:   `{"data":{"type":"apps","id":"app-1","attributes":{"name":"App"}}}`,
			wantStdout: []string{`"id":"app-1"`},
		},
		{
			name:       "review submissions view",
			args:       []string{"testflight", "review", "submissions", "view", "--id", "submission-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaAppReviewSubmissions/submission-1",
			respBody:   `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1","attributes":{"betaReviewState":"WAITING_FOR_REVIEW"}}}`,
			wantStdout: []string{`"id":"submission-1"`},
		},
		{
			name:       "review submissions build",
			args:       []string{"testflight", "review", "submissions", "build", "--id", "submission-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaAppReviewSubmissions/submission-1/build",
			respBody:   `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0"}}}`,
			wantStdout: []string{`"id":"build-1"`},
		},
	})
}
