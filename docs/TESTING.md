# Testing Guidelines

## General Principles

- Write tests for all exported functions
- Use table-driven tests when testing multiple cases
- Mock external API calls
- Test error cases, not just happy paths
- Prefer test-driven development (write tests first, then implement)
- Prefer a small number of high-signal tests over broad repetitive matrices

## Coverage Requirements

For each client endpoint, cover:
1. Success path
2. Validation errors
3. API error responses

When consolidating repetitive client tests:
- Keep grouped/table-driven coverage for repeated request wiring
- Preserve at least one representative non-empty response assertion per response family
- Do not replace all list tests with `{"data":[]}` smoke checks; assert decoded fields for at least one realistic payload
- For user-facing renderers, assert output structure or headers in addition to value presence

## Test Patterns

### Table-Driven Tests

Table-driven tests are preferred for repetitive endpoint coverage, but they should not weaken the regression signal. Group repeated limit/next-url/request-wiring cases together, then keep one focused representative assertion that proves the endpoint still decodes the correct response shape.

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

### CLI Tests

- Add CLI-level tests for command output/parsing
- Tests should capture stderr for usage text (help output goes to stderr)

## Running Tests

```bash
ASC_BYPASS_KEYCHAIN=1 make test  # Run all tests
ASC_BYPASS_KEYCHAIN=1 go test -v ./...  # Verbose output
ASC_BYPASS_KEYCHAIN=1 go test -run TestName ./pkg  # Run specific test
```

Always set `ASC_BYPASS_KEYCHAIN=1` for manual test commands so host keychain profiles cannot affect results. The `make test` target also sets it internally.
