package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// bearerTokenCommandTimeout bounds one ASC_BEARER_TOKEN_COMMAND run, which
// usually asks a remote service to sign a token.
const bearerTokenCommandTimeout = 60 * time.Second

// bearerTokenCommandPipeWaitDelay bounds the wait for the command's stdout to
// close after the shell exits or is killed. Cancelling kills only the shell, so
// in a pipeline such as `signer | jq -r .token` the other processes could keep
// stdout open and hold asc indefinitely.
const bearerTokenCommandPipeWaitDelay = 250 * time.Millisecond

// readBearerTokenFromCommand runs ASC_BEARER_TOKEN_COMMAND and returns the
// token it prints. The command's stderr reaches the user; its stdout, the
// token, never does.
func readBearerTokenFromCommand(ctx context.Context, command string) (string, error) {
	// Keep the caller's deadline: a renewal inside a request must finish while
	// that request can still be sent.
	commandCtx, cancel := ContextWithResolvedTimeout(ctx, bearerTokenCommandTimeout)
	defer cancel()
	cmd := ShellCommand(commandCtx, command)
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = bearerTokenCommandPipeWaitDelay
	output, err := cmd.Output()
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		err = nil
	}
	if err != nil {
		if ctxErr := commandCtx.Err(); ctxErr != nil {
			return "", fmt.Errorf("%s interrupted: %w", bearerTokenCommandEnv, ctxErr)
		}
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("%s exited with status %d", bearerTokenCommandEnv, exitErr.ExitCode())
		}
		return "", fmt.Errorf("%s failed: %w", bearerTokenCommandEnv, err)
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("%s printed no token", bearerTokenCommandEnv)
	}
	return token, nil
}
