package shared

import (
	"net/http"
	"testing"
)

type countingTransport struct{ requests int }

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.requests++
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: req}, nil
}

func TestDefaultDepsHTTPTransportMatchesProcess(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	built, replaced := &countingTransport{}, &countingTransport{}
	http.DefaultTransport = built
	d := DefaultDeps()

	for _, test := range []struct {
		name    string
		process *countingTransport
	}{
		{name: "transport when the deps were built", process: built},
		{name: "transport replaced after the deps were built", process: replaced},
	} {
		t.Run(test.name, func(t *testing.T) {
			http.DefaultTransport = test.process
			builtBefore, replacedBefore := built.requests, replaced.requests
			for _, client := range []*http.Client{{}, {Transport: d.HTTPTransport()}} {
				resp, err := client.Get("https://example.invalid/")
				if err != nil {
					t.Fatalf("request: %v", err)
				}
				_ = resp.Body.Close()
			}
			got := [2]int{built.requests - builtBefore, replaced.requests - replacedBefore}
			want := [2]int{2, 0}
			if test.process == replaced {
				want = [2]int{0, 2}
			}
			if got != want {
				t.Fatalf("requests on the transport when built, replaced = %v, want %v (a default client and a client on d.HTTPTransport() both use the process transport of the moment)", got, want)
			}
		})
	}
}
