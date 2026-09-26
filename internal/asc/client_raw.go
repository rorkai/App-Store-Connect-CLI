package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
)

// ErrRawPaginationNotCollection reports a paginated raw request whose first
// page did not carry a JSON array under data.
var ErrRawPaginationNotCollection = errors.New("--paginate requires a collection response with a data array")

// RawRequest sends one authenticated App Store Connect request and returns the
// response body unmodified. It shares the typed client's retry, rate-limit,
// timeout, and error-parsing behavior; the caller owns method, path, and
// payload validation.
func (c *Client) RawRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if err := validateRawRequestTarget(path); err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	// Raw requests accept caller-controlled absolute URLs. Keep their validated
	// target from redirecting the client's bearer token, request body, or Referer
	// to another origin without changing the shared client's redirect policy.
	return c.doWithHTTPClient(ctx, method, path, reader, clientWithoutRedirects(c.httpClient))
}

// validateRawRequestTarget keeps absolute raw requests inside the authenticated
// App Store Connect API origin. Root-relative API paths retain newRequest's
// existing behavior; absolute pagination cursors must not redirect the client's
// JWT to another origin or bypass path validation.
func validateRawRequestTarget(target string) error {
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		if !strings.HasPrefix(target, "/") {
			return fmt.Errorf("API request path must be root-relative and start with '/'")
		}
		return validateAPIPath(target)
	}
	if strings.ContainsRune(target, '#') {
		return fmt.Errorf("rejected absolute request URL with fragment marker (#)")
	}

	parsedURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid absolute App Store Connect request URL")
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("rejected absolute request URL with insecure scheme %q (HTTPS required)", parsedURL.Scheme)
	}
	if parsedURL.User != nil {
		return fmt.Errorf("rejected absolute request URL containing userinfo")
	}
	if parsedURL.Fragment != "" || parsedURL.RawFragment != "" {
		return fmt.Errorf("rejected absolute request URL containing a fragment")
	}

	baseURL, err := url.Parse(BaseURL)
	if err != nil {
		return fmt.Errorf("invalid App Store Connect base URL")
	}
	if parsedURL.Host != baseURL.Host {
		return fmt.Errorf("rejected request URL from untrusted host %q (expected %q)", parsedURL.Host, baseURL.Host)
	}
	if err := validateAPIPath(parsedURL.EscapedPath()); err != nil {
		return fmt.Errorf("invalid API path in absolute request URL: %w", err)
	}
	return nil
}

// RawPaginatedGET fetches a collection and follows links.next until the last
// page, returning one merged envelope: data and included are concatenated (with
// duplicate included resources removed), while links and meta come from the
// first page with links.next dropped. Each next URL must stay on the App Store
// Connect host over HTTPS, and a repeated URL stops the loop with
// ErrRepeatedPaginationURL. requestContext, when non-nil, derives a fresh
// context for every page request.
func (c *Client) RawPaginatedGET(ctx context.Context, path string, requestContext RequestContextFunc) ([]byte, error) {
	var merged *rawEnvelope
	seenNext := make(map[string]struct{})
	target := path
	for page := 1; ; page++ {
		pageCtx, cancel := requestContextFor(ctx, requestContext)
		body, err := c.RawRequest(pageCtx, "GET", target, nil)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		envelope, err := parseRawEnvelope(body)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		if merged == nil {
			merged = envelope
		} else {
			merged.absorb(envelope)
		}

		next, err := envelope.next()
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		if next == "" {
			break
		}
		if err := validateNextURL(next); err != nil {
			return nil, fmt.Errorf("page %d: %w", page+1, err)
		}
		nextIdentity := PaginationURLIdentity(next)
		if _, ok := seenNext[nextIdentity]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
		}
		seenNext[nextIdentity] = struct{}{}
		target = next
	}
	return merged.marshal()
}

type rawEnvelope struct {
	data         []json.RawMessage
	included     []json.RawMessage
	includedKeys map[string]struct{}
	hasIncluded  bool
	links        map[string]json.RawMessage
	other        map[string]json.RawMessage
}

func parseRawEnvelope(body []byte) (*rawEnvelope, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("response is not a JSON object: %w", err)
	}
	envelope := &rawEnvelope{
		includedKeys: make(map[string]struct{}),
		other:        make(map[string]json.RawMessage),
	}
	rawData, ok := top["data"]
	if !ok || json.Unmarshal(rawData, &envelope.data) != nil {
		return nil, ErrRawPaginationNotCollection
	}
	// A JSON null unmarshals into a nil slice without error. An empty to-one
	// linkage is not a collection, so it must not be coerced into `[]`; only a
	// real `[]` yields a non-nil empty slice.
	if envelope.data == nil {
		return nil, ErrRawPaginationNotCollection
	}
	if rawIncluded, ok := top["included"]; ok {
		var included []json.RawMessage
		if err := json.Unmarshal(rawIncluded, &included); err != nil {
			return nil, fmt.Errorf("included is not a JSON array: %w", err)
		}
		envelope.hasIncluded = true
		envelope.included = make([]json.RawMessage, 0, len(included))
		for _, item := range included {
			envelope.addIncluded(item)
		}
	}
	if rawLinks, ok := top["links"]; ok {
		if err := json.Unmarshal(rawLinks, &envelope.links); err != nil {
			return nil, fmt.Errorf("links is not a JSON object: %w", err)
		}
	}
	for key, value := range top {
		switch key {
		case "data", "included", "links":
		default:
			envelope.other[key] = value
		}
	}
	return envelope, nil
}

func (e *rawEnvelope) addIncluded(item json.RawMessage) {
	key := rawJSONArrayItemKey(item)
	if key != "" {
		if _, seen := e.includedKeys[key]; seen {
			return
		}
		e.includedKeys[key] = struct{}{}
	}
	e.included = append(e.included, item)
}

func (e *rawEnvelope) absorb(page *rawEnvelope) {
	e.data = append(e.data, page.data...)
	if page.hasIncluded {
		e.hasIncluded = true
		for _, item := range page.included {
			e.addIncluded(item)
		}
	}
}

// next returns the links.next URL, or an empty string when the page is the
// last one. An absent key and an explicit JSON null both mean "last page"; any
// other non-string value is malformed and reported, because silently treating
// it as the last page would return a truncated collection with a zero exit.
func (e *rawEnvelope) next() (string, error) {
	if e.links == nil {
		return "", nil
	}
	raw, ok := e.links["next"]
	if !ok {
		return "", nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var next string
	if err := json.Unmarshal(raw, &next); err != nil {
		return "", fmt.Errorf("links.next is not a string: %w", err)
	}
	return next, nil
}

func (e *rawEnvelope) marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	writeField := func(name string, value any) error {
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		encodedName, err := json.Marshal(name)
		if err != nil {
			return err
		}
		buf.Write(encodedName)
		buf.WriteByte(':')
		encodedValue, err := encodeRawJSON(value)
		if err != nil {
			return err
		}
		buf.Write(encodedValue)
		return nil
	}

	if err := writeField("data", e.data); err != nil {
		return nil, err
	}
	if e.hasIncluded {
		if err := writeField("included", e.included); err != nil {
			return nil, err
		}
	}
	if e.links != nil {
		links := make(map[string]json.RawMessage, len(e.links))
		for key, value := range e.links {
			if key != "next" {
				links[key] = value
			}
		}
		if err := writeField("links", links); err != nil {
			return nil, err
		}
	}
	keys := make([]string, 0, len(e.other))
	for key := range e.other {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writeField(key, e.other[key]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeRawJSON marshals value without HTML escaping so URLs and other
// pass-through content keep the bytes Apple returned.
func encodeRawJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
