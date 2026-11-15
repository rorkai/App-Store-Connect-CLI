package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type certificatesListQuerySurfaceRequest struct {
	calls  int
	path   string
	query  url.Values
	client func() (*asc.Client, error)
}

func certificatesListQuerySurfaceStub(t *testing.T) *certificatesListQuerySurfaceRequest {
	t.Helper()
	return certificatesListQuerySurfaceStubBody(t, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"name":"Certificate","certificateType":"IOS_DISTRIBUTION"}}]}`)
}

func certificatesListQuerySurfaceStubBody(t *testing.T, body string) *certificatesListQuerySurfaceRequest {
	t.Helper()
	setupAuth(t)

	captured := &certificatesListQuerySurfaceRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured.calls++
		captured.path = req.URL.Path
		captured.query = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	client := newReviewTestServerClient(t, server)
	captured.client = func() (*asc.Client, error) {
		return client, nil
	}

	return captured
}

func certificatesListQuerySurfaceValidationStub(t *testing.T) *certificatesListQuerySurfaceRequest {
	t.Helper()

	captured := &certificatesListQuerySurfaceRequest{}
	captured.client = func() (*asc.Client, error) {
		captured.calls++
		return nil, errors.New("client factory must not run during validation")
	}
	return captured
}

func runCertificatesListQuerySurface(t *testing.T, captured *certificatesListQuerySurfaceRequest, args ...string) (string, string, error) {
	t.Helper()

	return runRootWithDeps(t, captured.client, args...)
}

func TestCertificatesListQuerySurfaceEmitsDocumentedParameters(t *testing.T) {
	captured := certificatesListQuerySurfaceStub(t)

	stdout, stderr, err := runCertificatesListQuerySurface(
		t, captured,
		"certificates", "list",
		"--display-name", "Alpha,Beta",
		"--certificate-type", "ios_distribution,pass_type_id",
		"--serial-number", "SN1,SN2",
		"--id", "cert-1,cert-2",
		"--sort=-displayName, serialNumber",
		"--fields", "displayName,serialNumber",
		"--pass-type-id-fields", "name,identifier",
		"--include", "passTypeId",
		"--limit", "5",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if captured.path != "/v1/certificates" {
		t.Fatalf("path = %q, want /v1/certificates", captured.path)
	}
	wantQuery := map[string]string{
		"filter[displayName]":     "Alpha,Beta",
		"filter[certificateType]": "IOS_DISTRIBUTION,PASS_TYPE_ID",
		"filter[serialNumber]":    "SN1,SN2",
		"filter[id]":              "cert-1,cert-2",
		"sort":                    "-displayName,serialNumber",
		"fields[certificates]":    "displayName,serialNumber",
		"fields[passTypeIds]":     "name,identifier",
		"include":                 "passTypeId",
		"limit":                   "5",
	}
	for key, want := range wantQuery {
		if got := captured.query.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q (full query %s)", key, got, want, captured.query.Encode())
		}
	}
	if len(captured.query) != len(wantQuery) {
		t.Fatalf("query = %s, want exactly %d parameters", captured.query.Encode(), len(wantQuery))
	}
	if !strings.Contains(stdout, `"id":"cert-1"`) {
		t.Fatalf("stdout = %q, want certificate response", stdout)
	}
}

func TestCertificatesListSparseFieldsPreserveIncludedPassTypeID(t *testing.T) {
	const includedPassTypeIDs = `[{"type":"passTypeIds","id":"ptid-1","attributes":{"name":"Pass Type"}}]`
	tests := []struct {
		name string
		body string
	}{
		{
			name: "relationships and included",
			body: `{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"Pass Cert"},"relationships":{"passTypeId":{"data":{"type":"passTypeIds","id":"ptid-1"}}}}],"links":{},"included":` + includedPassTypeIDs + `}`,
		},
		{
			name: "included without relationships",
			body: `{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"Pass Cert"}}],"links":{},"included":` + includedPassTypeIDs + `}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := certificatesListQuerySurfaceStubBody(t, test.body)

			stdout, stderr, err := runCertificatesListQuerySurface(
				t, captured,
				"certificates", "list",
				"--fields", "displayName",
				"--include", "passTypeId",
				"--pass-type-id-fields", "name",
				"--output", "json",
			)
			if err != nil {
				t.Fatalf("run error: %v (stderr=%q)", err, stderr)
			}
			if strings.TrimSpace(stderr) != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if captured.path != "/v1/certificates" {
				t.Fatalf("path = %q, want /v1/certificates", captured.path)
			}
			wantQuery := map[string]string{
				"fields[certificates]": "displayName",
				"fields[passTypeIds]":  "name",
				"include":              "passTypeId",
			}
			for key, want := range wantQuery {
				if got := captured.query.Get(key); got != want {
					t.Fatalf("query %s = %q, want %q (full query %s)", key, got, want, captured.query.Encode())
				}
			}
			if len(captured.query) != len(wantQuery) {
				t.Fatalf("query = %s, want exactly %d parameters", captured.query.Encode(), len(wantQuery))
			}
			if stdout != test.body+"\n" {
				t.Fatalf("stdout = %q, want byte-identical envelope %q", stdout, test.body+"\n")
			}
		})
	}
}

func TestCertificatesListRejectsQueryFlagsCombinedWithNext(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/certificates?cursor=PAGE2"

	for _, testCase := range []struct {
		name string
		flag string
		args []string
	}{
		{name: "display name", flag: "--display-name", args: []string{"--display-name", "Certificate"}},
		{name: "certificate type", flag: "--certificate-type", args: []string{"--certificate-type", "IOS_DISTRIBUTION"}},
		{name: "serial number", flag: "--serial-number", args: []string{"--serial-number", "SN1"}},
		{name: "id", flag: "--id", args: []string{"--id", "cert-1"}},
		{name: "sort", flag: "--sort", args: []string{"--sort", "displayName"}},
		{name: "fields", flag: "--fields", args: []string{"--fields", "displayName"}},
		{name: "pass type ID fields", flag: "--pass-type-id-fields", args: []string{"--pass-type-id-fields", "name"}},
		{name: "include", flag: "--include", args: []string{"--include", "passTypeId"}},
		{name: "limit", flag: "--limit", args: []string{"--limit", "5"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			captured := certificatesListQuerySurfaceValidationStub(t)
			args := append([]string{"certificates", "list", "--next", nextURL}, testCase.args...)
			_, stderr, err := runCertificatesListQuerySurface(t, captured, args...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			want := "certificates list: --next cannot be combined with " + testCase.flag
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if captured.calls != 0 {
				t.Fatalf("validation made %d request(s)", captured.calls)
			}
		})
	}
}

func TestCertificatesListRejectsInvalidQuerySelections(t *testing.T) {
	for _, testCase := range []struct {
		name string
		args []string
		want string
	}{
		{name: "sort", args: []string{"--sort", "createdDate"}, want: "--sort must be one of"},
		{name: "sort list", args: []string{"--sort", "displayName,createdDate"}, want: "--sort must be one of"},
		{name: "fields", args: []string{"--fields", "createdDate"}, want: "--fields must be one of"},
		{name: "pass type ID fields", args: []string{"--pass-type-id-fields", "createdDate"}, want: "--pass-type-id-fields must be one of"},
		{name: "include", args: []string{"--include", "profiles"}, want: "--include must be one of"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			captured := certificatesListQuerySurfaceValidationStub(t)
			_, stderr, err := runCertificatesListQuerySurface(t, captured, append([]string{"certificates", "list"}, testCase.args...)...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if !strings.Contains(stderr, testCase.want) {
				t.Fatalf("stderr = %q, want %q", stderr, testCase.want)
			}
			if captured.calls != 0 {
				t.Fatalf("validation made %d request(s)", captured.calls)
			}
		})
	}
}

func TestCertificatesListRejectsExplicitlyEmptySelectors(t *testing.T) {
	for _, name := range []string{
		"certificate-type",
		"display-name",
		"serial-number",
		"id",
		"sort",
		"fields",
		"pass-type-id-fields",
		"include",
	} {
		t.Run(name, func(t *testing.T) {
			captured := certificatesListQuerySurfaceValidationStub(t)
			_, stderr, err := runCertificatesListQuerySurface(t, captured, "certificates", "list", "--"+name, ",")
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			want := "certificates list: --" + name + " must not be empty"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if captured.calls != 0 {
				t.Fatalf("validation made %d client-factory call(s)", captured.calls)
			}
		})
	}
}

func TestCertificatesListPassTypeIDFieldsRequiresInclude(t *testing.T) {
	captured := certificatesListQuerySurfaceValidationStub(t)

	_, stderr, err := runCertificatesListQuerySurface(
		t, captured,
		"certificates", "list",
		"--pass-type-id-fields", "name",
	)
	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
	}
	if want := "Error: --pass-type-id-fields requires --include passTypeId\n"; stderr != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr, want)
	}
	if captured.calls != 0 {
		t.Fatalf("validation made %d client-factory call(s)", captured.calls)
	}
}
