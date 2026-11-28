package asc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// BetaTesterInvitationResult represents CLI output for invitations.
type BetaTesterInvitationResult struct {
	InvitationID string `json:"invitationId"`
	TesterID     string `json:"testerId,omitempty"`
	AppID        string `json:"appId,omitempty"`
	Email        string `json:"email,omitempty"`
}

// BetaTesterDeleteResult represents CLI output for deletions.
type BetaTesterDeleteResult struct {
	ID      string `json:"id"`
	Email   string `json:"email,omitempty"`
	Deleted bool   `json:"deleted"`
}

// BetaTesterGroupsUpdateResult represents CLI output for beta tester group updates.
type BetaTesterGroupsUpdateResult struct {
	TesterID string   `json:"testerId"`
	GroupIDs []string `json:"groupIds"`
	Action   string   `json:"action"`
}

// Actions reported by the beta group tester add and remove receipts.
const (
	// BetaGroupTestersActionAdded reports that App Store Connect created the
	// requested memberships.
	BetaGroupTestersActionAdded = "added"
	// BetaGroupTestersActionSkipped reports that a post-conflict read-back
	// confirmed every requested tester in the group.
	BetaGroupTestersActionSkipped = "skipped"
	// BetaGroupTestersActionRemoved reports that App Store Connect removed the
	// requested memberships.
	BetaGroupTestersActionRemoved = "removed"
)

// BetaGroupTestersUpdateResult represents CLI output for beta group tester
// additions and removals. AlreadyPresent is set when App Store Connect
// rejected the add with a conflict and a membership read-back confirmed every
// requested tester is already in the group.
type BetaGroupTestersUpdateResult struct {
	GroupID        string   `json:"groupId"`
	TesterIDs      []string `json:"testerIds"`
	Action         string   `json:"action"`
	AlreadyPresent bool     `json:"alreadyPresent,omitempty"`
}

// BetaTesterAppsUpdateResult represents CLI output for beta tester app updates.
type BetaTesterAppsUpdateResult struct {
	TesterID string   `json:"testerId"`
	AppIDs   []string `json:"appIds"`
	Action   string   `json:"action"`
}

// BetaTesterBuildsUpdateResult represents CLI output for beta tester build updates.
type BetaTesterBuildsUpdateResult struct {
	TesterID string   `json:"testerId"`
	BuildIDs []string `json:"buildIds"`
	Action   string   `json:"action"`
}

// AppBetaTestersUpdateResult represents CLI output for app beta tester updates.
type AppBetaTestersUpdateResult struct {
	AppID     string   `json:"appId"`
	TesterIDs []string `json:"testerIds"`
	Action    string   `json:"action"`
}

// BuildBetaGroupMembershipResult describes the TestFlight groups that give a
// build access, including explicit relationship membership and all-build access.
type BuildBetaGroupMembershipResult struct {
	BuildID      string                            `json:"buildId"`
	AppID        string                            `json:"appId"`
	Complete     bool                              `json:"complete"`
	LookupMethod string                            `json:"lookupMethod"`
	GroupCount   int                               `json:"groupCount"`
	Groups       []BuildBetaGroupMembershipGroup   `json:"groups"`
	Failures     []BuildBetaGroupMembershipFailure `json:"failures,omitempty"`
}

// BuildBetaGroupMembershipGroup is one group that contains or implicitly
// receives a build.
type BuildBetaGroupMembershipGroup struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Type                 string `json:"type"`
	Membership           string `json:"membership"`
	HasAccessToAllBuilds bool   `json:"hasAccessToAllBuilds"`
}

// BuildBetaGroupMembershipFailure records an inverse relationship lookup that
// could not be completed.
type BuildBetaGroupMembershipFailure struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName,omitempty"`
	Error     string `json:"error"`
}

// BetaTestersExportSummary represents CLI output for beta tester CSV exports.
type BetaTestersExportSummary struct {
	AppID         string `json:"appId"`
	OutputFile    string `json:"outputFile"`
	Total         int    `json:"total"`
	IncludeGroups bool   `json:"includeGroups"`
}

// BetaTestersImportFailure records one CSV row that could not be imported.
type BetaTestersImportFailure struct {
	Row   int    `json:"row"`
	Email string `json:"email,omitempty"`
	Error string `json:"error"`
}

// BetaTestersImportSummary represents CLI output for beta tester CSV imports.
type BetaTestersImportSummary struct {
	Groups          []BetaTestersImportGroup   `json:"groups,omitempty"`
	AppID           string                     `json:"appId"`
	InputFile       string                     `json:"inputFile"`
	DryRun          bool                       `json:"dryRun"`
	Invite          bool                       `json:"invite"`
	SkipExisting    bool                       `json:"skipExisting"`
	ContinueOnError bool                       `json:"continueOnError"`
	AppliedGroup    string                     `json:"appliedGroup,omitempty"`
	Total           int                        `json:"total"`
	Created         int                        `json:"created"`
	Existed         int                        `json:"existed"`
	Updated         int                        `json:"updated"`
	Invited         int                        `json:"invited"`
	Failed          int                        `json:"failed"`
	Failures        []BetaTestersImportFailure `json:"failures,omitempty"`
}

// TestFlightSyncSummary represents CLI output for testflight sync pull.
type TestFlightSyncSummary struct {
	File    string `json:"file"`
	App     string `json:"app"`
	Groups  int    `json:"groups"`
	Builds  int    `json:"builds"`
	Testers int    `json:"testers"`
}

func formatBetaTesterName(attr BetaTesterAttributes) string {
	first := strings.TrimSpace(attr.FirstName)
	last := strings.TrimSpace(attr.LastName)
	switch {
	case first == "" && last == "":
		return ""
	case first == "":
		return last
	case last == "":
		return first
	default:
		return first + " " + last
	}
}

func betaGroupsRows(resp *BetaGroupsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Internal", "Public Link Enabled", "Public Link"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			fmt.Sprintf("%t", item.Attributes.IsInternalGroup),
			fmt.Sprintf("%t", item.Attributes.PublicLinkEnabled),
			item.Attributes.PublicLink,
		})
	}
	return headers, rows
}

func buildBetaGroupMembershipRows(result *BuildBetaGroupMembershipResult) ([]string, [][]string) {
	headers := []string{"Build ID", "App ID", "Group ID", "Name", "Type", "Membership", "All Builds", "Complete", "Error"}
	rows := make([][]string, 0, len(result.Groups)+len(result.Failures))
	for _, group := range result.Groups {
		rows = append(rows, []string{
			result.BuildID,
			result.AppID,
			group.ID,
			compactWhitespace(group.Name),
			group.Type,
			group.Membership,
			fmt.Sprintf("%t", group.HasAccessToAllBuilds),
			fmt.Sprintf("%t", result.Complete),
			"",
		})
	}
	for _, failure := range result.Failures {
		rows = append(rows, []string{
			result.BuildID,
			result.AppID,
			failure.GroupID,
			compactWhitespace(failure.GroupName),
			"",
			"error",
			"",
			"false",
			compactWhitespace(failure.Error),
		})
	}
	return headers, rows
}

// BetaTesterDevicesResponse keeps the tester envelope with an opt-in device renderer.
type BetaTesterDevicesResponse struct {
	BetaTestersResponse
}

func betaTesterDevicesRows(resp *BetaTesterDevicesResponse) ([]string, [][]string) {
	headers := []string{"Tester ID", "Model", "Platform", "OS", "App Build"}
	rows := make([][]string, 0, len(resp.Data))
	for _, tester := range resp.Data {
		if len(tester.Attributes.AppDevices) == 0 {
			state := "None reported"
			if tester.Attributes.AppDevices == nil {
				state = "Not reported"
			}
			rows = append(rows, []string{compactWhitespace(tester.ID), state, "", "", ""})
		}
		for _, device := range tester.Attributes.AppDevices {
			rows = append(rows, []string{
				compactWhitespace(tester.ID),
				compactWhitespace(device.Model),
				compactWhitespace(device.Platform),
				compactWhitespace(device.OSVersion),
				compactWhitespace(device.AppBuildVersion),
			})
		}
	}
	return headers, rows
}

func betaTestersRows(resp *BetaTestersResponse) ([]string, [][]string) {
	headers := []string{"ID", "Email", "Name", "State", "Invite"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Email,
			compactWhitespace(formatBetaTesterName(item.Attributes)),
			string(item.Attributes.State),
			string(item.Attributes.InviteType),
		})
	}
	return headers, rows
}

func betaTesterDeleteResultRows(result *BetaTesterDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Email", "Deleted"}
	rows := [][]string{{result.ID, result.Email, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func betaTesterGroupsUpdateResultRows(result *BetaTesterGroupsUpdateResult) ([]string, [][]string) {
	headers := []string{"Tester ID", "Group IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.GroupIDs, ","), result.Action}}
	return headers, rows
}

func betaGroupTestersUpdateResultRows(result *BetaGroupTestersUpdateResult) ([]string, [][]string) {
	headers := []string{"Group ID", "Tester IDs", "Action", "Already Present"}
	rows := [][]string{{result.GroupID, strings.Join(result.TesterIDs, ","), result.Action, fmt.Sprintf("%t", result.AlreadyPresent)}}
	return headers, rows
}

func betaTesterAppsUpdateResultRows(result *BetaTesterAppsUpdateResult) ([]string, [][]string) {
	headers := []string{"Tester ID", "App IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.AppIDs, ","), result.Action}}
	return headers, rows
}

func betaTesterBuildsUpdateResultRows(result *BetaTesterBuildsUpdateResult) ([]string, [][]string) {
	headers := []string{"Tester ID", "Build IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.BuildIDs, ","), result.Action}}
	return headers, rows
}

func appBetaTestersUpdateResultRows(result *AppBetaTestersUpdateResult) ([]string, [][]string) {
	headers := []string{"App ID", "Tester IDs", "Action"}
	rows := [][]string{{result.AppID, strings.Join(result.TesterIDs, ","), result.Action}}
	return headers, rows
}

func betaTesterInvitationResultRows(result *BetaTesterInvitationResult) ([]string, [][]string) {
	headers := []string{"Invitation ID", "Tester ID", "App ID", "Email"}
	rows := [][]string{{result.InvitationID, result.TesterID, result.AppID, result.Email}}
	return headers, rows
}

func betaTestersExportSummaryRows(summary *BetaTestersExportSummary) ([]string, [][]string) {
	headers := []string{"App ID", "Output File", "Total", "Include Groups"}
	rows := [][]string{{
		summary.AppID,
		summary.OutputFile,
		fmt.Sprintf("%d", summary.Total),
		fmt.Sprintf("%t", summary.IncludeGroups),
	}}
	return headers, rows
}

func betaTestersImportSummaryRows(summary *BetaTestersImportSummary) ([]string, [][]string) {
	headers := []string{"App ID", "Input File", "Dry Run", "Total", "Created", "Existed", "Updated", "Invited", "Failed"}
	rows := [][]string{{
		summary.AppID,
		summary.InputFile,
		fmt.Sprintf("%t", summary.DryRun),
		fmt.Sprintf("%d", summary.Total),
		fmt.Sprintf("%d", summary.Created),
		fmt.Sprintf("%d", summary.Existed),
		fmt.Sprintf("%d", summary.Updated),
		fmt.Sprintf("%d", summary.Invited),
		fmt.Sprintf("%d", summary.Failed),
	}}
	return headers, rows
}

func betaTestersImportFailureRows(failures []BetaTestersImportFailure) ([]string, [][]string) {
	headers := []string{"Row", "Email", "Error"}
	rows := make([][]string, 0, len(failures))
	for _, failure := range failures {
		rows = append(rows, []string{
			fmt.Sprintf("%d", failure.Row),
			failure.Email,
			compactWhitespace(failure.Error),
		})
	}
	return headers, rows
}

func testFlightSyncSummaryRows(summary *TestFlightSyncSummary) ([]string, [][]string) {
	headers := []string{"File", "App", "Groups", "Builds", "Testers"}
	rows := [][]string{{
		summary.File,
		compactWhitespace(summary.App),
		fmt.Sprintf("%d", summary.Groups),
		fmt.Sprintf("%d", summary.Builds),
		fmt.Sprintf("%d", summary.Testers),
	}}
	return headers, rows
}

// BetaTesterUsagesPage is the printed page for tester usage metrics: the raw
// metric data plus an optional resolved-testers sidecar keyed by tester ID.
// The data elements stay byte-identical to Apple's response.
type BetaTesterUsagesPage struct {
	Data     []json.RawMessage                    `json:"data"`
	Links    Links                                `json:"links"`
	Included json.RawMessage                      `json:"included,omitempty"`
	Meta     json.RawMessage                      `json:"meta,omitempty"`
	Testers  map[string]BetaTesterUsageTesterInfo `json:"testers,omitempty"`
}

// BetaTesterUsageTesterInfo describes one resolved beta tester in the
// testers sidecar keyed by tester ID.
type BetaTesterUsageTesterInfo struct {
	ID         string `json:"id"`
	Email      string `json:"email,omitempty"`
	FirstName  string `json:"firstName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	State      string `json:"state,omitempty"`
	InviteType string `json:"inviteType,omitempty"`
}

func betaTesterUsagesPageTables(v *BetaTesterUsagesPage, render func([]string, [][]string)) error {
	type metricEntry struct {
		DataPoints []struct {
			Start  string `json:"start"`
			End    string `json:"end"`
			Values struct {
				SessionCount  *int `json:"sessionCount"`
				CrashCount    *int `json:"crashCount"`
				FeedbackCount *int `json:"feedbackCount"`
			} `json:"values"`
		} `json:"dataPoints"`
		Dimensions struct {
			BetaTesters struct {
				Data *MetricDimensionData `json:"data"`
			} `json:"betaTesters"`
		} `json:"dimensions"`
	}
	formatCount := func(n *int) string {
		if n == nil {
			return ""
		}
		return fmt.Sprintf("%d", *n)
	}
	if v == nil {
		render([]string{"Tester ID", "Start", "End", "Sessions", "Crashes", "Feedback"}, nil)
		return nil
	}
	rows := make([][]string, 0, len(v.Data))
	for i, raw := range v.Data {
		var entry metricEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return fmt.Errorf("parse data[%d]: %w", i, err)
		}
		testerID := ""
		if entry.Dimensions.BetaTesters.Data != nil {
			testerID = strings.TrimSpace(entry.Dimensions.BetaTesters.Data.ID)
		}
		for _, point := range entry.DataPoints {
			rows = append(rows, []string{
				testerID,
				point.Start,
				point.End,
				formatCount(point.Values.SessionCount),
				formatCount(point.Values.CrashCount),
				formatCount(point.Values.FeedbackCount),
			})
		}
	}
	render([]string{"Tester ID", "Start", "End", "Sessions", "Crashes", "Feedback"}, rows)

	if len(v.Testers) > 0 {
		ids := make([]string, 0, len(v.Testers))
		for id := range v.Testers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		testerRows := make([][]string, 0, len(ids))
		for _, id := range ids {
			tester := v.Testers[id]
			testerRows = append(testerRows, []string{
				tester.ID, tester.Email, tester.FirstName, tester.LastName, tester.State,
			})
		}
		render([]string{"Tester ID", "Email", "First Name", "Last Name", "State"}, testerRows)
	}
	return nil
}

// GetLinks lets the usages page participate in pagination warnings.
func (p *BetaTesterUsagesPage) GetLinks() *Links {
	if p == nil {
		return nil
	}
	return &p.Links
}

// GetData exposes the page's data array for page-size reporting.
func (p *BetaTesterUsagesPage) GetData() any {
	if p == nil {
		return nil
	}
	return p.Data
}

func betaBuildUsagesRows(resp *BetaBuildUsagesResponse) ([]string, [][]string, error) {
	headers := []string{"Start", "End", "Installs", "Sessions", "Crashes", "Feedback", "Invites"}
	if len(resp.Data) == 0 {
		return headers, nil, nil
	}
	var payload struct {
		Data []struct {
			DataPoints []struct {
				Start  string `json:"start"`
				End    string `json:"end"`
				Values struct {
					InstallCount  *int `json:"installCount"`
					SessionCount  *int `json:"sessionCount"`
					CrashCount    *int `json:"crashCount"`
					FeedbackCount *int `json:"feedbackCount"`
					InviteCount   *int `json:"inviteCount"`
				} `json:"values"`
			} `json:"dataPoints"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return nil, nil, fmt.Errorf("parse beta build usages: %w", err)
	}
	var rows [][]string
	for _, metric := range payload.Data {
		for _, point := range metric.DataPoints {
			v := point.Values
			rows = append(rows, []string{
				point.Start,
				point.End,
				formatOptionalInt(v.InstallCount),
				formatOptionalInt(v.SessionCount),
				formatOptionalInt(v.CrashCount),
				formatOptionalInt(v.FeedbackCount),
				formatOptionalInt(v.InviteCount),
			})
		}
	}
	return headers, rows, nil
}

// BetaTestersImportGroup records a group planned or created by tester import.
type BetaTestersImportGroup struct {
	Name   string `json:"name"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func betaTestersImportGroupRows(groups []BetaTestersImportGroup) ([]string, [][]string) {
	rows := make([][]string, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, []string{group.Name, group.ID, group.Status, group.Error})
	}
	return []string{"Group", "ID", "Status", "Error"}, rows
}
