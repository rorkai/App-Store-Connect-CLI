package asc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAPIErrorError_SanitizesControlCharacters(t *testing.T) {
	err := &APIError{
		Title:  "Bad\x1b[31m",
		Detail: "Detail\x07",
		Code:   "CODE\x1b",
		AssociatedErrors: map[string][]APIAssociatedError{
			"/v1/resource\x1b[33m": {
				{
					Code:   "ENTITY_ERROR\x1b",
					Detail: "Associated detail\x07",
				},
			},
		},
	}

	message := err.Error()
	if strings.ContainsAny(message, "\x1b\x07") {
		t.Fatalf("expected control characters to be stripped, got %q", message)
	}
	if !strings.Contains(message, "Bad") || !strings.Contains(message, "Detail") {
		t.Fatalf("expected title and detail in message, got %q", message)
	}
	if !strings.Contains(message, "Associated detail") {
		t.Fatalf("expected associated detail in message, got %q", message)
	}
}

func TestAPIErrorIs_AuthSentinels(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		wantUnauthorized bool
		wantForbidden    bool
	}{
		{
			// Apple returns 401 responses with code NOT_AUTHORIZED, not UNAUTHORIZED.
			name:             "401 NOT_AUTHORIZED",
			err:              ParseErrorWithStatus([]byte(`{"errors":[{"id":"7091e344-4b31-4b6b-9f04-16d61a1c8c9e","status":"401","code":"NOT_AUTHORIZED","title":"Authentication credentials are missing or invalid.","detail":"Provide a properly configured and signed bearer token, and make sure that it has not expired."}]}`), 401),
			wantUnauthorized: true,
		},
		{
			name:          "403 FORBIDDEN_ERROR",
			err:           ParseErrorWithStatus([]byte(`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"The given operation is not allowed","detail":"This request is forbidden for security reasons"}]}`), 403),
			wantForbidden: true,
		},
		{name: "UNAUTHORIZED code without status", err: &APIError{Code: "UNAUTHORIZED"}, wantUnauthorized: true},
		{name: "FORBIDDEN code without status", err: &APIError{Code: "FORBIDDEN"}, wantForbidden: true},
		{name: "qualified FORBIDDEN code without status", err: &APIError{Code: "FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED"}, wantForbidden: true},
		{name: "qualified FORBIDDEN_ERROR code without status", err: &APIError{Code: "FORBIDDEN_ERROR.PLA_NOT_VALID"}, wantForbidden: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := errors.Is(tc.err, ErrUnauthorized); got != tc.wantUnauthorized {
				t.Errorf("errors.Is(%v, ErrUnauthorized) = %t, want %t", tc.err, got, tc.wantUnauthorized)
			}
			if got := errors.Is(tc.err, ErrForbidden); got != tc.wantForbidden {
				t.Errorf("errors.Is(%v, ErrForbidden) = %t, want %t", tc.err, got, tc.wantForbidden)
			}
		})
	}
}

func TestAPIErrorError_AssociatedErrorsSortedByResourcePath(t *testing.T) {
	err := &APIError{
		Title:  "Cannot submit",
		Detail: "Fix associated errors",
		AssociatedErrors: map[string][]APIAssociatedError{
			"/v1/b": {
				{Detail: "B detail"},
			},
			"/v1/a": {
				{Detail: "A detail"},
			},
		},
	}

	message := err.Error()
	aIndex := strings.Index(message, "Associated errors for /v1/a:")
	bIndex := strings.Index(message, "Associated errors for /v1/b:")
	if aIndex == -1 || bIndex == -1 {
		t.Fatalf("expected associated error sections, got %q", message)
	}
	if aIndex > bIndex {
		t.Fatalf("expected associated errors to be sorted by path, got %q", message)
	}
}

func TestParseErrorWithStatus_RetainsEveryErrorCode(t *testing.T) {
	// Apple's live 409 for a duplicate versionString on
	// POST /v1/appStoreVersions carries two errors, and the duplicate code is
	// the second one. Captured against app 6759231657 on 2026-09-15.
	payload := `{"errors":[{"id":"b068c5c0-b3fa-4d12-aa89-f1b9aa061b28","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}},{"id":"eb1884a7-e427-42db-ac95-26c49c84a5c2","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"The version number has been previously used.","source":{"pointer":"/data/attributes/versionString"}}]}`

	err := ParseErrorWithStatus([]byte(payload), 409)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ParseErrorWithStatus returned %T, want *APIError", err)
	}
	if apiErr.Code != "ENTITY_ERROR.RELATIONSHIP.INVALID" {
		t.Fatalf("Code = %q, want the first error's code", apiErr.Code)
	}
	want := []string{"ENTITY_ERROR.RELATIONSHIP.INVALID", "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}
	if len(apiErr.AllCodes) != len(want) {
		t.Fatalf("AllCodes = %v, want %v", apiErr.AllCodes, want)
	}
	for i, code := range want {
		if apiErr.AllCodes[i] != code {
			t.Fatalf("AllCodes = %v, want %v", apiErr.AllCodes, want)
		}
	}
}

func TestParseErrorWithStatus_RetainsEveryErrorDetail(t *testing.T) {
	payload := `{"errors":[{"code":"FIRST","detail":"first detail"},{"code":"SECOND","detail":"second detail"},{"code":"THIRD","detail":""}]}`

	err := ParseErrorWithStatus([]byte(payload), 409)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ParseErrorWithStatus returned %T, want *APIError", err)
	}
	want := []string{"first detail", "second detail", ""}
	if len(apiErr.AllDetails) != len(want) {
		t.Fatalf("AllDetails = %v, want %v", apiErr.AllDetails, want)
	}
	for i, detail := range want {
		if apiErr.AllDetails[i] != detail {
			t.Fatalf("AllDetails = %v, want %v", apiErr.AllDetails, want)
		}
	}
}

func TestParseErrorWithStatus_KeepsEachCodeWithItsDetail(t *testing.T) {
	payload := `{"errors":[{"detail":"no code here"},{"code":" SECOND ","detail":"second detail"}]}`

	err := ParseErrorWithStatus([]byte(payload), 409)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ParseErrorWithStatus returned %T, want *APIError", err)
	}
	want := []APIErrorEntry{{Code: "", Detail: "no code here"}, {Code: "SECOND", Detail: "second detail"}}
	if len(apiErr.Entries) != len(want) {
		t.Fatalf("Entries = %+v, want %+v", apiErr.Entries, want)
	}
	for i, entry := range want {
		if apiErr.Entries[i] != entry {
			t.Fatalf("Entries = %+v, want %+v", apiErr.Entries, want)
		}
	}
}

func TestAPIErrorError_ListsLaterErrorEntries(t *testing.T) {
	const first = "An attribute value is invalid.: The version string is not valid."
	tests := []struct {
		name    string
		entries string
		want    string
	}{
		{
			name:    "single entry is unchanged",
			entries: `{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"The version string is not valid."}`,
			want:    first,
		},
		{
			name: "later details are listed once in order",
			entries: `{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"The version string is not valid."},` +
				`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"earliestReleaseDate must be in the future\u0007"},` +
				`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"The version string is not valid."},` +
				`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"earliestReleaseDate must be in the future"},` +
				`{"code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"A relationship value is invalid.","detail":""}`,
			want: first + "\n\nAdditional errors:\n  - earliestReleaseDate must be in the future\n  - ENTITY_ERROR.RELATIONSHIP.INVALID",
		},
		{
			name:    "additional entries are capped",
			entries: `{"title":"T","detail":"d0"},{"detail":"d1"},{"detail":"d2"},{"detail":"d3"},{"detail":"d4"},{"detail":"d5"},{"detail":"d6"},{"detail":"d7"}`,
			want:    "T: d0\n\nAdditional errors:\n  - d1\n  - d2\n  - d3\n  - d4\n  - d5\n  - and 2 more",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ParseErrorWithStatus([]byte(`{"errors":[`+test.entries+`]}`), 409)
			if got := err.Error(); got != test.want {
				t.Fatalf("Error() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestIsMissingResourceOfType(t *testing.T) {
	const missingAvailability = `{"errors":[{"id":"b8a2b802-0512-4f42-b46a-cb444c0dc8db","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAvailabilities' with id '6807733044'"}]}`
	const missingApp = `{"errors":[{"id":"1c5bfc66-b18d-46ce-9863-635985130e62","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '999999999999'"}]}`
	const missingIAPAvailability = `{"errors":[{"id":"3ee73c04-a4b6-48e1-b29f-a6242beee89e","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'inAppPurchaseAvailabilities' with id '6760268101'"}]}`
	const missingScheduleManualPrices = `{"errors":[{"id":"0f2d2c4e-2b7a-4a4b-9a1c-5f6e7d8c9b0a","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'null' with id '6807733044'"}]}`
	const legacyMissingAvailability = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"No appAvailabilities resource exists"}]}`

	tests := []struct {
		name         string
		err          error
		resourceType string
		want         bool
	}{
		{name: "nil error", err: nil, resourceType: "appAvailabilities", want: false},
		{name: "missing app availability", err: ParseErrorWithStatus([]byte(missingAvailability), 404), resourceType: "appAvailabilities", want: true},
		{name: "wrapped missing app availability", err: fmt.Errorf("pricing availability view: %w", ParseErrorWithStatus([]byte(missingAvailability), 404)), resourceType: "appAvailabilities", want: true},
		{name: "missing app is not missing availability", err: ParseErrorWithStatus([]byte(missingApp), 404), resourceType: "appAvailabilities", want: false},
		{name: "missing iap availability", err: ParseErrorWithStatus([]byte(missingIAPAvailability), 404), resourceType: "inAppPurchaseAvailabilities", want: true},
		{name: "missing iap availability is not missing iap", err: ParseErrorWithStatus([]byte(missingIAPAvailability), 404), resourceType: "inAppPurchases", want: false},
		{name: "missing price schedule reported as type null", err: ParseErrorWithStatus([]byte(missingScheduleManualPrices), 404), resourceType: "null", want: true},
		{name: "type null is not app price schedules", err: ParseErrorWithStatus([]byte(missingScheduleManualPrices), 404), resourceType: "appPriceSchedules", want: false},
		{name: "legacy no resource exists wording", err: ParseErrorWithStatus([]byte(legacyMissingAvailability), 404), resourceType: "appAvailabilities", want: true},
		{name: "matching detail without not found status", err: ParseErrorWithStatus([]byte(missingAvailability), 400), resourceType: "appAvailabilities", want: false},
		{name: "not found without detail", err: &APIError{Code: "NOT_FOUND", StatusCode: 404, Title: "not found"}, resourceType: "appAvailabilities", want: false},
		{name: "plain not found sentinel", err: ErrNotFound, resourceType: "appAvailabilities", want: false},
		{name: "empty resource type", err: ParseErrorWithStatus([]byte(missingAvailability), 404), resourceType: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsMissingResourceOfType(test.err, test.resourceType); got != test.want {
				t.Fatalf("IsMissingResourceOfType() = %v, want %v", got, test.want)
			}
		})
	}
}
