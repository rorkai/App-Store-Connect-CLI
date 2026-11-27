//go:build !unix

package web

import "testing"

func makeWebReviewBodyFIFO(t *testing.T, path string) {
	t.Helper()
	t.Skip("FIFOs are not available on this platform")
}
