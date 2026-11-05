package reviews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// TestLoadReviewBatchTargetsReadsFileAtSizeLimit proves an at-limit file
// passes the size gate: the single response inside it is oversized, so the
// error must come from the per-response ceiling, not the file ceiling.
func TestLoadReviewBatchTargetsReadsFileAtSizeLimit(t *testing.T) {
	path := writeSizedReviewBatchFile(t, reviewBatchMaxFileBytes)

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected the oversized response inside an at-limit file to be rejected")
	}
	if strings.Contains(err.Error(), "--file must not exceed") {
		t.Fatalf("file at the size limit must pass the file-size gate, got %v", err)
	}
	want := fmt.Sprintf("replies[0].response must not exceed %d bytes", reviewBatchMaxResponseBytes)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q, got %v", want, err)
	}
}

// TestLoadReviewBatchTargetsAcceptsMaximalMultibyteBatch pins the worst-case
// legitimate batch: every permitted review id paired with a distinct response
// of maximal decoded size written entirely in four-byte UTF-8. The raw file is
// far larger than the old one-byte-per-character arithmetic assumed, and it
// must still be accepted whole.
func TestLoadReviewBatchTargetsAcceptsMaximalMultibyteBatch(t *testing.T) {
	response := strings.Repeat("\U0001D44E", reviewBatchMaxResponseBytes/4)
	if len(response) != reviewBatchMaxResponseBytes {
		t.Fatalf("expected a %d byte response, got %d", reviewBatchMaxResponseBytes, len(response))
	}

	replies := make([]reviewBatchReplyInput, 0, reviewBatchMaxTargets)
	for i := range reviewBatchMaxTargets {
		replies = append(replies, reviewBatchReplyInput{
			Response:  response,
			ReviewIDs: []string{fmt.Sprintf("review-%d", i)},
		})
	}
	body, err := json.Marshal(reviewBatchInput{Replies: replies})
	if err != nil {
		t.Fatalf("marshal batch file: %v", err)
	}
	if len(body) > reviewBatchMaxFileBytes {
		t.Fatalf("maximal legitimate batch serializes to %d bytes, above the %d byte file limit", len(body), reviewBatchMaxFileBytes)
	}
	path := writeReviewBatchTestFile(t, body)

	targets, err := loadReviewBatchTargets(path)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != reviewBatchMaxTargets {
		t.Fatalf("expected %d targets, got %d", reviewBatchMaxTargets, len(targets))
	}
}

// TestLoadReviewBatchTargetsAcceptsMaximalEscapedMultibyteBatch pins the most
// expensive legitimate on-disk encoding: the same maximal four-byte-script
// batch written with \u surrogate-pair escapes (Python's default ensure_ascii
// output), where every character costs twelve raw bytes. The file must still
// fit under the file ceiling and decode to the full target set.
func TestLoadReviewBatchTargetsAcceptsMaximalEscapedMultibyteBatch(t *testing.T) {
	escaped := strings.Repeat(`\ud835\udc4e`, reviewBatchMaxResponseBytes/4)

	var builder strings.Builder
	builder.WriteString(`{"replies":[`)
	for i := range reviewBatchMaxTargets {
		if i > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(&builder, `{"response":"%s","reviewIds":["review-%d"]}`, escaped, i)
	}
	builder.WriteString(`]}`)
	body := []byte(builder.String())
	if len(body) > reviewBatchMaxFileBytes {
		t.Fatalf("maximal escaped batch serializes to %d bytes, above the %d byte file limit", len(body), reviewBatchMaxFileBytes)
	}
	path := writeReviewBatchTestFile(t, body)

	targets, err := loadReviewBatchTargets(path)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != reviewBatchMaxTargets {
		t.Fatalf("expected %d targets, got %d", reviewBatchMaxTargets, len(targets))
	}
	if len(targets[0].Response) != reviewBatchMaxResponseBytes {
		t.Fatalf("expected a %d byte decoded response, got %d", reviewBatchMaxResponseBytes, len(targets[0].Response))
	}
}

func TestLoadReviewBatchTargetsAcceptsResponseAtByteLimit(t *testing.T) {
	body, err := json.Marshal(reviewBatchInput{
		Replies: []reviewBatchReplyInput{{
			Response:  strings.Repeat("a", reviewBatchMaxResponseBytes),
			ReviewIDs: []string{"review-1"},
		}},
	})
	if err != nil {
		t.Fatalf("marshal batch file: %v", err)
	}
	path := writeReviewBatchTestFile(t, body)

	targets, err := loadReviewBatchTargets(path)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != 1 || len(targets[0].Response) != reviewBatchMaxResponseBytes {
		t.Fatalf("unexpected targets at response byte limit: %d", len(targets))
	}
}

func TestLoadReviewBatchTargetsRejectsResponseOneByteOverLimit(t *testing.T) {
	body, err := json.Marshal(reviewBatchInput{
		Replies: []reviewBatchReplyInput{{
			Response:  strings.Repeat("a", reviewBatchMaxResponseBytes+1),
			ReviewIDs: []string{"review-1"},
		}},
	})
	if err != nil {
		t.Fatalf("marshal batch file: %v", err)
	}
	path := writeReviewBatchTestFile(t, body)

	_, err = loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected oversized response rejection, got nil")
	}
	want := fmt.Sprintf("replies[0].response must not exceed %d bytes", reviewBatchMaxResponseBytes)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q, got %v", want, err)
	}
}

func TestLoadReviewBatchTargetsRejectsFileOneByteOverSizeLimit(t *testing.T) {
	path := writeSizedReviewBatchFile(t, reviewBatchMaxFileBytes+1)

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected oversized --file rejection, got nil")
	}
	want := fmt.Sprintf("--file must not exceed %d bytes", reviewBatchMaxFileBytes)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q, got %v", want, err)
	}
}

func TestLoadReviewBatchTargetsAcceptsMaximumReviewIDs(t *testing.T) {
	path := writeReviewBatchFileWithReviewIDs(t, reviewBatchMaxTargets)

	targets, err := loadReviewBatchTargets(path)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != reviewBatchMaxTargets {
		t.Fatalf("expected %d targets, got %d", reviewBatchMaxTargets, len(targets))
	}
}

func TestLoadReviewBatchTargetsRejectsOneReviewIDOverLimit(t *testing.T) {
	path := writeReviewBatchFileWithReviewIDs(t, reviewBatchMaxTargets+1)

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected too-many-review-ids rejection, got nil")
	}
	want := fmt.Sprintf("--file must not contain more than %d review ids", reviewBatchMaxTargets)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q, got %v", want, err)
	}
}

// reviewBatchFileWithEmptyIDs returns a document whose token count is exactly
// idCount + 12: one reply object with one response and idCount empty review
// ids. Empty ids are the cheapest way to pack tokens into a small file.
func reviewBatchFileWithEmptyIDs(t *testing.T, idCount int) string {
	t.Helper()

	var builder strings.Builder
	builder.WriteString(`{"replies":[{"response":"x","reviewIds":[`)
	for i := range idCount {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`""`)
	}
	builder.WriteString(`]}]}`)
	return writeReviewBatchTestFile(t, []byte(builder.String()))
}

// TestLoadReviewBatchTargetsRejectsTokenFloodBeforeMaterializing pins the
// anti-flood guard: a file that stays under the byte ceiling but packs more
// JSON tokens than any permitted batch can produce must fail on the token
// budget — before the document is materialized — not on the later target
// count.
func TestLoadReviewBatchTargetsRejectsTokenFloodBeforeMaterializing(t *testing.T) {
	path := reviewBatchFileWithEmptyIDs(t, reviewBatchMaxFileTokens-12+1)

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected token flood rejection, got nil")
	}
	want := fmt.Sprintf("--file must not contain more than %d JSON tokens", reviewBatchMaxFileTokens)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q, got %v", want, err)
	}
}

// TestLoadReviewBatchTargetsTokenBudgetAdmitsFileAtTokenLimit proves the
// budget boundary: a document holding exactly the permitted number of tokens
// passes the pre-pass and fails only on the later, materialized checks.
func TestLoadReviewBatchTargetsTokenBudgetAdmitsFileAtTokenLimit(t *testing.T) {
	path := reviewBatchFileWithEmptyIDs(t, reviewBatchMaxFileTokens-12)

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected empty review ids to be rejected, got nil")
	}
	if strings.Contains(err.Error(), "JSON tokens") {
		t.Fatalf("file at the token limit must pass the token budget, got %v", err)
	}
}

func TestLoadReviewBatchTargetsRejectsInvalidUTF8(t *testing.T) {
	path := writeReviewBatchTestFile(t, []byte("{\"replies\":[{\"response\":\"\xff\",\"reviewIds\":[\"review-1\"]}]}"))

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected invalid UTF-8 rejection, got nil")
	}
	if !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("expected invalid UTF-8 error, got %v", err)
	}
}

func TestLoadReviewBatchTargetsRejectsUnpairedSurrogateEscape(t *testing.T) {
	path := writeReviewBatchTestFile(t, []byte(`{"replies":[{"response":"\ud835x","reviewIds":["review-1"]}]}`))

	_, err := loadReviewBatchTargets(path)
	if err == nil {
		t.Fatal("expected unpaired surrogate rejection, got nil")
	}
	if !strings.Contains(err.Error(), "surrogate") {
		t.Fatalf("expected surrogate error, got %v", err)
	}
}

func TestLoadReviewBatchTargetsRejectsDuplicateObjectKeys(t *testing.T) {
	for _, body := range []string{
		`{"replies":[],"replies":[]}`,
		`{"replies":[{"response":"first","response":"second","reviewIds":["review-1"]}]}`,
		`{"replies":[{"response":"first","reviewIds":["review-1"],"reviewIds":["review-2"]}]}`,
	} {
		path := writeReviewBatchTestFile(t, []byte(body))
		_, err := loadReviewBatchTargets(path)
		if err == nil {
			t.Fatalf("expected duplicate-key rejection for %s", body)
		}
		if !strings.Contains(err.Error(), "duplicate JSON field") {
			t.Fatalf("expected duplicate-field error for %s, got %v", body, err)
		}
	}
}

func TestLoadReviewBatchTargetsClonesTrimmedStrings(t *testing.T) {
	body := `{"replies":[{"response":"  Thanks  ","reviewIds":["  review-1  "]}]}`
	path := writeReviewBatchTestFile(t, []byte(body))

	targets, err := loadReviewBatchTargets(path)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != 1 || targets[0].Response != "Thanks" || targets[0].ReviewID != "review-1" {
		t.Fatalf("unexpected trimmed target: %+v", targets)
	}
}

func TestLoadReviewBatchTargetsPreservesTrailingSpaceInPath(t *testing.T) {
	directory := t.TempDir()
	plainPath := filepath.Join(directory, "replies.json")
	spacedPath := plainPath + " "
	plainBody := []byte(`{"replies":[{"response":"plain","reviewIds":["plain-review"]}]}`)
	spacedBody := []byte(`{"replies":[{"response":"spaced","reviewIds":["spaced-review"]}]}`)
	if err := os.WriteFile(plainPath, plainBody, 0o600); err != nil {
		t.Fatalf("write plain batch file: %v", err)
	}
	if err := os.WriteFile(spacedPath, spacedBody, 0o600); err != nil {
		t.Fatalf("write trailing-space batch file: %v", err)
	}

	targets, err := loadReviewBatchTargets(spacedPath)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != 1 || targets[0].ReviewID != "spaced-review" || targets[0].Response != "spaced" {
		t.Fatalf("expected exact trailing-space path, got %+v", targets)
	}
}

func TestLoadReviewBatchTargetsAcceptsAllSpaceFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), " ")
	body := []byte(`{"replies":[{"response":"spaces are a legal filename","reviewIds":["review-1"]}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write all-space batch file: %v", err)
	}

	targets, err := loadReviewBatchTargets(path)
	if err != nil {
		t.Fatalf("loadReviewBatchTargets() error: %v", err)
	}
	if len(targets) != 1 || targets[0].ReviewID != "review-1" {
		t.Fatalf("unexpected targets: %+v", targets)
	}
}

func TestFetchReviewBatchReviewInfoRejectsRepeatedPaginationURL(t *testing.T) {
	requests := 0
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		return testJSONResponse(http.StatusOK, `{
			"data": [],
			"links": {"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews?cursor=same"}
		}`), nil
	})
	client := newTestHistoryClient(t, transport)

	_, err := fetchReviewBatchReviewInfo(
		context.Background(),
		client,
		"app-1",
		[]reviewBatchTarget{{ReviewID: "review-1", Response: "Thanks"}},
	)
	if err == nil {
		t.Fatal("expected repeated pagination URL rejection, got nil")
	}
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected two requests before repeated-link rejection, got %d", requests)
	}
}

func TestFetchReviewBatchReviewInfoAppliesPageBound(t *testing.T) {
	requests := 0
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		return testJSONResponse(http.StatusOK, fmt.Sprintf(`{
			"data": [],
			"links": {"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews?cursor=%d"}
		}`, requests)), nil
	})
	client := newTestHistoryClient(t, transport)

	_, err := fetchReviewBatchReviewInfoWithPageLimit(
		context.Background(),
		client,
		"app-1",
		[]reviewBatchTarget{{ReviewID: "review-1", Response: "Thanks"}},
		2,
	)
	if err == nil {
		t.Fatal("expected pagination page-limit rejection, got nil")
	}
	if !strings.Contains(err.Error(), "must not exceed 2 pages") {
		t.Fatalf("expected page-limit error, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected two requests before the page bound, got %d", requests)
	}
}

// writeSizedReviewBatchFile writes a valid single-target batch file whose byte
// length is exactly totalSize, padding the response body to reach it.
func writeSizedReviewBatchFile(t *testing.T, totalSize int) string {
	t.Helper()

	envelope := func(response string) []byte {
		body, err := json.Marshal(reviewBatchInput{
			Replies: []reviewBatchReplyInput{{Response: response, ReviewIDs: []string{"review-1"}}},
		})
		if err != nil {
			t.Fatalf("marshal batch file: %v", err)
		}
		return body
	}

	overhead := len(envelope(""))
	if overhead > totalSize {
		t.Fatalf("cannot build a batch file as small as %d bytes", totalSize)
	}
	body := envelope(strings.Repeat("a", totalSize-overhead))
	if len(body) != totalSize {
		t.Fatalf("expected a %d byte batch file, got %d", totalSize, len(body))
	}
	return writeReviewBatchTestFile(t, body)
}

func writeReviewBatchFileWithReviewIDs(t *testing.T, count int) string {
	t.Helper()

	reviewIDs := make([]string, 0, count)
	for i := range count {
		reviewIDs = append(reviewIDs, fmt.Sprintf("review-%d", i))
	}
	body, err := json.Marshal(reviewBatchInput{
		Replies: []reviewBatchReplyInput{{Response: "Thanks for the feedback.", ReviewIDs: reviewIDs}},
	})
	if err != nil {
		t.Fatalf("marshal batch file: %v", err)
	}
	return writeReviewBatchTestFile(t, body)
}

func writeReviewBatchTestFile(t *testing.T, body []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "replies.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write batch file: %v", err)
	}
	return path
}
