package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

func TestSetMedicalDeviceRegionPreservesFormAndVerifiesReadback(t *testing.T) {
	var requirementsCalls int
	var formCalls int
	var putBody map[string]any

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				fixture.Respond(w, "requirements contentId = %q, want app-123", got)
				return
			}
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED","formId":"form-123"}]}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				fixture.Respond(w, "form contentId = %q, want app-123", got)
				return
			}
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"data":{
						"accountId":"account-123",
						"contentId":"app-123",
						"requirementId":"req-123",
						"requirementName":"MEDICAL_DEVICE",
						"formId":"form-123",
						"opaqueOuter":{"keep":true},
						"countriesOrRegions":["USA","GBR","EEA"],
						"medicalDeviceData":{
							"declaration":"yes",
							"opaqueMedical":{"keep":true},
							"contactInformation":[{"countriesOrRegions":["GBR"],"opaqueContact":{"ignoredWithoutLegalEntity":true}},{"legalEntityId":"entity-1","countriesOrRegions":["GBR"],"phone":"phone","email":"email","address":{"line":"keep"},"opaqueContact":{"keep":true}}],
							"registrationInfo":[
								{"countriesOrRegions":["USA"],"declaration":"yes","opaque":"usa"},
								{"countriesOrRegions":["GBR"],"declaration":"no","opaque":"gbr","registrationNumber":"stale"},
								{"countriesOrRegions":["EEA"],"declaration":"yes","opaque":"eea"}
							]
						}
					},
					"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","required":true,"options":[{"value":"USA"},{"value":"GBR"},{"value":"EU"}]}}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"data":{
					"accountId":"account-123",
					"contentId":"app-123",
					"requirementId":"req-123",
					"requirementName":"MEDICAL_DEVICE",
					"formId":"form-123",
					"opaqueOuter":{"keep":true},
					"countriesOrRegions":["USA","GBR","EEA"],
					"medicalDeviceData":{
						"declaration":"yes",
						"opaqueMedical":{"keep":true},
					"contactInformation":[{"countriesOrRegions":["GBR"],"opaqueContact":{"ignoredWithoutLegalEntity":true}},{"legalEntityId":"entity-1","countriesOrRegions":["GBR"],"phone":"phone","email":"email","address":{"line":"keep"},"opaqueContact":{"keep":true}}],
						"registrationInfo":[
							{"countriesOrRegions":["USA"],"declaration":"yes","opaque":"usa"},
							{"countriesOrRegions":["EEA"],"declaration":"yes","opaque":"eea"},
							{"countriesOrRegions":["GBR"],"declaration":"yes","opaque":"gbr","supportInfo":[{"locale":"en-US","instruction":"https://example.invalid/ifu","statement":"Use","safetyInfo":"Safe"}]}
						]
					}
				},
				"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","required":true,"options":[{"value":"USA"},{"value":"GBR"},{"value":"EU"}]}}
			}`))
		case r.Method == http.MethodPut && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			if got := r.Header.Get("X-Csrf-Itc"); got != "itc" {
				fixture.Respond(w, "X-Csrf-Itc = %q, want itc", got)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				fixture.Respond(w, "decode PUT body: %v", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.String())
			return
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceRegion(
		context.Background(),
		"account-123",
		"app-123",
		"GBR",
		MedicalDeviceRegionOptions{
			Declaration: true,
			SupportInfo: []MedicalDeviceRegionSupportInfo{{
				Locale:      "en-US",
				Instruction: "https://example.invalid/ifu",
				Statement:   "Use",
				SafetyInfo:  "Safe",
			}},
		},
	)
	if err != nil {
		t.Fatalf("SetMedicalDeviceRegion() error = %v", err)
	}
	if got == nil || got.Region != "GBR" || !got.Declared || !got.Changed {
		t.Fatalf("unexpected result: %#v", got)
	}
	if requirementsCalls != 2 || formCalls != 2 {
		t.Fatalf("verification calls = requirements %d, forms %d; want 2 each", requirementsCalls, formCalls)
	}

	if putBody["opaqueOuter"].(map[string]any)["keep"] != true {
		t.Fatalf("opaque outer field was not preserved: %#v", putBody)
	}
	if !reflect.DeepEqual(putBody["countriesOrRegions"], []any{"USA", "GBR", "EEA"}) {
		t.Fatalf("top-level regions changed: %#v", putBody["countriesOrRegions"])
	}
	medicalData := putBody["medicalDeviceData"].(map[string]any)
	if medicalData["opaqueMedical"].(map[string]any)["keep"] != true {
		t.Fatalf("opaque medical field was not preserved: %#v", medicalData)
	}
	contacts := medicalData["contactInformation"].([]any)
	if len(contacts) != 2 || contacts[1].(map[string]any)["opaqueContact"].(map[string]any)["keep"] != true {
		t.Fatalf("contact information was not preserved: %#v", medicalData["contactInformation"])
	}
	rows := medicalData["registrationInfo"].([]any)
	if len(rows) != 3 {
		t.Fatalf("registration rows = %d, want 3: %#v", len(rows), rows)
	}
	var target map[string]any
	for _, raw := range rows {
		row := raw.(map[string]any)
		if medicalDeviceRegistrationRegion(row) == "GBR" {
			target = row
			continue
		}
		if row["opaque"] == nil {
			t.Fatalf("non-target row lost opaque field: %#v", row)
		}
	}
	if target == nil {
		t.Fatalf("GBR target row missing: %#v", rows)
	}
	if target["declaration"] != "yes" || target["opaque"] != "gbr" {
		t.Fatalf("target row was not preserved and updated: %#v", target)
	}
	if _, ok := target["registrationNumber"]; ok {
		t.Fatalf("GBR target row retained registrationNumber: %#v", target)
	}
	support := target["supportInfo"].([]any)
	if len(support) != 1 || support[0].(map[string]any)["instruction"] != "https://example.invalid/ifu" {
		t.Fatalf("unexpected target support info: %#v", support)
	}
	if strings.Contains(fmt.Sprint(got), "phone") {
		t.Fatalf("result unexpectedly exposed contact data: %#v", got)
	}
}

func TestSetMedicalDeviceRegionRejectsIncompleteContactBeforePUT(t *testing.T) {
	var putCalls int
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				fixture.Respond(w, "requirements contentId = %q, want app-123", got)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED","formId":"form-123"}]}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				fixture.Respond(w, "form contentId = %q, want app-123", got)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{
				"data":{
					"contentId":"app-123",
					"medicalDeviceData":{
						"declaration":"yes",
						"contactInformation":[{"legalEntityId":"entity-1","countriesOrRegions":["GBR"],"phone":"phone"}],
						"registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"no"}]
					}
				},
				"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}
			}`)
		case r.Method == http.MethodPut:
			putCalls++
			fixture.Respond(w, "unexpected PUT after incomplete contact preflight: %s", r.URL.String())
			return
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.String())
			return
		}
	}))
	defer server.Close()

	_, err := testWebClient(server).SetMedicalDeviceRegion(
		context.Background(),
		"account-123",
		"app-123",
		"GBR",
		MedicalDeviceRegionOptions{
			Declaration: true,
			SupportInfo: []MedicalDeviceRegionSupportInfo{{
				Locale:      "en-US",
				Instruction: "https://example.invalid/ifu",
				Statement:   "Use",
				SafetyInfo:  "Safe",
			}},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("SetMedicalDeviceRegion() error = %v, want incomplete contact email error", err)
	}
	if putCalls != 0 {
		t.Fatalf("PUT calls = %d, want 0", putCalls)
	}
}

func TestSetMedicalDeviceRegionRejectsInvalidFormBeforePUT(t *testing.T) {
	declareYes := MedicalDeviceRegionOptions{
		Declaration: true,
		SupportInfo: []MedicalDeviceRegionSupportInfo{{
			Locale:      "en-US",
			Instruction: "https://example.invalid/ifu",
			Statement:   "Use",
			SafetyInfo:  "Safe",
		}},
	}
	tests := []struct {
		name    string
		form    string
		options MedicalDeviceRegionOptions
		wantErr string
	}{
		{
			name:    "scalar contact region",
			form:    `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","contactInformation":[{"legalEntityId":"entity-a","countriesOrRegions":"GBR","phone":"phone-a","email":"email-a","address":{"line":"address-a"}}],"registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"no"}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}}`,
			options: declareYes,
			wantErr: "no complete contact coverage",
		},
		{
			name:    "missing constraint contact",
			form:    `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","contactInformation":[{"legalEntityId":"entity-a","countriesOrRegions":["GBR"],"phone":"phone-a","email":"email-a","address":{"line":"address-a"}}],"registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"no"}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]},"$[*].medicalDeviceData.contactInformation[1].legalEntityId":{"attributeName":"legalEntityId","required":true,"options":[{"value":"entity-b"}]},"$[*].medicalDeviceData.contactInformation[1].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"listValues":["GBR"]}]},"$[*].medicalDeviceData.contactInformation[1].phone":{"attributeName":"phone","required":true,"options":[{"value":"phone-b"}]},"$[*].medicalDeviceData.contactInformation[1].email":{"attributeName":"email","required":true,"options":[{"value":"email-b"}]},"$[*].medicalDeviceData.contactInformation[1].address.addressLine1":{"attributeName":"address.addressLine1","required":true,"options":[{"value":"address-b"}]}}}`,
			options: declareYes,
			wantErr: "form-defined contact candidate",
		},
		{
			name:    "missing target row",
			form:    `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":["USA"],"declaration":"yes"}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}}`,
			wantErr: "registrationInfo row",
		},
		{
			name:    "malformed existing row",
			form:    `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":[],"declaration":"yes"},{"countriesOrRegions":["GBR"],"declaration":"yes"}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}}`,
			wantErr: "without a country/region",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var putCalls int
			fixture := handlertest.New(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
					writeMedicalDeviceRegionRequirements(w)
				case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
					writeMedicalDeviceRegionForm(w, tc.form)
				case r.Method == http.MethodPut:
					putCalls++
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprint(w, `{}`)
				default:
					fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.String())
					return
				}
			}))
			defer server.Close()

			_, err := testWebClient(server).SetMedicalDeviceRegion(context.Background(), "account-123", "app-123", "GBR", tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("SetMedicalDeviceRegion() error = %v, want %q", err, tc.wantErr)
			}
			if putCalls != 0 {
				t.Fatalf("PUT calls = %d, want 0", putCalls)
			}
		})
	}
}

func TestSetMedicalDeviceRegionAllowsConstraintContactPresentOnlyForAnotherRegion(t *testing.T) {
	var formCalls, putCalls int
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			writeMedicalDeviceRegionRequirements(w)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			if formCalls == 1 {
				writeMedicalDeviceRegionForm(w, `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","contactInformation":[{"legalEntityId":"entity-a","countriesOrRegions":["GBR"],"phone":"phone-a","email":"email-a","address":{"line":"address-a"}},{"legalEntityId":"entity-b","countriesOrRegions":["USA"],"phone":"phone-b","email":"email-b","address":{"line":"address-b"}}],"registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"no"}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]},"$[*].medicalDeviceData.contactInformation[1].legalEntityId":{"attributeName":"legalEntityId","required":true,"options":[{"value":"entity-b"}]},"$[*].medicalDeviceData.contactInformation[1].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"listValues":["GBR"]}]},"$[*].medicalDeviceData.contactInformation[1].phone":{"attributeName":"phone","required":true,"options":[{"value":"phone-b"}]},"$[*].medicalDeviceData.contactInformation[1].email":{"attributeName":"email","required":true,"options":[{"value":"email-b"}]},"$[*].medicalDeviceData.contactInformation[1].address.addressLine1":{"attributeName":"address.addressLine1","required":true,"options":[{"value":"address-b"}]}}}`)
				return
			}
			writeMedicalDeviceRegionForm(w, `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","contactInformation":[{"legalEntityId":"entity-a","countriesOrRegions":["GBR"],"phone":"phone-a","email":"email-a","address":{"line":"address-a"}},{"legalEntityId":"entity-b","countriesOrRegions":["USA"],"phone":"phone-b","email":"email-b","address":{"line":"address-b"}}],"registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"yes","supportInfo":[{"locale":"en-US","instruction":"https://example.invalid/ifu","statement":"Use","safetyInfo":"Safe"}]}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]},"$[*].medicalDeviceData.contactInformation[1].legalEntityId":{"attributeName":"legalEntityId","required":true,"options":[{"value":"entity-b"}]},"$[*].medicalDeviceData.contactInformation[1].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"listValues":["GBR"]}]},"$[*].medicalDeviceData.contactInformation[1].phone":{"attributeName":"phone","required":true,"options":[{"value":"phone-b"}]},"$[*].medicalDeviceData.contactInformation[1].email":{"attributeName":"email","required":true,"options":[{"value":"email-b"}]},"$[*].medicalDeviceData.contactInformation[1].address.addressLine1":{"attributeName":"address.addressLine1","required":true,"options":[{"value":"address-b"}]}}}`)
		case r.Method == http.MethodPut:
			putCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{}`)
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.String())
			return
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceRegion(
		context.Background(), "account-123", "app-123", "GBR",
		MedicalDeviceRegionOptions{
			Declaration: true,
			SupportInfo: []MedicalDeviceRegionSupportInfo{{
				Locale:      "en-US",
				Instruction: "https://example.invalid/ifu",
				Statement:   "Use",
				SafetyInfo:  "Safe",
			}},
		},
	)
	if err != nil {
		t.Fatalf("SetMedicalDeviceRegion() error = %v, want source-compatible existing contact", err)
	}
	if got == nil || !got.Changed {
		t.Fatalf("unexpected result: %#v", got)
	}
	if formCalls != 2 {
		t.Fatalf("form calls = %d, want 2", formCalls)
	}
	if putCalls != 1 {
		t.Errorf("PUT calls = %d, want 1 after Qh deduplication", putCalls)
	}
}

func TestMedicalDeviceRegionRejectsConstraintContactWithNoRecognizedRegion(t *testing.T) {
	constraints := map[string]complianceConstraint{
		"$[*].medicalDeviceData.contactInformation[1].legalEntityId": {
			AttributeName: "legalEntityId",
			Options:       []complianceConstraintOption{{Value: "entity-b"}},
		},
		"$[*].medicalDeviceData.contactInformation[1].countriesOrRegions": {
			AttributeName: "countriesOrRegions",
			Options:       []complianceConstraintOption{{ListValues: []string{"GBR"}}},
		},
	}
	medicalData := map[string]any{
		"contactInformation": []any{
			map[string]any{
				"legalEntityId":      "entity-b",
				"countriesOrRegions": []any{"CAN"},
			},
		},
	}
	if err := validateMedicalDeviceRegionConstraintContactCandidates(medicalData, constraints, "GBR"); err == nil || !strings.Contains(err.Error(), "form-defined contact candidate") {
		t.Fatalf("constraint contact preflight error = %v, want missing recognized-region contact", err)
	}
}

func TestSetMedicalDeviceRegionFalsePreservesOpaqueDetails(t *testing.T) {
	var formCalls int
	var putBody map[string]any
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				fixture.Respond(w, "requirements contentId = %q, want app-123", got)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED","formId":"form-123"}]}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				fixture.Respond(w, "form contentId = %q, want app-123", got)
				return
			}
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			declaration := "yes"
			if formCalls > 1 {
				declaration = "no"
			}
			_, _ = fmt.Fprintf(w, `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","opaqueOuter":{"keep":true},"medicalDeviceData":{"declaration":"yes","opaqueMedical":{"keep":true},"registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"%s","registrationNumber":"keep","supportInfo":[{"locale":"en-US","instruction":"https://example.invalid/old","statement":"Old","safetyInfo":"Old"}],"opaqueRow":{"keep":true}},{"countriesOrRegions":["USA"],"declaration":"yes","opaqueRow":{"keep":true}}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"},{"value":"USA"}]}}}`, declaration)
		case r.Method == http.MethodPut && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				fixture.Respond(w, "decode PUT body: %v", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{}`)
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.String())
			return
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceRegion(
		context.Background(),
		"account-123",
		"app-123",
		"GBR",
		MedicalDeviceRegionOptions{},
	)
	if err != nil {
		t.Fatalf("SetMedicalDeviceRegion() error = %v", err)
	}
	if got == nil || got.Declared || !got.Changed {
		t.Fatalf("unexpected result: %#v", got)
	}
	if formCalls != 2 {
		t.Fatalf("form calls = %d, want 2", formCalls)
	}
	if putBody["opaqueOuter"].(map[string]any)["keep"] != true {
		t.Fatalf("opaque outer field was not preserved: %#v", putBody)
	}
	medicalData := putBody["medicalDeviceData"].(map[string]any)
	if medicalData["opaqueMedical"].(map[string]any)["keep"] != true {
		t.Fatalf("opaque medical field was not preserved: %#v", medicalData)
	}
	rows := medicalData["registrationInfo"].([]any)
	if len(rows) != 2 {
		t.Fatalf("registration rows = %d, want 2: %#v", len(rows), rows)
	}
	var target map[string]any
	for _, raw := range rows {
		row := raw.(map[string]any)
		if medicalDeviceRegistrationRegion(row) == "GBR" {
			target = row
			break
		}
	}
	if target == nil {
		t.Fatalf("GBR target row missing: %#v", rows)
	}
	if target["declaration"] != "no" || target["registrationNumber"] != "keep" || target["opaqueRow"].(map[string]any)["keep"] != true {
		t.Fatalf("target details were not preserved: %#v", target)
	}
}

func TestSetMedicalDeviceRegionNoOpPreservesRegistrationRowOrder(t *testing.T) {
	var formCalls, putCalls int
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			writeMedicalDeviceRegionRequirements(w)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			writeMedicalDeviceRegionForm(w, `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"no","opaque":"gbr"},{"countriesOrRegions":["USA"],"declaration":"yes","opaque":"usa"}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"},{"value":"USA"}]}}}`)
		case r.Method == http.MethodPut:
			putCalls++
			fixture.Respond(w, "unexpected PUT for a matching regional no-op: %s", r.URL.String())
			return
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.String())
			return
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceRegion(
		context.Background(), "account-123", "app-123", "GBR", MedicalDeviceRegionOptions{},
	)
	if err != nil {
		t.Fatalf("SetMedicalDeviceRegion() error = %v", err)
	}
	if got == nil || got.Changed {
		t.Fatalf("unexpected no-op result: %#v", got)
	}
	if formCalls != 1 {
		t.Fatalf("form calls = %d, want 1", formCalls)
	}
	if putCalls != 0 {
		t.Fatalf("PUT calls = %d, want 0", putCalls)
	}
}

func TestSetMedicalDeviceRegionRequiresTopLevelAvailabilityConstraint(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		wantError   string
	}{
		{"registration selector only", `"$[*].medicalDeviceData.registrationInfo[0].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}`, "constraints are missing"},
		{"contact selector only", `"$[*].medicalDeviceData.contactInformation[0].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}`, "constraints are missing"},
		{"nested selector cannot widen availability", `"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"USA"}]},"$[*].medicalDeviceData.registrationInfo[0].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}`, "not available"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var putCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
					writeMedicalDeviceRegionRequirements(w)
				case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
					writeMedicalDeviceRegionForm(w, `{"data":{"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE","formId":"form-123","medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":"yes"}]}},"constraints":{`+tc.constraints+`}}`)
				case r.Method == http.MethodPut:
					putCalls++
					writeMedicalDeviceRegionForm(w, `{}`)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			_, err := testWebClient(server).SetMedicalDeviceRegion(context.Background(), "account-123", "app-123", "GBR", MedicalDeviceRegionOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("SetMedicalDeviceRegion() error = %v, want %q", err, tc.wantError)
			}
			if putCalls != 0 {
				t.Errorf("PUT calls = %d, want 0 without top-level region availability", putCalls)
			}
		})
	}
}

func TestSetMedicalDeviceRegionSupportsLegacyIdentityMetadata(t *testing.T) {
	for _, declaration := range []string{"yes", "no"} {
		t.Run(declaration, func(t *testing.T) {
			fixture := handlertest.New(t)
			var forms, puts int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/requirements"):
					writeMedicalDeviceRegionRequirements(w)
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/forms"):
					forms++
					identity, answer := "", declaration
					if forms > 1 {
						identity = `"accountId":"account-123","contentId":"app-123","requirementId":"req-123","requirementName":"MEDICAL_DEVICE",`
						answer = "no"
					}
					writeMedicalDeviceRegionForm(w, fmt.Sprintf(`{"data":{%s"formId":"form-123","medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":%q}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}}`, identity, answer))
				case r.Method == http.MethodPut:
					puts++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						fixture.Respond(w, "decode legacy-form PUT: %v", err)
						return
					}
					for key, want := range map[string]string{"accountId": "account-123", "contentId": "app-123", "requirementId": "req-123", "requirementName": "MEDICAL_DEVICE"} {
						if body[key] != want {
							t.Errorf("PUT %s = %v, want %q", key, body[key], want)
						}
					}
					writeMedicalDeviceRegionForm(w, `{}`)
				default:
					fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()
			result, err := testWebClient(server).SetMedicalDeviceRegion(context.Background(), "account-123", "app-123", "GBR", MedicalDeviceRegionOptions{})
			if err != nil {
				t.Fatalf("legacy form update: %v", err)
			}
			wantPuts := 0
			if declaration == "yes" {
				wantPuts = 1
			}
			if result == nil || result.Changed != (wantPuts == 1) || puts != wantPuts {
				t.Fatalf("result=%+v PUTs=%d, want %d writes", result, puts, wantPuts)
			}
		})
	}
}

func TestSetMedicalDeviceRegionRejectsMismatchedPreflightIdentity(t *testing.T) {
	for _, field := range []string{"accountId", "contentId", "requirementId", "requirementName", "formId"} {
		for _, declaration := range []string{"yes", "no"} {
			t.Run(field+"/"+declaration, func(t *testing.T) {
				puts := 0
				fixture := handlertest.New(t)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/requirements"):
						writeMedicalDeviceRegionRequirements(w)
					case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/forms"):
						writeMedicalDeviceRegionForm(w, fmt.Sprintf(`{"data":{%q:"other","medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":%q}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}}`, field, declaration))
					case r.Method == http.MethodPut:
						puts++
						writeMedicalDeviceRegionForm(w, `{}`)
					default:
						fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
					}
				}))
				defer server.Close()
				result, err := testWebClient(server).SetMedicalDeviceRegion(context.Background(), "account-123", "app-123", "GBR", MedicalDeviceRegionOptions{})
				if err == nil || !strings.Contains(err.Error(), "different "+field) || result != nil || puts != 0 {
					t.Fatalf("result=%+v error=%v PUTs=%d, want identity refusal before no-op or mutation", result, err, puts)
				}
			})
		}
	}
}

func TestSetMedicalDeviceRegionOptionalReadbackIdentity(t *testing.T) {
	for _, tc := range []struct {
		name             string
		formID           string
		returnedIdentity string
		wantError        string
	}{
		{"omitted metadata", "form-123", "", ""},
		{"unknown form ID", "", "", ""},
		{"wrong account", "form-123", `"accountId":"other",`, "different accountId"},
		{"wrong app", "form-123", `"contentId":"other",`, "different contentId"},
		{"wrong requirement", "form-123", `"requirementId":"other",`, "different requirementId"},
		{"wrong form", "form-123", `"formId":"other",`, "different formId"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var forms, puts int
			fixture := handlertest.New(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/requirements"):
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED","formId":%q}]}]}`, tc.formID)
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/forms"):
					forms++
					identity, answer := "", "yes"
					if forms > 1 {
						identity, answer = tc.returnedIdentity, "no"
					}
					writeMedicalDeviceRegionForm(w, fmt.Sprintf(`{"data":{%s"opaqueOuter":{"keep":true},"medicalDeviceData":{"declaration":"yes","registrationInfo":[{"countriesOrRegions":["GBR"],"declaration":%q}]}},"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"GBR"}]}}}`, identity, answer))
				case r.Method == http.MethodPut:
					puts++
					writeMedicalDeviceRegionForm(w, `{}`)
				default:
					fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()
			result, err := testWebClient(server).SetMedicalDeviceRegion(context.Background(), "account-123", "app-123", "GBR", MedicalDeviceRegionOptions{})
			if puts != 1 {
				t.Fatalf("PUT calls=%d, want 1", puts)
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || result != nil {
					t.Fatalf("result=%+v error=%v, want %q", result, err, tc.wantError)
				}
			} else if err != nil || result == nil || !result.Changed {
				t.Fatalf("result=%+v error=%v, want verified changed region", result, err)
			}
		})
	}
}

func writeMedicalDeviceRegionRequirements(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED","formId":"form-123"}]}]}`)
}

func writeMedicalDeviceRegionForm(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, body)
}
