package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestListAppComplianceRequirements(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []AppComplianceRequirement
	}{
		{
			name: "answered and unanswered requirements",
			body: `{"data":[
				{"id":"req-1","formName":"SECURITYASF395E","entityId":"app-123","entityType":"APP",
				 "status":"COLLECTED_VERIFY_PENDING","isRequired":true,"countries":["CHN"],"formId":"form-1",
				 "dateSigned":"2026-01-01T00:00:00.000Z","lastUpdatedDate":"2026-01-02T00:00:00.000Z",
				 "requiredActions":["APP_BINARY_SUBMISSION"]},
				{"id":"req-2","formName":"AIGENERATEFD1DE","entityId":"app-123","entityType":"APP",
				 "status":"PENDING_COLLECT","isRequired":true,"countries":["CHN"]},
				{"id":"req-3","formName":"FUTUREFORM123","status":"PENDING_COLLECT","isRequired":false}
			]}`,
			want: []AppComplianceRequirement{
				{
					AppID: "app-123", RequirementID: "req-1", RequirementName: "SECURITY_ASSESSMENT", FormName: "SECURITYASF395E",
					Status: "COLLECTED_VERIFY_PENDING", Required: true, CountriesOrRegions: []string{"CHN"}, FormID: "form-1",
					DateSigned: "2026-01-01T00:00:00.000Z", LastUpdatedDate: "2026-01-02T00:00:00.000Z",
					RequiredActions: []string{"APP_BINARY_SUBMISSION"},
				},
				{
					AppID: "app-123", RequirementID: "req-2", RequirementName: "AI_GENERATED_CONTENT", FormName: "AIGENERATEFD1DE",
					Status: "PENDING_COLLECT", Required: true, CountriesOrRegions: []string{"CHN"},
				},
				{
					AppID: "app-123", RequirementID: "req-3", RequirementName: "FUTUREFORM123", FormName: "FUTUREFORM123",
					Status: "PENDING_COLLECT",
				},
			},
		},
		{
			name: "no requirements omits data",
			body: `{}`,
			want: []AppComplianceRequirement{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ppm/compliancerequirements/v1/accounts/account-123/requirements" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				}
				if got := r.URL.RawQuery; got != "entity_id=app-123" {
					t.Fatalf("expected query entity_id=app-123, got %q", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			got, err := testWebClient(server).ListAppComplianceRequirements(context.Background(), "account-123", " app-123 ")
			if err != nil {
				t.Fatalf("ListAppComplianceRequirements() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected requirements:\n got %#v\nwant %#v", got, tc.want)
			}
		})
	}
}

// appComplianceFormServer imitates the App Compliance form service: it lists
// the two requirements, answers each draft with the fields that draft opens,
// and records the request that saves a declaration.
type appComplianceFormServer struct {
	t            *testing.T
	requirements string
	readOnly     bool
	unknownField bool
	closedBranch bool
	saved        *http.Request
	savedBody    string
}

const appComplianceTestRequirements = `{"data":[
	{"id":"req-sec","formName":"SECURITYASF395E","status":"PENDING_COLLECT","isRequired":true,"countries":["CHN"]},
	{"id":"req-ai","formName":"AIGENERATEFD1DE","status":"COLLECTED_VERIFY_NOT_NECESSARY","isRequired":true,"countries":["CHN"],"formId":"form-ai"}
]}`

func (s *appComplianceFormServer) constraint(form, id, slug, inputType string, options ...string) string {
	editable := `,"editable":true`
	if s.readOnly {
		editable = ""
	}
	extra := ""
	if inputType == "FILE" {
		extra = `,"allowedFileTypes":[".pdf",".png"]`
	}
	opts := make([]string, 0, len(options))
	for _, option := range options {
		opts = append(opts, `{"valueId":"`+option+`"}`)
	}
	return `"` + id + `":{"presentation":{"id":"` + id + `","inputType":"` + inputType +
		`","label":"PPM.compliance.form.` + form + `.field.` + slug + `.label","options":[` + strings.Join(opts, ",") +
		`]},"validation":{"required":true` + editable + extra + `}}`
}

func (s *appComplianceFormServer) metadata(requirementID string, draft map[string]string) string {
	var constraints []string
	if requirementID == "req-sec" {
		form := "SECURITYASF395E"
		constraints = append(constraints, s.constraint(form, "sec-q", "doesyourap2852f", "RADIO", "YES", "NO"))
		if draft["sec-q"] == "YES" && !s.closedBranch {
			constraints = append(constraints,
				s.constraint(form, "sec-proof", "proofofacocb097", "FILE"),
				s.constraint(form, "sec-report", "securityascf4ff1", "FILE"))
		}
	} else {
		form := "AIGENERATEFD1DE"
		constraints = append(constraints, s.constraint(form, "ai-q", "doesyourap268df2", "RADIO", "YES", "NO"))
		if draft["ai-q"] == "YES" {
			constraints = append(constraints,
				s.constraint(form, "ai-type", "whichfilinc6deb2", "RADIO", "LARGE", "ALGOR"),
				s.constraint(form, "ai-third", "areyourappd75c12", "RADIO", "YES", "NO"))
			switch draft["ai-type"] {
			case "LARGE":
				constraints = append(constraints,
					s.constraint(form, "ai-llm-name", "llmname2", "TEXT"),
					s.constraint(form, "ai-llm-filing", "llmfilingnumber4", "TEXT"))
			case "ALGOR":
				constraints = append(constraints,
					s.constraint(form, "ai-algo-name", "algorithmname2", "TEXT"),
					s.constraint(form, "ai-algo-filing", "algorithmf737252", "TEXT"))
			}
			if draft["ai-third"] == "YES" {
				constraints = append(constraints, s.constraint(form, "ai-auth", "authorizat9286a3", "FILE"))
			}
		}
		if s.unknownField {
			constraints = append(constraints, s.constraint(form, "ai-new", "anewquestion1", "TEXT"))
		}
	}
	return `{"constraints":{` + strings.Join(constraints, ",") + `}}`
}

func (s *appComplianceFormServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const base = "/ppm/compliancerequirements/v1/accounts/account-123"
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == base+"/requirements":
		_, _ = w.Write([]byte(s.requirements))
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, base+"/entities/app-123/requirements/") && strings.HasSuffix(r.URL.Path, "/metadata"):
		var request struct {
			AccountID     string `json:"accountId"`
			EntityID      string `json:"entityId"`
			RequirementID string `json:"requirementId"`
			Form          struct {
				Fields []struct {
					ID     string `json:"id"`
					SValue string `json:"sValue"`
				} `json:"fields"`
			} `json:"form"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			s.t.Errorf("metadata body %s: %v", body, err)
		}
		if request.AccountID != "account-123" || request.EntityID != "app-123" ||
			r.URL.Path != base+"/entities/app-123/requirements/"+request.RequirementID+"/metadata" {
			s.t.Errorf("metadata request does not name its account, app and requirement: %s %s", r.URL.Path, body)
		}
		draft := map[string]string{}
		for _, field := range request.Form.Fields {
			draft[field.ID] = field.SValue
		}
		_, _ = w.Write([]byte(s.metadata(request.RequirementID, draft)))
	case strings.HasPrefix(r.URL.Path, base+"/entities/app-123/forms"):
		if s.saved != nil {
			s.t.Errorf("declaration saved twice: %s %s", r.Method, r.URL.Path)
		}
		s.saved, s.savedBody = r, string(body)
		_, _ = w.Write([]byte(`{"data":{"id":"form-saved","status":"COLLECTED_VERIFY_PENDING"}}`))
	default:
		s.t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSetAppComplianceDeclaration(t *testing.T) {
	pdf := &AppComplianceFile{Name: "doc.pdf", Data: []byte("pdf")}
	const pdfField = `"fileContent":{"fileData":"cGRm","fileName":"doc.pdf","fileType":"application/pdf"}`
	tests := []struct {
		name        string
		server      appComplianceFormServer
		declaration AppComplianceDeclaration
		wantSave    string
		wantBody    string
		wantErr     string
		wantResult  AppComplianceDeclarationResult
	}{
		{
			name:        "first answer no creates the declaration",
			declaration: AppComplianceDeclaration{Requirement: "SECURITY_ASSESSMENT"},
			wantSave:    "POST /ppm/compliancerequirements/v1/accounts/account-123/entities/app-123/forms",
			wantBody:    `{"accountId":"account-123","entityId":"app-123","requirementId":"req-sec","fields":[{"id":"sec-q","sValue":"NO"}]}`,
			wantResult: AppComplianceDeclarationResult{
				AppID: "app-123", RequirementID: "req-sec", RequirementName: "SECURITY_ASSESSMENT",
				Status: "COLLECTED_VERIFY_PENDING", FormID: "form-saved",
			},
		},
		{
			name: "security assessment yes attaches both files",
			declaration: AppComplianceDeclaration{
				Requirement: "SECURITY_ASSESSMENT", Declared: true, Proof: pdf,
				Report: &AppComplianceFile{Name: "Report.PNG", Data: []byte("png")},
			},
			wantSave: "POST /ppm/compliancerequirements/v1/accounts/account-123/entities/app-123/forms",
			wantBody: `{"accountId":"account-123","entityId":"app-123","requirementId":"req-sec","fields":[` +
				`{"id":"sec-q","sValue":"YES"},{"id":"sec-proof",` + pdfField + `},` +
				`{"id":"sec-report","fileContent":{"fileData":"cG5n","fileName":"Report.PNG","fileType":"image/png"}}]}`,
			wantResult: AppComplianceDeclarationResult{
				AppID: "app-123", RequirementID: "req-sec", RequirementName: "SECURITY_ASSESSMENT", Declared: true,
				Status: "COLLECTED_VERIFY_PENDING", FormID: "form-saved",
			},
		},
		{
			name: "changing an answered declaration replaces it with only the chosen branch",
			declaration: AppComplianceDeclaration{
				Requirement: "AI_GENERATED_CONTENT", Declared: true, FilingType: "LLM",
				FilingName: "Example Model", FilingNumber: "Example-0000",
				ThirdPartyTechnology: true, ThirdPartyAuthorization: pdf,
			},
			wantSave: "PUT /ppm/compliancerequirements/v1/accounts/account-123/entities/app-123/forms/form-ai",
			wantBody: `{"accountId":"account-123","entityId":"app-123","requirementId":"req-ai","formId":"form-ai","fields":[` +
				`{"id":"ai-q","sValue":"YES"},{"id":"ai-type","sValue":"LARGE"},{"id":"ai-llm-name","sValue":"Example Model"},` +
				`{"id":"ai-llm-filing","sValue":"Example-0000"},{"id":"ai-third","sValue":"YES"},{"id":"ai-auth",` + pdfField + `}]}`,
			wantResult: AppComplianceDeclarationResult{
				AppID: "app-123", RequirementID: "req-ai", RequirementName: "AI_GENERATED_CONTENT", Declared: true,
				Status: "COLLECTED_VERIFY_PENDING", FormID: "form-saved",
			},
		},
		{
			name: "algorithm filing without third-party technology",
			declaration: AppComplianceDeclaration{
				Requirement: "AI_GENERATED_CONTENT", Declared: true, FilingType: "ALGORITHM",
				FilingName: "Example Algorithm", FilingNumber: "Example-1111",
			},
			wantSave: "PUT /ppm/compliancerequirements/v1/accounts/account-123/entities/app-123/forms/form-ai",
			wantBody: `{"accountId":"account-123","entityId":"app-123","requirementId":"req-ai","formId":"form-ai","fields":[` +
				`{"id":"ai-q","sValue":"YES"},{"id":"ai-type","sValue":"ALGOR"},{"id":"ai-algo-name","sValue":"Example Algorithm"},` +
				`{"id":"ai-algo-filing","sValue":"Example-1111"},{"id":"ai-third","sValue":"NO"}]}`,
			wantResult: AppComplianceDeclarationResult{
				AppID: "app-123", RequirementID: "req-ai", RequirementName: "AI_GENERATED_CONTENT", Declared: true,
				Status: "COLLECTED_VERIFY_PENDING", FormID: "form-saved",
			},
		},
		{
			name:        "dry run reads the requirement without saving",
			declaration: AppComplianceDeclaration{Requirement: "AI_GENERATED_CONTENT", DryRun: true},
			wantResult: AppComplianceDeclarationResult{
				AppID: "app-123", RequirementID: "req-ai", RequirementName: "AI_GENERATED_CONTENT",
				Status: "COLLECTED_VERIFY_NOT_NECESSARY", FormID: "form-ai", DryRun: true,
			},
		},
		{
			name: "declaration pending verification is not changed",
			server: appComplianceFormServer{requirements: `{"data":[
				{"id":"req-sec","formName":"SECURITYASF395E","status":"COLLECTED_VERIFY_PENDING","formId":"form-sec"}]}`},
			declaration: AppComplianceDeclaration{Requirement: "SECURITY_ASSESSMENT"},
			wantErr:     "pending verification",
		},
		{
			name:        "read-only form is not saved",
			server:      appComplianceFormServer{readOnly: true},
			declaration: AppComplianceDeclaration{Requirement: "SECURITY_ASSESSMENT"},
			wantErr:     "does not allow editing",
		},
		{
			name:        "form field this command does not know is not saved",
			server:      appComplianceFormServer{unknownField: true},
			declaration: AppComplianceDeclaration{Requirement: "AI_GENERATED_CONTENT"},
			wantErr:     `unrecognized field "anewquestion1"`,
		},
		{
			name: "file type the form refuses is not saved",
			declaration: AppComplianceDeclaration{
				Requirement: "SECURITY_ASSESSMENT", Declared: true, Proof: pdf,
				Report: &AppComplianceFile{Name: "report.docx", Data: []byte("docx")},
			},
			wantErr: `"report.docx" is not an accepted file type (.pdf, .png)`,
		},
		{
			name:   "affirmative answer whose evidence the form never asked for is not saved",
			server: appComplianceFormServer{closedBranch: true},
			declaration: AppComplianceDeclaration{
				Requirement: "SECURITY_ASSESSMENT", Declared: true, Proof: pdf, Report: pdf,
			},
			wantErr: "did not ask for all of the evidence",
		},
		{
			name:        "requirement the app does not have",
			server:      appComplianceFormServer{requirements: `{}`},
			declaration: AppComplianceDeclaration{Requirement: "SECURITY_ASSESSMENT"},
			wantErr:     `requirement "SECURITY_ASSESSMENT" was not found for app "app-123"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := tc.server
			handler.t = t
			if handler.requirements == "" {
				handler.requirements = appComplianceTestRequirements
			}
			server := httptest.NewServer(&handler)
			defer server.Close()

			got, err := testWebClient(server).SetAppComplianceDeclaration(context.Background(), "account-123", "app-123", tc.declaration)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				if handler.saved != nil {
					t.Fatalf("refused declaration was still saved: %s %s", handler.saved.Method, handler.saved.URL.Path)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetAppComplianceDeclaration() error = %v", err)
			}
			if *got != tc.wantResult {
				t.Fatalf("unexpected result:\n got %#v\nwant %#v", *got, tc.wantResult)
			}
			gotSave := ""
			if handler.saved != nil {
				gotSave = handler.saved.Method + " " + handler.saved.URL.Path
			}
			if gotSave != tc.wantSave || handler.savedBody != tc.wantBody {
				t.Fatalf("unexpected save:\n got %s %s\nwant %s %s", gotSave, handler.savedBody, tc.wantSave, tc.wantBody)
			}
		})
	}
}

type deadlineRecordingTransport struct {
	base      http.RoundTripper
	deadlines map[string]time.Time
	saveErr   error
}

func (t *deadlineRecordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	deadline, _ := r.Context().Deadline()
	t.deadlines[r.Method+" "+r.URL.Path] = deadline
	if t.saveErr != nil && strings.HasSuffix(r.URL.Path, "/forms") {
		return nil, t.saveErr
	}
	return t.base.RoundTrip(r)
}

func TestSetAppComplianceDeclarationWithEvidence(t *testing.T) {
	pdf := &AppComplianceFile{Name: "doc.pdf", Data: []byte("pdf")}
	declaration := AppComplianceDeclaration{Requirement: "SECURITY_ASSESSMENT", Declared: true, Proof: pdf, Report: pdf}
	const savePath = "POST /ppm/compliancerequirements/v1/accounts/account-123/entities/app-123/forms"
	for _, tc := range []struct {
		name    string
		saveErr error
		wantErr string
	}{
		// The files travel in the last form-walk request and again in the save,
		// so the caller's upload deadline, not the per-request client timeout, bounds them.
		{name: "caller deadline bounds the uploads"},
		{name: "failed save leaves the outcome unknown", saveErr: context.DeadlineExceeded, wantErr: "save outcome is unknown; check asc web apps compliance list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := appComplianceFormServer{t: t, requirements: appComplianceTestRequirements}
			server := httptest.NewServer(&handler)
			defer server.Close()
			transport := &deadlineRecordingTransport{base: server.Client().Transport, deadlines: map[string]time.Time{}, saveErr: tc.saveErr}
			client := testWebClient(server)
			client.httpClient = &http.Client{Timeout: time.Minute, Transport: transport}

			deadline := time.Now().Add(time.Hour)
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()
			_, err := client.SetAppComplianceDeclaration(ctx, "account-123", "app-123", declaration)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetAppComplianceDeclaration() error = %v", err)
			}
			for request, got := range transport.deadlines {
				if strings.Contains(request, "/requirements/") || request == savePath {
					if !got.Equal(deadline) {
						t.Fatalf("%s deadline = %v, want the caller's %v", request, got, deadline)
					}
				}
			}
			if _, ok := transport.deadlines[savePath]; !ok {
				t.Fatalf("declaration was not saved; requests: %v", transport.deadlines)
			}
		})
	}
}

// getOnlyTransport fails the test on any request that is not a GET or that
// carries a body, so nothing the user answered can leave the machine.
type getOnlyTransport struct {
	t    *testing.T
	base http.RoundTripper
	sent []string
}

func (g *getOnlyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	g.sent = append(g.sent, r.Method+" "+r.URL.Path)
	if r.Method != http.MethodGet || (r.Body != nil && r.Body != http.NoBody) {
		g.t.Errorf("request sent under dry run or read-only mode: %s %s", r.Method, r.URL.Path)
	}
	return g.base.RoundTrip(r)
}

func TestSetAppComplianceDeclarationSendsNothingUnderDryRunOrReadOnly(t *testing.T) {
	pdf := &AppComplianceFile{Name: "doc.pdf", Data: []byte("evidence")}
	declarations := map[string]AppComplianceDeclaration{
		"security assessment with evidence": {Requirement: "SECURITY_ASSESSMENT", Declared: true, Proof: pdf, Report: pdf},
		"AI content with authorization": {
			Requirement: "AI_GENERATED_CONTENT", Declared: true, FilingType: "LLM", FilingName: "Example Model",
			FilingNumber: "Example-0000", ThirdPartyTechnology: true, ThirdPartyAuthorization: pdf,
		},
		"negative answer": {Requirement: "SECURITY_ASSESSMENT"},
	}
	for _, mode := range []struct {
		name     string
		dryRun   bool
		readOnly bool
	}{
		{name: "dry run", dryRun: true},
		{name: "read-only", readOnly: true},
		{name: "dry run in read-only mode", dryRun: true, readOnly: true},
	} {
		for name, declaration := range declarations {
			t.Run(mode.name+"/"+name, func(t *testing.T) {
				if mode.readOnly {
					t.Setenv(readonly.EnvVar, "1")
				} else {
					t.Setenv(readonly.EnvVar, "")
				}
				handler := appComplianceFormServer{t: t, requirements: appComplianceTestRequirements}
				server := httptest.NewServer(&handler)
				defer server.Close()
				transport := &getOnlyTransport{t: t, base: server.Client().Transport}
				client := testWebClient(server)
				client.httpClient = &http.Client{Transport: transport}

				declaration.DryRun = mode.dryRun
				got, err := client.SetAppComplianceDeclaration(context.Background(), "account-123", "app-123", declaration)
				if mode.dryRun {
					if err != nil {
						t.Fatalf("SetAppComplianceDeclaration() error = %v", err)
					}
					if !got.DryRun || got.RequirementName != declaration.Requirement {
						t.Fatalf("unexpected dry-run result: %#v", got)
					}
				} else if !errors.Is(err, readonly.ErrRefused) {
					t.Fatalf("SetAppComplianceDeclaration() error = %v, want readonly.ErrRefused", err)
				}
				if len(transport.sent) != 1 || !strings.HasSuffix(transport.sent[0], "/requirements") {
					t.Fatalf("requests sent = %v, want only the requirements list", transport.sent)
				}
			})
		}
	}
}

func TestSetAppComplianceDeclarationDryRunChecksEvidenceLocally(t *testing.T) {
	handler := appComplianceFormServer{t: t, requirements: appComplianceTestRequirements}
	server := httptest.NewServer(&handler)
	defer server.Close()

	_, err := testWebClient(server).SetAppComplianceDeclaration(context.Background(), "account-123", "app-123", AppComplianceDeclaration{
		Requirement: "SECURITY_ASSESSMENT", Declared: true, DryRun: true,
		Proof:  &AppComplianceFile{Name: "proof.pdf", Data: []byte("pdf")},
		Report: &AppComplianceFile{Name: "report.txt", Data: []byte("txt")},
	})
	if err == nil || !strings.Contains(err.Error(), `"report.txt" is not an accepted file type`) {
		t.Fatalf("expected a file type error, got %v", err)
	}
}
