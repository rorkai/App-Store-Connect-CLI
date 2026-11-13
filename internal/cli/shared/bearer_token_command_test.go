package shared

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A renewal inside a request must end with that request's timeout; a token
// that arrives after it is useless because the request can no longer start.
func TestReadBearerTokenFromCommandKeepsTheRequestDeadline(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")
	ctx, cancel := ContextWithResolvedTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	if token, err := readBearerTokenFromCommand(ctx, "echo token"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readBearerTokenFromCommand() = %q, %v; want the request's context.DeadlineExceeded", token, err)
	}
}
