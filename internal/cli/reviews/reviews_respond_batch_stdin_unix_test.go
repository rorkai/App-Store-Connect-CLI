//go:build darwin || linux

package reviews

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestReviewBatchPreservesRedirectedRegularFile(t *testing.T) {
	path := writeReviewBatchTestFile(t, []byte(`{"replies":[{"response":"Thanks","reviewIds":["review-1"]}]}`))
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	d, _, stderr := newReviewTestDeps(nil)
	targets, err := loadReviewBatchTargets(d, fmt.Sprintf("/dev/fd/%d", file.Fd()))
	if err != nil || len(targets) != 1 || targets[0].ReviewID != "review-1" {
		t.Fatalf("targets=%+v error=%v", targets, err)
	}
	if !strings.Contains(stderr.String(), "Warning: --file non-regular or descriptor input is deprecated") {
		t.Fatalf("missing deprecation warning: %q", stderr.String())
	}
}
