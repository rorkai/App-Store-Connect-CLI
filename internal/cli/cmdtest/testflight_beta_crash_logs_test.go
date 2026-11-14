package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightCrashesLogNotFoundPointsToCrashSubmissions(t *testing.T) {
	const hint = "Hint: Crash logs are fetched by crash submission ID: list them with `asc testflight crashes list --app APP_ID` and pass one to --submission-id.\n"
	tests := []struct {
		name string
		args []string
		path string
		kind string
	}{
		{name: "submission id", args: []string{"--submission-id", "missing"}, path: "/v1/betaFeedbackCrashSubmissions/missing/crashLog", kind: "betaFeedbackCrashSubmissions"},
		{name: "crash log id", args: []string{"--crash-log-id", "missing"}, path: "/v1/betaCrashLogs/missing", kind: "betaCrashLogs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			detail := "There is no resource of type '" + test.kind + "' with id 'missing'"
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.path),
				asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"`+detail+`"}]}`))

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(append([]string{"testflight", "crashes", "log"}, test.args...), "1.2.3")
			})
			if code != rootcmd.ExitNotFound {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitNotFound, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, detail) || !strings.HasSuffix(stderr, hint) {
				t.Fatalf("stderr = %q, want Apple's detail and hint %q", stderr, hint)
			}
		})
	}
}
