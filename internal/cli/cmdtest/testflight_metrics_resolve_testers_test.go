package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const resolveTestersMetricsBody = `{
  "data": [
    {
      "dataPoints": [{"start": "2026-08-01", "end": "2026-08-07", "values": {"crashCount": 0, "sessionCount": 3, "feedbackCount": 1}}],
      "dimensions": {"betaTesters": {"data": "tester-1"}}
    },
    {
      "dataPoints": [{"start": "2026-08-01", "end": "2026-08-07", "values": {"crashCount": 1, "sessionCount": 5, "feedbackCount": 0}}],
      "dimensions": {"betaTesters": {"data": "tester-2"}}
    }
  ],
  "included": [
    {"type": "betaTesters", "id": "tester-1", "attributes": {"firstName": "Ada", "lastName": "Lovelace", "email": "ada@example.com", "inviteType": "EMAIL", "state": "INSTALLED"}}
  ],
  "links": {"next": ""}
}`

func setResolveTestersEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_APP_ID", "")

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func TestTestFlightMetricsBetaTesterUsagesResolveTesters(t *testing.T) {
	setResolveTestersEnv(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "groupBy=betaTesters"),
		asctest.JSON(http.StatusOK, resolveTestersMetricsBody))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[id]=tester-2", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-2","attributes":{"firstName":"Grace","lastName":"Hopper","email":"grace@example.com","inviteType":"PUBLIC_LINK","state":"INVITED"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "metrics", "app-testers", "--app", "app-123", "--resolve-testers"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if callCount := len(reg.Requests()); callCount != 2 {
		t.Fatalf("expected 2 requests, got %d", callCount)
	}

	var printed struct {
		Data    []json.RawMessage `json:"data"`
		Testers map[string]struct {
			ID         string `json:"id"`
			Email      string `json:"email"`
			FirstName  string `json:"firstName"`
			LastName   string `json:"lastName"`
			State      string `json:"state"`
			InviteType string `json:"inviteType"`
		} `json:"testers"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatalf("failed to parse printed JSON: %v\noutput: %q", err, stdout)
	}
	if len(printed.Data) != 2 {
		t.Fatalf("expected 2 data rows, got %d", len(printed.Data))
	}
	if len(printed.Testers) != 2 {
		t.Fatalf("expected 2 resolved testers, got %d", len(printed.Testers))
	}

	included, ok := printed.Testers["tester-1"]
	if !ok {
		t.Fatalf("expected tester-1 in testers sidecar, got %q", stdout)
	}
	if included.ID != "tester-1" || included.Email != "ada@example.com" || included.FirstName != "Ada" || included.LastName != "Lovelace" || included.State != "INSTALLED" || included.InviteType != "EMAIL" {
		t.Fatalf("unexpected tester-1 sidecar entry: %+v", included)
	}

	fetched, ok := printed.Testers["tester-2"]
	if !ok {
		t.Fatalf("expected tester-2 in testers sidecar, got %q", stdout)
	}
	if fetched.ID != "tester-2" || fetched.Email != "grace@example.com" || fetched.FirstName != "Grace" || fetched.LastName != "Hopper" || fetched.State != "INVITED" || fetched.InviteType != "PUBLIC_LINK" {
		t.Fatalf("unexpected tester-2 sidecar entry: %+v", fetched)
	}
}

func TestTestFlightMetricsBetaTesterUsagesWithoutResolveTestersOmitsSidecar(t *testing.T) {
	setResolveTestersEnv(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "groupBy=betaTesters"),
		asctest.JSON(http.StatusOK, resolveTestersMetricsBody))

	stdout, stderr, err := runCommand(t, []string{"testflight", "metrics", "app-testers", "--app", "app-123"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"tester-1\"") || !strings.Contains(stdout, "\"tester-2\"") {
		t.Fatalf("expected raw usage rows in output, got %q", stdout)
	}

	var printed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatalf("failed to parse printed JSON: %v\noutput: %q", err, stdout)
	}
	if _, ok := printed["testers"]; ok {
		t.Fatalf("expected no testers key without --resolve-testers, got %q", stdout)
	}
}
