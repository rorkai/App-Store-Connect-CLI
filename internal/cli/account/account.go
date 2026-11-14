package account

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AccountCommand returns the account command group.
func AccountCommand() *ffcli.Command {
	fs := flag.NewFlagSet("account", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "account",
		ShortUsage: "asc account <subcommand> [flags]",
		ShortHelp:  "Inspect account-level health and access signals.",
		LongHelp: `Inspect account-level health and access signals.

Examples:
  asc account status
  asc account status --app "123456789"
  asc account status --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			accountStatusCommand(),
		},
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
	}
}

func accountStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("account status", flag.ExitOnError)

	appID := fs.String("app", "", "Optional app ID for access probe (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc account status [flags]",
		ShortHelp:  "Show account/workspace health checks.",
		LongHelp: `Show account/workspace health checks.

Checks currently include:
  - authentication health (local credential/config diagnostics)
  - API access probe (read-only)
  - account agreements availability (public API capability note)

Examples:
  asc account status
  asc account status --app "123456789"
  asc account status --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			resp, err := collectAccountStatus(ctx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("account status: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderAccountStatus(resp, false); return nil },
				func() error { renderAccountStatus(resp, true); return nil },
			)
		},
	}
}

type accountStatusResponse struct {
	Summary     accountSummary `json:"summary"`
	Checks      []accountCheck `json:"checks"`
	GeneratedAt string         `json:"generatedAt"`
}

type accountSummary struct {
	Health       string `json:"health"`
	NextAction   string `json:"nextAction"`
	ErrorCount   int    `json:"errorCount"`
	WarningCount int    `json:"warningCount"`
}

type accountCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func collectAccountStatus(ctx context.Context, appID string) (*accountStatusResponse, error) {
	checks := make([]accountCheck, 0, 3)
	checks = append(checks, authHealthCheck())

	apiCheck, err := apiAccessCheck(ctx, appID)
	if err != nil {
		return nil, err
	}
	checks = append(checks, apiCheck)

	checks = append(checks, agreementsAvailabilityCheck())

	summary := summarizeAccountChecks(checks)
	return &accountStatusResponse{
		Summary:     summary,
		Checks:      checks,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func authHealthCheck() accountCheck {
	report := authsvc.Doctor(authsvc.DoctorOptions{
		Profile:    shared.ResolveProfileName(),
		StrictAuth: shared.StrictAuthEnabled(),
	})
	switch {
	case report.Summary.Errors > 0:
		return accountCheck{
			Name:    "authentication",
			Status:  "fail",
			Message: fmt.Sprintf("auth doctor found %d error(s)", report.Summary.Errors),
		}
	case report.Summary.Warnings > 0:
		return accountCheck{
			Name:    "authentication",
			Status:  "warn",
			Message: fmt.Sprintf("auth doctor found %d warning(s)", report.Summary.Warnings),
		}
	default:
		return accountCheck{
			Name:    "authentication",
			Status:  "ok",
			Message: "credentials and local auth configuration look healthy",
		}
	}
}

func apiAccessCheck(ctx context.Context, appID string) (accountCheck, error) {
	client, err := shared.GetASCClient()
	if err != nil {
		return accountCheck{
			Name:    "api_access",
			Status:  "fail",
			Message: fmt.Sprintf("failed to initialize API client: %v", err),
		}, nil
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if strings.TrimSpace(appID) != "" {
		_, err = client.GetApp(requestCtx, appID)
		if err == nil {
			return accountCheck{
				Name:    "api_access",
				Status:  "ok",
				Message: fmt.Sprintf("able to read app %s", appID),
			}, nil
		}
		if isPermissionWarning(err) {
			return accountCheck{
				Name:    "api_access",
				Status:  "warn",
				Message: fmt.Sprintf("credentials are valid but do not have access to app %s", appID),
			}, nil
		}
		if errorsIsNotFound(err) {
			return accountCheck{
				Name:    "api_access",
				Status:  "warn",
				Message: fmt.Sprintf("app %s was not found", appID),
			}, nil
		}
		return accountCheck{
			Name:    "api_access",
			Status:  "fail",
			Message: fmt.Sprintf("app access probe failed: %v", err),
		}, nil
	}

	_, err = client.GetApps(requestCtx, asc.Limit[asc.AppsOption](1))
	if err == nil {
		return accountCheck{
			Name:    "api_access",
			Status:  "ok",
			Message: "able to read apps list",
		}, nil
	}
	if isPermissionWarning(err) {
		return accountCheck{
			Name:    "api_access",
			Status:  "warn",
			Message: "credentials are valid but do not have permission to list apps",
		}, nil
	}
	return accountCheck{
		Name:    "api_access",
		Status:  "fail",
		Message: fmt.Sprintf("apps list probe failed: %v", err),
	}, nil
}

func agreementsAvailabilityCheck() accountCheck {
	return accountCheck{
		Name:    "agreements",
		Status:  "unavailable",
		Message: "account-level agreement status is not exposed in the current public App Store Connect API surface",
	}
}

func summarizeAccountChecks(checks []accountCheck) accountSummary {
	summary := accountSummary{
		Health:     "green",
		NextAction: "No action needed.",
	}

	for _, check := range checks {
		switch check.Status {
		case "fail":
			summary.ErrorCount++
		case "warn", "unavailable":
			summary.WarningCount++
		}
	}

	if summary.ErrorCount > 0 {
		summary.Health = "red"
		summary.NextAction = firstCheckMessageByStatus(checks, "fail")
		return summary
	}
	if summary.WarningCount > 0 {
		summary.Health = "yellow"
		message := firstCheckMessageByStatus(checks, "warn")
		if message == "" {
			message = firstCheckMessageByStatus(checks, "unavailable")
		}
		if message != "" {
			summary.NextAction = message
		}
	}

	return summary
}

func firstCheckMessageByStatus(checks []accountCheck, status string) string {
	for _, check := range checks {
		if check.Status == status {
			return check.Message
		}
	}
	return ""
}

func isPermissionWarning(err error) bool {
	return errors.Is(err, asc.ErrForbidden)
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, asc.ErrNotFound) || asc.IsNotFound(err)
}

func renderAccountStatus(resp *accountStatusResponse, markdown bool) {
	summaryRows := [][]string{
		{"health", resp.Summary.Health},
		{"nextAction", resp.Summary.NextAction},
		{"errorCount", strconv.Itoa(resp.Summary.ErrorCount)},
		{"warningCount", strconv.Itoa(resp.Summary.WarningCount)},
		{"generatedAt", resp.GeneratedAt},
	}
	shared.RenderSection("Summary", []string{"field", "value"}, summaryRows, markdown)

	checkRows := make([][]string, 0, len(resp.Checks))
	for _, check := range resp.Checks {
		checkRows = append(checkRows, []string{check.Name, check.Status, check.Message})
	}
	shared.RenderSection("Checks", []string{"check", "status", "message"}, checkRows, markdown)
}
