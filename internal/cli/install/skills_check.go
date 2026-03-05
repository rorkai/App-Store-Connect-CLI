package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const (
	skillsAutoCheckEnvVar  = "ASC_SKILLS_AUTO_CHECK"
	skillsCheckInterval    = 24 * time.Hour
	skillsCheckTimeout     = 8 * time.Second
	skillsCheckedAtLayout  = time.RFC3339
	skillsUpdateMessageFmt = "skills updates may be available. Run 'npx skills update' to refresh installed skills."
)

var (
	loadConfigForSkillsCheck = config.Load
	saveConfigForSkillsCheck = config.Save
	nowForSkillsCheck        = time.Now
	runSkillsCheckCommand    = defaultRunSkillsCheckCommand
	progressEnabledForCheck  = shared.ProgressEnabled
)

// MaybeCheckForSkillUpdates checks for skill updates once per interval and prints
// a non-blocking stderr notice when updates appear available.
func MaybeCheckForSkillUpdates(ctx context.Context) {
	if !skillsAutoCheckEnabled(strings.TrimSpace(os.Getenv(skillsAutoCheckEnvVar))) {
		return
	}
	if os.Getenv("CI") != "" {
		return
	}
	if !progressEnabledForCheck() {
		return
	}

	cfg, err := loadConfigForSkillsCheck()
	if err != nil {
		// Keep command execution unaffected when config is absent or unreadable.
		return
	}
	if cfg == nil {
		return
	}

	now := nowForSkillsCheck().UTC()
	if !shouldRunSkillsCheck(now, cfg.SkillsCheckedAt) {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, skillsCheckTimeout)
	defer cancel()

	output, runErr := runSkillsCheckCommand(checkCtx)

	// Persist the check timestamp regardless of check result to avoid repeated
	// invocation overhead during failures.
	cfg.SkillsCheckedAt = now.Format(skillsCheckedAtLayout)
	_ = saveConfigForSkillsCheck(cfg)

	if runErr != nil {
		return
	}
	if !skillsOutputHasUpdates(output) {
		return
	}

	fmt.Fprintln(os.Stderr, skillsUpdateMessageFmt)
}

func shouldRunSkillsCheck(now time.Time, lastCheckedAt string) bool {
	lastCheckedAt = strings.TrimSpace(lastCheckedAt)
	if lastCheckedAt == "" {
		return true
	}

	lastChecked, err := time.Parse(skillsCheckedAtLayout, lastCheckedAt)
	if err != nil {
		return true
	}
	return now.Sub(lastChecked.UTC()) >= skillsCheckInterval
}

func skillsAutoCheckEnabled(value string) bool {
	if value == "" {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return false
	}
}

func skillsOutputHasUpdates(output string) bool {
	normalized := strings.ToLower(strings.TrimSpace(output))
	if normalized == "" {
		return false
	}

	switch {
	case strings.Contains(normalized, "all skills are up to date"):
		return false
	case strings.Contains(normalized, "no updates available"), strings.Contains(normalized, "no update available"):
		return false
	case strings.Contains(normalized, "update available"):
		return true
	case strings.Contains(normalized, "updates available"):
		return true
	default:
		return false
	}
}

func defaultRunSkillsCheckCommand(ctx context.Context) (string, error) {
	npxPath, err := lookupNpx("npx")
	if err != nil {
		return "", nil
	}

	cmd := exec.CommandContext(ctx, npxPath, "--yes", "skills", "check")
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Run(); err != nil {
		return combined.String(), err
	}
	return combined.String(), nil
}
