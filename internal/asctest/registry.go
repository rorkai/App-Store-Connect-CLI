// Package asctest stubs App Store Connect HTTP traffic in tests.
//
// A Registry is an http.RoundTripper that answers each request with the first
// registered stub whose Matcher accepts it. Each stub answers once, so a
// paginated sequence is a list of Register calls. A request no stub accepts
// fails the test through handlertest and returns an error to the client under
// test; a stub that never answered fails the test when it ends.
package asctest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

const apiHost = "api.appstoreconnect.apple.com"

var defaultTransportMu sync.Mutex

// SwapDefaultTransport replaces http.DefaultTransport with rt until the test
// ends. The swap holds a package lock, so tests that use it never share the
// global transport.
func SwapDefaultTransport(t testing.TB, rt http.RoundTripper) {
	t.Helper()

	defaultTransportMu.Lock()
	original := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() {
		http.DefaultTransport = original
		defaultTransportMu.Unlock()
	})
}

// Matcher selects the requests a stub answers.
type Matcher struct {
	method string
	path   string
	query  url.Values
	body   func([]byte) bool
	desc   string
}

// GET matches a GET request to path on the App Store Connect API host. Each
// query argument is one unescaped key=value pair; the request must carry
// exactly these pairs and no others.
func GET(path string, query ...string) Matcher {
	return newMatcher(http.MethodGet, path, query)
}

// POST matches a POST request the way GET matches a GET request.
func POST(path string, query ...string) Matcher {
	return newMatcher(http.MethodPost, path, query)
}

// PATCH matches a PATCH request the way GET matches a GET request.
func PATCH(path string, query ...string) Matcher {
	return newMatcher(http.MethodPatch, path, query)
}

// DELETE matches a DELETE request the way GET matches a GET request.
func DELETE(path string, query ...string) Matcher {
	return newMatcher(http.MethodDelete, path, query)
}

func newMatcher(method, path string, query []string) Matcher {
	rawQuery := strings.Join(query, "&")
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		panic(fmt.Sprintf("asctest: invalid query %q: %v", rawQuery, err))
	}
	desc := method + " " + path
	if rawQuery != "" {
		desc += "?" + rawQuery
	}
	return Matcher{method: method, path: path, query: values, desc: desc}
}

// JSONBody also requires the request body to be JSON equal to want, ignoring
// formatting and key order.
func (m Matcher) JSONBody(want string) Matcher {
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		panic(fmt.Sprintf("asctest: invalid JSON body %q: %v", want, err))
	}
	m.body = func(got []byte) bool {
		var gotValue any
		return json.Unmarshal(got, &gotValue) == nil && reflect.DeepEqual(gotValue, wantValue)
	}
	m.desc += " body " + want
	return m
}

func (m Matcher) String() string { return m.desc }

func (m Matcher) matches(req *http.Request, body []byte) bool {
	return req.Method == m.method &&
		req.URL.Scheme == "https" &&
		req.URL.Host == apiHost &&
		req.URL.EscapedPath() == m.path &&
		reflect.DeepEqual(req.URL.Query(), m.query) &&
		(m.body == nil || m.body(body))
}

// Responder builds the response for a matched request. The request body has
// already been read; Registry.Requests holds it.
type Responder func(*http.Request) (*http.Response, error)

// JSON responds with status and a JSON body.
func JSON(status int, body string) Responder {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
}

// Request is one request the registry received, with its body read.
type Request struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   []byte
}

type stub struct {
	matcher   Matcher
	responder Responder
	matched   bool
}

// Registry is an http.RoundTripper that answers requests from registered stubs.
type Registry struct {
	t        testing.TB
	fixture  *handlertest.Asserter
	mu       sync.Mutex
	stubs    []*stub
	requests []Request
}

// NewRegistry returns an empty registry that fails t at cleanup if any
// registered stub was never matched.
func NewRegistry(t testing.TB) *Registry {
	t.Helper()

	r := &Registry{t: t, fixture: handlertest.New(t)}
	t.Cleanup(r.verify)
	return r
}

// Install makes the registry http.DefaultTransport until the test ends.
func (r *Registry) Install() {
	r.t.Helper()
	SwapDefaultTransport(r.t, r)
}

// Register adds a stub that answers the first unanswered request m matches.
func (r *Registry) Register(m Matcher, resp Responder) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stubs = append(r.stubs, &stub{matcher: m, responder: resp})
}

// Requests returns every request received so far, in order.
func (r *Registry) Requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]Request(nil), r.requests...)
}

// RoundTrip implements http.RoundTripper.
func (r *Registry) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, r.fixture.Errorf("asctest: read body of %s %s: %w", req.Method, req.URL, err)
		}
	}

	r.mu.Lock()
	reqURL := *req.URL
	r.requests = append(r.requests, Request{Method: req.Method, URL: &reqURL, Header: req.Header.Clone(), Body: body})
	var found *stub
	for _, s := range r.stubs {
		if !s.matched && s.matcher.matches(req, body) {
			found = s
			break
		}
	}
	if found == nil {
		pending := r.pendingLocked()
		r.mu.Unlock()
		return nil, r.fixture.Errorf("asctest: unexpected request %s %s body %q; unanswered stubs: [%s]", req.Method, req.URL, body, strings.Join(pending, "; "))
	}
	found.matched = true
	r.mu.Unlock()

	return found.responder(req)
}

func (r *Registry) pendingLocked() []string {
	var pending []string
	for _, s := range r.stubs {
		if !s.matched {
			pending = append(pending, s.matcher.String())
		}
	}
	return pending
}

func (r *Registry) verify() {
	r.t.Helper()

	r.mu.Lock()
	pending := r.pendingLocked()
	r.mu.Unlock()
	if len(pending) > 0 {
		r.t.Errorf("asctest: stubs never matched: [%s]", strings.Join(pending, "; "))
	}
}
