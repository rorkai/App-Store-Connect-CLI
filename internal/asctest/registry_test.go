package asctest

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type recorder struct {
	testing.TB
	errors   []string
	cleanups []func()
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recorder) Cleanup(fn func()) { r.cleanups = append(r.cleanups, fn) }

func (r *recorder) finish() {
	for i := len(r.cleanups) - 1; i >= 0; i-- {
		r.cleanups[i]()
	}
}

func send(t *testing.T, rt http.RoundTripper, method, rawURL, body string) (string, error) {
	t.Helper()

	req, err := http.NewRequest(method, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := (&http.Client{Transport: rt}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return string(data), nil
}

func TestRegistryMatching(t *testing.T) {
	const base = "https://" + apiHost
	tests := []struct {
		name    string
		matcher Matcher
		method  string
		url     string
		body    string
		want    bool
	}{
		{"method path and query", GET("/v1/apps", "limit=50", "filter[bundleId]=com.example"), http.MethodGet, base + "/v1/apps?filter%5BbundleId%5D=com.example&limit=50", "", true},
		{"other method", GET("/v1/apps"), http.MethodPost, base + "/v1/apps", "", false},
		{"other host", GET("/v1/apps"), http.MethodGet, "https://example.com/v1/apps", "", false},
		{"other path", GET("/v1/apps"), http.MethodGet, base + "/v1/apps/1", "", false},
		{"extra query pair", GET("/v1/apps", "limit=50"), http.MethodGet, base + "/v1/apps?limit=50&sort=name", "", false},
		{"missing query pair", GET("/v1/apps", "limit=50"), http.MethodGet, base + "/v1/apps", "", false},
		{"other query value", GET("/v1/apps", "limit=50"), http.MethodGet, base + "/v1/apps?limit=51", "", false},
		{"equal JSON body", PATCH("/v1/apps/1").JSONBody(`{"data":{"id":"1","type":"apps"}}`), http.MethodPatch, base + "/v1/apps/1", `{"data":{"type":"apps","id":"1"}}`, true},
		{"other JSON body", POST("/v1/apps").JSONBody(`{"data":{"id":"1"}}`), http.MethodPost, base + "/v1/apps", `{"data":{"id":"2"}}`, false},
		{"body that is not JSON", DELETE("/v1/apps/1").JSONBody(`{}`), http.MethodDelete, base + "/v1/apps/1", `not json`, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := &recorder{TB: t}
			reg := NewRegistry(rec)
			reg.Register(test.matcher, JSON(http.StatusOK, `{"data":[]}`))

			got, err := send(t, reg, test.method, test.url, test.body)
			rec.finish()

			if test.want {
				if err != nil || got != `{"data":[]}` || len(rec.errors) != 0 {
					t.Fatalf("want stub response and no failures, got body %q, err %v, failures %q", got, err, rec.errors)
				}
				return
			}
			if err == nil {
				t.Fatalf("want transport error for unmatched request, got body %q", got)
			}
			if len(rec.errors) != 2 ||
				!strings.Contains(rec.errors[0], "unexpected request "+test.method+" "+test.url) ||
				!strings.Contains(rec.errors[1], "stubs never matched: ["+test.matcher.String()+"]") {
				t.Fatalf("want unexpected-request and unmatched-stub failures, got %q", rec.errors)
			}
		})
	}
}

func TestRegistryAnswersEachStubOnceInRegistrationOrder(t *testing.T) {
	original := http.DefaultTransport
	rec := &recorder{TB: t}
	reg := NewRegistry(rec)
	reg.Install()
	reg.Register(POST("/v1/apps"), JSON(http.StatusCreated, `"first"`))
	reg.Register(POST("/v1/apps"), JSON(http.StatusOK, `"second"`))

	url := "https://" + apiHost + "/v1/apps"
	for _, want := range []string{`"first"`, `"second"`} {
		got, err := send(t, http.DefaultTransport, http.MethodPost, url, want)
		if err != nil || got != want {
			t.Fatalf("want %s, got %q, err %v", want, got, err)
		}
	}
	if _, err := send(t, http.DefaultTransport, http.MethodPost, url, `"third"`); err == nil {
		t.Fatal("want transport error once every stub has answered")
	}
	rec.finish()

	if http.DefaultTransport != original {
		t.Fatal("want http.DefaultTransport restored at cleanup")
	}
	if len(rec.errors) != 1 || !strings.Contains(rec.errors[0], `unexpected request POST `+url+` body "\"third\""`) {
		t.Fatalf("want one unexpected-request failure, got %q", rec.errors)
	}
	var log []string
	for _, req := range reg.Requests() {
		log = append(log, req.Method+" "+req.URL.String()+" "+string(req.Body))
	}
	if want := []string{"POST " + url + ` "first"`, "POST " + url + ` "second"`, "POST " + url + ` "third"`}; fmt.Sprint(log) != fmt.Sprint(want) {
		t.Fatalf("want request log %q, got %q", want, log)
	}
}
