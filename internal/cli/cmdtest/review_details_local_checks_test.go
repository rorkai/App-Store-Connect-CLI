package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

var reviewDetailCompleteContactArgs = []string{
	"--contact-first-name", "Dev",
	"--contact-last-name", "Support",
	"--contact-email", "dev@example.com",
	"--contact-phone", "+1 408 555 0100",
}

func reviewDetailCreateArgs(extra ...string) []string {
	args := []string{"review", "details-create", "--version-id", "version-1"}
	args = append(args, reviewDetailCompleteContactArgs...)
	return append(args, extra...)
}

// failOnReviewDetailHTTP proves a local check rejected the input before any
// App Store Connect request.
func failOnReviewDetailHTTP(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()
}

// captureReviewDetailWrite answers one review detail write and returns its body once sent.
func captureReviewDetailWrite(t *testing.T, method, path string) func() string {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	matcher := asctest.PATCH(path)
	if method == http.MethodPost {
		matcher = asctest.POST(path)
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(matcher, asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{}}}`))
	return func() string {
		requests := reg.Requests()
		if len(requests) == 0 {
			return ""
		}
		return string(requests[0].Body)
	}
}

func runReviewDetailCommand(t *testing.T, args []string) (string, string) {
	t.Helper()
	return captureOutput(t, func() {
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
}

func assertReviewDetailUsageExit(t *testing.T, args []string, wantErr string) {
	t.Helper()
	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, wantErr) {
		t.Fatalf("expected stderr to contain %q, got %q", wantErr, stderr)
	}
	if strings.Contains(stderr, "USAGE") {
		t.Fatalf("expected a one-line diagnostic without the usage page, got %q", stderr)
	}
}

func TestReviewDetailsRejectOverlongNotesBeforeRequest(t *testing.T) {
	notes := strings.Repeat("a", 4163)
	tests := []struct {
		name string
		args []string
	}{
		{name: "details-create", args: reviewDetailCreateArgs("--notes", notes)},
		{name: "details-create --if-exists update", args: reviewDetailCreateArgs("--notes", notes, "--if-exists", "update")},
		{name: "details-update", args: []string{"review", "details-update", "--id", "detail-1", "--notes", notes}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			failOnReviewDetailHTTP(t)
			assertReviewDetailUsageExit(t, test.args, "Error: --notes is 4,163 characters; App Store review notes allow at most 4,000. Remove at least 163 characters.")
		})
	}
}

func TestReviewDetailsRejectNotesOneCharacterOverLimit(t *testing.T) {
	failOnReviewDetailHTTP(t)
	assertReviewDetailUsageExit(
		t,
		[]string{"review", "details-update", "--id", "detail-1", "--notes", strings.Repeat("a", 4001)},
		"Error: --notes is 4,001 characters; App Store review notes allow at most 4,000. Remove at least 1 character.",
	)
}

func TestReviewDetailsSendNotesAtLimit(t *testing.T) {
	// 4,000 characters that are 8,000 bytes: the limit counts characters.
	notes := strings.Repeat("é", 4000)
	tests := []struct {
		name   string
		method string
		path   string
		args   []string
	}{
		{name: "details-create", method: http.MethodPost, path: "/v1/appStoreReviewDetails", args: reviewDetailCreateArgs("--notes", notes)},
		{name: "details-update", method: http.MethodPatch, path: "/v1/appStoreReviewDetails/detail-1", args: []string{"review", "details-update", "--id", "detail-1", "--notes", notes}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := captureReviewDetailWrite(t, test.method, test.path)
			runReviewDetailCommand(t, test.args)
			if !strings.Contains(body(), `"notes":"`+notes+`"`) {
				t.Fatalf("expected notes in request body, got %d bytes", len(body()))
			}
		})
	}
}

func TestReviewDetailsTrimNotesBeforeCountingAndCountCRLFOnce(t *testing.T) {
	// Surrounding whitespace is trimmed before sending, and a CRLF line break
	// counts once, so neither may push otherwise valid notes over the limit.
	// Counting CRLF as two characters would make these notes 5,998 long.
	notes := "  \n" + strings.Repeat("a\r\n", 2000) + "\n  "
	body := captureReviewDetailWrite(t, http.MethodPatch, "/v1/appStoreReviewDetails/detail-1")
	runReviewDetailCommand(t, []string{"review", "details-update", "--id", "detail-1", "--notes", notes})
	if !strings.Contains(body(), `"notes":"a\r\na\r\n`) {
		t.Fatalf("expected trimmed notes in request body, got %q", body()[:min(len(body()), 120)])
	}
}

func TestReviewDetailsRejectNorthAmericanPhoneWithWrongDigitCount(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "details-create seven digits",
			args:    []string{"review", "details-create", "--version-id", "version-1", "--contact-first-name", "Dev", "--contact-last-name", "Support", "--contact-email", "dev@example.com", "--contact-phone", "+1 555 0100"},
			wantErr: "Error: --contact-phone has 7 digits after +1; North American (+1) numbers need exactly 10 (3-digit area code and 7-digit number), for example +1 408 555 0100",
		},
		{
			name:    "details-create eleven digits",
			args:    []string{"review", "details-create", "--version-id", "version-1", "--contact-first-name", "Dev", "--contact-last-name", "Support", "--contact-email", "dev@example.com", "--contact-phone", "+197209757881"},
			wantErr: "Error: --contact-phone has 11 digits after +1;",
		},
		{
			name:    "details-update",
			args:    []string{"review", "details-update", "--id", "detail-1", "--contact-phone", "+1 408 555 010"},
			wantErr: "Error: --contact-phone has 9 digits after +1;",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			failOnReviewDetailHTTP(t)
			assertReviewDetailUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestReviewDetailsLeaveOtherPhoneFormatsToAppStoreConnect(t *testing.T) {
	// App Store Connect accepted numbers without a plus sign in agent runs, and
	// the CLI has no per-country numbering data, so only +1 numbers are checked.
	for _, phone := range []string{"+1 (408) 555-0100", "+44 844 209 0611", "4085550100", "+1 408 555 0100 ext 12"} {
		t.Run(phone, func(t *testing.T) {
			body := captureReviewDetailWrite(t, http.MethodPatch, "/v1/appStoreReviewDetails/detail-1")
			runReviewDetailCommand(t, []string{"review", "details-update", "--id", "detail-1", "--contact-phone", phone})
			if !strings.Contains(body(), `"contactPhone":"`+phone+`"`) {
				t.Fatalf("expected contactPhone %q in body, got %s", phone, body())
			}
		})
	}
}

func TestReviewDetailsCreateRequiresContactFields(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "email only",
			args:    []string{"review", "details-create", "--version-id", "version-1", "--contact-email", "dev@example.com", "--notes", "Guest flow"},
			wantErr: "Error: review details-create needs --contact-first-name and --contact-phone; App Store Connect rejects a new review detail without them.",
		},
		{
			name:    "phone missing",
			args:    []string{"review", "details-create", "--version-id", "version-1", "--contact-first-name", "Dev", "--contact-last-name", "Support", "--contact-email", "dev@example.com"},
			wantErr: "Error: review details-create needs --contact-phone; App Store Connect rejects a new review detail without them.",
		},
		{
			name:    "blank value counts as missing",
			args:    []string{"review", "details-create", "--version-id", "version-1", "--contact-first-name", " ", "--contact-last-name", "Support", "--contact-email", "dev@example.com", "--contact-phone", "+1 408 555 0100"},
			wantErr: "Error: review details-create needs --contact-first-name;",
		},
		{
			name:    "no attributes",
			args:    []string{"review", "details-create", "--version-id", "version-1"},
			wantErr: "Error: review details-create needs --contact-first-name and --contact-phone;",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			failOnReviewDetailHTTP(t)
			assertReviewDetailUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestReviewDetailsCreateMissingContactsPointsToUpdate(t *testing.T) {
	failOnReviewDetailHTTP(t)
	assertReviewDetailUsageExit(
		t,
		[]string{"review", "details-create", "--version-id", "version-1", "--notes", "Guest flow"},
		"To change a version that already has review details, use asc review details-update or --if-exists update.",
	)
}

func TestReviewDetailsCreateSendsCompleteContacts(t *testing.T) {
	body := captureReviewDetailWrite(t, http.MethodPost, "/v1/appStoreReviewDetails")
	runReviewDetailCommand(t, reviewDetailCreateArgs("--notes", "Guest flow"))
	for _, want := range []string{
		`"contactFirstName":"Dev"`,
		`"contactLastName":"Support"`,
		`"contactEmail":"dev@example.com"`,
		`"contactPhone":"+1 408 555 0100"`,
	} {
		if !strings.Contains(body(), want) {
			t.Fatalf("expected %s in body, got %s", want, body())
		}
	}
}

func TestReviewDetailsCreateLeavesLastNameAndEmailToAppStoreConnect(t *testing.T) {
	// Only first name and phone were observed as required on create, so a
	// create without last name or email is sent and App Store Connect decides.
	body := captureReviewDetailWrite(t, http.MethodPost, "/v1/appStoreReviewDetails")
	runReviewDetailCommand(t, []string{
		"review", "details-create", "--version-id", "version-1",
		"--contact-first-name", "Dev", "--contact-phone", "+1 408 555 0100",
		"--notes", "Guest flow",
	})
	for _, want := range []string{`"contactFirstName":"Dev"`, `"contactPhone":"+1 408 555 0100"`} {
		if !strings.Contains(body(), want) {
			t.Fatalf("expected %s in body, got %s", want, body())
		}
	}
	for _, unwanted := range []string{"contactLastName", "contactEmail"} {
		if strings.Contains(body(), unwanted) {
			t.Fatalf("expected no %s in body, got %s", unwanted, body())
		}
	}
}

func TestReviewDetailsUpdateDoesNotRequireContactFields(t *testing.T) {
	body := captureReviewDetailWrite(t, http.MethodPatch, "/v1/appStoreReviewDetails/detail-1")
	runReviewDetailCommand(t, []string{"review", "details-update", "--id", "detail-1", "--notes", "Guest flow"})
	if !strings.Contains(body(), `"notes":"Guest flow"`) {
		t.Fatalf("expected notes in body, got %s", body())
	}
}
