# Testing Guidelines

The rules for which tests to write, and which not to, are in the `Tests` section of `AGENTS.md`. This file covers patterns.

## General Principles

- Mock external API calls (`httptest` or a stub transport); only the opt-in `make test-integration` suite calls App Store Connect.
- Write the failing test first, then implement.
- Prefer a few high-signal tests over broad repetitive matrices.

## Client Endpoint Coverage

Shared request, validation, and error-decoding code is tested once. Do not add a success, validation, and API-error test to every endpoint; test only what an endpoint does differently, such as its path, query, body, or response shape.

When consolidating repetitive client tests:
- Keep grouped/table-driven coverage for repeated request wiring
- Preserve at least one representative non-empty response assertion per response family
- Do not replace all list tests with `{"data":[]}` smoke checks; assert decoded fields for at least one realistic payload
- For user-facing renderers, assert output structure or headers in addition to value presence

## Test Patterns

### Table-Driven Tests

Group repeated limit, next-URL, and request-wiring cases in one table, keeping the representative decode assertions listed above.

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "foo", "bar", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Something(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Helper Functions

- Use `t.Helper()` in test helper functions
- For JSON assertions, unmarshal and assert fields (not `strings.Contains`)
- For renderer/output assertions, avoid token-only checks when structure matters; include header or formatting assertions for representative cases

### Assertions Outside the Test Goroutine

`t.Fatal`, `t.Fatalf`, and `t.FailNow` may only be called from the goroutine running the test. Inside an `httptest` handler they skip the response write and leave the client under test waiting; inside a worker goroutine or an injected dependency stub they terminate that goroutine before it can send its result, so the test deadlocks instead of failing and the real failure is buried in a timeout dump.

Use `internal/handlertest` instead. It records the failure with `Errorf`, which is safe from any goroutine, and returns a value the fixture hands back so the code under test unwinds normally:

```go
fixture := handlertest.New(t)

// RoundTrippers, dependency stubs, and worker goroutines return the error.
transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
    return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL)
})

// httptest handlers cannot return, so answer with a 500 and stop.
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
    fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
}))

// Fixture builders that must return a response answer with a 500 body.
func territoryResponse(fixture *handlertest.Asserter, payload any) *http.Response {
    body, err := json.Marshal(payload)
    if err != nil {
        return fixture.Response("marshal territory response: %v", err)
    }
    return jsonHTTPResponse(http.StatusOK, string(body))
}
```

`Errorf` renders with `fmt.Errorf`, so `%w` keeps wrapping the operand and still prints readably in the test log. Keep using `t.Fatal` for setup that runs on the test goroutine.

### CLI Tests

- Use `internal/cli/cmdtest` for command output and parsing tests
- Capture stderr for usage text (help output goes to stderr)

## Running Tests

```bash
ASC_BYPASS_KEYCHAIN=1 make test  # Run all tests
ASC_BYPASS_KEYCHAIN=1 go test -v ./...  # Verbose output
ASC_BYPASS_KEYCHAIN=1 go test -run TestName ./pkg  # Run specific test
```

Always set `ASC_BYPASS_KEYCHAIN=1` for manual test commands so host keychain profiles cannot affect results. The `make test` target also sets it internally.
