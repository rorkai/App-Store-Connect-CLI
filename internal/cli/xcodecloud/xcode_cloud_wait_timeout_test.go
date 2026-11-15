package xcodecloud

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestXcodeCloudWaitsKeepContextCause(t *testing.T) {
	waits := map[string]func(context.Context) error{
		"status": func(ctx context.Context) error {
			return waitForBuildCompletion(ctx, &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}, nil, "run-1", time.Millisecond, "json", false)
		},
		"doctor": func(ctx context.Context) error {
			_, err := waitForBuildRunForDoctor(ctx, nil, "run-1", time.Millisecond)
			return err
		},
	}
	for name, wait := range waits {
		t.Run(name+" deadline", func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			err := wait(ctx)
			if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "--timeout") {
				t.Fatalf("error = %v, want wrapped deadline that names --timeout", err)
			}
		})
		t.Run(name+" canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := wait(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want wrapped cancellation", err)
			}
		})
	}
}
