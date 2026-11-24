package testflight

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// betaTesterAddConflictOutcome is what the read-back after a 409 on
// POST /v1/betaTesters found. Apple answers "Tester(s) cannot be assigned"
// both when the email is already a tester in a requested group and when the
// tester cannot join a group at all, so only the read-back can tell them apart.
type betaTesterAddConflictOutcome struct {
	tester        *asc.BetaTesterResponse
	addedGroupIDs []string
	addedBuildIDs []string
	// internalGroups names the requested internal groups when the email is not
	// a tester of the app yet.
	internalGroups []string
}

// resolveBetaTesterAddConflict reads the selected apps' exact-email tester after a
// 409 on create. When one exists, it adds whatever requested groups and builds
// the tester is still missing, so the end state is the one the caller asked
// for. It returns a nil tester when no tester with this email exists, so the
// caller keeps the original conflict.
func resolveBetaTesterAddConflict(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appIDs []string,
	email string,
	groupIDs, buildIDs []string,
) (*betaTesterAddConflictOutcome, error) {
	var fields []string
	if len(groupIDs) == 0 {
		fields = []string{"email"}
	}
	testers, err := fetchExistingTestersByEmail(ctx, d, client, appIDs, []betaTestersCSVRow{{email: email}}, fields...)
	if err != nil {
		return nil, fmt.Errorf("look up tester %q: %w", email, err)
	}
	tester := testers[strings.ToLower(strings.TrimSpace(email))]
	if tester == nil {
		internal, err := internalBetaGroupNames(ctx, d, client, groupIDs)
		if err != nil {
			return nil, err
		}
		return &betaTesterAddConflictOutcome{internalGroups: internal}, nil
	}
	return reconcileBetaTesterRelationships(ctx, d, client, tester, groupIDs, buildIDs)
}

func reconcileBetaTesterRelationships(ctx context.Context, d *shared.Deps, client *asc.Client, tester *asc.BetaTesterResponse, groupIDs, buildIDs []string) (*betaTesterAddConflictOutcome, error) {
	testerID := strings.TrimSpace(tester.Data.ID)

	missingGroups, err := missingBetaTesterLinkages(ctx, d, groupIDs, func(ctx context.Context, opts ...asc.LinkagesOption) (*asc.LinkagesResponse, error) {
		return client.GetBetaTesterBetaGroupsRelationships(ctx, testerID, opts...)
	})
	if err != nil {
		return nil, fmt.Errorf("read groups of tester %s: %w", testerID, err)
	}
	var missingBuilds []string
	for _, buildID := range uniqueSortedStrings(buildIDs) {
		missing, err := missingBetaTesterLinkages(ctx, d, []string{testerID}, func(ctx context.Context, opts ...asc.LinkagesOption) (*asc.LinkagesResponse, error) {
			response, err := client.GetBuildIndividualTestersRelationships(ctx, buildID, opts...)
			if err != nil {
				return nil, err
			}
			if next := response.Links.Next; next != "" {
				parsed, err := url.Parse(next)
				if err != nil || parsed.Path != "/v1/builds/"+buildID+"/relationships/individualTesters" {
					return nil, fmt.Errorf("tester membership pagination changed the requested build")
				}
			}
			return response, nil
		})
		if err != nil {
			return nil, fmt.Errorf("read testers of build %s: %w", buildID, err)
		}
		if len(missing) > 0 {
			missingBuilds = append(missingBuilds, buildID)
		}
	}

	if len(missingGroups) > 0 {
		requestCtx, cancel := d.Timeout(ctx)
		err := client.AddBetaTesterToGroups(requestCtx, testerID, missingGroups)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("tester %s exists; adding it to group(s) %s failed: %w", testerID, strings.Join(missingGroups, ", "), err)
		}
	}
	if len(missingBuilds) > 0 {
		requestCtx, cancel := d.Timeout(ctx)
		err := client.AddBuildsToBetaTester(requestCtx, testerID, missingBuilds)
		cancel()
		if err != nil {
			added := ""
			if len(missingGroups) > 0 {
				added = fmt.Sprintf(" added it to group(s) %s, but", strings.Join(missingGroups, ", "))
			}
			return nil, fmt.Errorf("tester %s exists;%s adding build(s) %s failed: %w", testerID, added, strings.Join(missingBuilds, ", "), err)
		}
	}

	return &betaTesterAddConflictOutcome{
		tester:        tester,
		addedGroupIDs: missingGroups,
		addedBuildIDs: missingBuilds,
	}, nil
}

// missingBetaTesterLinkages returns the requested IDs the tester's
// relationship does not hold yet, reading every page.
func missingBetaTesterLinkages(
	ctx context.Context,
	d *shared.Deps,
	requested []string,
	read func(context.Context, ...asc.LinkagesOption) (*asc.LinkagesResponse, error),
) ([]string, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	requestCtx, cancel := d.Timeout(ctx)
	first, err := read(requestCtx, asc.Limit[asc.LinkagesOption](200))
	cancel()
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		return read(requestCtx, asc.NextURL[asc.LinkagesOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	linkages, ok := all.(*asc.LinkagesResponse)
	if !ok || linkages == nil {
		return nil, fmt.Errorf("unexpected relationship response type")
	}
	present := make(map[string]struct{}, len(linkages.Data))
	for _, linkage := range linkages.Data {
		present[strings.TrimSpace(linkage.ID)] = struct{}{}
	}
	var missing []string
	for _, id := range uniqueSortedStrings(requested) {
		if _, ok := present[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// internalBetaGroupNames returns the names of the requested groups that are
// internal. Internal groups only take users of the App Store Connect team, so
// an email that is not a team user cannot join one.
func internalBetaGroupNames(ctx context.Context, d *shared.Deps, client *asc.Client, groupIDs []string) ([]string, error) {
	var names []string
	for _, groupID := range groupIDs {
		requestCtx, cancel := d.Timeout(ctx)
		group, err := client.GetBetaGroup(requestCtx, groupID)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("read group %s: %w", groupID, err)
		}
		if group.Data.Attributes.IsInternalGroup {
			names = append(names, fmt.Sprintf("%q (%s)", strings.TrimSpace(group.Data.Attributes.Name), groupID))
		}
	}
	return names, nil
}

// reportBetaTesterAddConflict writes what the read-back changed to stderr.
func reportBetaTesterAddConflict(d *shared.Deps, email string, outcome *betaTesterAddConflictOutcome) {
	testerID := outcome.tester.Data.ID
	if len(outcome.addedGroupIDs) == 0 && len(outcome.addedBuildIDs) == 0 {
		fmt.Fprintf(d.IO.ErrOut, "testers add: %s is already tester %s in every requested group and build; left unchanged\n", email, testerID)
		return
	}
	var added []string
	if len(outcome.addedGroupIDs) > 0 {
		added = append(added, "group(s) "+strings.Join(outcome.addedGroupIDs, ", "))
	}
	if len(outcome.addedBuildIDs) > 0 {
		added = append(added, "build(s) "+strings.Join(outcome.addedBuildIDs, ", "))
	}
	fmt.Fprintf(d.IO.ErrOut, "testers add: %s is already tester %s; added it to %s\n", email, testerID, strings.Join(added, " and "))
}
