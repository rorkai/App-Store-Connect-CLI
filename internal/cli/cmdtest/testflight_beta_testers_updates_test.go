package cmdtest

import (
	"net/http"
	"testing"
)

func TestTestFlightBetaTesterRelationshipUpdatesSendRequest(t *testing.T) {
	runTestFlightRequestCases(t, []testFlightRequestCase{
		{
			name:       "add builds",
			args:       []string{"testflight", "testers", "add-builds", "--id", "tester-1", "--build-id", "build-1,build-2"},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/betaTesters/tester-1/relationships/builds",
			wantBody:   `{"data":[{"type":"builds","id":"build-1"},{"type":"builds","id":"build-2"}]}`,
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"testerId":"tester-1"`, `"buildIds"`, `"action":"added"`},
			wantStderr: "Successfully added tester tester-1",
		},
		{
			name:       "remove builds",
			args:       []string{"testflight", "testers", "remove-builds", "--id", "tester-2", "--build-id", "build-3,build-4", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaTesters/tester-2/relationships/builds",
			wantBody:   `{"data":[{"type":"builds","id":"build-3"},{"type":"builds","id":"build-4"}]}`,
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"testerId":"tester-2"`, `"buildIds"`, `"action":"removed"`},
			wantStderr: "Successfully removed tester tester-2",
		},
		{
			name:       "remove apps",
			args:       []string{"testflight", "testers", "remove-apps", "--id", "tester-3", "--app", "app-1,app-2", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaTesters/tester-3/relationships/apps",
			wantBody:   `{"data":[{"type":"apps","id":"app-1"},{"type":"apps","id":"app-2"}]}`,
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"testerId":"tester-3"`, `"appIds"`, `"action":"removed"`},
			wantStderr: "Successfully removed tester tester-3",
		},
		{
			name:       "add groups",
			args:       []string{"testflight", "testers", "add-groups", "--id", "tester-1", "--group", "group-1,group-2"},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/betaTesters/tester-1/relationships/betaGroups",
			wantBody:   `{"data":[{"type":"betaGroups","id":"group-1"},{"type":"betaGroups","id":"group-2"}]}`,
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"action":"added"`},
			wantStderr: "Successfully added tester tester-1",
		},
		{
			name:       "remove groups",
			args:       []string{"testflight", "testers", "remove-groups", "--id", "tester-2", "--group", "group-3", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaTesters/tester-2/relationships/betaGroups",
			wantBody:   `{"data":[{"type":"betaGroups","id":"group-3"}]}`,
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"action":"removed"`},
			wantStderr: "Successfully removed tester tester-2",
		},
	})
}
