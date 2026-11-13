package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
	errPrivacySkippedDeletesRemain = errors.New("remote declaration includes usages without a usage id that apply cannot delete")
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
	Tuple       privacyTuple
	UsageIDs    []string
	IDLessCount int
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

// privacyStaleToken names one declaration token that the live Apple catalog no
// longer accepts, so plan can flag it and apply can refuse before mutating.
type privacyStaleToken struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type privacyPlanOutput struct {
	AppID          string                 `json:"appId"`
	File           string                 `json:"file"`
	Updates        []privacyPlanChange    `json:"updates,omitempty"`
	Adds           []privacyPlanChange    `json:"adds"`
	Deletes        []privacyPlanChange    `json:"deletes"`
	SkippedDeletes []privacySkippedDelete `json:"skippedDeletes,omitempty"`
	StaleTokens    []privacyStaleToken    `json:"staleTokens,omitempty"`
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

// privacyApplyRecheck records the post-failure remote re-read that resolves
// which attempted actions actually committed.
type privacyApplyRecheck struct {
	Performed bool `json:"performed"`
	Succeeded bool `json:"succeeded"`
	// RemainingChanges is absent when the re-read failed: no count was
	// computed, and a zero there would read as convergence.
	RemainingChanges *int `json:"remainingChanges,omitempty"`
}

type privacyApplyOutput struct {
	AppID          string                 `json:"appId"`
	File           string                 `json:"file"`
	Updates        []privacyPlanChange    `json:"updates,omitempty"`
	Adds           []privacyPlanChange    `json:"adds"`
	Deletes        []privacyPlanChange    `json:"deletes"`
	SkippedDeletes []privacySkippedDelete `json:"skippedDeletes,omitempty"`
	Applied        bool                   `json:"applied"`
	// Changed is omitted when an attempted action is still unresolved: a
	// false here would read as a confirmed no-op.
	Changed           *bool                `json:"changed,omitempty"`
	Actions           []privacyApplyAction `json:"actions,omitempty"`
	UnknownActions    []privacyApplyAction `json:"unknownActions,omitempty"`
	NotAppliedActions []privacyApplyAction `json:"notAppliedActions,omitempty"`
	Recheck           *privacyApplyRecheck `json:"recheck,omitempty"`
	APICalls          []privacyAPICall     `json:"apiCalls,omitempty"`
}

type privacyPublishState struct {
	ID             string `json:"id,omitempty"`
	Published      bool   `json:"published"`
	PublishedKnown bool   `json:"publishedKnown"`
}

type privacyPullOutput struct {
	AppID                string                 `json:"appId"`
	Declaration          privacyDeclarationFile `json:"declaration"`
	PublishState         privacyPublishState    `json:"publishState"`
	UnrepresentableCount int                    `json:"unrepresentableCount"`
	Out                  string                 `json:"out,omitempty"`
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

type privacyUsageReader interface {
	ListAppDataUsages(ctx context.Context, appID string) ([]webcore.AppDataUsage, error)
}

type privacyCatalogClient interface {
	ListAppDataUsageCategories(ctx context.Context) ([]webcore.AppDataUsageCategory, error)
	ListAppDataUsagePurposes(ctx context.Context) ([]webcore.AppDataUsagePurpose, error)
	ListAppDataUsageDataProtections(ctx context.Context) ([]webcore.AppDataUsageDataProtection, error)
}

// privacyCatalogTokens maps each catalog dimension to token id -> deleted.
type privacyCatalogTokens struct {
	Categories      map[string]bool
	Purposes        map[string]bool
	DataProtections map[string]bool
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
			if protection == dataProtectionUnknown {
				return nil, fmt.Errorf("dataUsages[%d].dataProtections contains unrepresentable remote privacy data %q; resolve those entries before apply", index, protection)
			}
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
		} else {
			current.IDLessCount++
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
	for _, value := range remoteStateFromDataUsages(usages) {
		tuple := value.Tuple
		if isUnrepresentableDataProtection(tuple.DataProtection) {
			tuple.DataProtection = dataProtectionUnknown
		}
		tuples[privacyTupleKey(tuple)] = tuple
	}
	return declarationFromTupleSet(tuples)
}

func isUnrepresentableDataProtection(protection string) bool {
	_, known := knownDataProtections[normalizeToken(protection)]
	return !known
}

func countUnrepresentableRemoteUsages(usages []webcore.AppDataUsage) int {
	count := 0
	for _, usage := range usages {
		if isUnrepresentableDataProtection(usage.DataProtection) {
			count++
		}
	}
	return count
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
			skippedDeletes = appendIDLessSkippedDeletes(skippedDeletes, key, state, 0)
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
		// An identified usage already satisfies the desired key, so every
		// ID-less sibling is extra. If the only remote members are ID-less,
		// keep one of them — removing it would make the desired tuple absent.
		keepIDLess := 0
		if len(state.UsageIDs) == 0 {
			keepIDLess = 1
		}
		skippedDeletes = appendIDLessSkippedDeletes(skippedDeletes, key, state, keepIDLess)
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

func appendIDLessSkippedDeletes(skipped []privacySkippedDelete, key string, state privacyRemoteState, keep int) []privacySkippedDelete {
	if state.IDLessCount <= keep {
		return skipped
	}
	return append(skipped, privacySkippedDelete{
		Key:            key,
		Category:       state.Tuple.Category,
		Purpose:        state.Tuple.Purpose,
		DataProtection: state.Tuple.DataProtection,
		Reason:         "missing_usage_id",
	})
}

// loadPrivacyCatalogTokens reads the live category, purpose, and
// data-protection catalogs so plan and apply can detect tokens Apple has
// deleted before a declaration is sent back as a relationship id.
func loadPrivacyCatalogTokens(ctx context.Context, client privacyCatalogClient) (privacyCatalogTokens, error) {
	tokens := privacyCatalogTokens{
		Categories:      map[string]bool{},
		Purposes:        map[string]bool{},
		DataProtections: map[string]bool{},
	}

	categories, err := client.ListAppDataUsageCategories(ctx)
	if err != nil {
		return privacyCatalogTokens{}, err
	}
	for _, category := range categories {
		id := normalizeToken(category.ID)
		if id == "" {
			continue
		}
		tokens.Categories[id] = category.Deleted
	}

	purposes, err := client.ListAppDataUsagePurposes(ctx)
	if err != nil {
		return privacyCatalogTokens{}, err
	}
	for _, purpose := range purposes {
		id := normalizeToken(purpose.ID)
		if id == "" {
			continue
		}
		tokens.Purposes[id] = purpose.Deleted
	}

	protections, err := client.ListAppDataUsageDataProtections(ctx)
	if err != nil {
		return privacyCatalogTokens{}, err
	}
	for _, protection := range protections {
		id := normalizeToken(protection.ID)
		if id == "" {
			continue
		}
		tokens.DataProtections[id] = protection.Deleted
	}

	return tokens, nil
}

// privacyStaleTokens reports declaration tokens that the catalog marks deleted
// or no longer lists at all. A dimension Apple returned empty proves nothing,
// so it is skipped instead of failing every token in it.
func privacyStaleTokens(desired map[string]privacyTuple, catalog privacyCatalogTokens) []privacyStaleToken {
	used := map[string]map[string]struct{}{
		"category":       {},
		"purpose":        {},
		"dataProtection": {},
	}
	for _, tuple := range desired {
		for kind, value := range map[string]string{
			"category":       tuple.Category,
			"purpose":        tuple.Purpose,
			"dataProtection": tuple.DataProtection,
		} {
			token := normalizeToken(value)
			if token == "" {
				continue
			}
			used[kind][token] = struct{}{}
		}
	}

	known := map[string]map[string]bool{
		"category":       catalog.Categories,
		"purpose":        catalog.Purposes,
		"dataProtection": catalog.DataProtections,
	}

	stale := make([]privacyStaleToken, 0)
	for kind, tokens := range used {
		catalogTokens := known[kind]
		if len(catalogTokens) == 0 {
			continue
		}
		for token := range tokens {
			deleted, exists := catalogTokens[token]
			switch {
			case !exists:
				stale = append(stale, privacyStaleToken{Kind: kind, ID: token, Reason: "unknown"})
			case deleted:
				stale = append(stale, privacyStaleToken{Kind: kind, ID: token, Reason: "deleted"})
			}
		}
	}

	sort.Slice(stale, func(i, j int) bool {
		if stale[i].Kind == stale[j].Kind {
			return stale[i].ID < stale[j].ID
		}
		return stale[i].Kind < stale[j].Kind
	})
	return stale
}

func formatPrivacyStaleTokens(stale []privacyStaleToken) string {
	parts := make([]string, 0, len(stale))
	for _, token := range stale {
		parts = append(parts, fmt.Sprintf("%s %s (%s)", token.Kind, token.ID, token.Reason))
	}
	return strings.Join(parts, ", ")
}

// privacyPlannedStep is one ordered mutation in an apply sequence.
type privacyPlannedStep struct {
	Action string
	Change privacyPlanChange
}

// privacyApplySteps orders a plan so a mid-sequence failure leaves a superset
// of the desired declaration rather than a hole. Updates run first because an
// in-place linkage flip never leaves a tuple missing, creates run next, and
// deletes run last. The exception is a delete that a later create depends on:
// DATA_NOT_COLLECTED and collected tuples are mutually exclusive, so those
// deletes are prerequisites and must run before the creates that replace them.
func privacyApplySteps(plan privacyPlanOutput) []privacyPlannedStep {
	desiredNotCollected := false
	for _, add := range plan.Adds {
		if normalizeToken(add.DataProtection) == dataProtectionNotCollected {
			desiredNotCollected = true
			break
		}
	}

	prerequisiteDeletes := make([]privacyPlanChange, 0, len(plan.Deletes))
	trailingDeletes := make([]privacyPlanChange, 0, len(plan.Deletes))
	for _, deletion := range plan.Deletes {
		if desiredNotCollected || normalizeToken(deletion.DataProtection) == dataProtectionNotCollected {
			prerequisiteDeletes = append(prerequisiteDeletes, deletion)
			continue
		}
		trailingDeletes = append(trailingDeletes, deletion)
	}

	steps := make([]privacyPlannedStep, 0, len(plan.Updates)+len(plan.Adds)+len(plan.Deletes))
	for _, deletion := range prerequisiteDeletes {
		steps = append(steps, privacyPlannedStep{Action: "delete", Change: deletion})
	}
	for _, update := range plan.Updates {
		steps = append(steps, privacyPlannedStep{Action: "update", Change: update})
	}
	for _, add := range plan.Adds {
		steps = append(steps, privacyPlannedStep{Action: "create", Change: add})
	}
	for _, deletion := range trailingDeletes {
		steps = append(steps, privacyPlannedStep{Action: "delete", Change: deletion})
	}
	return steps
}

func privacyActionFromStep(step privacyPlannedStep, usageID string) privacyApplyAction {
	return privacyApplyAction{
		Action:         step.Action,
		Key:            step.Change.Key,
		UsageID:        strings.TrimSpace(usageID),
		Category:       step.Change.Category,
		Purpose:        step.Change.Purpose,
		DataProtection: step.Change.DataProtection,
	}
}

// privacyApplyResult is the receipt for one apply sequence: what committed,
// what was attempted without a confirmed outcome, and what never ran.
type privacyApplyResult struct {
	Applied    []privacyApplyAction
	Unknown    []privacyApplyAction
	NotApplied []privacyApplyAction
}

func applyPrivacyPlan(ctx context.Context, client privacyMutationClient, appID string, plan privacyPlanOutput) (privacyApplyResult, error) {
	result := privacyApplyResult{
		Applied:    make([]privacyApplyAction, 0, len(plan.Updates)+len(plan.Adds)+len(plan.Deletes)),
		Unknown:    make([]privacyApplyAction, 0),
		NotApplied: make([]privacyApplyAction, 0),
	}
	steps := privacyApplySteps(plan)
	// Validation aborts before the first mutation, so every planned step is
	// proven not applied. Leaving the buckets empty would print a receipt that
	// accounts for none of the plan, and would let a concurrently converged
	// re-read report the invocation as fully applied.
	if err := validateApplyPlanUsageIDs(plan); err != nil {
		for _, step := range steps {
			result.NotApplied = append(result.NotApplied, privacyActionFromStep(step, step.Change.UsageID))
		}
		return result, err
	}

	for index, step := range steps {
		usageID, err := executePrivacyStep(ctx, client, appID, step)
		if err != nil {
			result.Unknown = append(result.Unknown, privacyActionFromStep(step, step.Change.UsageID))
			for _, pending := range steps[index+1:] {
				result.NotApplied = append(result.NotApplied, privacyActionFromStep(pending, pending.Change.UsageID))
			}
			return result, err
		}
		result.Applied = append(result.Applied, privacyActionFromStep(step, usageID))
	}

	return result, nil
}

func executePrivacyStep(ctx context.Context, client privacyMutationClient, appID string, step privacyPlannedStep) (string, error) {
	tuple := webcore.DataUsageTuple{
		Category:       step.Change.Category,
		Purpose:        step.Change.Purpose,
		DataProtection: step.Change.DataProtection,
	}
	switch step.Action {
	case "delete":
		if err := client.DeleteAppDataUsage(ctx, step.Change.UsageID); err != nil {
			return "", err
		}
		return step.Change.UsageID, nil
	case "update":
		updated, err := client.UpdateAppDataUsage(ctx, step.Change.UsageID, tuple)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(updated.ID), nil
	case "create":
		created, err := client.CreateAppDataUsage(ctx, appID, tuple)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(created.ID), nil
	default:
		return "", fmt.Errorf("web privacy apply failed: unsupported action %q", step.Action)
	}
}

// recheckPrivacyRemoteUsages re-reads remote usages on a fresh timeout budget.
// The apply request context can already be past its deadline - a timeout is
// exactly the failure where reconciliation matters most - so this derives a new
// deadline from the command context while still honouring its cancellation.
func recheckPrivacyRemoteUsages(ctx context.Context, client privacyUsageReader, appID string) ([]webcore.AppDataUsage, error) {
	recheckCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return withWebSpinnerValue(
		"Re-reading privacy state after failure",
		func() ([]webcore.AppDataUsage, error) {
			return client.ListAppDataUsages(recheckCtx, appID)
		},
	)
}

// privacyApplyFailureMessage describes an interrupted apply without claiming
// more than the receipt proves: an apply that committed nothing is not a
// partial apply, and one the re-read shows fully converged is not partial
// either. The cause is rendered inline because the returned error is already
// reported and would otherwise never reach the operator; every web error
// string here is redacted and carries no raw Apple response body.
func privacyApplyFailureMessage(appID string, payload privacyApplyOutput, cause, recheckErr error) string {
	summary := fmt.Sprintf(
		"%d committed, %d unknown, %d not applied",
		len(payload.Actions),
		len(payload.UnknownActions),
		len(payload.NotAppliedActions),
	)
	lead := fmt.Sprintf("web privacy apply partially applied changes for app %s", appID)
	trailer := "the receipt above lists each action, and rerunning the same command converges from current remote state"
	switch {
	case payload.Applied:
		lead = fmt.Sprintf("web privacy apply failed for app %s after every planned change committed", appID)
		trailer = "the re-read shows remote state already matches the file, so a rerun is a no-op"
	case len(payload.Actions) == 0:
		lead = fmt.Sprintf("web privacy apply failed for app %s without a confirmed change", appID)
	}
	if !payload.Applied && len(payload.SkippedDeletes) > 0 {
		trailer = "the receipt above lists each action; skipped deletes have no usage id, so a rerun cannot remove them"
	}
	message := fmt.Sprintf("%s: %s; %s", lead, summary, trailer)
	if cause != nil {
		if causeText := strings.TrimSpace(cause.Error()); causeText != "" {
			message = fmt.Sprintf("%s; cause: %s", message, causeText)
		}
	}
	if recheckErr != nil {
		if recheckText := strings.TrimSpace(recheckErr.Error()); recheckText != "" {
			message = fmt.Sprintf("%s; recheck failed: %s", message, recheckText)
		}
	}
	return message
}

// privacyApplyChanged reports whether this invocation confirmed a mutation.
// When the only attempted action is still unknown, the field is omitted
// rather than serialized as false, which automation would read as a no-op.
func privacyApplyChanged(result privacyApplyResult) *bool {
	if len(result.Applied) == 0 && len(result.Unknown) > 0 {
		return nil
	}
	changed := len(result.Applied) > 0
	return &changed
}

func formatPrivacyApplyChanged(changed *bool) string {
	if changed == nil {
		return "unknown"
	}
	return fmt.Sprintf("%t", *changed)
}

// privacyApplyConverged reports whether the post-failure re-read proves the
// remote declaration already matches the file. Residual skipped deletes count
// against convergence: an undesired remote tuple Apple returned without a usage
// id cannot be deleted, so it is a known mismatch that no rerun clears, and
// claiming a match would be wrong in the one direction that matters.
func privacyApplyConverged(residual privacyPlanOutput, result privacyApplyResult) bool {
	return len(residual.Updates) == 0 &&
		len(residual.Adds) == 0 &&
		len(residual.Deletes) == 0 &&
		len(residual.SkippedDeletes) == 0 &&
		len(result.Unknown) == 0 &&
		len(result.NotApplied) == 0
}

// resolvePrivacyApplyResult reclassifies attempted-but-unconfirmed actions
// using a fresh remote read. A 5xx can still have committed the write, so the
// remote state is the only honest evidence.
func resolvePrivacyApplyResult(result privacyApplyResult, remote map[string]privacyRemoteState) privacyApplyResult {
	if len(result.Unknown) == 0 {
		return result
	}

	remoteUsageIDs := map[string]string{}
	for key, state := range remote {
		for _, usageID := range state.UsageIDs {
			remoteUsageIDs[usageID] = key
		}
	}

	resolved := privacyApplyResult{
		Applied:    append([]privacyApplyAction{}, result.Applied...),
		Unknown:    make([]privacyApplyAction, 0, len(result.Unknown)),
		NotApplied: append([]privacyApplyAction{}, result.NotApplied...),
	}
	for _, action := range result.Unknown {
		switch action.Action {
		case "create":
			state, exists := remote[action.Key]
			if !exists {
				resolved.NotApplied = append(resolved.NotApplied, action)
				continue
			}
			if len(state.UsageIDs) > 0 {
				action.UsageID = state.UsageIDs[0]
			}
			resolved.Applied = append(resolved.Applied, action)
		case "delete":
			if _, exists := remoteUsageIDs[action.UsageID]; exists {
				resolved.NotApplied = append(resolved.NotApplied, action)
				continue
			}
			// The targeted id is gone, but the tuple may still be present
			// without an id — alone or beside another identified usage.
			// That leftover is a skipped delete; treating the mutation as
			// committed would put it in actions and set changed=true while
			// the extra declaration remains.
			if state, exists := remote[action.Key]; exists && (len(state.UsageIDs) == 0 || state.IDLessCount > 0) {
				resolved.Unknown = append(resolved.Unknown, action)
				continue
			}
			resolved.Applied = append(resolved.Applied, action)
		case "update":
			key, exists := remoteUsageIDs[action.UsageID]
			switch {
			case exists && key == action.Key:
				resolved.Applied = append(resolved.Applied, action)
			case exists:
				resolved.NotApplied = append(resolved.NotApplied, action)
			default:
				resolved.Unknown = append(resolved.Unknown, action)
			}
		default:
			resolved.Unknown = append(resolved.Unknown, action)
		}
	}
	return resolved
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
	if path == "" {
		return privacyDeclarationFile{}, fmt.Errorf("file path is required")
	}
	root, name, err := privacyDeclarationRoot(path)
	if err != nil {
		return privacyDeclarationFile{}, err
	}
	data, err := root.ReadFile(name)
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

// privacyDeclarationRoot anchors privacy declaration reads and writes to the
// parent directory of the operator-selected path, so the final component and any
// component created below it cannot resolve through a symlink.
func privacyDeclarationRoot(path string) (rootfs.Root, string, error) {
	if path == "" {
		return rootfs.Root{}, "", fmt.Errorf("file path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return rootfs.Root{}, "", err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return rootfs.Root{}, "", err
	}
	return root, filepath.Base(absolute), nil
}

func writePrivacyDeclarationFile(path string, declaration privacyDeclarationFile) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is required")
	}
	root, name, err := privacyDeclarationRoot(path)
	if err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(declaration, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal privacy declaration: %w", err)
	}
	jsonData = append(jsonData, '\n')
	// An existing declaration keeps its permissions, matching the previous
	// in-place write; new files default to 0600.
	if err := root.WriteFilePreservingMode(name, jsonData, 0o600); err != nil {
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

func buildPrivacyRecheckRows(recheck privacyApplyRecheck) [][]string {
	remaining := "unknown"
	if recheck.RemainingChanges != nil {
		remaining = fmt.Sprintf("%d", *recheck.RemainingChanges)
	}
	return [][]string{
		{"Recheck Performed", fmt.Sprintf("%t", recheck.Performed)},
		{"Recheck Succeeded", fmt.Sprintf("%t", recheck.Succeeded)},
		{"Remaining Changes", remaining},
	}
}

func buildPrivacyStaleTokenRows(stale []privacyStaleToken) [][]string {
	rows := make([][]string, 0, len(stale))
	for _, token := range stale {
		rows = append(rows, []string{token.Kind, token.ID, token.Reason})
	}
	if len(rows) == 0 {
		return [][]string{{"none", "", ""}}
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

func privacyPublishStateFromRemote(state *webcore.AppDataUsagesPublishState) privacyPublishState {
	if state == nil {
		return privacyPublishState{}
	}
	return privacyPublishState{
		ID:             strings.TrimSpace(state.ID),
		Published:      state.Published,
		PublishedKnown: state.PublishedKnown,
	}
}

func formatPrivacyPublished(state privacyPublishState) string {
	if !state.PublishedKnown {
		return "unknown"
	}
	return fmt.Sprintf("%t", state.Published)
}

func renderPrivacyPullTable(payload privacyPullOutput) error {
	fmt.Printf("App ID: %s\n", payload.AppID)
	fmt.Printf("Published: %s\n", formatPrivacyPublished(payload.PublishState))
	if payload.UnrepresentableCount > 0 {
		fmt.Printf("Unrepresentable: %d\n", payload.UnrepresentableCount)
	}
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
	fmt.Printf("**Published:** %s\n\n", formatPrivacyPublished(payload.PublishState))
	if payload.UnrepresentableCount > 0 {
		fmt.Printf("**Unrepresentable:** %d\n\n", payload.UnrepresentableCount)
	}
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
	fmt.Printf("Skipped Deletes: %d\n", len(payload.SkippedDeletes))
	fmt.Printf("Stale Tokens: %d\n\n", len(payload.StaleTokens))
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
	if len(payload.StaleTokens) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"Kind", "Token", "Reason"}, buildPrivacyStaleTokenRows(payload.StaleTokens))
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
	fmt.Printf("**Stale Tokens:** %d\n\n", len(payload.StaleTokens))
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
	if len(payload.StaleTokens) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"Kind", "Token", "Reason"}, buildPrivacyStaleTokenRows(payload.StaleTokens))
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
	fmt.Printf("Changed: %s\n", formatPrivacyApplyChanged(payload.Changed))
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
		fmt.Println("Applied Actions")
		asc.RenderTable(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.Actions),
		)
	}
	if len(payload.UnknownActions) > 0 {
		fmt.Println()
		fmt.Println("Unknown Actions")
		asc.RenderTable(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.UnknownActions),
		)
	}
	if len(payload.NotAppliedActions) > 0 {
		fmt.Println()
		fmt.Println("Not Applied Actions")
		asc.RenderTable(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.NotAppliedActions),
		)
	}
	if payload.Recheck != nil {
		fmt.Println()
		asc.RenderTable([]string{"Field", "Value"}, buildPrivacyRecheckRows(*payload.Recheck))
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
	fmt.Printf("**Changed:** %s\n\n", formatPrivacyApplyChanged(payload.Changed))
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
		fmt.Printf("**Applied Actions**\n\n")
		asc.RenderMarkdown(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.Actions),
		)
	}
	if len(payload.UnknownActions) > 0 {
		fmt.Println()
		fmt.Printf("**Unknown Actions**\n\n")
		asc.RenderMarkdown(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.UnknownActions),
		)
	}
	if len(payload.NotAppliedActions) > 0 {
		fmt.Println()
		fmt.Printf("**Not Applied Actions**\n\n")
		asc.RenderMarkdown(
			[]string{"Action", "Key", "Category", "Purpose", "Data Protection", "Usage ID"},
			buildPrivacyActionRows(payload.NotAppliedActions),
		)
	}
	if payload.Recheck != nil {
		fmt.Println()
		asc.RenderMarkdown([]string{"Field", "Value"}, buildPrivacyRecheckRows(*payload.Recheck))
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
		{"Published", formatPrivacyPublished(payload.PublishState)},
		{"Was Published", fmt.Sprintf("%t", payload.WasPublished)},
		{"Changed", fmt.Sprintf("%t", payload.Changed)},
	})
	return nil
}

func renderPrivacyPublishMarkdown(payload privacyPublishOutput) error {
	asc.RenderMarkdown([]string{"Field", "Value"}, [][]string{
		{"App ID", payload.AppID},
		{"Publish State ID", valueOrNA(payload.PublishState.ID)},
		{"Published", formatPrivacyPublished(payload.PublishState)},
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
		ShortHelp:  "App privacy declaration workflows.",
		LongHelp: `WEB SESSION WORKFLOWS

Agent-friendly app privacy declaration workflows over Apple web-session /iris endpoints.
Use pull/plan/apply/publish with explicit mutation controls.

Subcommands:
  catalog  List category/purpose/data-protection tokens for declaration authoring
  pull     Fetch current app data usage declarations as canonical JSON
  plan     Diff local declaration file against remote state
  apply    Apply planned changes (does not call Apple's publish endpoint;
          usage mutations may update published-state metadata)
  publish  Explicitly publish app data usage declarations

`,
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
		ShortHelp:  "List app privacy catalog values.",
		LongHelp: `WEB SESSION WORKFLOWS

Fetch category, purpose, and data-protection tokens that can be used in
privacy declaration files.

Examples:
  asc web privacy catalog --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			payload := privacyCatalogOutput{}
			err = withWebSpinner("Loading privacy catalog", func() error {
				categories, err := client.ListAppDataUsageCategories(requestCtx)
				if err != nil {
					return err
				}
				purposes, err := client.ListAppDataUsagePurposes(requestCtx)
				if err != nil {
					return err
				}
				protections, err := client.ListAppDataUsageDataProtections(requestCtx)
				if err != nil {
					return err
				}

				sort.Slice(categories, func(i, j int) bool { return categories[i].ID < categories[j].ID })
				sort.Slice(purposes, func(i, j int) bool { return purposes[i].ID < purposes[j].ID })
				sort.Slice(protections, func(i, j int) bool { return protections[i].ID < protections[j].ID })

				payload = privacyCatalogOutput{
					Categories:      categories,
					Purposes:        purposes,
					DataProtections: protections,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "web privacy catalog")
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
		ShortHelp:  "Pull app privacy declaration state.",
		LongHelp: `WEB SESSION WORKFLOWS

Fetch current app data usage declarations from web-session endpoints and emit
canonical JSON that can be used with plan/apply.

Unrepresentable remote data-protection values are preserved as
UNKNOWN_OR_MISSING and counted in unrepresentableCount. apply refuses those
files until the entries are resolved.

Examples:
  asc web privacy pull --app "123456789"
  asc web privacy pull --app "123456789" --out "./privacy.json"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.WithDiagnostic(
					shared.UsageError("--app is required (or set ASC_APP_ID)"),
					shared.DiagnosticRequiredInputMissing,
					"--app",
				)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			var (
				remoteUsages []webcore.AppDataUsage
				publishState *webcore.AppDataUsagesPublishState
			)
			err = withWebSpinner("Loading app privacy state", func() error {
				var err error
				remoteUsages, err = client.ListAppDataUsages(requestCtx, resolvedAppID)
				if err != nil {
					return err
				}
				publishState, err = client.GetAppDataUsagesPublishState(requestCtx, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web privacy pull")
			}

			declaration := declarationFromRemoteDataUsages(remoteUsages)
			outPath := *out
			if outPath != "" {
				if err := writePrivacyDeclarationFile(outPath, declaration); err != nil {
					return err
				}
			} else {
				outPath = ""
			}

			payload := privacyPullOutput{
				AppID:                resolvedAppID,
				Declaration:          declaration,
				PublishState:         privacyPublishStateFromRemote(publishState),
				UnrepresentableCount: countUnrepresentableRemoteUsages(remoteUsages),
				Out:                  outPath,
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
		ShortHelp:  "Plan app privacy declaration changes.",
		LongHelp: `WEB SESSION WORKFLOWS

Compute a deterministic diff between local declaration JSON and remote
app data usage tuples.

Declarations that contain UNKNOWN_OR_MISSING (unrepresentable remote data)
are rejected before any planning against the live app.

Category, purpose, and data-protection tokens are checked against the live
catalog. Tokens Apple has deleted, or no longer lists, are reported as
staleTokens. plan stays read-only and still exits zero.

Examples:
  asc web privacy plan --app "123456789" --file "./privacy.json"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			resolvedFilePath := *filePath
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

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)
			plan := privacyPlanOutput{}
			err = withWebSpinner("Planning privacy changes", func() error {
				remoteUsages, err := client.ListAppDataUsages(requestCtx, resolvedAppID)
				if err != nil {
					return err
				}
				catalog, err := loadPrivacyCatalogTokens(requestCtx, client)
				if err != nil {
					return err
				}
				remoteState := remoteStateFromDataUsages(remoteUsages)
				plan = planFromDesiredAndRemote(resolvedAppID, resolvedFilePath, desiredTuples, remoteState)
				plan.StaleTokens = privacyStaleTokens(desiredTuples, catalog)
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "web privacy plan")
			}

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
		ShortHelp:  "Apply app privacy declaration changes.",
		LongHelp: `WEB SESSION WORKFLOWS

Apply local declaration tuples to remote app data usages.
This command never calls Apple's publish endpoint automatically, but applying
usage mutations may still update Apple's published-state metadata.

Declarations that contain UNKNOWN_OR_MISSING (unrepresentable remote data)
are rejected before any create, update, or delete.

apply refuses to mutate when the declaration references catalog tokens Apple
has deleted or no longer lists. Updates run first, then creates, then the
deletes that are safe to defer, so an interruption usually leaves extra tuples
rather than missing ones. The exception is a delete a later create depends on:
DATA_NOT_COLLECTED and collected tuples cannot coexist, so that delete must run
first and an interruption after it can leave a tuple missing until a rerun.
A mid-sequence failure re-reads remote state, prints a receipt that splits every
step into applied, unknown, and not applied, and exits non-zero. Rerunning the
same file converges and reports changed=false once nothing executable is left,
except leftover usages Apple returned without a usage id: those stay in
skippedDeletes and need manual cleanup.

Examples:
  asc web privacy apply --app "123456789" --file "./privacy.json"
  asc web privacy apply --app "123456789" --file "./privacy.json" --allow-deletes --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			resolvedFilePath := *filePath
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

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)
			plan := privacyPlanOutput{}
			err = withWebSpinner("Planning privacy changes", func() error {
				remoteUsages, err := client.ListAppDataUsages(requestCtx, resolvedAppID)
				if err != nil {
					return err
				}
				catalog, err := loadPrivacyCatalogTokens(requestCtx, client)
				if err != nil {
					return err
				}
				remoteState := remoteStateFromDataUsages(remoteUsages)
				plan = planFromDesiredAndRemote(resolvedAppID, resolvedFilePath, desiredTuples, remoteState)
				plan.StaleTokens = privacyStaleTokens(desiredTuples, catalog)
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "web privacy apply")
			}

			if len(plan.StaleTokens) > 0 {
				return shared.UsageErrorf(
					"web privacy apply: declaration references catalog tokens Apple no longer accepts: %s; refresh the catalog and the declaration before applying",
					formatPrivacyStaleTokens(plan.StaleTokens),
				)
			}
			if len(plan.Deletes) > 0 && !*allowDeletes {
				return shared.UsageError("--allow-deletes is required to apply delete operations")
			}
			if len(plan.Deletes) > 0 && !*confirm {
				return shared.UsageError("--confirm is required when applying delete operations")
			}

			result, applyErr := withWebSpinnerValue("Applying privacy changes", func() (privacyApplyResult, error) {
				return applyPrivacyPlan(requestCtx, client, resolvedAppID, plan)
			})

			payload := privacyApplyOutput{
				AppID:          resolvedAppID,
				File:           resolvedFilePath,
				Updates:        plan.Updates,
				Adds:           plan.Adds,
				Deletes:        plan.Deletes,
				SkippedDeletes: plan.SkippedDeletes,
				Applied:        applyErr == nil,
				APICalls:       plan.APICalls,
			}
			var recheckErr error
			if applyErr != nil {
				recheck := privacyApplyRecheck{Performed: true}
				remoteUsages, readErr := recheckPrivacyRemoteUsages(ctx, client, resolvedAppID)
				if readErr != nil {
					recheckErr = withWebAuthHint(readErr, "web privacy apply recheck")
				}
				if readErr == nil {
					remoteState := remoteStateFromDataUsages(remoteUsages)
					result = resolvePrivacyApplyResult(result, remoteState)
					residual := planFromDesiredAndRemote(resolvedAppID, resolvedFilePath, desiredTuples, remoteState)
					remaining := len(residual.Updates) + len(residual.Adds) + len(residual.Deletes)
					recheck.Succeeded = true
					recheck.RemainingChanges = &remaining
					// Residual skipped deletes are the current remote leftovers
					// a rerun still cannot delete. Keep them on the receipt so
					// the failure diagnostic does not promise convergence.
					payload.SkippedDeletes = residual.SkippedDeletes
					// Apple can commit the last mutation and still fail the
					// response. When the re-read proves every planned change
					// landed, the receipt says so; the exit stays non-zero
					// because the transport failure is still real.
					if privacyApplyConverged(residual, result) {
						payload.Applied = true
					}
				}
				payload.Recheck = &recheck
			}
			payload.Changed = privacyApplyChanged(result)
			payload.Actions = result.Applied
			payload.UnknownActions = result.Unknown
			payload.NotAppliedActions = result.NotApplied

			if applyErr == nil && len(payload.SkippedDeletes) > 0 {
				// Executable mutations may have succeeded, but an ID-less
				// leftover is a known mismatch no rerun can delete. Treat that
				// as an unsuccessful apply instead of applied:true.
				payload.Applied = false
				applyErr = errPrivacySkippedDeletesRemain
			}

			if applyErr != nil {
				if renderErr := shared.PrintOutputWithRenderers(
					payload,
					*output.Output,
					*output.Pretty,
					func() error { return renderPrivacyApplyTable(payload) },
					func() error { return renderPrivacyApplyMarkdown(payload) },
				); renderErr != nil {
					return renderErr
				}
				cause := withWebAuthHint(applyErr, "web privacy apply")
				message := privacyApplyFailureMessage(resolvedAppID, payload, cause, recheckErr)
				fmt.Fprintf(os.Stderr, "Error: %s\n", shared.SanitizeTerminal(message))
				return shared.NewStderrReportedError(
					shared.NewErrorWithCause(fmt.Errorf("%s", message), cause),
				)
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
		ShortHelp:  "Publish app privacy declarations.",
		LongHelp: `WEB SESSION WORKFLOWS

Explicitly publish app data usage declarations after apply.

Requires a publish-state id before PATCHing and exits non-zero unless the
response confirms published=true. Unknown or omitted publication state is
never reported as success.

Examples:
  asc web privacy publish --app "123456789" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			stateBefore, err := withWebSpinnerValue("Loading privacy publish state", func() (*webcore.AppDataUsagesPublishState, error) {
				return client.GetAppDataUsagesPublishState(requestCtx, resolvedAppID)
			})
			if err != nil {
				return withWebAuthHint(err, "web privacy publish")
			}
			if stateBefore == nil {
				return fmt.Errorf("web privacy publish: publish state is missing")
			}
			stateAfter := stateBefore
			if !stateBefore.PublishedKnown || !stateBefore.Published {
				if strings.TrimSpace(stateBefore.ID) == "" {
					return fmt.Errorf("web privacy publish: publish-state id is missing; cannot publish")
				}
				stateAfter, err = withWebSpinnerValue("Publishing app privacy declarations", func() (*webcore.AppDataUsagesPublishState, error) {
					return client.SetAppDataUsagesPublished(requestCtx, stateBefore.ID, true)
				})
				if err != nil {
					return withWebAuthHint(err, "web privacy publish")
				}
			}
			if stateAfter == nil || !stateAfter.PublishedKnown || !stateAfter.Published {
				return fmt.Errorf("web privacy publish: publication could not be verified")
			}

			payload := privacyPublishOutput{
				AppID:        resolvedAppID,
				PublishState: privacyPublishStateFromRemote(stateAfter),
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
