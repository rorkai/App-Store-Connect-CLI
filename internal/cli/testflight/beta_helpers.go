package testflight

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var errBetaTesterNotFound = errors.New("beta tester not found")

// usageErrorFromValidation turns a validation failure raised by a parsing
// helper into a usage error so the command exits 2, preserving both the
// diagnostic the helper attached and the command-path rewrites the command
// tree installed on ctx.
func usageErrorFromValidation(ctx context.Context, format string, err error) error {
	usageErr := shared.UsageErrorfCtx(ctx, format, err)
	if diagnostic, ok := shared.DiagnosticFromError(err); ok {
		return shared.WithDiagnostic(usageErr, diagnostic.Code, diagnostic.Parameter)
	}
	return usageErr
}

// parseCreateRelationshipIDs reads a create command's relationship flag. A
// provided flag with no IDs is a usage error, so it is never dropped silently.
func parseCreateRelationshipIDs(value *shared.OnceCSVValue, flagName string) ([]string, error) {
	ids := shared.SplitUniqueCSV(value.String())
	if value.Provided() && len(ids) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %s must include at least one ID\n", flagName)
		return nil, shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, flagName)
	}
	return ids, nil
}

func resolveBetaGroupID(ctx context.Context, client *asc.Client, appID, group string) (string, error) {
	ids, err := resolveBetaGroupIDs(ctx, client, appID, group)
	if err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("expected a single beta group, got %d from %q", len(ids), group)
	}
	return ids[0], nil
}

// resolveBetaGroupIDs resolves a raw --group value against a single beta-groups
// fetch. A value that exactly matches one group ID or name is used as-is, even
// when it contains commas; otherwise the value is treated as a comma-separated
// list of group names or IDs.
func resolveBetaGroupIDs(ctx context.Context, client *asc.Client, appID, rawGroups string) ([]string, error) {
	if strings.TrimSpace(rawGroups) == "" {
		return nil, fmt.Errorf("beta group name is required")
	}

	groups, err := client.GetBetaGroups(ctx, appID, asc.Limit[asc.BetaGroupsOption](200))
	if err != nil {
		return nil, err
	}
	for groups.Links.Next != "" {
		page, err := client.GetBetaGroups(ctx, appID, asc.NextURL[asc.BetaGroupsOption](groups.Links.Next))
		if err != nil {
			return nil, err
		}
		groups.Data = append(groups.Data, page.Data...)
		groups.Links.Next = page.Links.Next
	}

	id, err := matchBetaGroupID(groups, strings.TrimSpace(rawGroups))
	if err == nil {
		return []string{id}, nil
	}
	if !errors.Is(err, errBetaGroupNotFound) {
		return nil, err
	}

	tokens := strings.Split(rawGroups, ",")
	if len(tokens) == 1 {
		return nil, err
	}

	ids := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("empty group name in --group value %q", rawGroups)
		}
		id, err := matchBetaGroupID(groups, token)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

var errBetaGroupNotFound = errors.New("not found")

func matchBetaGroupID(groups *asc.BetaGroupsResponse, group string) (string, error) {
	for _, item := range groups.Data {
		if item.ID == group {
			return item.ID, nil
		}
	}

	matches := betaGroupNameMatches(groups, group, strings.TrimSpace)
	if len(matches) == 0 {
		matches = betaGroupNameMatches(groups, group, shared.NormalizeBetaGroupName)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("beta group %q %w", group, errBetaGroupNotFound)
	case 1:
		return matches[0].ID, nil
	default:
		return "", shared.AmbiguousError("beta group", "--group", group, matches)
	}
}

func betaGroupNameMatches(groups *asc.BetaGroupsResponse, group string, normalize func(string) string) []shared.AmbiguousCandidate {
	name := normalize(group)
	matches := make([]shared.AmbiguousCandidate, 0, 1)
	for _, item := range groups.Data {
		if strings.EqualFold(normalize(item.Attributes.Name), name) {
			kind := "external"
			if item.Attributes.IsInternalGroup {
				kind = "internal"
			}
			matches = append(matches, shared.AmbiguousCandidate{ID: strings.TrimSpace(item.ID), Label: strings.TrimSpace(item.Attributes.Name), Extra: kind})
		}
	}
	return matches
}

func findBetaTesterIDByEmail(ctx context.Context, client *asc.Client, appID, email string) (string, error) {
	testers, err := client.GetBetaTesters(ctx, appID, asc.WithBetaTestersEmail(email))
	if err != nil {
		return "", err
	}
	if testers == nil {
		return "", fmt.Errorf("empty beta testers response")
	}

	pageHasNext := strings.TrimSpace(testers.Links.Next) != ""
	if len(testers.Data) == 0 && !pageHasNext {
		return "", errBetaTesterNotFound
	}
	if len(testers.Data) > 1 || pageHasNext {
		ambiguous := &shared.AmbiguousSelectionError{
			Kind:        "beta tester",
			Description: fmt.Sprintf("email %q", strings.TrimSpace(email)),
			Candidates:  shared.BetaTesterCandidates(testers.Data),
			Hint:        "This command selects testers by --email only; inspect the duplicates with `asc testflight testers view --id <ID>`.",
		}
		if pageHasNext {
			return "", shared.MarkAmbiguousSelectionSample(ambiguous)
		}
		return "", ambiguous
	}

	return testers.Data[0].ID, nil
}
