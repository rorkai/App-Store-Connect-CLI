package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPerformanceDiagnosticsListNotFoundHint(t *testing.T) {
	const detail = "There is no resource of type '/v1/builds/{id}/diagnosticSignatures' with id 'build-1'"
	const hint = "Hint: The build may exist without diagnostic data: Apple reports diagnostics for App Store versions only after a few days of significant usage. Check that the build exists with `asc builds info --build-id \"build-1\"`.\n"
	for _, test := range []struct {
		extra []string
		query []string
	}{
		{},
		{extra: []string{"--paginate"}, query: []string{"limit=200"}},
	} {
		extra := test.extra
		t.Run(strings.Join(append([]string{"default"}, extra...), " "), func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/builds/build-1/diagnosticSignatures", test.query...),
				asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"`+detail+`"}]}`))

			args := append([]string{"performance", "diagnostics", "list", "--build-id", "build-1"}, extra...)
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(args, "1.2.3")
			})
			if code != cmd.ExitNotFound {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitNotFound, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, detail) || !strings.HasSuffix(stderr, hint) || strings.Count(stderr, "Hint:") != 1 {
				t.Fatalf("stderr = %q, want Apple's detail and one hint line %q", stderr, hint)
			}
		})
	}
}
