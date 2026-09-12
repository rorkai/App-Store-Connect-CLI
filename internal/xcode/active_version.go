package xcode

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const activeXcodeVersionDiagnosticLimit = 4 * 1024

var activeXcodeVersionPattern = regexp.MustCompile(`(?m)^Xcode[\t ]+([0-9]+)(?:[.][0-9]+)*(?:[\t ].*)?$`)

// ActiveXcodeMajorVersion returns the major version reported by the active
// xcodebuild selected through DEVELOPER_DIR or xcode-select.
func ActiveXcodeMajorVersion(ctx context.Context) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	cmd, err := trustedXcodeCommand(ctx, "xcodebuild", []string{"-version"}, nil)
	if err != nil {
		return 0, err
	}
	var stdout bytes.Buffer
	stderr := newTailBuffer(activeXcodeVersionDiagnosticLimit)
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	if err := runXcodeCommand(cmd); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return 0, fmt.Errorf("xcodebuild -version failed: %s: %w", detail, err)
		}
		return 0, fmt.Errorf("xcodebuild -version failed: %w", err)
	}

	return parseActiveXcodeMajorVersion(stdout.String())
}

func parseActiveXcodeMajorVersion(output string) (int, error) {
	match := activeXcodeVersionPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		detail := strings.TrimSpace(output)
		if detail == "" {
			detail = "empty output"
		} else {
			detail = truncateUTF8Prefix(detail, 256)
		}
		return 0, fmt.Errorf("parse active Xcode version: unexpected xcodebuild -version output: %q", detail)
	}

	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("parse active Xcode major version %q: %w", match[1], err)
	}
	if major < 1 {
		return 0, fmt.Errorf("parse active Xcode major version %q", match[1])
	}
	return major, nil
}
