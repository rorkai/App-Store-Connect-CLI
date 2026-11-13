package reviews

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	reviewBatchStatusCreated = "created"
	reviewBatchStatusFailed  = "failed"
	reviewBatchStatusPlanned = "planned"
	reviewBatchStatusSkipped = "skipped"
)

// reviewBatchMaxTargets bounds how many resolved review ids a single
// respond-batch invocation may act on.
//
// Every resolved target costs one authenticated response mutation plus its
// share of the paginated review lookup, and the whole run shares one request
// timeout. 500 covers realistic bulk replies — an app's unresponded backlog
// after a release, or a curated set of reviews exported from `asc reviews
// list` — while keeping the worst-case fan-out from a checked-in batch file
// bounded and reviewable. Larger campaigns split cleanly into several files.
const reviewBatchMaxTargets = 500

// reviewBatchMaxResponseBytes bounds the decoded size of a single response
// body before it is expanded across its review ids.
//
// App Store Connect caps a review response at 5,970 characters and a character
// costs at most four bytes in UTF-8, so no response the API accepts exceeds
// 23,880 decoded bytes. 24 KiB admits every legitimate response — including
// one written entirely in a four-byte script — while stopping a single
// multi-megabyte body from being multiplied into up to reviewBatchMaxTargets
// request payloads.
const reviewBatchMaxResponseBytes = 24 << 10

// reviewBatchMaxFileBytes bounds how much of --file is expanded into memory
// before it is parsed.
//
// The worst-case legitimate batch pairs each of the reviewBatchMaxTargets
// permitted review ids with a distinct response of reviewBatchMaxResponseBytes,
// which serializes to about 12 MiB of raw UTF-8 — and tooling that escapes
// non-ASCII characters on output (for example Python's default ensure_ascii
// encoding) triples that on disk for a four-byte script, to about 37 MiB,
// because every character becomes a twelve-byte surrogate-pair escape. 64 MiB
// accepts every plausible encoding of a maximal batch and rejects anything
// that is no longer a usable review batch. None of these limits has an
// override flag: raising the ceiling is the same as removing it.
const reviewBatchMaxFileBytes = 64 << 20

// reviewBatchMaxFileTokens bounds how many JSON tokens --file may contain.
//
// The byte and target ceilings alone still let a small file decode into
// millions of tiny values: tens of MiB of empty review ids materialize
// hundreds of megabytes of string headers before the target-count check can
// run. A maximal legitimate batch — reviewBatchMaxTargets replies, each with
// one response and one review id — is about four thousand tokens, so 16,384
// leaves fourfold headroom while stopping a flood of tiny elements before the
// document is materialized.
const reviewBatchMaxFileTokens = 16 << 10

// reviewBatchMaxPages is a final backstop for a malicious or broken
// pagination chain. With the command's first-page limit of 200 this still
// permits scanning ten million reviews, far beyond a normal batch lookup,
// while guaranteeing that unique-but-never-ending next links terminate.
const reviewBatchMaxPages = 50_000

type reviewBatchInput struct {
	Replies []reviewBatchReplyInput `json:"replies"`
}

type reviewBatchReplyInput struct {
	Response  string   `json:"response"`
	ReviewIDs []string `json:"reviewIds"`
}

type reviewBatchTarget struct {
	ReviewID string
	Response string
}

type reviewBatchResult struct {
	AppID   string                    `json:"appId"`
	DryRun  bool                      `json:"dryRun"`
	Summary reviewBatchSummary        `json:"summary"`
	Results []reviewBatchReviewResult `json:"results"`

	refused error
}

type reviewBatchSummary struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Planned int `json:"planned"`
}

type reviewBatchReviewResult struct {
	ReviewID           string `json:"reviewId"`
	Status             string `json:"status"`
	ResponseID         string `json:"responseId,omitempty"`
	ExistingResponseID string `json:"existingResponseId,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Error              string `json:"error,omitempty"`
}

type reviewBatchReviewInfo struct {
	ReviewID           string
	ExistingResponseID string
}

// ReviewsRespondBatchCommand returns the reviews respond-batch subcommand.
func ReviewsRespondBatchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("respond-batch", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	filePath := fs.String("file", "", "Path to grouped JSON replies file (required)")
	dryRun := fs.Bool("dry-run", false, "Preview responses without creating them")
	confirm := fs.Bool("confirm", false, "Confirm publishing the responses (required unless --dry-run)")
	skipExisting := fs.Bool("skip-existing", false, "Skip reviews that already have a published response")
	responseState := fs.String("response-state", reviewResponseStateAny, "Filter by response state: any, unresponded/unreplied, responded/replied")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "respond-batch",
		ShortUsage: "asc reviews respond-batch [flags]",
		ShortHelp:  "Create responses for multiple customer reviews.",
		LongHelp: fmt.Sprintf(`Create responses for multiple customer reviews from a grouped JSON file.

The input file must contain a top-level replies array. Each reply has one
response body and one or more reviewIds.

A single run accepts at most %d bytes of input and %d review ids across
all replies, and each response must not exceed %d bytes. Split larger
campaigns into several files.

Example input:
  {
    "replies": [
      {
        "response": "Thanks for the feedback.",
        "reviewIds": ["REVIEW_ID_1", "REVIEW_ID_2"]
      }
    ]
  }

Examples:
  asc reviews respond-batch --app "123456789" --file replies.json --dry-run
  asc reviews respond-batch --app "123456789" --file replies.json --confirm
  asc reviews respond-batch --app "123456789" --file replies.json --skip-existing --output json --confirm
  asc reviews respond-batch --app "123456789" --file replies.json --response-state unresponded --confirm`,
			reviewBatchMaxFileBytes, reviewBatchMaxTargets, reviewBatchMaxResponseBytes),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				return shared.WithDiagnostic(shared.UsageError("--app is required (or set ASC_APP_ID)"), shared.DiagnosticRequiredInputMissing, "--app")
			}
			if *filePath == "" {
				return shared.WithDiagnostic(shared.UsageError("--file is required"), shared.DiagnosticRequiredInputMissing, "--file")
			}

			normalizedResponseState, err := normalizeReviewResponseState(*responseState)
			if err != nil {
				return shared.WithDiagnostic(shared.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--response-state")
			}

			if err := shared.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			targets, err := loadReviewBatchTargets(*filePath)
			if err != nil {
				code := shared.DiagnosticFileInvalidFormat
				switch {
				case errors.Is(err, os.ErrNotExist):
					code = shared.DiagnosticFileNotFound
				case errors.Is(err, os.ErrPermission):
					code = shared.DiagnosticFilePermissionDenied
				}
				return shared.WithDiagnostic(shared.UsageError(err.Error()), code, "--file")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews respond-batch: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := executeReviewsRespondBatch(
				requestCtx,
				client,
				resolvedAppID,
				targets,
				*dryRun,
				*skipExisting,
				normalizedResponseState,
			)
			if err != nil {
				return err
			}

			if err := printReviewBatchResult(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.Summary.Failed > 0 {
				return shared.NewReportedError(shared.NewErrorWithCause(fmt.Errorf("reviews respond-batch: %d review(s) failed", result.Summary.Failed), result.refused))
			}
			return nil
		},
	}
}

func loadReviewBatchTargets(path string) ([]reviewBatchTarget, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read --file: %w", err)
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil {
		return nil, fmt.Errorf("failed to read --file: %w", err)
	} else if info.Size() > reviewBatchMaxFileBytes {
		return nil, fmt.Errorf("--file must not exceed %d bytes", reviewBatchMaxFileBytes)
	}
	if err := checkReviewBatchFileBudget(file); err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to read --file: %w", err)
	}

	stream := &reviewBatchTokenStream{
		decoder: json.NewDecoder(newStrictReviewBatchReader(file)),
	}
	return stream.parse()
}

// checkReviewBatchFileBudget is a constant-memory lexical pass. Unlike a
// json.Decoder token pre-pass it never materializes string values, so a single
// near-limit string cannot create another file-sized allocation. The real
// schema parser below repeats both strict encoding and token checks in case a
// concurrently modified file changes after this pass.
func checkReviewBatchFileBudget(file io.Reader) error {
	counter := &reviewBatchLexicalCounter{}
	if _, err := io.Copy(counter, newStrictReviewBatchReader(file)); err != nil {
		if strings.Contains(err.Error(), "--file must not exceed") {
			return err
		}
		return fmt.Errorf("failed to parse --file: %w", err)
	}
	return nil
}

type reviewBatchLexicalCounter struct {
	tokens      int
	inString    bool
	escaped     bool
	inPrimitive bool
}

func (c *reviewBatchLexicalCounter) Write(data []byte) (int, error) {
	for index, current := range data {
		if c.inString {
			if c.escaped {
				c.escaped = false
			} else if current == '\\' {
				c.escaped = true
			} else if current == '"' {
				c.inString = false
			}
			continue
		}
		if c.inPrimitive && isReviewBatchJSONSeparator(current) {
			c.inPrimitive = false
		}
		switch {
		case current == '"':
			c.inString = true
			if err := c.addToken(); err != nil {
				return index, err
			}
		case current == '{' || current == '}' || current == '[' || current == ']':
			if err := c.addToken(); err != nil {
				return index, err
			}
		case isReviewBatchJSONSeparator(current):
			continue
		case !c.inPrimitive:
			c.inPrimitive = true
			if err := c.addToken(); err != nil {
				return index, err
			}
		}
	}
	return len(data), nil
}

func (c *reviewBatchLexicalCounter) addToken() error {
	c.tokens++
	if c.tokens > reviewBatchMaxFileTokens {
		return fmt.Errorf("--file must not contain more than %d JSON tokens", reviewBatchMaxFileTokens)
	}
	return nil
}

func isReviewBatchJSONSeparator(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', ',', ':', '{', '}', '[', ']':
		return true
	default:
		return false
	}
}

type reviewBatchTokenStream struct {
	decoder *json.Decoder
	tokens  int
	seenIDs map[string]struct{}
	targets []reviewBatchTarget
}

func (s *reviewBatchTokenStream) token() (json.Token, error) {
	token, err := s.decoder.Token()
	if err != nil {
		return nil, err
	}
	s.tokens++
	if s.tokens > reviewBatchMaxFileTokens {
		return nil, fmt.Errorf("--file must not contain more than %d JSON tokens", reviewBatchMaxFileTokens)
	}
	return token, nil
}

func (s *reviewBatchTokenStream) parse() ([]reviewBatchTarget, error) {
	first, err := s.token()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("--file must not be empty")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse --file: %w", err)
	}
	if first != json.Delim('{') {
		return nil, fmt.Errorf("failed to parse --file: top-level value must be an object")
	}

	s.seenIDs = make(map[string]struct{})
	seenFields := make(map[string]struct{}, 1)
	replies := 0
	for {
		token, err := s.token()
		if err != nil {
			return nil, fmt.Errorf("failed to parse --file: %w", err)
		}
		if token == json.Delim('}') {
			break
		}
		field, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse --file: expected a top-level field name")
		}
		if _, exists := seenFields[field]; exists {
			return nil, fmt.Errorf("failed to parse --file: duplicate JSON field %q", field)
		}
		seenFields[field] = struct{}{}
		if field != "replies" {
			return nil, fmt.Errorf("failed to parse --file: unknown field %q", field)
		}

		start, err := s.token()
		if err != nil {
			return nil, fmt.Errorf("failed to parse --file: %w", err)
		}
		if start != json.Delim('[') {
			return nil, fmt.Errorf("failed to parse --file: replies must be an array")
		}
		for {
			next, err := s.token()
			if err != nil {
				return nil, fmt.Errorf("failed to parse --file: %w", err)
			}
			if next == json.Delim(']') {
				break
			}
			if next != json.Delim('{') {
				return nil, fmt.Errorf("failed to parse --file: replies[%d] must be an object", replies)
			}
			if err := s.parseReply(replies); err != nil {
				return nil, err
			}
			replies++
		}
	}
	if _, ok := seenFields["replies"]; !ok || replies == 0 {
		return nil, fmt.Errorf("replies must contain at least one item")
	}

	extra, err := s.token()
	if err == nil {
		_ = extra
		return nil, fmt.Errorf("failed to parse --file: multiple JSON values are not allowed")
	}
	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to parse --file: %w", err)
	}
	return s.targets, nil
}

func (s *reviewBatchTokenStream) parseReply(replyIndex int) error {
	seenFields := make(map[string]struct{}, 2)
	response := ""
	hasResponse := false
	reviewIDs := make([]string, 0, 1)

	for {
		token, err := s.token()
		if err != nil {
			return fmt.Errorf("failed to parse --file: %w", err)
		}
		if token == json.Delim('}') {
			break
		}
		field, ok := token.(string)
		if !ok {
			return fmt.Errorf("failed to parse --file: expected a field name in replies[%d]", replyIndex)
		}
		if _, exists := seenFields[field]; exists {
			return fmt.Errorf("failed to parse --file: duplicate JSON field %q in replies[%d]", field, replyIndex)
		}
		seenFields[field] = struct{}{}

		switch field {
		case "response":
			value, err := s.token()
			if err != nil {
				return fmt.Errorf("failed to parse --file: %w", err)
			}
			raw, ok := value.(string)
			if !ok {
				return fmt.Errorf("failed to parse --file: replies[%d].response must be a string", replyIndex)
			}
			trimmed := strings.TrimSpace(raw)
			if len(trimmed) > reviewBatchMaxResponseBytes {
				return fmt.Errorf("replies[%d].response must not exceed %d bytes", replyIndex, reviewBatchMaxResponseBytes)
			}
			response = cloneTrimmedReviewBatchString(raw, trimmed)
			hasResponse = true
		case "reviewIds":
			start, err := s.token()
			if err != nil {
				return fmt.Errorf("failed to parse --file: %w", err)
			}
			if start != json.Delim('[') {
				return fmt.Errorf("failed to parse --file: replies[%d].reviewIds must be an array", replyIndex)
			}
			for {
				value, err := s.token()
				if err != nil {
					return fmt.Errorf("failed to parse --file: %w", err)
				}
				if value == json.Delim(']') {
					break
				}
				raw, ok := value.(string)
				if !ok {
					return fmt.Errorf("failed to parse --file: replies[%d].reviewIds[%d] must be a string", replyIndex, len(reviewIDs))
				}
				trimmed := strings.TrimSpace(raw)
				reviewID := cloneTrimmedReviewBatchString(raw, trimmed)
				if reviewID == "" {
					return fmt.Errorf("replies[%d].reviewIds[%d] is required", replyIndex, len(reviewIDs))
				}
				if _, exists := s.seenIDs[reviewID]; exists {
					return fmt.Errorf("duplicate review id %q", reviewID)
				}
				if len(s.targets)+len(reviewIDs) >= reviewBatchMaxTargets {
					return fmt.Errorf("--file must not contain more than %d review ids", reviewBatchMaxTargets)
				}
				s.seenIDs[reviewID] = struct{}{}
				reviewIDs = append(reviewIDs, reviewID)
			}
		default:
			return fmt.Errorf("failed to parse --file: unknown field %q in replies[%d]", field, replyIndex)
		}
	}

	if !hasResponse || response == "" {
		return fmt.Errorf("replies[%d].response is required", replyIndex)
	}
	if len(reviewIDs) == 0 {
		return fmt.Errorf("replies[%d].reviewIds must contain at least one review id", replyIndex)
	}
	for _, reviewID := range reviewIDs {
		s.targets = append(s.targets, reviewBatchTarget{
			ReviewID: reviewID,
			Response: response,
		})
	}
	return nil
}

func cloneTrimmedReviewBatchString(raw, trimmed string) string {
	if len(raw) == len(trimmed) {
		return raw
	}
	return strings.Clone(trimmed)
}

// strictReviewBatchReader validates bytes while json.Decoder consumes them.
// encoding/json deliberately replaces malformed UTF-8 and invalid UTF-16
// surrogate escapes with U+FFFD; mutation input must reject both instead.
// The reader also enforces the raw byte ceiling without retaining the file.
type strictReviewBatchReader struct {
	source      *bufio.Reader
	pending     [12]byte
	pendingFrom int
	pendingTo   int
	bytesRead   int
	inString    bool
	deferredErr error
}

func newStrictReviewBatchReader(source io.Reader) io.Reader {
	return &strictReviewBatchReader{source: bufio.NewReader(source)}
}

func (r *strictReviewBatchReader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if r.pendingFrom < r.pendingTo {
			n := copy(p[written:], r.pending[r.pendingFrom:r.pendingTo])
			r.pendingFrom += n
			written += n
			continue
		}
		if r.deferredErr != nil {
			err := r.deferredErr
			r.deferredErr = nil
			if written > 0 {
				r.deferredErr = err
				return written, nil
			}
			return 0, err
		}
		if err := r.fillPending(); err != nil {
			if written > 0 {
				r.deferredErr = err
				return written, nil
			}
			return 0, err
		}
	}
	return written, nil
}

func (r *strictReviewBatchReader) fillPending() error {
	r.pendingFrom = 0
	r.pendingTo = 0

	value, err := r.readRune()
	if err != nil {
		if errors.Is(err, io.EOF) && r.inString {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	r.pendingTo += utf8.EncodeRune(r.pending[r.pendingTo:], value)

	if !r.inString {
		if value == '"' {
			r.inString = true
		}
		return nil
	}
	switch {
	case value == '"':
		r.inString = false
		return nil
	case value < 0x20:
		return fmt.Errorf("unescaped control character in JSON string")
	case value != '\\':
		return nil
	}

	escaped, err := r.readRune()
	if err != nil {
		return fmt.Errorf("incomplete JSON escape: %w", err)
	}
	r.pendingTo += utf8.EncodeRune(r.pending[r.pendingTo:], escaped)
	switch escaped {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return nil
	case 'u':
		unit, err := r.readHexCodeUnit()
		if err != nil {
			return err
		}
		if utf16.IsSurrogate(rune(unit)) {
			if unit >= 0xDC00 {
				return fmt.Errorf("invalid JSON Unicode surrogate: unexpected low surrogate")
			}
			slash, err := r.readRune()
			if err != nil {
				return fmt.Errorf("invalid JSON Unicode surrogate pair: %w", err)
			}
			r.pendingTo += utf8.EncodeRune(r.pending[r.pendingTo:], slash)
			u, err := r.readRune()
			if err != nil {
				return fmt.Errorf("invalid JSON Unicode surrogate pair: %w", err)
			}
			r.pendingTo += utf8.EncodeRune(r.pending[r.pendingTo:], u)
			if slash != '\\' || u != 'u' {
				return fmt.Errorf("invalid JSON Unicode surrogate pair")
			}
			low, err := r.readHexCodeUnit()
			if err != nil {
				return err
			}
			if low < 0xDC00 || low > 0xDFFF {
				return fmt.Errorf("invalid JSON Unicode surrogate pair")
			}
		}
		return nil
	default:
		return fmt.Errorf("invalid JSON escape %q", escaped)
	}
}

func (r *strictReviewBatchReader) readHexCodeUnit() (uint16, error) {
	var value uint16
	for range 4 {
		digit, err := r.readRune()
		if err != nil {
			return 0, fmt.Errorf("incomplete JSON Unicode escape: %w", err)
		}
		r.pendingTo += utf8.EncodeRune(r.pending[r.pendingTo:], digit)
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint16(digit-'A') + 10
		default:
			return 0, fmt.Errorf("invalid JSON Unicode escape")
		}
	}
	return value, nil
}

func (r *strictReviewBatchReader) readRune() (rune, error) {
	value, size, err := r.source.ReadRune()
	if err != nil {
		return 0, err
	}
	r.bytesRead += size
	if r.bytesRead > reviewBatchMaxFileBytes {
		return 0, fmt.Errorf("--file must not exceed %d bytes", reviewBatchMaxFileBytes)
	}
	if value == utf8.RuneError && size == 1 {
		return 0, fmt.Errorf("invalid UTF-8 in --file")
	}
	return value, nil
}

func executeReviewsRespondBatch(ctx context.Context, client *asc.Client, appID string, targets []reviewBatchTarget, dryRun bool, skipExisting bool, responseState string) (reviewBatchResult, error) {
	result := reviewBatchResult{
		AppID:  appID,
		DryRun: dryRun,
		Summary: reviewBatchSummary{
			Total: len(targets),
		},
		Results: make([]reviewBatchReviewResult, 0, len(targets)),
	}

	reviews, err := fetchReviewBatchReviewInfo(ctx, client, appID, targets)
	if err != nil {
		return result, fmt.Errorf("reviews respond-batch: failed to fetch reviews: %w", err)
	}

	for _, target := range targets {
		info, ok := reviews[target.ReviewID]
		if !ok {
			result.append(reviewBatchReviewResult{
				ReviewID: target.ReviewID,
				Status:   reviewBatchStatusFailed,
				Error:    "review not found for app",
			})
			continue
		}

		if skip, reason := shouldSkipForResponseState(responseState, info.ExistingResponseID != ""); skip {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusSkipped,
				ExistingResponseID: info.ExistingResponseID,
				Reason:             reason,
			})
			continue
		}

		if skipExisting && info.ExistingResponseID != "" {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusSkipped,
				ExistingResponseID: info.ExistingResponseID,
				Reason:             "existing-response",
			})
			continue
		}

		if dryRun {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusPlanned,
				ExistingResponseID: info.ExistingResponseID,
			})
			continue
		}

		created, err := client.CreateCustomerReviewResponse(ctx, target.ReviewID, target.Response)
		if err != nil {
			result.refused = shared.KeepReadOnlyRefusal(result.refused, err)
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusFailed,
				ExistingResponseID: info.ExistingResponseID,
				Error:              err.Error(),
			})
			continue
		}

		result.append(reviewBatchReviewResult{
			ReviewID:           target.ReviewID,
			Status:             reviewBatchStatusCreated,
			ResponseID:         created.Data.ID,
			ExistingResponseID: info.ExistingResponseID,
		})
	}

	return result, nil
}

func fetchReviewBatchReviewInfo(ctx context.Context, client *asc.Client, appID string, targets []reviewBatchTarget) (map[string]reviewBatchReviewInfo, error) {
	return fetchReviewBatchReviewInfoWithPageLimit(ctx, client, appID, targets, reviewBatchMaxPages)
}

func fetchReviewBatchReviewInfoWithPageLimit(ctx context.Context, client *asc.Client, appID string, targets []reviewBatchTarget, maxPages int) (map[string]reviewBatchReviewInfo, error) {
	wanted := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		wanted[target.ReviewID] = struct{}{}
	}

	found := make(map[string]reviewBatchReviewInfo, len(targets))
	nextURL := ""
	seenNextURLs := make(map[string]struct{})
	page := 0
	for {
		if page >= maxPages {
			return found, fmt.Errorf("review pagination must not exceed %d pages", maxPages)
		}
		page++
		var (
			response *asc.ReviewsResponse
			err      error
		)
		if nextURL != "" {
			response, err = client.GetReviews(ctx, appID, asc.NextURL[asc.ReviewOption](nextURL))
		} else {
			response, err = client.GetReviews(ctx, appID, asc.Limit[asc.ReviewOption](200), asc.WithReviewIncludeResponse())
		}
		if err != nil {
			return found, err
		}

		for _, review := range response.Data {
			if _, ok := wanted[review.ID]; !ok {
				continue
			}
			existingResponseID, _ := asc.CustomerReviewPublishedResponseID(review)
			found[review.ID] = reviewBatchReviewInfo{
				ReviewID:           review.ID,
				ExistingResponseID: existingResponseID,
			}
		}

		next := strings.TrimSpace(response.Links.Next)
		if len(found) == len(wanted) || next == "" {
			break
		}
		if _, exists := seenNextURLs[next]; exists {
			return found, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNextURLs[next] = struct{}{}
		nextURL = next
	}

	return found, nil
}

func shouldSkipForResponseState(responseState string, hasResponse bool) (bool, string) {
	switch responseState {
	case reviewResponseStateUnresponded:
		if hasResponse {
			return true, "response-state-mismatch"
		}
	case reviewResponseStateResponded:
		if !hasResponse {
			return true, "response-state-mismatch"
		}
	}
	return false, ""
}

func (r *reviewBatchResult) append(item reviewBatchReviewResult) {
	r.Results = append(r.Results, item)
	switch item.Status {
	case reviewBatchStatusCreated:
		r.Summary.Created++
	case reviewBatchStatusFailed:
		r.Summary.Failed++
	case reviewBatchStatusPlanned:
		r.Summary.Planned++
	case reviewBatchStatusSkipped:
		r.Summary.Skipped++
	}
}

func printReviewBatchResult(result reviewBatchResult, output string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			renderReviewBatchResult(result, false)
			return nil
		},
		func() error {
			renderReviewBatchResult(result, true)
			return nil
		},
	)
}

func renderReviewBatchResult(result reviewBatchResult, markdown bool) {
	headers := []string{"Review ID", "Status", "Response ID", "Existing Response ID", "Reason", "Error"}
	rows := make([][]string, 0, len(result.Results)+1)
	rows = append(rows, []string{
		"summary",
		fmt.Sprintf("created=%d skipped=%d failed=%d planned=%d", result.Summary.Created, result.Summary.Skipped, result.Summary.Failed, result.Summary.Planned),
		"",
		"",
		fmt.Sprintf("total=%d dryRun=%t", result.Summary.Total, result.DryRun),
		"",
	})
	for _, item := range result.Results {
		rows = append(rows, []string{
			item.ReviewID,
			item.Status,
			item.ResponseID,
			item.ExistingResponseID,
			item.Reason,
			item.Error,
		})
	}
	if markdown {
		asc.RenderMarkdown(headers, rows)
		return
	}
	asc.RenderTable(headers, rows)
}
