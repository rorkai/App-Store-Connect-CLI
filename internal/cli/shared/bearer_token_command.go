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

// readBearerTokenFromCommand runs ASC_BEARER_TOKEN_COMMAND and returns the
// token it prints. The command's stderr reaches the user; its stdout, the
// token, never does.
func readBearerTokenFromCommand(ctx context.Context, command string) (string, error) {
	commandCtx, cancel := ContextWithResolvedTimeout(ContextWithoutTimeout(ctx), bearerTokenCommandTimeout)
	defer cancel()
	cmd := ShellCommand(commandCtx, command)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
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
