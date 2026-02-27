package web

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const (
	privacySchemaVersion       = 1
	dataProtectionNotCollected = "DATA_NOT_COLLECTED"
	dataProtectionLinked       = "DATA_LINKED_TO_YOU"
	dataProtectionNotLinked    = "DATA_NOT_LINKED_TO_YOU"
	dataProtectionTracking     = "DATA_USED_TO_TRACK_YOU"
	dataProtectionUnknown      = "UNKNOWN_OR_MISSING"
)

var (
	privacyTokenPattern  = regexp.MustCompile(`^[A-Z0-9_]+$`)
	knownDataProtections = map[string]struct{}{
		dataProtectionNotCollected: {},
		dataProtectionLinked:       {},
		dataProtectionNotLinked:    {},
		dataProtectionTracking:     {},
	}
)

type privacyDeclarationFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	DataUsages    []privacyUsage `json:"dataUsages"`
}

type privacyUsage struct {
	Category        string   `json:"category,omitempty"`
	Purposes        []string `json:"purposes,omitempty"`
	DataProtections []string `json:"dataProtections"`
}

type privacyTuple struct {
	Category       string
	Purpose        string
	DataProtection string
}

type privacyRemoteState struct {
	Tuple    privacyTuple
	UsageIDs []string
}

type privacyPlanChange struct {
	Key            string `json:"key"`
	Category       string `json:"category,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	DataProtection string `json:"dataProtection"`
	UsageID        string `json:"usageId,omitempty"`
}

type privacySkippedDelete struct {
	Key            string `json:"key"`
	Category       string `json:"category,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	DataProtection string `json:"dataProtection"`
	Reason         string `json:"reason"`
}

type privacyAPICall struct {
	Operation string `json:"operation"`
	Count     int    `json:"count"`
}

type privacyPlanOutput struct {
	AppID          string                 `json:"appId"`
	File           string                 `json:"file"`
	Updates        []privacyPlanChange    `json:"updates,omitempty"`
	Adds           []privacyPlanChange    `json:"adds"`
	Deletes        []privacyPlanChange    `json:"deletes"`
	SkippedDeletes []privacySkippedDelete `json:"skippedDeletes,omitempty"`
	APICalls       []privacyAPICall       `json:"apiCalls,omitempty"`
}

type privacyApplyAction struct {
	Action         string `json:"action"`
	Key            string `json:"key"`
	UsageID        string `json:"usageId,omitempty"`
	Category       string `json:"category,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	DataProtection string `json:"dataProtection"`
}

type privacyApplyOutput struct {
	AppID          string                 `json:"appId"`
	File           string                 `json:"file"`
	Updates        []privacyPlanChange    `json:"updates,omitempty"`
	Adds           []privacyPlanChange    `json:"adds"`
	Deletes        []privacyPlanChange    `json:"deletes"`
	SkippedDeletes []privacySkippedDelete `json:"skippedDeletes,omitempty"`
	Applied        bool                   `json:"applied"`
	Actions        []privacyApplyAction   `json:"actions,omitempty"`
	APICalls       []privacyAPICall       `json:"apiCalls,omitempty"`
}

type privacyPublishState struct {
	ID        string `json:"id,omitempty"`
	Published bool   `json:"published"`
}

type privacyPullOutput struct {
	AppID        string                 `json:"appId"`
	Declaration  privacyDeclarationFile `json:"declaration"`
	PublishState privacyPublishState    `json:"publishState"`
	Out          string                 `json:"out,omitempty"`
}

type privacyPublishOutput struct {
	AppID        string              `json:"appId"`
	PublishState privacyPublishState `json:"publishState"`
	WasPublished bool                `json:"wasPublished"`
	Changed      bool                `json:"changed"`
}

type privacyCatalogOutput struct {
	Categories      []webcore.AppDataUsageCategory       `json:"categories"`
	Purposes        []webcore.AppDataUsagePurpose        `json:"purposes"`
	DataProtections []webcore.AppDataUsageDataProtection `json:"dataProtections"`
}

type privacyMutationClient interface {
	CreateAppDataUsage(ctx context.Context, appID string, tuple webcore.DataUsageTuple) (*webcore.AppDataUsage, error)
	UpdateAppDataUsage(ctx context.Context, appDataUsageID string, tuple webcore.DataUsageTuple) (*webcore.AppDataUsage, error)
	DeleteAppDataUsage(ctx context.Context, appDataUsageID string) error
}

func normalizeToken(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func validPrivacyToken(value string) bool {
	return privacyTokenPattern.MatchString(value)
}

func normalizeStringList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeToken(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func privacyTupleKey(tuple privacyTuple) string {
	return strings.Join([]string{
		normalizeToken(tuple.Category),
		normalizeToken(tuple.Purpose),
		normalizeToken(tuple.DataProtection),
	}, "|")
}

func usageKey(usage privacyUsage) string {
	purposes := normalizeStringList(usage.Purposes)
	protections := normalizeStringList(usage.DataProtections)
	return strings.Join([]string{
		normalizeToken(usage.Category),
		strings.Join(purposes, ","),
		strings.Join(protections, ","),
	}, "|")
}

func declarationToTupleSet(declaration privacyDeclarationFile) (map[string]privacyTuple, error) {
	if declaration.SchemaVersion == 0 {
		declaration.SchemaVersion = privacySchemaVersion
	}
	if declaration.SchemaVersion != privacySchemaVersion {
		return nil, fmt.Errorf("schemaVersion must be %d", privacySchemaVersion)
	}
	if len(declaration.DataUsages) == 0 {
		return nil, fmt.Errorf("dataUsages must contain at least one entry")
	}

	tuples := make(map[string]privacyTuple)
	sawNotCollected := false
	sawCollected := false
	for index, usage := range declaration.DataUsages {
		category := normalizeToken(usage.Category)
		if category != "" && !validPrivacyToken(category) {
			return nil, fmt.Errorf("dataUsages[%d].category must match [A-Z0-9_]+", index)
		}

		purposes := normalizeStringList(usage.Purposes)
		for _, purpose := range purposes {
			if !validPrivacyToken(purpose) {
				return nil, fmt.Errorf("dataUsages[%d].purposes contains invalid value %q", index, purpose)
			}
		}

		protections := normalizeStringList(usage.DataProtections)
		if len(protections) == 0 {
			return nil, fmt.Errorf("dataUsages[%d].dataProtections is required", index)
		}
		for _, protection := range protections {
			if _, ok := knownDataProtections[protection]; !ok {
				return nil, fmt.Errorf("dataUsages[%d].dataProtections contains unsupported value %q", index, protection)
			}
		}

		if slices.Contains(protections, dataProtectionNotCollected) {
			if sawCollected {
				return nil, fmt.Errorf("dataUsages[%d] with DATA_NOT_COLLECTED cannot be combined with collected data usages", index)
			}
			if len(protections) != 1 {
				return nil, fmt.Errorf("dataUsages[%d] with DATA_NOT_COLLECTED cannot include other dataProtections", index)
			}
			if category != "" {
				return nil, fmt.Errorf("dataUsages[%d] with DATA_NOT_COLLECTED cannot include category", index)
			}
			if len(purposes) != 0 {
				return nil, fmt.Errorf("dataUsages[%d] with DATA_NOT_COLLECTED cannot include purposes", index)
			}
			tuple := privacyTuple{DataProtection: dataProtectionNotCollected}
			tuples[privacyTupleKey(tuple)] = tuple
			sawNotCollected = true
			continue
		}
		if sawNotCollected {
			return nil, fmt.Errorf("dataUsages[%d] with collected data cannot be combined with DATA_NOT_COLLECTED", index)
		}
		sawCollected = true

		if category == "" {
			return nil, fmt.Errorf("dataUsages[%d].category is required when data is collected", index)
		}
		hasIdentityProtection := slices.Contains(protections, dataProtectionLinked) || slices.Contains(protections, dataProtectionNotLinked)
		hasTrackingProtection := slices.Contains(protections, dataProtectionTracking)
		if !hasIdentityProtection && !hasTrackingProtection {
			return nil, fmt.Errorf("dataUsages[%d].dataProtections must include a supported collected-data protection", index)
		}
		if hasIdentityProtection && len(purposes) == 0 {
			return nil, fmt.Errorf("dataUsages[%d].purposes is required when data is collected", index)
		}

		for _, protection := range protections {
			if protection == dataProtectionTracking {
				tuple := privacyTuple{
					Category: category,
					// Tracking is represented category-wide in ASC responses.
					// Canonicalize purpose away to keep pull/plan idempotent.
					Purpose:        "",
					DataProtection: protection,
				}
				tuples[privacyTupleKey(tuple)] = tuple
				continue
			}
			for _, purpose := range purposes {
				tuple := privacyTuple{
					Category:       category,
					Purpose:        purpose,
					DataProtection: protection,
				}
				tuples[privacyTupleKey(tuple)] = tuple
			}
		}
	}

	if len(tuples) == 0 {
		return nil, fmt.Errorf("no usable data usage tuples were found")
	}
	return tuples, nil
}

func declarationFromTupleSet(tuples map[string]privacyTuple) privacyDeclarationFile {
	groupedProtections := map[string]map[string]struct{}{}
	groupMeta := map[string]privacyTuple{}
	for _, tuple := range tuples {
		groupKey := strings.Join([]string{
			normalizeToken(tuple.Category),
			normalizeToken(tuple.Purpose),
		}, "|")
		if _, exists := groupedProtections[groupKey]; !exists {
			groupedProtections[groupKey] = map[string]struct{}{}
		}
		groupedProtections[groupKey][normalizeToken(tuple.DataProtection)] = struct{}{}
		groupMeta[groupKey] = privacyTuple{
			Category: normalizeToken(tuple.Category),
			Purpose:  normalizeToken(tuple.Purpose),
		}
	}

	groupKeys := make([]string, 0, len(groupedProtections))
	for key := range groupedProtections {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	usages := make([]privacyUsage, 0, len(groupKeys))
	for _, key := range groupKeys {
		meta := groupMeta[key]
		protections := make([]string, 0, len(groupedProtections[key]))
		for protection := range groupedProtections[key] {
			protections = append(protections, protection)
		}
		sort.Strings(protections)

		usage := privacyUsage{
			Category:        meta.Category,
			DataProtections: protections,
		}
		if meta.Purpose != "" {
			usage.Purposes = []string{meta.Purpose}
		}
		usages = append(usages, usage)
	}

	sort.Slice(usages, func(i, j int) bool {
		return usageKey(usages[i]) < usageKey(usages[j])
	})
	return privacyDeclarationFile{
		SchemaVersion: privacySchemaVersion,
		DataUsages:    usages,
	}
}

func remoteStateFromDataUsages(usages []webcore.AppDataUsage) map[string]privacyRemoteState {
	state := make(map[string]privacyRemoteState)
	for _, usage := range usages {
		tuple := privacyTuple{
			Category:       normalizeToken(usage.Category),
			Purpose:        normalizeToken(usage.Purpose),
			DataProtection: normalizeToken(usage.DataProtection),
		}
		if tuple.DataProtection == "" {
			// Preserve malformed usages so plan/apply can explicitly delete them.
			tuple.DataProtection = dataProtectionUnknown
		}
		key := privacyTupleKey(tuple)
		current := state[key]
		current.Tuple = tuple
		usageID := strings.TrimSpace(usage.ID)
		if usageID != "" {
			current.UsageIDs = append(current.UsageIDs, usageID)
		}
		state[key] = current
	}

	for key, value := range state {
		sort.Strings(value.UsageIDs)
		state[key] = value
	}
	return state
}

func declarationFromRemoteDataUsages(usages []webcore.AppDataUsage) privacyDeclarationFile {
	if len(usages) == 0 {
		return privacyDeclarationFile{
			SchemaVersion: privacySchemaVersion,
			DataUsages: []privacyUsage{
				{
					DataProtections: []string{dataProtectionNotCollected},
				},
			},
		}
	}

	tuples := make(map[string]privacyTuple)
	for key, value := range remoteStateFromDataUsages(usages) {
		if _, known := knownDataProtections[value.Tuple.DataProtection]; !known {
			continue
		}
		tuples[key] = value.Tuple
	}
	if len(tuples) == 0 {
		return privacyDeclarationFile{
			SchemaVersion: privacySchemaVersion,
			DataUsages: []privacyUsage{
				{
					DataProtections: []string{dataProtectionNotCollected},
				},
			},
		}
	}
	return declarationFromTupleSet(tuples)
}

func pairChangesIntoUpdates(adds []privacyPlanChange, deletes []privacyPlanChange) ([]privacyPlanChange, []privacyPlanChange, []privacyPlanChange) {
	if len(adds) == 0 || len(deletes) == 0 {
		return []privacyPlanChange{}, adds, deletes
	}

	type pairIndex struct {
		addIdx    int
		deleteIdx int
	}
	pairsByScope := map[string][]pairIndex{}
	for addIdx, add := range adds {
		for deleteIdx, deletion := range deletes {
			if !canPairAsUpdate(add, deletion) {
				continue
			}
			scope := strings.Join([]string{add.Category, add.Purpose}, "|")
			pairsByScope[scope] = append(pairsByScope[scope], pairIndex{
				addIdx:    addIdx,
				deleteIdx: deleteIdx,
			})
		}
	}
	if len(pairsByScope) == 0 {
		return []privacyPlanChange{}, adds, deletes
	}

	scopes := make([]string, 0, len(pairsByScope))
	for scope := range pairsByScope {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	usedAdds := make([]bool, len(adds))
	usedDeletes := make([]bool, len(deletes))
	updates := make([]privacyPlanChange, 0)
	for _, scope := range scopes {
		candidates := pairsByScope[scope]
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].addIdx == candidates[j].addIdx {
				return candidates[i].deleteIdx < candidates[j].deleteIdx
			}
			return candidates[i].addIdx < candidates[j].addIdx
		})
		for _, candidate := range candidates {
			if usedAdds[candidate.addIdx] || usedDeletes[candidate.deleteIdx] {
				continue
			}
			usedAdds[candidate.addIdx] = true
			usedDeletes[candidate.deleteIdx] = true

			add := adds[candidate.addIdx]
			deletion := deletes[candidate.deleteIdx]
			updates = append(updates, privacyPlanChange{
				Key:            add.Key,
				Category:       add.Category,
				Purpose:        add.Purpose,
				DataProtection: add.DataProtection,
				UsageID:        deletion.UsageID,
			})
		}
	}

	remainingAdds := make([]privacyPlanChange, 0, len(adds))
	for idx, add := range adds {
		if usedAdds[idx] {
			continue
		}
		remainingAdds = append(remainingAdds, add)
	}
	remainingDeletes := make([]privacyPlanChange, 0, len(deletes))
	for idx, deletion := range deletes {
		if usedDeletes[idx] {
			continue
		}
		remainingDeletes = append(remainingDeletes, deletion)
	}

	sort.Slice(updates, func(i, j int) bool {
		if updates[i].Key == updates[j].Key {
			return updates[i].UsageID < updates[j].UsageID
		}
		return updates[i].Key < updates[j].Key
	})

	return updates, remainingAdds, remainingDeletes
}

func canPairAsUpdate(add privacyPlanChange, deletion privacyPlanChange) bool {
	if strings.TrimSpace(add.UsageID) != "" {
		return false
	}
	if strings.TrimSpace(deletion.UsageID) == "" {
		return false
	}
	if normalizeToken(add.Category) == "" || normalizeToken(add.Purpose) == "" {
		return false
	}
	if normalizeToken(deletion.Category) == "" || normalizeToken(deletion.Purpose) == "" {
		return false
	}
	addProtection := normalizeToken(add.DataProtection)
	deleteProtection := normalizeToken(deletion.DataProtection)
	if addProtection == dataProtectionNotCollected || deleteProtection == dataProtectionNotCollected {
		return false
	}

	// Keep update pairing conservative: prefer linkage flips (linked <-> not-linked).
	// Tracking-only rewrites can always be represented as explicit delete+create.
	if !isIdentityDataProtection(addProtection) || !isIdentityDataProtection(deleteProtection) {
		return false
	}
	if addProtection == deleteProtection {
		return false
	}

	return normalizeToken(add.Category) == normalizeToken(deletion.Category) &&
		normalizeToken(add.Purpose) == normalizeToken(deletion.Purpose)
}

func isIdentityDataProtection(value string) bool {
	value = normalizeToken(value)
	return value == dataProtectionLinked || value == dataProtectionNotLinked
}

func planFromDesiredAndRemote(appID, file string, desired map[string]privacyTuple, remote map[string]privacyRemoteState) privacyPlanOutput {
	adds := make([]privacyPlanChange, 0)
	var updates []privacyPlanChange
	deletes := make([]privacyPlanChange, 0)
	skippedDeletes := make([]privacySkippedDelete, 0)

	for key, tuple := range desired {
		if _, exists := remote[key]; exists {
			continue
		}
		adds = append(adds, privacyPlanChange{
			Key:            key,
			Category:       tuple.Category,
			Purpose:        tuple.Purpose,
			DataProtection: tuple.DataProtection,
		})
	}

	for key, state := range remote {
		if _, exists := desired[key]; !exists {
			if len(state.UsageIDs) == 0 {
				skippedDeletes = append(skippedDeletes, privacySkippedDelete{
					Key:            key,
					Category:       state.Tuple.Category,
					Purpose:        state.Tuple.Purpose,
					DataProtection: state.Tuple.DataProtection,
					Reason:         "missing_usage_id",
				})
				continue
			}
			for _, usageID := range state.UsageIDs {
				deletes = append(deletes, privacyPlanChange{
					Key:            key,
					Category:       state.Tuple.Category,
					Purpose:        state.Tuple.Purpose,
					DataProtection: state.Tuple.DataProtection,
					UsageID:        usageID,
				})
			}
			continue
		}

		// Keep one matching tuple if duplicates exist remotely; plan deletes for extras.
		if len(state.UsageIDs) > 1 {
			for _, usageID := range state.UsageIDs[1:] {
				deletes = append(deletes, privacyPlanChange{
					Key:            key,
					Category:       state.Tuple.Category,
					Purpose:        state.Tuple.Purpose,
					DataProtection: state.Tuple.DataProtection,
					UsageID:        usageID,
				})
			}
		}
	}

	sort.Slice(adds, func(i, j int) bool {
		return adds[i].Key < adds[j].Key
	})
	sort.Slice(deletes, func(i, j int) bool {
		if deletes[i].Key == deletes[j].Key {
			return deletes[i].UsageID < deletes[j].UsageID
		}
		return deletes[i].Key < deletes[j].Key
	})
	updates, adds, deletes = pairChangesIntoUpdates(adds, deletes)
	sort.Slice(skippedDeletes, func(i, j int) bool {
		if skippedDeletes[i].Key == skippedDeletes[j].Key {
			return skippedDeletes[i].Reason < skippedDeletes[j].Reason
		}
		return skippedDeletes[i].Key < skippedDeletes[j].Key
	})

	apiCalls := make([]privacyAPICall, 0, 2)
	if len(adds) > 0 {
		apiCalls = append(apiCalls, privacyAPICall{
			Operation: "create_data_usage",
			Count:     len(adds),
		})
	}
	if len(updates) > 0 {
		apiCalls = append(apiCalls, privacyAPICall{
			Operation: "update_data_usage",
			Count:     len(updates),
		})
	}
	if len(deletes) > 0 {
		apiCalls = append(apiCalls, privacyAPICall{
			Operation: "delete_data_usage",
			Count:     len(deletes),
		})
	}

	return privacyPlanOutput{
		AppID:          appID,
		File:           file,
		Updates:        updates,
		Adds:           adds,
		Deletes:        deletes,
		SkippedDeletes: skippedDeletes,
		APICalls:       apiCalls,
	}
}

func applyPrivacyPlan(ctx context.Context, client privacyMutationClient, appID string, plan privacyPlanOutput) ([]privacyApplyAction, error) {
	if err := validateApplyPlanUsageIDs(plan); err != nil {
		return nil, err
	}
	actions := make([]privacyApplyAction, 0, len(plan.Updates)+len(plan.Adds)+len(plan.Deletes))

	for _, deletion := range plan.Deletes {
		if err := client.DeleteAppDataUsage(ctx, deletion.UsageID); err != nil {
			return nil, err
		}
		actions = append(actions, privacyApplyAction{
			Action:         "delete",
			Key:            deletion.Key,
			UsageID:        deletion.UsageID,
			Category:       deletion.Category,
			Purpose:        deletion.Purpose,
			DataProtection: deletion.DataProtection,
		})
	}

	for _, update := range plan.Updates {
		updated, err := client.UpdateAppDataUsage(ctx, update.UsageID, webcore.DataUsageTuple{
			Category:       update.Category,
			Purpose:        update.Purpose,
			DataProtection: update.DataProtection,
		})
		if err != nil {
			return nil, err
		}
		actions = append(actions, privacyApplyAction{
			Action:         "update",
			Key:            update.Key,
			UsageID:        strings.TrimSpace(updated.ID),
			Category:       update.Category,
			Purpose:        update.Purpose,
			DataProtection: update.DataProtection,
		})
	}

	for _, add := range plan.Adds {
		created, err := client.CreateAppDataUsage(ctx, appID, webcore.DataUsageTuple{
			Category:       add.Category,
			Purpose:        add.Purpose,
			DataProtection: add.DataProtection,
		})
		if err != nil {
			return nil, err
		}
		actions = append(actions, privacyApplyAction{
			Action:         "create",
			Key:            add.Key,
			UsageID:        strings.TrimSpace(created.ID),
			Category:       add.Category,
			Purpose:        add.Purpose,
			DataProtection: add.DataProtection,
		})
	}

	return actions, nil
}

func validateApplyPlanUsageIDs(plan privacyPlanOutput) error {
	updateUsageIDs := map[string]string{}
	for _, update := range plan.Updates {
		usageID := strings.TrimSpace(update.UsageID)
		if usageID == "" {
			return fmt.Errorf("web privacy apply failed: missing usage id for update key %s", update.Key)
		}
		if existingKey, exists := updateUsageIDs[usageID]; exists {
			return fmt.Errorf("web privacy apply failed: duplicate update usage id %s for keys %s and %s", usageID, existingKey, update.Key)
		}
		updateUsageIDs[usageID] = update.Key
	}

	deleteUsageIDs := map[string]string{}
	for _, deletion := range plan.Deletes {
		usageID := strings.TrimSpace(deletion.UsageID)
		if usageID == "" {
			return fmt.Errorf("web privacy apply failed: missing usage id for delete key %s", deletion.Key)
		}
		if existingKey, exists := deleteUsageIDs[usageID]; exists {
			return fmt.Errorf("web privacy apply failed: duplicate delete usage id %s for keys %s and %s", usageID, existingKey, deletion.Key)
		}
		if existingUpdateKey, exists := updateUsageIDs[usageID]; exists {
			return fmt.Errorf("web privacy apply failed: usage id %s is scheduled for both delete (%s) and update (%s)", usageID, deletion.Key, existingUpdateKey)
		}
		deleteUsageIDs[usageID] = deletion.Key
	}

	return nil
}

func parsePrivacyDeclarationFile(path string) (privacyDeclarationFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return privacyDeclarationFile{}, fmt.Errorf("file path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return privacyDeclarationFile{}, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var declaration privacyDeclarationFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declaration); err != nil {
		return privacyDeclarationFile{}, fmt.Errorf("invalid privacy declaration JSON: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return privacyDeclarationFile{}, fmt.Errorf("invalid privacy declaration JSON: multiple JSON values found")
		}
		return privacyDeclarationFile{}, fmt.Errorf("invalid privacy declaration JSON: %w", err)
	}

	tuples, err := declarationToTupleSet(declaration)
	if err != nil {
		return privacyDeclarationFile{}, err
	}
	return declarationFromTupleSet(tuples), nil
}

func writePrivacyDeclarationFile(path string, declaration privacyDeclarationFile) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	jsonData, err := json.MarshalIndent(declaration, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal privacy declaration: %w", err)
	}
	jsonData = append(jsonData, '\n')
	if err := os.WriteFile(path, jsonData, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func buildPrivacyRows(usages []privacyUsage) [][]string {
	if len(usages) == 0 {
		return [][]string{{"n/a", "n/a", "n/a"}}
	}
	rows := make([][]string, 0, len(usages))
	for _, usage := range usages {
		category := usage.Category
		if strings.TrimSpace(category) == "" {
			category = "n/a"
		}
		purposes := "n/a"
		if len(usage.Purposes) > 0 {
			purposes = strings.Join(usage.Purposes, ", ")
		}
		rows = append(rows, []string{
			category,
			purposes,
			strings.Join(usage.DataProtections, ", "),
		})
	}
	return rows
}

func buildPrivacyPlanRows(updates []privacyPlanChange, adds []privacyPlanChange, deletes []privacyPlanChange) [][]string {
	rows := make([][]string, 0, len(updates)+len(adds)+len(deletes))
	for _, update := range updates {
		rows = append(rows, []string{
			"update",
			update.Key,
			valueOrNA(update.Category),
			valueOrNA(update.Purpose),
			update.DataProtection,
			valueOrNA(update.UsageID),
		})
	}
	for _, add := range adds {
		rows = append(rows, []string{
			"add",
			add.Key,
			valueOrNA(add.Category),
			valueOrNA(add.Purpose),
			add.DataProtection,
			"",
		})
	}
	for _, deletion := range deletes {
		rows = append(rows, []string{
			"delete",
			deletion.Key,
			valueOrNA(deletion.Category),
			valueOrNA(deletion.Purpose),
			deletion.DataProtection,
			valueOrNA(deletion.UsageID),
		})
	}
	if len(rows) == 0 {
		return [][]string{{"none", "", "", "", "", ""}}
	}
	return rows
}

func buildPrivacySkippedDeleteRows(skipped []privacySkippedDelete) [][]string {
	if len(skipped) == 0 {
		return [][]string{{"none", "", "", "", "", ""}}
	}
	rows := make([][]string, 0, len(skipped))
	for _, item := range skipped {
		rows = append(rows, []string{
			item.Key,
			valueOrNA(item.Category),
			valueOrNA(item.Purpose),
			item.DataProtection,
			item.Reason,
			"",
		})
	}
	return rows
}

func buildPrivacyAPICallRows(calls []privacyAPICall) [][]string {
	rows := make([][]string, 0, len(calls))
	for _, call := range calls {
		rows = append(rows, []string{
			call.Operation,
			fmt.Sprintf("%d", call.Count),
		})
	}
	return rows
}

func buildPrivacyActionRows(actions []privacyApplyAction) [][]string {
	if len(actions) == 0 {
		return [][]string{{"none", "", "", "", "", ""}}
	}
	rows := make([][]string, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, []string{
			action.Action,
			action.Key,
			valueOrNA(action.Category),
			valueOrNA(action.Purpose),
			action.DataProtection,
			valueOrNA(action.UsageID),
		})
	}
	return rows
}

func buildCatalogRows(categories []webcore.AppDataUsageCategory, purposes []webcore.AppDataUsagePurpose, protections []webcore.AppDataUsageDataProtection) [][]string {
	rows := make([][]string, 0, len(categories)+len(purposes)+len(protections))

	for _, category := range categories {
		rows = append(rows, []string{
			"category",
			category.ID,
			valueOrNA(category.Grouping),
			fmt.Sprintf("%t", category.Deleted),
		})
	}
	for _, purpose := range purposes {
		rows = append(rows, []string{
			"purpose",
			purpose.ID,
			"n/a",
			fmt.Sprintf("%t", purpose.Deleted),
		})
	}
	for _, protection := range protections {
		rows = append(rows, []string{
			"dataProtection",
			protection.ID,
			"n/a",
			fmt.Sprintf("%t", protection.Deleted),
		})
	}

	if len(rows) == 0 {
		return [][]string{{"none", "n/a", "n/a", "n/a"}}
	}
	return rows
}

func valueOrNA(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}
	return trimmed
}

func renderPrivacyPullTable(payload privacyPullOutput) error {
	fmt.Printf("App ID: %s\n", payload.AppID)
	fmt.Printf("Published: %t\n", payload.PublishState.Published)
	if strings.TrimSpace(payload.Out) != "" {
		fmt.Printf("Output File: %s\n", payload.Out)
	}
	fmt.Println()
	asc.RenderTable(
		[]string{"Category", "Purposes", "Data Protections"},
		buildPrivacyRows(payload.Declaration.DataUsages),
	)
	return nil
}

func renderPrivacyPullMarkdown(payload privacyPullOutput) error {
	fmt.Printf("**App ID:** %s\n\n", payload.AppID)
	fmt.Printf("**Published:** %t\n\n", payload.PublishState.Published)
	if strings.TrimSpace(payload.Out) != "" {
		fmt.Printf("**Output File:** %s\n\n", payload.Out)
	}
	asc.RenderMarkdown(
		[]string{"Category", "Purposes", "Data Protections"},
		buildPrivacyRows(payload.Declaration.DataUsages),
	)
	return nil
}

func renderPrivacyPlanTable(payload privacyPlanOutput) error {
	fmt.Printf("App ID: %s\n", payload.AppID)
	fmt.Printf("File: %s\n", payload.File)
	fmt.Printf("Updates: %d\n", len(payload.Updates))
	fmt.Printf("Adds: %d\n", len(payload.Adds))
	fmt.Printf("Deletes: %d\n", len(payload.Deletes))
	fmt.Printf("Skipped Deletes: %d\n\n", len(payload.SkippedDeletes))
	asc.RenderTable(
		[]string{"Change", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
		buildPrivacyPlanRows(payload.Updates, payload.Adds, payload.Deletes),
	)
	if len(payload.SkippedDeletes) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Key", "Category", "Purpose", "Data Protection", "Reason", "Usage ID"},
			buildPrivacySkippedDeleteRows(payload.SkippedDeletes),
		)
	}
	if len(payload.APICalls) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"Operation", "Count"}, buildPrivacyAPICallRows(payload.APICalls))
	}
	return nil
}

func renderPrivacyPlanMarkdown(payload privacyPlanOutput) error {
	fmt.Printf("**App ID:** %s\n\n", payload.AppID)
	fmt.Printf("**File:** %s\n\n", payload.File)
	fmt.Printf("**Updates:** %d\n\n", len(payload.Updates))
	fmt.Printf("**Adds:** %d\n\n", len(payload.Adds))
	fmt.Printf("**Deletes:** %d\n\n", len(payload.Deletes))
	fmt.Printf("**Skipped Deletes:** %d\n\n", len(payload.SkippedDeletes))
	asc.RenderMarkdown(
		[]string{"Change", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
		buildPrivacyPlanRows(payload.Updates, payload.Adds, payload.Deletes),
	)
	if len(payload.SkippedDeletes) > 0 {
		fmt.Println()
		asc.RenderMarkdown(
			[]string{"Key", "Category", "Purpose", "Data Protection", "Reason", "Usage ID"},
			buildPrivacySkippedDeleteRows(payload.SkippedDeletes),
		)
	}
	if len(payload.APICalls) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"Operation", "Count"}, buildPrivacyAPICallRows(payload.APICalls))
	}
	return nil
}

func renderPrivacyApplyTable(payload privacyApplyOutput) error {
	fmt.Printf("App ID: %s\n", payload.AppID)
	fmt.Printf("File: %s\n", payload.File)
	fmt.Printf("Applied: %t\n", payload.Applied)
	fmt.Printf("Updates: %d\n", len(payload.Updates))
	fmt.Printf("Adds: %d\n", len(payload.Adds))
	fmt.Printf("Deletes: %d\n", len(payload.Deletes))
	fmt.Printf("Skipped Deletes: %d\n\n", len(payload.SkippedDeletes))
	asc.RenderTable(
		[]string{"Change", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
		buildPrivacyPlanRows(payload.Updates, payload.Adds, payload.Deletes),
	)
	if len(payload.SkippedDeletes) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Key", "Category", "Purpose", "Data Protection", "Reason", "Usage ID"},
			buildPrivacySkippedDeleteRows(payload.SkippedDeletes),
		)
	}
	if len(payload.Actions) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.Actions),
		)
	}
	if len(payload.APICalls) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"Operation", "Count"}, buildPrivacyAPICallRows(payload.APICalls))
	}
	return nil
}

func renderPrivacyApplyMarkdown(payload privacyApplyOutput) error {
	fmt.Printf("**App ID:** %s\n\n", payload.AppID)
	fmt.Printf("**File:** %s\n\n", payload.File)
	fmt.Printf("**Applied:** %t\n\n", payload.Applied)
	fmt.Printf("**Updates:** %d\n\n", len(payload.Updates))
	fmt.Printf("**Adds:** %d\n\n", len(payload.Adds))
	fmt.Printf("**Deletes:** %d\n\n", len(payload.Deletes))
	fmt.Printf("**Skipped Deletes:** %d\n\n", len(payload.SkippedDeletes))
	asc.RenderMarkdown(
		[]string{"Change", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
		buildPrivacyPlanRows(payload.Updates, payload.Adds, payload.Deletes),
	)
	if len(payload.SkippedDeletes) > 0 {
		fmt.Println()
		asc.RenderMarkdown(
			[]string{"Key", "Category", "Purpose", "Data Protection", "Reason", "Usage ID"},
			buildPrivacySkippedDeleteRows(payload.SkippedDeletes),
		)
	}
	if len(payload.Actions) > 0 {
		fmt.Println()
		asc.RenderMarkdown(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.Actions),
		)
	}
	if len(payload.APICalls) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"Operation", "Count"}, buildPrivacyAPICallRows(payload.APICalls))
	}
	return nil
}

func renderPrivacyPublishTable(payload privacyPublishOutput) error {
	asc.RenderTable([]string{"Field", "Value"}, [][]string{
		{"App ID", payload.AppID},
		{"Publish State ID", valueOrNA(payload.PublishState.ID)},
		{"Published", fmt.Sprintf("%t", payload.PublishState.Published)},
		{"Was Published", fmt.Sprintf("%t", payload.WasPublished)},
		{"Changed", fmt.Sprintf("%t", payload.Changed)},
	})
	return nil
}

func renderPrivacyPublishMarkdown(payload privacyPublishOutput) error {
	asc.RenderMarkdown([]string{"Field", "Value"}, [][]string{
		{"App ID", payload.AppID},
		{"Publish State ID", valueOrNA(payload.PublishState.ID)},
		{"Published", fmt.Sprintf("%t", payload.PublishState.Published)},
		{"Was Published", fmt.Sprintf("%t", payload.WasPublished)},
		{"Changed", fmt.Sprintf("%t", payload.Changed)},
	})
	return nil
}

func renderPrivacyCatalogTable(payload privacyCatalogOutput) error {
	fmt.Printf("Categories: %d\n", len(payload.Categories))
	fmt.Printf("Purposes: %d\n", len(payload.Purposes))
	fmt.Printf("Data Protections: %d\n\n", len(payload.DataProtections))
	asc.RenderTable(
		[]string{"Kind", "ID", "Grouping", "Deleted"},
		buildCatalogRows(payload.Categories, payload.Purposes, payload.DataProtections),
	)
	return nil
}

func renderPrivacyCatalogMarkdown(payload privacyCatalogOutput) error {
	fmt.Printf("**Categories:** %d\n\n", len(payload.Categories))
	fmt.Printf("**Purposes:** %d\n\n", len(payload.Purposes))
	fmt.Printf("**Data Protections:** %d\n\n", len(payload.DataProtections))
	asc.RenderMarkdown(
		[]string{"Kind", "ID", "Grouping", "Deleted"},
		buildCatalogRows(payload.Categories, payload.Purposes, payload.DataProtections),
	)
	return nil
}

// WebPrivacyCommand returns the detached web privacy command group.
func WebPrivacyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web privacy", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "privacy",
		ShortUsage: "asc web privacy <subcommand> [flags]",
		ShortHelp:  "EXPERIMENTAL: App privacy declaration workflows.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Agent-friendly app privacy declaration workflows over Apple web-session /iris endpoints.
Use pull/plan/apply/publish with explicit mutation controls.

Subcommands:
  catalog  List category/purpose/data-protection tokens for declaration authoring
  pull     Fetch current app data usage declarations as canonical JSON
  plan     Diff local declaration file against remote state
  apply    Apply planned changes (never publishes automatically)
  publish  Explicitly publish app data usage declarations

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebPrivacyCatalogCommand(),
			WebPrivacyPullCommand(),
			WebPrivacyPlanCommand(),
			WebPrivacyApplyCommand(),
			WebPrivacyPublishCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebPrivacyCatalogCommand lists available privacy declaration catalog values.
func WebPrivacyCatalogCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web privacy catalog", flag.ExitOnError)

	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "catalog",
		ShortUsage: "asc web privacy catalog [flags]",
		ShortHelp:  "EXPERIMENTAL: List app privacy catalog values.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Fetch category, purpose, and data-protection tokens that can be used in
privacy declaration files.

Examples:
  asc web privacy catalog --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web privacy catalog does not accept positional arguments")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			categories, err := client.ListAppDataUsageCategories(requestCtx)
			if err != nil {
				return withWebAuthHint(err, "web privacy catalog")
			}
			purposes, err := client.ListAppDataUsagePurposes(requestCtx)
			if err != nil {
				return withWebAuthHint(err, "web privacy catalog")
			}
			protections, err := client.ListAppDataUsageDataProtections(requestCtx)
			if err != nil {
				return withWebAuthHint(err, "web privacy catalog")
			}

			sort.Slice(categories, func(i, j int) bool { return categories[i].ID < categories[j].ID })
			sort.Slice(purposes, func(i, j int) bool { return purposes[i].ID < purposes[j].ID })
			sort.Slice(protections, func(i, j int) bool { return protections[i].ID < protections[j].ID })

			payload := privacyCatalogOutput{
				Categories:      categories,
				Purposes:        purposes,
				DataProtections: protections,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderPrivacyCatalogTable(payload) },
				func() error { return renderPrivacyCatalogMarkdown(payload) },
			)
		},
	}
}

// WebPrivacyPullCommand pulls remote app privacy declarations into canonical JSON.
func WebPrivacyPullCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web privacy pull", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	out := fs.String("out", "", "Optional output file path for canonical declaration JSON")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc web privacy pull --app APP_ID [--out FILE] [flags]",
		ShortHelp:  "EXPERIMENTAL: Pull app privacy declaration state.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Fetch current app data usage declarations from web-session endpoints and emit
canonical JSON that can be used with plan/apply.

Examples:
  asc web privacy pull --app "123456789"
  asc web privacy pull --app "123456789" --out "./privacy.json"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web privacy pull does not accept positional arguments")
			}
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			remoteUsages, err := client.ListAppDataUsages(requestCtx, resolvedAppID)
			if err != nil {
				return withWebAuthHint(err, "web privacy pull")
			}
			publishState, err := client.GetAppDataUsagesPublishState(requestCtx, resolvedAppID)
			if err != nil {
				return withWebAuthHint(err, "web privacy pull")
			}

			declaration := declarationFromRemoteDataUsages(remoteUsages)
			outPath := strings.TrimSpace(*out)
			if outPath != "" {
				if err := writePrivacyDeclarationFile(outPath, declaration); err != nil {
					return err
				}
			}

			payload := privacyPullOutput{
				AppID:       resolvedAppID,
				Declaration: declaration,
				PublishState: privacyPublishState{
					ID:        strings.TrimSpace(publishState.ID),
					Published: publishState.Published,
				},
				Out: outPath,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderPrivacyPullTable(payload) },
				func() error { return renderPrivacyPullMarkdown(payload) },
			)
		},
	}
}

// WebPrivacyPlanCommand compares local declaration file with remote state.
func WebPrivacyPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web privacy plan", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	filePath := fs.String("file", "", "Path to canonical privacy declaration JSON")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc web privacy plan --app APP_ID --file FILE [flags]",
		ShortHelp:  "EXPERIMENTAL: Plan app privacy declaration changes.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Compute a deterministic diff between local declaration JSON and remote
app data usage tuples.

Examples:
  asc web privacy plan --app "123456789" --file "./privacy.json"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web privacy plan does not accept positional arguments")
			}
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			resolvedFilePath := strings.TrimSpace(*filePath)
			if resolvedFilePath == "" {
				return shared.UsageError("--file is required")
			}

			declaration, err := parsePrivacyDeclarationFile(resolvedFilePath)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			desiredTuples, err := declarationToTupleSet(declaration)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)
			remoteUsages, err := client.ListAppDataUsages(requestCtx, resolvedAppID)
			if err != nil {
				return withWebAuthHint(err, "web privacy plan")
			}
			remoteState := remoteStateFromDataUsages(remoteUsages)
			plan := planFromDesiredAndRemote(resolvedAppID, resolvedFilePath, desiredTuples, remoteState)

			return shared.PrintOutputWithRenderers(
				plan,
				*output.Output,
				*output.Pretty,
				func() error { return renderPrivacyPlanTable(plan) },
				func() error { return renderPrivacyPlanMarkdown(plan) },
			)
		},
	}
}

// WebPrivacyApplyCommand applies local declaration changes to remote app data usages.
func WebPrivacyApplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web privacy apply", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	filePath := fs.String("file", "", "Path to canonical privacy declaration JSON")
	allowDeletes := fs.Bool("allow-deletes", false, "Allow delete operations when remote tuples are missing locally")
	confirm := fs.Bool("confirm", false, "Confirm delete operations (required with --allow-deletes)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc web privacy apply --app APP_ID --file FILE [--allow-deletes --confirm] [flags]",
		ShortHelp:  "EXPERIMENTAL: Apply app privacy declaration changes.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Apply local declaration tuples to remote app data usages.
This command never publishes automatically.

Examples:
  asc web privacy apply --app "123456789" --file "./privacy.json"
  asc web privacy apply --app "123456789" --file "./privacy.json" --allow-deletes --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web privacy apply does not accept positional arguments")
			}
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			resolvedFilePath := strings.TrimSpace(*filePath)
			if resolvedFilePath == "" {
				return shared.UsageError("--file is required")
			}
			if *allowDeletes && !*confirm {
				return shared.UsageError("--confirm is required when --allow-deletes is set")
			}

			declaration, err := parsePrivacyDeclarationFile(resolvedFilePath)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			desiredTuples, err := declarationToTupleSet(declaration)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)
			remoteUsages, err := client.ListAppDataUsages(requestCtx, resolvedAppID)
			if err != nil {
				return withWebAuthHint(err, "web privacy apply")
			}
			remoteState := remoteStateFromDataUsages(remoteUsages)
			plan := planFromDesiredAndRemote(resolvedAppID, resolvedFilePath, desiredTuples, remoteState)

			if len(plan.Deletes) > 0 && !*allowDeletes {
				return shared.UsageError("--allow-deletes is required to apply delete operations")
			}
			if len(plan.Deletes) > 0 && !*confirm {
				return shared.UsageError("--confirm is required when applying delete operations")
			}

			actions, err := applyPrivacyPlan(requestCtx, client, resolvedAppID, plan)
			if err != nil {
				return withWebAuthHint(err, "web privacy apply")
			}

			payload := privacyApplyOutput{
				AppID:          resolvedAppID,
				File:           resolvedFilePath,
				Updates:        plan.Updates,
				Adds:           plan.Adds,
				Deletes:        plan.Deletes,
				SkippedDeletes: plan.SkippedDeletes,
				Applied:        true,
				Actions:        actions,
				APICalls:       plan.APICalls,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderPrivacyApplyTable(payload) },
				func() error { return renderPrivacyApplyMarkdown(payload) },
			)
		},
	}
}

// WebPrivacyPublishCommand explicitly publishes app data usage declarations.
func WebPrivacyPublishCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web privacy publish", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	confirm := fs.Bool("confirm", false, "Confirm publish operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "publish",
		ShortUsage: "asc web privacy publish --app APP_ID --confirm [flags]",
		ShortHelp:  "EXPERIMENTAL: Publish app privacy declarations.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Explicitly publish app data usage declarations after apply.

Examples:
  asc web privacy publish --app "123456789" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web privacy publish does not accept positional arguments")
			}
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			stateBefore, err := client.GetAppDataUsagesPublishState(requestCtx, resolvedAppID)
			if err != nil {
				return withWebAuthHint(err, "web privacy publish")
			}
			stateAfter := stateBefore
			if !stateBefore.Published {
				stateAfter, err = client.SetAppDataUsagesPublished(requestCtx, stateBefore.ID, true)
				if err != nil {
					return withWebAuthHint(err, "web privacy publish")
				}
			}

			payload := privacyPublishOutput{
				AppID: resolvedAppID,
				PublishState: privacyPublishState{
					ID:        strings.TrimSpace(stateAfter.ID),
					Published: stateAfter.Published,
				},
				WasPublished: stateBefore.Published,
				Changed:      !stateBefore.Published && stateAfter.Published,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderPrivacyPublishTable(payload) },
				func() error { return renderPrivacyPublishMarkdown(payload) },
			)
		},
	}
}
