package testflight

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func validateBetaTesterImportRows(d *shared.Deps, rows []betaTestersCSVRow) error {
	if len(rows) == 0 {
		return d.UsageError("CSV must contain at least one tester")
	}
	seen := map[string]int{}
	for i, row := range rows {
		email := strings.TrimSpace(row.email)
		if !isValidTesterEmail(email) {
			return d.UsageErrorf("CSV row %d: invalid email format", i+1)
		}
		key := strings.ToLower(email)
		if first, ok := seen[key]; ok {
			return d.UsageErrorf("CSV row %d: duplicate email in input (already seen at row %d)", i+1, first)
		}
		seen[key] = i + 1
	}
	return nil
}

func planBetaTesterImportGroups(resolver *betaGroupResolver, rows []betaTestersCSVRow, flagGroups []string, createMissing bool) ([]string, error) {
	requested := append([]string(nil), flagGroups...)
	for _, row := range rows {
		requested = append(requested, row.groups...)
	}
	missing := map[string]string{}
	for index, name := range requested {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := resolver.Resolve(name); err != nil {
			if ambiguous, ok := errors.AsType[*shared.AmbiguousSelectionError](err); ok && index >= len(flagGroups) {
				ambiguous.Flag = ""
				ambiguous.Hint = "Use one of these group IDs in the CSV groups column instead of the name."
			}
			if !createMissing || !errors.Is(err, errBetaGroupNotFound) {
				return nil, err
			}
			key := strings.ToLower(shared.NormalizeBetaGroupName(name))
			if _, ok := missing[key]; !ok {
				missing[key] = name
			}
		}
	}
	names := make([]string, 0, len(missing))
	for _, name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
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
	missing, err := planBetaTesterImportGroups(resolver, nil, requested, true)
	return requested, missing, err
}
