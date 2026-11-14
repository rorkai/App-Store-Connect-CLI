package web

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	appCreateAccessFull    = "full"
	appCreateAccessLimited = "limited"
)

func normalizeAppCreateAccess(access string, userIDs []string) (string, []string, error) {
	access = strings.ToLower(strings.TrimSpace(access))
	for _, userID := range userIDs {
		if strings.TrimSpace(userID) == "" {
			return "", nil, shared.WithDiagnostic(
				shared.UsageError("--user must not be empty"),
				shared.DiagnosticInvalidInput,
				"--user",
			)
		}
	}
	userIDs = uniqueAppCreateUserIDs(userIDs)

	switch access {
	case "":
		if len(userIDs) > 0 {
			return "", nil, shared.WithDiagnostic(
				shared.UsageError("--user requires --access limited"),
				shared.DiagnosticConflictingInput,
				"--user",
			)
		}
		return "", nil, nil
	case appCreateAccessFull:
		if len(userIDs) > 0 {
			return "", nil, shared.WithDiagnostic(
				shared.UsageError("--user requires --access limited"),
				shared.DiagnosticConflictingInput,
				"--user",
			)
		}
		return appCreateAccessFull, nil, nil
	case appCreateAccessLimited:
		if len(userIDs) == 0 {
			return "", nil, shared.WithDiagnostic(
				shared.UsageError("--access limited requires at least one --user"),
				shared.DiagnosticRequiredInputMissing,
				"--user",
			)
		}
		return appCreateAccessLimited, userIDs, nil
	default:
		return "", nil, shared.WithDiagnostic(
			shared.UsageError("--access must be full or limited"),
			shared.DiagnosticInvalidInput,
			"--access",
		)
	}
}

func uniqueAppCreateUserIDs(userIDs []string) []string {
	if len(userIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(userIDs))
	out := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		out = append(out, userID)
	}
	return out
}

func ensureAppCreateUsersExist(ctx context.Context, client *asc.Client, userIDs []string) error {
	for _, userID := range userIDs {
		user, err := client.GetUser(ctx, userID)
		if err != nil {
			if asc.IsNotFound(err) {
				return shared.WithDiagnostic(
					shared.UsageError(fmt.Sprintf("unknown user ID %q", userID)),
					shared.DiagnosticInvalidInput,
					"--user",
				)
			}
			return fmt.Errorf("web apps create failed: lookup user %q: %w", userID, err)
		}
		if user.Data.Attributes.AllAppsVisible {
			return shared.WithDiagnostic(
				shared.UsageError(fmt.Sprintf("user ID %q has access to all apps; --access limited requires users with limited app visibility", userID)),
				shared.DiagnosticInvalidInput,
				"--user",
			)
		}
	}
	return nil
}

func ensureAppCreateAPIAccess(ctx context.Context, client *asc.Client) error {
	_, err := client.GetUsers(ctx, asc.Limit[asc.UsersOption](1))
	return err
}

func applyAndReadAppCreateAccess(ctx context.Context, client *asc.Client, appID, requestedAccess string, userIDs []string) (*asc.WebAppCreateResult, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("web apps create failed: created app id is missing")
	}
	if requestedAccess == appCreateAccessLimited {
		granted := make([]string, 0, len(userIDs))
		for _, userID := range userIDs {
			if err := client.AddUserVisibleApps(ctx, userID, []string{appID}); err != nil {
				grantErr := fmt.Errorf("web apps create failed: grant app access to user %q: %w", userID, err)
				rollbackUsers := append(append([]string{}, granted...), userID)
				if rollbackErr := rollbackAppCreateVisibleApps(ctx, client, appID, rollbackUsers); rollbackErr != nil {
					return nil, errors.Join(grantErr, rollbackErr)
				}
				return nil, grantErr
			}
			granted = append(granted, userID)
		}
	}

	access, users, err := readAppCreateAccess(ctx, client, appID)
	if err != nil {
		return nil, fmt.Errorf("web apps create failed: re-read app access: %w", err)
	}
	return &asc.WebAppCreateResult{
		ID:     appID,
		Access: access,
		Users:  users,
	}, nil
}

func rollbackAppCreateVisibleApps(ctx context.Context, client *asc.Client, appID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	rollbackCtx, cancel := shared.ContextWithTimeout(context.WithoutCancel(shared.ContextWithoutTimeout(ctx)))
	defer cancel()
	var rollbackErrs []error
	for _, userID := range userIDs {
		if err := client.RemoveUserVisibleApps(rollbackCtx, userID, []string{appID}); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove app access from user %q: %w", userID, err))
		}
	}
	if len(rollbackErrs) == 0 {
		return nil
	}
	return fmt.Errorf("web apps create failed: access grant partially applied; manual access repair may be required: %w", errors.Join(rollbackErrs...))
}

func readAppCreateAccess(ctx context.Context, client *asc.Client, appID string) (string, []string, error) {
	firstPage, err := client.GetUsers(ctx, asc.WithUsersVisibleAppIDs([]string{appID}), asc.Limit[asc.UsersOption](200))
	if err != nil {
		return "", nil, err
	}
	aggregated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetUsers(ctx, asc.NextURL[asc.UsersOption](nextURL))
	})
	if err != nil {
		return "", nil, err
	}
	users, ok := aggregated.(*asc.UsersResponse)
	if !ok || users == nil {
		return "", nil, fmt.Errorf("unexpected users response type %T", aggregated)
	}

	ids := make([]string, 0, len(users.Data))
	for _, item := range users.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return appCreateAccessFull, ids, nil
	}
	return appCreateAccessLimited, ids, nil
}
