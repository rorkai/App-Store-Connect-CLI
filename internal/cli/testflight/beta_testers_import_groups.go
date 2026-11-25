package testflight

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func validateBetaTesterImportRows(rows []betaTestersCSVRow, continueOnError bool) []betaTestersCSVRow {
	seen := make(map[string]int)
	for i := range rows {
		row := &rows[i]
		key := strings.ToLower(row.email)
		switch {
		case row.email == "":
			row.failure = "email is required"
		case !isValidTesterEmail(row.email):
			row.failure = "invalid email format"
		case seen[key] != 0:
			row.failure = fmt.Sprintf("duplicate email in input (already seen at row %d)", seen[key])
		default:
			seen[key] = i + 1
		}
		if row.failure != "" && !continueOnError {
			return rows[:i+1]
		}
	}
	return rows
}

func planBetaTesterImportGroups(resolver *betaGroupResolver, rows []betaTestersCSVRow, defaultGroup string, createMissing, continueOnError bool) ([]betaTestersCSVRow, []string, error) {
	missing := make(map[string]string)
	if err := collectMissingBetaGroups(resolver, []string{defaultGroup}, createMissing, missing); err != nil {
		return nil, nil, err
	}
	hasEligibleRows := false
	for i := range rows {
		row := &rows[i]
		if row.failure != "" {
			continue
		}
		if row.wholeGroup != "" {
			if _, err := resolver.Resolve(row.wholeGroup); !errors.Is(err, errBetaGroupNotFound) {
				row.groups = []string{row.wholeGroup}
			}
		}
		err := collectMissingBetaGroups(resolver, row.groups, createMissing, missing)
		if err == nil {
			hasEligibleRows = true
			continue
		}
		if !errors.Is(err, errBetaGroupNotFound) {
			if ambiguous, ok := errors.AsType[*shared.AmbiguousSelectionError](err); ok {
				ambiguous.Flag = ""
				ambiguous.Hint = "Use one of these group IDs in the CSV groups column instead of the name."
			}
			return nil, nil, err
		}
		row.failure = err.Error()
		if !continueOnError {
			rows = rows[:i+1]
			break
		}
	}
	if !hasEligibleRows {
		return rows, nil, nil
	}
	return rows, slices.Sorted(maps.Values(missing)), nil
}

func collectMissingBetaGroups(resolver *betaGroupResolver, names []string, createMissing bool, missing map[string]string) error {
	var firstMissing error
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, err := resolver.Resolve(name); err != nil {
			if !errors.Is(err, errBetaGroupNotFound) {
				return err
			}
			if !createMissing {
				if firstMissing == nil {
					firstMissing = err
				}
				continue
			}
			key := strings.ToLower(shared.NormalizeBetaGroupName(name))
			if _, ok := missing[key]; !ok {
				missing[key] = name
			}
		}
	}
	return firstMissing
}

func ensureBetaTesterImportGroup(ctx context.Context, d *shared.Deps, client *asc.Client, appID, name string) (string, error) {
	external, public := false, false
	requestCtx, cancel := d.Timeout(ctx)
	group, err := client.CreateBetaGroupWithAttributes(requestCtx, appID, asc.BetaGroupCreateAttributes{Name: name, IsInternalGroup: &external, PublicLinkEnabled: &public}, nil, nil)
	cancel()
	if err != nil {
		if isHTTPConflict(err) {
			refreshed, readErr := newBetaGroupResolver(ctx, d, client, appID)
			if readErr != nil {
				return "", fmt.Errorf("%w (failed to verify beta group: %w)", err, readErr)
			}
			if id, resolveErr := refreshed.Resolve(name); resolveErr == nil && !refreshed.internalByID[id] {
				return id, nil
			}
		}
		return "", err
	}
	if group == nil || strings.TrimSpace(group.Data.ID) == "" {
		return "", fmt.Errorf("created beta group %q returned no ID", name)
	}
	return group.Data.ID, nil
}

func (r *betaGroupResolver) addImportGroup(name, id string) {
	r.groups.Data = append(r.groups.Data, asc.Resource[asc.BetaGroupAttributes]{ID: id, Type: asc.ResourceTypeBetaGroups, Attributes: asc.BetaGroupAttributes{Name: name}})

	r.byID[id] = name
	r.idsByName[strings.ToLower(name)] = []string{id}
	r.uniqueNameByID[id] = true
}

func planBetaTesterAddGroups(resolver *betaGroupResolver, raw string) ([]string, []string, error) {
	raw = strings.TrimSpace(raw)
	if _, err := resolver.Resolve(raw); err == nil {
		return []string{raw}, nil, nil
	} else if !errors.Is(err, errBetaGroupNotFound) {
		return nil, nil, err
	}
	requested := strings.Split(raw, ",")
	for i := range requested {
		requested[i] = strings.TrimSpace(requested[i])
		if requested[i] == "" {
			return nil, nil, fmt.Errorf("empty group name in --group value %q", raw)
		}
	}
	missing := make(map[string]string)
	err := collectMissingBetaGroups(resolver, requested, true, missing)
	return requested, slices.Sorted(maps.Values(missing)), err
}
