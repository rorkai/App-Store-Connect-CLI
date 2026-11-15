package signing

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	signingResignKeychainGroupsEntitlement    = "keychain-access-groups"
	signingResignKVStoreEntitlement           = "com.apple.developer.ubiquity-kvstore-identifier"
	signingResignParentEntitlement            = "com.apple.developer.parent-application-identifiers"
	signingResignAssociatedAppClipEntitlement = "com.apple.developer.associated-appclip-app-identifiers"
)

// signingResignEntitlementRewrite is an internal, target-local record. The
// public receipt adds the target path and bundle identifier when the plan is
// converted to output.
type signingResignEntitlementRewrite struct {
	Key   string
	Index *int
	From  any
	To    any
}

type signingResignTargetEntitlementPlan struct {
	Entitlements     map[string]any
	EntitlementsData []byte
	Rewrites         []signingResignEntitlementRewrite
}

// signingResignRebaseGraph contains only facts established during immutable
// preflight. In particular, destination prefixes come from each profile's
// ApplicationIdentifierPrefix and never from TeamID.
type signingResignRebaseGraph struct {
	TargetByBundle      map[string]signingResignTarget
	SourcePrefixes      map[string]string
	DestinationPrefixes map[string]string
	KeychainMapping     map[string]string
	KVStoreMapping      map[string]string
	AppClipMapping      map[string]string
	MainBundleID        string
}

func planSigningResignEntitlements(archive signingResignArchive, profiles map[string]signingResignProfile, rebaseTeamClaims bool) ([]signingResignTargetEntitlementPlan, error) {
	if len(archive.Targets) == 0 {
		return nil, fmt.Errorf("IPA contains no app-like targets")
	}
	if err := validateSigningResignTargetIDs(archive.Targets); err != nil {
		return nil, err
	}
	var graph *signingResignRebaseGraph
	if rebaseTeamClaims {
		var err error
		graph, err = buildSigningResignRebaseGraph(archive, profiles)
		if err != nil {
			return nil, err
		}
	}
	plans := make([]signingResignTargetEntitlementPlan, len(archive.Targets))
	for index, target := range archive.Targets {
		profile, ok := profiles[target.BundleID]
		if !ok {
			return nil, fmt.Errorf("missing profile for target %s", target.BundleID)
		}
		entitlements, rewrites, err := buildSigningResignEntitlementPlan(target.ExistingEntitlements, profile.Entitlements, profile.Class, target.BundleID, graph)
		if err != nil {
			return nil, wrapSigningResignPublicDetail(fmt.Sprintf("target %s entitlements", target.BundleID), err)
		}
		entitlementsData, err := marshalSigningResignEntitlements(entitlements)
		if err != nil {
			return nil, fmt.Errorf("target %s entitlements: %w", target.BundleID, err)
		}
		plans[index] = signingResignTargetEntitlementPlan{
			Entitlements:     entitlements,
			EntitlementsData: entitlementsData,
			Rewrites:         rewrites,
		}
	}
	return plans, nil
}

func buildSigningResignEntitlementPlan(existing, profile map[string]any, profileClass, bundleID string, graph *signingResignRebaseGraph) (map[string]any, []signingResignEntitlementRewrite, error) {
	if profile == nil {
		return nil, nil, fmt.Errorf("profile entitlements are missing")
	}
	for _, key := range []string{"application-identifier", "com.apple.application-identifier"} {
		if value, exists := profile[key]; exists {
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) != text || strings.ContainsRune(text, '*') {
				return nil, nil, fmt.Errorf("replacement profile entitlement %s must be an exact value", key)
			}
		}
	}
	for _, key := range []string{"application-identifier", "com.apple.application-identifier"} {
		if value, exists := existing[key]; exists {
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" || strings.ContainsRune(text, '*') {
				return nil, nil, fmt.Errorf("existing entitlement %s is invalid", key)
			}
		}
	}
	if value, exists := existing["com.apple.developer.team-identifier"]; exists {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, nil, fmt.Errorf("existing entitlement %s is invalid", "com.apple.developer.team-identifier")
		}
	}
	if value, exists := existing["get-task-allow"]; exists {
		if _, ok := value.(bool); !ok {
			return nil, nil, fmt.Errorf("existing entitlement get-task-allow is invalid")
		}
	}
	result := make(map[string]any, len(existing)+4)
	existingKeys := make([]string, 0, len(existing))
	for key := range existing {
		existingKeys = append(existingKeys, key)
	}
	sort.Strings(existingKeys)
	var unauthorized []signingResignUnauthorizedClaim
	var rewrites []signingResignEntitlementRewrite
	for _, key := range existingKeys {
		value := existing[key]
		if _, identityKey := signingResignIdentityEntitlementKeys[key]; identityKey {
			profileValue, exists := profile[key]
			if !exists {
				unauthorized = append(unauthorized, signingResignUnauthorizedClaim{Key: key, Existing: value})
				continue
			}
			resolvedValue := value
			if graph != nil {
				changed := false
				if key == signingResignParentEntitlement || key == signingResignAssociatedAppClipEntitlement {
					if _, err := signingResignAppClipRelationshipList(value, key); err != nil {
						return nil, nil, err
					}
				}
				if signingResignRebaseAllowlistedKey(key) {
					var transformed any
					var didChange bool
					var err error
					if (key == signingResignParentEntitlement || key == signingResignAssociatedAppClipEntitlement) && !graph.appClipMappingExists(value) {
						transformed = value
					} else {
						transformed, didChange, err = graph.rebaseClaim(key, value)
					}
					if err != nil {
						return nil, nil, err
					}
					changed = didChange
					if changed {
						// Every actual rebase must be anchored to the source
						// target's signed application identifier. KVS has its own
						// transfer-aware destination mapping, but it still cannot
						// be rebased from a target with no source identity.
						if _, err := graph.sourcePrefix(bundleID); err != nil {
							return nil, nil, err
						}
						resolvedValue = transformed
						var err error
						rewrites, err = appendSigningResignEntitlementRewrites(rewrites, key, value, transformed)
						if err != nil {
							return nil, nil, err
						}
					}
				}
				profilePermits := true
				if signingResignRebaseProfileKey(key) {
					profilePermits = signingResignStrictEntitlementValuePermits(profileValue, resolvedValue)
					if changed && key != signingResignKVStoreEntitlement && key != signingResignParentEntitlement && key != signingResignAssociatedAppClipEntitlement {
						profilePermits = signingResignRebaseProfilePermits(profileValue, value, resolvedValue, graph.DestinationPrefixes[bundleID])
					}
				}
				if !profilePermits {
					unauthorized = append(unauthorized, signingResignUnauthorizedClaim{Key: key, Existing: resolvedValue, Profile: profileValue})
					continue
				}
			}
			resolved, err := resolveSigningResignIdentityEntitlement(key, resolvedValue, profileValue)
			if err != nil {
				var claimErr signingResignClaimUnauthorizedError
				if errors.As(err, &claimErr) {
					unauthorized = append(unauthorized, signingResignUnauthorizedClaim{Key: key, Existing: resolvedValue, Profile: profileValue})
					continue
				}
				return nil, nil, err
			}
			result[key] = cloneSigningResignEntitlementValue(resolved)
			continue
		}
		profileValue, permitted := profile[key]
		if profileClass != "" {
			if resolved, handled, err := resolveSigningResignProfileClassEntitlement(key, profileClass, value, true, profileValue, permitted); handled {
				if err != nil {
					return nil, nil, err
				}
				result[key] = cloneSigningResignEntitlementValue(resolved)
				continue
			}
		}
		if graph != nil {
			if key == signingResignAssociatedAppClipEntitlement {
				if _, err := signingResignAppClipRelationshipList(value, key); err != nil {
					return nil, nil, err
				}
			}
			// Rebasing is opt-in and must validate the complete profile
			// authorization grammar; the legacy no-flag path remains unchanged.
			permitted = permitted && signingResignStrictEntitlementValuePermits(profileValue, value)
		} else {
			permitted = permitted && signingResignEntitlementValuePermits(profileValue, value)
		}
		if !permitted {
			unauthorized = append(unauthorized, signingResignUnauthorizedClaim{Key: key, Existing: value, Profile: profileValue})
			continue
		}
		result[key] = cloneSigningResignEntitlementValue(value)
	}
	sort.SliceStable(rewrites, func(left, right int) bool {
		first, second := rewrites[left], rewrites[right]
		firstRank, secondRank := signingResignEntitlementRewriteKeyRank(first.Key), signingResignEntitlementRewriteKeyRank(second.Key)
		if firstRank != secondRank {
			return firstRank < secondRank
		}
		if first.Key != second.Key {
			return first.Key < second.Key
		}
		if (first.Index == nil) != (second.Index == nil) {
			return first.Index == nil
		}
		if first.Index != nil && second.Index != nil && *first.Index != *second.Index {
			return *first.Index < *second.Index
		}
		firstFrom, secondFrom := signingResignRewriteValueSortKey(first.From), signingResignRewriteValueSortKey(second.From)
		if firstFrom != secondFrom {
			return firstFrom < secondFrom
		}
		return signingResignRewriteValueSortKey(first.To) < signingResignRewriteValueSortKey(second.To)
	})
	if len(unauthorized) > 0 {
		return nil, nil, signingResignUnauthorizedClaimsError(unauthorized)
	}
	for _, key := range signingResignProfileRequiredEntitlementKeyOrder {
		if _, exists := existing[key]; exists {
			continue
		}
		value, exists := profile[key]
		if !exists {
			continue
		}
		if _, isBool := value.(bool); !isBool {
			return nil, nil, fmt.Errorf("replacement profile entitlement %s is not a concrete boolean value", key)
		}
		result[key] = cloneSigningResignEntitlementValue(value)
	}
	for _, key := range signingResignIdentityEntitlementKeyOrder {
		if _, exists := existing[key]; exists {
			continue
		}
		if signingResignOptionalIdentityEntitlementKey(key) {
			// Optional identity capabilities are granted only when the
			// existing signature already claims them. The profile value,
			// wildcard or concrete, is an authorization boundary: signing an
			// unclaimed capability in would widen the app's access.
			continue
		}
		value, exists := profile[key]
		if !exists {
			return nil, nil, fmt.Errorf("replacement profile entitlement %s is missing", key)
		}
		if signingResignEntitlementContainsWildcard(value) {
			return nil, nil, fmt.Errorf("replacement profile entitlement %s is wildcard-only and has no concrete signed value", key)
		}
		result[key] = cloneSigningResignEntitlementValue(value)
	}
	return result, rewrites, nil
}

// appendSigningResignEntitlementRewrites flattens list-valued claims into one
// receipt per element. The optional index distinguishes a list element from a
// scalar rewrite while preserving the original order and duplicates.
func appendSigningResignEntitlementRewrites(existing []signingResignEntitlementRewrite, key string, from, to any) ([]signingResignEntitlementRewrite, error) {
	fromList, fromIsList := signingResignEntitlementList(from)
	toList, toIsList := signingResignEntitlementList(to)
	if fromIsList || toIsList {
		if !fromIsList || !toIsList || len(fromList) != len(toList) {
			return nil, fmt.Errorf("rebased entitlement %s changed its value shape", key)
		}
		for index := range fromList {
			if signingResignEntitlementValuesEqual(fromList[index], toList[index]) {
				continue
			}
			elementIndex := index
			existing = append(existing, signingResignEntitlementRewrite{
				Key:   key,
				Index: &elementIndex,
				From:  cloneSigningResignEntitlementValue(fromList[index]),
				To:    cloneSigningResignEntitlementValue(toList[index]),
			})
		}
		return existing, nil
	}
	return append(existing, signingResignEntitlementRewrite{
		Key:  key,
		From: cloneSigningResignEntitlementValue(from),
		To:   cloneSigningResignEntitlementValue(to),
	}), nil
}

func cloneSigningResignEntitlementValue(value any) any {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneSigningResignEntitlementValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = cloneSigningResignEntitlementValue(item)
		}
		return result
	default:
		return value
	}
}

func signingResignRebaseAllowlistedKey(key string) bool {
	switch key {
	case signingResignKeychainGroupsEntitlement, signingResignKVStoreEntitlement,
		signingResignParentEntitlement, signingResignAssociatedAppClipEntitlement:
		return true
	default:
		return false
	}
}

func signingResignRebaseProfileKey(key string) bool {
	switch key {
	case signingResignKeychainGroupsEntitlement, signingResignKVStoreEntitlement,
		signingResignParentEntitlement, signingResignAssociatedAppClipEntitlement:
		return true
	default:
		return false
	}
}

// signingResignStrictTerminalWildcardPrefix is used only by the opt-in
// planner. The legacy matcher intentionally retains its historical wildcard
// behavior for no-flag compatibility; rebasing accepts only one terminal '*'
// after a non-empty dotted prefix.
func signingResignStrictTerminalWildcardPrefix(value string) (string, bool) {
	if !strings.HasSuffix(value, "*") {
		return "", false
	}
	prefix := strings.TrimSuffix(value, "*")
	if prefix == "" || !strings.HasSuffix(prefix, ".") || len(prefix) == 1 || strings.ContainsRune(prefix, '*') || signingResignClaimStringHasInvalidCharacters(prefix) {
		return "", false
	}
	return prefix, true
}

func signingResignStrictProfileString(value string) bool {
	if strings.TrimSpace(value) != value || value == "" || signingResignClaimStringHasInvalidCharacters(value) {
		return false
	}
	if strings.ContainsRune(value, '*') {
		_, ok := signingResignStrictTerminalWildcardPrefix(value)
		return ok
	}
	return true
}

func signingResignStrictConcreteString(value string) bool {
	return signingResignStrictProfileString(value) && !strings.ContainsRune(value, '*')
}

func signingResignStrictPrefixedConcreteString(value string) bool {
	if !signingResignStrictConcreteString(value) {
		return false
	}
	separator := strings.IndexByte(value, '.')
	if separator <= 0 || separator == len(value)-1 {
		return false
	}
	return validateSigningResignTeamID(value[:separator]) == nil
}

// signingResignStrictEntitlementValuePermits validates every profile entry
// before matching any candidate. This prevents an invalid array entry from
// being skipped just because another entry happens to authorize the value.
func signingResignStrictEntitlementValuePermits(profileValue, signedValue any) bool {
	// Exact equality is the strongest authorization boundary. This preserves
	// literal wildcard strings and structured arrays without interpreting them
	// as patterns.
	if reflect.DeepEqual(profileValue, signedValue) {
		return true
	}
	profileString, profileIsString := profileValue.(string)
	signedString, signedIsString := signedValue.(string)
	if profileIsString || signedIsString {
		if !profileIsString || !signedIsString {
			return false
		}
		if profileString == signedString {
			return true
		}
		if !signingResignStrictProfileString(profileString) || !signingResignStrictConcreteString(signedString) {
			return false
		}
		if strings.ContainsRune(profileString, '*') {
			prefix, ok := signingResignStrictTerminalWildcardPrefix(profileString)
			return ok && strings.HasPrefix(signedString, prefix) && len(signedString) > len(prefix)
		}
		return signedString == profileString
	}
	// Non-string scalar claims retain exact authorization semantics. Only
	// string/list claims use the wildcard grammar below; do not reject valid
	// boolean or dictionary entitlement values merely because they are not
	// rebasable strings.
	if _, profileIsList := signingResignEntitlementList(profileValue); !profileIsList {
		_, signedIsList := signingResignEntitlementList(signedValue)
		return !signedIsList && reflect.DeepEqual(profileValue, signedValue)
	}
	profileList, profileIsList := signingResignEntitlementList(profileValue)
	signedList, signedIsList := signingResignEntitlementList(signedValue)
	if !profileIsList || !signedIsList || len(profileList) == 0 || len(signedList) == 0 {
		return false
	}
	for _, profileItem := range profileList {
		profileText, ok := profileItem.(string)
		if !ok {
			return reflect.DeepEqual(profileValue, signedValue)
		}
		if !signingResignStrictProfileString(profileText) {
			return false
		}
	}
	for _, signedItem := range signedList {
		signedText, ok := signedItem.(string)
		if !ok || !signingResignStrictConcreteString(signedText) {
			return false
		}
		permitted := false
		for _, profileItem := range profileList {
			profileText := profileItem.(string)
			if strings.ContainsRune(profileText, '*') {
				prefix, ok := signingResignStrictTerminalWildcardPrefix(profileText)
				if !ok || !strings.HasPrefix(signedText, prefix) || len(signedText) <= len(prefix) {
					continue
				}
				permitted = true
				break
			}
			if profileText == signedText {
				permitted = true
				break
			}
		}
		if !permitted {
			return false
		}
	}
	return true
}

func signingResignRebaseProfilePermits(profileValue, existingValue, plannedValue any, destinationPrefix string) bool {
	if !signingResignStrictEntitlementValuePermits(profileValue, plannedValue) {
		return false
	}

	// A profile may authorize a mixed keychain list with both destination-
	// prefixed values that were rebased and concrete values from another
	// namespace that were preserved. Only a wildcard that is actually used to
	// authorize a changed value must be rooted at this target's destination
	// prefix; a wildcard used solely by an unchanged value is independent.
	existingList, existingIsList := signingResignEntitlementList(existingValue)
	plannedList, plannedIsList := signingResignEntitlementList(plannedValue)
	if existingIsList || plannedIsList {
		if !existingIsList || !plannedIsList || len(existingList) != len(plannedList) {
			return false
		}
		for index, plannedItem := range plannedList {
			if signingResignEntitlementValuesEqual(existingList[index], plannedItem) {
				continue
			}
			plannedText, ok := plannedItem.(string)
			if !ok || !signingResignRebaseProfileAuthorizesChangedValue(profileValue, plannedText, destinationPrefix) {
				return false
			}
		}
		return true
	}
	plannedText, ok := plannedValue.(string)
	return ok && signingResignRebaseProfileAuthorizesChangedValue(profileValue, plannedText, destinationPrefix)
}

func signingResignRebaseProfileAuthorizesChangedValue(profileValue any, plannedValue, destinationPrefix string) bool {
	for _, profileText := range signingResignProfileStrings(profileValue) {
		if strings.ContainsRune(profileText, '*') {
			prefix, ok := signingResignStrictTerminalWildcardPrefix(profileText)
			if !ok || !strings.HasPrefix(plannedValue, prefix) || len(plannedValue) <= len(prefix) {
				continue
			}
			if !strings.HasPrefix(prefix, destinationPrefix+".") {
				continue
			}
			return true
		}
		if profileText == plannedValue {
			return true
		}
	}
	return false
}

func signingResignProfileStrings(value any) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}
	list, ok := signingResignEntitlementList(value)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func buildSigningResignRebaseGraph(archive signingResignArchive, profiles map[string]signingResignProfile) (*signingResignRebaseGraph, error) {
	graph := &signingResignRebaseGraph{
		TargetByBundle:      make(map[string]signingResignTarget, len(archive.Targets)),
		SourcePrefixes:      make(map[string]string, len(archive.Targets)),
		DestinationPrefixes: make(map[string]string, len(archive.Targets)),
		KeychainMapping:     make(map[string]string),
		KVStoreMapping:      make(map[string]string),
		AppClipMapping:      make(map[string]string),
	}
	for _, target := range archive.Targets {
		if _, exists := graph.TargetByBundle[target.BundleID]; exists {
			return nil, fmt.Errorf("duplicate bundle identifier in IPA targets: %s", target.BundleID)
		}
		graph.TargetByBundle[target.BundleID] = target
		if target.RelativePath == archive.MainPath {
			if graph.MainBundleID != "" {
				return nil, fmt.Errorf("IPA has multiple main application targets")
			}
			graph.MainBundleID = target.BundleID
		}
		profile, ok := profiles[target.BundleID]
		if !ok {
			return nil, fmt.Errorf("missing profile for target %s", target.BundleID)
		}
		prefix, err := signingResignProfilePrefix(profile, target.BundleID)
		if err != nil {
			return nil, fmt.Errorf("profile for target %s: %w", target.BundleID, err)
		}
		graph.DestinationPrefixes[target.BundleID] = prefix
	}
	if graph.MainBundleID == "" {
		return nil, fmt.Errorf("IPA main application target is missing from entitlement graph")
	}
	if graph.TargetByBundle[graph.MainBundleID].Kind != "application" {
		return nil, fmt.Errorf("IPA main entitlement graph target is not an application")
	}
	if err := buildSigningResignKeychainMapping(graph, profiles); err != nil {
		return nil, err
	}
	if err := buildSigningResignKVStoreMapping(graph, profiles); err != nil {
		return nil, err
	}
	if err := buildSigningResignAppClipMapping(graph); err != nil {
		return nil, err
	}
	return graph, nil
}

func buildSigningResignAppClipMapping(graph *signingResignRebaseGraph) error {
	var associatedCount, parentCount int
	invalid := false
	associatedEdges := make(map[string]struct{})
	parentEdges := make(map[string]struct{})
	for bundleID, target := range graph.TargetByBundle {
		for _, key := range []string{signingResignParentEntitlement, signingResignAssociatedAppClipEntitlement} {
			value, ok := target.ExistingEntitlements[key]
			if !ok {
				continue
			}
			values, err := signingResignAppClipRelationshipList(value, key)
			if err != nil {
				return err
			}
			for _, old := range values {
				parts := strings.SplitN(old, ".", 2)
				if len(parts) != 2 {
					return fmt.Errorf("app clip entitlement %s contains an invalid relationship reference", key)
				}
				referenced, exists := graph.TargetByBundle[parts[1]]
				if !exists {
					invalid = true
					continue
				}
				if key == signingResignAssociatedAppClipEntitlement && (target.BundleID != graph.MainBundleID || target.Kind != "application" || referenced.Kind != "app-clip") {
					invalid = true
					continue
				}
				if key == signingResignParentEntitlement && (target.Kind != "app-clip" || referenced.Kind != "application" || referenced.RelativePath != graph.TargetByBundle[graph.MainBundleID].RelativePath) {
					invalid = true
					continue
				}
				oldPrefix, err := graph.sourcePrefix(referenced.BundleID)
				if err != nil {
					invalid = true
					continue
				}
				if parts[0] != oldPrefix {
					invalid = true
					continue
				}
				counterpartKey := signingResignParentEntitlement
				if key == counterpartKey {
					counterpartKey = signingResignAssociatedAppClipEntitlement
				}
				counterpart, exists := referenced.ExistingEntitlements[counterpartKey]
				if !exists {
					invalid = true
					continue
				}
				counterpartValues, err := signingResignAppClipRelationshipList(counterpart, counterpartKey)
				if err != nil {
					invalid = true
					continue
				}
				source, sourceErr := graph.sourcePrefix(bundleID)
				if sourceErr != nil {
					invalid = true
					continue
				}
				expectedReciprocal := source + "." + bundleID
				reciprocal := false
				for _, candidate := range counterpartValues {
					if candidate == expectedReciprocal {
						reciprocal = true
						break
					}
				}
				if !reciprocal {
					invalid = true
					continue
				}
				if key == signingResignAssociatedAppClipEntitlement {
					edge := target.BundleID + "\x00" + referenced.BundleID
					if _, seen := associatedEdges[edge]; !seen {
						associatedEdges[edge] = struct{}{}
						associatedCount++
					}
					if associatedCount > 1 {
						invalid = true
						continue
					}
				} else {
					edge := target.BundleID + "\x00" + referenced.BundleID
					if _, seen := parentEdges[edge]; !seen {
						parentEdges[edge] = struct{}{}
						parentCount++
					}
					if parentCount > 1 {
						invalid = true
						continue
					}
				}
				graph.AppClipMapping[old] = graph.DestinationPrefixes[parts[1]] + "." + parts[1]
			}
		}
	}
	if invalid {
		clear(graph.AppClipMapping)
	}
	return nil
}

func (graph *signingResignRebaseGraph) appClipMappingExists(value any) bool {
	values, err := signingResignAppClipRelationshipList(value, "App Clip relationship")
	if err != nil {
		return false
	}
	for _, value := range values {
		if _, ok := graph.AppClipMapping[value]; !ok {
			return false
		}
	}
	return true
}

// buildSigningResignKeychainMapping chooses one planned value for every
// distinct full keychain group in the IPA. The mapping is graph-wide: two
// targets that carry the same old group must not silently receive different
// destinations. An exact existing value already authorized by a replacement
// profile is retained, including when its prefix is unrelated to either app
// prefix. Otherwise the value must be rebased from that target's source app
// prefix; profile authorization for the resulting value is checked again in
// buildSigningResignEntitlementPlan.
func buildSigningResignKeychainMapping(graph *signingResignRebaseGraph, profiles map[string]signingResignProfile) error {
	type claim struct {
		bundleID     string
		oldValue     string
		profileValue any
	}
	bundleIDs := make([]string, 0, len(graph.TargetByBundle))
	for bundleID := range graph.TargetByBundle {
		bundleIDs = append(bundleIDs, bundleID)
	}
	sort.Strings(bundleIDs)
	claimsByOldValue := make(map[string][]claim)
	for _, bundleID := range bundleIDs {
		target := graph.TargetByBundle[bundleID]
		value, exists := target.ExistingEntitlements[signingResignKeychainGroupsEntitlement]
		if !exists {
			continue
		}
		values, err := signingResignConcreteStringList(value, signingResignKeychainGroupsEntitlement)
		if err != nil {
			return err
		}
		profile, ok := profiles[bundleID]
		if !ok {
			return fmt.Errorf("missing profile for target %s", bundleID)
		}
		profileValue, ok := profile.Entitlements[signingResignKeychainGroupsEntitlement]
		if !ok {
			return fmt.Errorf("replacement profile entitlement %s is missing for target %s", signingResignKeychainGroupsEntitlement, bundleID)
		}
		for _, oldValue := range values {
			claimsByOldValue[oldValue] = append(claimsByOldValue[oldValue], claim{
				bundleID:     bundleID,
				oldValue:     oldValue,
				profileValue: profileValue,
			})
		}
	}
	if len(claimsByOldValue) == 0 {
		return nil
	}
	oldValues := make([]string, 0, len(claimsByOldValue))
	for oldValue := range claimsByOldValue {
		oldValues = append(oldValues, oldValue)
	}
	sort.Strings(oldValues)
	plannedOwners := make(map[string]string, len(oldValues))
	for _, oldValue := range oldValues {
		candidates := make(map[string]struct{})
		for _, item := range claimsByOldValue[oldValue] {
			if signingResignKeychainProfilePreservesExisting(item.profileValue, oldValue) {
				candidates[oldValue] = struct{}{}
				continue
			}
			sourcePrefix, err := graph.sourcePrefix(item.bundleID)
			if err != nil {
				return err
			}
			destinationPrefix := graph.DestinationPrefixes[item.bundleID]
			plannedValue, err := signingResignRebasePrefixedValue(oldValue, signingResignKeychainGroupsEntitlement, sourcePrefix, destinationPrefix)
			if err != nil {
				return fmt.Errorf("target %s keychain groups: %w", item.bundleID, err)
			}
			candidates[plannedValue] = struct{}{}
		}
		if len(candidates) != 1 {
			return fmt.Errorf("replacement profile entitlements must choose one planned full keychain value for existing value %q", oldValue)
		}
		for plannedValue := range candidates {
			if priorOldValue, exists := plannedOwners[plannedValue]; exists && priorOldValue != oldValue {
				return fmt.Errorf("replacement profile entitlements must map distinct full keychain values one-to-one: %q and %q both resolve to %q", priorOldValue, oldValue, plannedValue)
			}
			plannedOwners[plannedValue] = oldValue
			graph.KeychainMapping[oldValue] = plannedValue
		}
	}
	return nil
}

// signingResignKeychainProfilePreservesExisting accepts only an exact
// concrete authorization or a non-empty, terminal wildcard that explicitly
// covers the existing group. A bare "*" is intentionally not enough to
// preserve an old keychain namespace; rebased values must be authorized under
// the replacement prefix.
func signingResignKeychainProfilePreservesExisting(profileValue any, existing string) bool {
	if !signingResignStrictPrefixedConcreteString(existing) {
		return false
	}
	var candidates []string
	switch typed := profileValue.(type) {
	case string:
		candidates = []string{typed}
	case []string:
		candidates = append([]string(nil), typed...)
	case []any:
		candidates = make([]string, len(typed))
		for index, item := range typed {
			text, ok := item.(string)
			if !ok {
				return false
			}
			candidates[index] = text
		}
	default:
		return false
	}
	if len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if !signingResignStrictProfileString(candidate) {
			return false
		}
	}
	for _, candidate := range candidates {
		if signingResignStrictEntitlementValuePermits(candidate, existing) {
			return true
		}
	}
	return false
}

// buildSigningResignKVStoreMapping chooses one mapping per distinct full KVS
// value in the IPA. KVS has its own prefix syntax; its prefix may differ from
// the application-identifier prefix. Preserve an exact existing value when
// every relevant profile authorizes it. Otherwise, require one exact
// replacement-profile value with the same validated suffix. A wildcard may
// authorize a chosen exact value, but it never supplies the destination.
func buildSigningResignKVStoreMapping(graph *signingResignRebaseGraph, profiles map[string]signingResignProfile) error {
	type claim struct {
		bundleID     string
		oldValue     string
		suffix       string
		profileValue any
	}
	bundleIDs := make([]string, 0, len(graph.TargetByBundle))
	for bundleID := range graph.TargetByBundle {
		bundleIDs = append(bundleIDs, bundleID)
	}
	sort.Strings(bundleIDs)
	claimsByOldValue := make(map[string][]claim)
	for _, bundleID := range bundleIDs {
		target := graph.TargetByBundle[bundleID]
		value, exists := target.ExistingEntitlements[signingResignKVStoreEntitlement]
		if !exists {
			continue
		}
		oldValue, suffix, err := signingResignKVSValueParts(value, signingResignKVStoreEntitlement)
		if err != nil {
			return err
		}
		profile, ok := profiles[bundleID]
		if !ok {
			return fmt.Errorf("missing profile for target %s", bundleID)
		}
		profileValue, ok := profile.Entitlements[signingResignKVStoreEntitlement]
		if !ok {
			return fmt.Errorf("replacement profile entitlement %s is missing for target %s", signingResignKVStoreEntitlement, bundleID)
		}
		claimsByOldValue[oldValue] = append(claimsByOldValue[oldValue], claim{bundleID: bundleID, oldValue: oldValue, suffix: suffix, profileValue: profileValue})
	}
	if len(claimsByOldValue) == 0 {
		return nil
	}
	oldValues := make([]string, 0, len(claimsByOldValue))
	for oldValue := range claimsByOldValue {
		oldValues = append(oldValues, oldValue)
	}
	sort.Strings(oldValues)
	plannedOwners := make(map[string]string, len(oldValues))
	for _, oldValue := range oldValues {
		claims := claimsByOldValue[oldValue]
		suffix := claims[0].suffix
		for _, item := range claims[1:] {
			if item.suffix != suffix {
				return fmt.Errorf("existing %s value %q has inconsistent validated suffixes", signingResignKVStoreEntitlement, oldValue)
			}
		}
		candidates := make(map[string]struct{})
		needsCandidate := false
		for _, item := range claims {
			candidate, exact, err := signingResignExactKVSProfileValue(item.profileValue, suffix)
			if err != nil {
				return fmt.Errorf("replacement profile %s for target %s: %w", signingResignKVStoreEntitlement, item.bundleID, err)
			}
			if signingResignStrictEntitlementValuePermits(item.profileValue, oldValue) {
				candidates[oldValue] = struct{}{}
				continue
			}
			if !exact {
				needsCandidate = true
				continue
			}
			candidates[candidate] = struct{}{}
		}
		if len(candidates) > 1 {
			return fmt.Errorf("replacement profile entitlements must choose one planned full KVS value for existing value %q", oldValue)
		}
		if len(candidates) == 0 {
			if needsCandidate {
				return fmt.Errorf("replacement profile KVS wildcard cannot choose one planned full KVS value for existing value %q", oldValue)
			}
			graph.KVStoreMapping[oldValue] = oldValue
			continue
		}
		var plannedValue string
		for value := range candidates {
			plannedValue = value
		}
		if priorOldValue, exists := plannedOwners[plannedValue]; exists && priorOldValue != oldValue {
			return fmt.Errorf("replacement profile entitlements must map distinct full KVS values one-to-one: %q and %q both resolve to %q", priorOldValue, oldValue, plannedValue)
		}
		plannedOwners[plannedValue] = oldValue
		for _, item := range claims {
			if !signingResignStrictEntitlementValuePermits(item.profileValue, plannedValue) {
				return fmt.Errorf("replacement profile for target %s does not authorize one planned full KVS value %q", item.bundleID, plannedValue)
			}
		}
		graph.KVStoreMapping[oldValue] = plannedValue
	}
	return nil
}

func signingResignKVSValueParts(value any, key string) (string, string, error) {
	text, ok := value.(string)
	if !ok || !signingResignStrictConcreteString(text) {
		return "", "", fmt.Errorf("existing entitlement %s must be a concrete string", key)
	}
	separator := strings.IndexByte(text, '.')
	if separator <= 0 || separator == len(text)-1 {
		return "", "", fmt.Errorf("existing entitlement %s must contain a non-empty prefix and suffix", key)
	}
	prefix := text[:separator]
	if err := validateSigningResignTeamID(prefix); err != nil {
		return "", "", fmt.Errorf("existing entitlement %s has an invalid prefix", key)
	}
	suffix := text[separator+1:]
	if err := validateSigningResignBundleID(suffix); err != nil {
		return "", "", fmt.Errorf("existing entitlement %s has an invalid suffix", key)
	}
	return text, suffix, nil
}

func signingResignExactKVSProfileValue(value any, suffix string) (string, bool, error) {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) != text || text == "" || signingResignClaimStringHasInvalidCharacters(text) {
		return "", false, fmt.Errorf("must be one exact concrete string or a strict terminal wildcard")
	}
	if strings.ContainsRune(text, '*') {
		if _, ok := signingResignStrictTerminalWildcardPrefix(text); !ok {
			return "", false, fmt.Errorf("contains an invalid wildcard")
		}
		return "", false, nil
	}
	_, profileSuffix, err := signingResignKVSValueParts(text, signingResignKVStoreEntitlement)
	if err != nil || profileSuffix != suffix {
		return "", false, fmt.Errorf("must contain the validated suffix %q", suffix)
	}
	return text, true, nil
}

func signingResignProfilePrefix(profile signingResignProfile, bundleID string) (string, error) {
	applicationID, ok := profile.Entitlements["application-identifier"].(string)
	if !ok || strings.TrimSpace(applicationID) != applicationID {
		return "", fmt.Errorf("profile application identifier is missing or invalid")
	}
	parsedPrefix, err := signingResignApplicationIdentifierPrefix(applicationID, bundleID)
	if err != nil {
		return "", fmt.Errorf("profile application identifier: %w", err)
	}
	if profile.ApplicationIdentifierPrefix != "" && profile.ApplicationIdentifierPrefix != parsedPrefix {
		return "", fmt.Errorf("profile application identifier prefix does not match its application identifier")
	}
	return parsedPrefix, nil
}

func (graph *signingResignRebaseGraph) sourcePrefix(bundleID string) (string, error) {
	if prefix, ok := graph.SourcePrefixes[bundleID]; ok {
		return prefix, nil
	}
	target, ok := graph.TargetByBundle[bundleID]
	if !ok {
		return "", fmt.Errorf("entitlement graph target %s is missing", bundleID)
	}
	prefix, err := signingResignExistingApplicationIdentifierPrefix(target.ExistingEntitlements, target.sourceBundleID())
	if err != nil {
		return "", fmt.Errorf("target %s cannot derive its source ApplicationIdentifierPrefix: %w", bundleID, err)
	}
	graph.SourcePrefixes[bundleID] = prefix
	return prefix, nil
}

func signingResignExistingApplicationIdentifierPrefix(existing map[string]any, bundleID string) (string, error) {
	var prefix string
	for _, key := range []string{"application-identifier", "com.apple.application-identifier"} {
		value, exists := existing[key]
		if !exists {
			continue
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) != text {
			return "", fmt.Errorf("existing entitlement %s is invalid", key)
		}
		candidate, err := signingResignApplicationIdentifierPrefix(text, bundleID)
		if err != nil {
			return "", fmt.Errorf("existing entitlement %s is invalid: %w", key, err)
		}
		if prefix != "" && prefix != candidate {
			return "", fmt.Errorf("existing application identifiers are contradictory")
		}
		prefix = candidate
	}
	if prefix == "" {
		return "", fmt.Errorf("existing application-identifier is required for team claim rebasing")
	}
	return prefix, nil
}

func signingResignConcreteStringList(value any, key string) ([]string, error) {
	list, isList := signingResignEntitlementList(value)
	if !isList || len(list) == 0 {
		return nil, fmt.Errorf("existing entitlement %s must be a non-empty array of concrete strings", key)
	}
	result := make([]string, len(list))
	for index, item := range list {
		text, ok := item.(string)
		if !ok || !signingResignStrictPrefixedConcreteString(text) {
			return nil, fmt.Errorf("existing entitlement %s contains an invalid concrete value", key)
		}
		result[index] = text
	}
	return result, nil
}

// signingResignAppClipRelationshipList validates the bundle identifier suffix
// in each relationship reference before an unresolved edge can be preserved.
// The generic prefixed-string validator intentionally permits dotted values
// for claims such as keychain groups, but App Clip relationships must point to
// a concrete target bundle identifier.
func signingResignAppClipRelationshipList(value any, key string) ([]string, error) {
	values, err := signingResignConcreteStringList(value, key)
	if err != nil {
		return nil, err
	}
	for _, relationship := range values {
		separator := strings.IndexByte(relationship, '.')
		if separator <= 0 || separator == len(relationship)-1 {
			return nil, fmt.Errorf("existing entitlement %s contains an invalid relationship reference", key)
		}
		if err := validateSigningResignBundleID(relationship[separator+1:]); err != nil {
			return nil, fmt.Errorf("existing entitlement %s contains an invalid relationship suffix", key)
		}
	}
	return values, nil
}

func signingResignClaimStringHasControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || unicode.Is(unicode.Cf, character) || unicode.In(character, unicode.Bidi_Control) {
			return true
		}
	}
	return false
}

func signingResignClaimStringHasInvalidCharacters(value string) bool {
	if !utf8.ValidString(value) {
		return true
	}
	for _, character := range value {
		if unicode.IsSpace(character) || character == '/' || character == '\\' || character == 0 || signingResignClaimStringHasControl(string(character)) {
			return true
		}
	}
	return false
}

func signingResignListWithStringType(original any, values []string) any {
	switch original.(type) {
	case []string:
		return append([]string(nil), values...)
	default:
		result := make([]any, len(values))
		for index, value := range values {
			result[index] = value
		}
		return result
	}
}

func signingResignRebasePrefixedValue(value, key, sourcePrefix, destinationPrefix string) (string, error) {
	if !signingResignStrictPrefixedConcreteString(value) {
		return "", fmt.Errorf("existing entitlement %s must be a concrete string", key)
	}
	oldPrefix := sourcePrefix + "."
	if !strings.HasPrefix(value, oldPrefix) || len(value) <= len(oldPrefix) {
		return "", fmt.Errorf("existing entitlement %s contains a value outside its source ApplicationIdentifierPrefix", key)
	}
	return destinationPrefix + value[len(sourcePrefix):], nil
}

func (graph *signingResignRebaseGraph) rebaseClaim(key string, value any) (any, bool, error) {
	if key == signingResignKVStoreEntitlement {
		text, ok := value.(string)
		if !ok {
			return nil, false, fmt.Errorf("existing entitlement %s must be a concrete string", key)
		}
		transformed, ok := graph.KVStoreMapping[text]
		if !ok {
			return nil, false, fmt.Errorf("existing entitlement %s has no whole-IPA planned value", key)
		}
		return transformed, !signingResignEntitlementValuesEqual(value, transformed), nil
	}
	if key == signingResignKeychainGroupsEntitlement {
		values, err := signingResignConcreteStringList(value, key)
		if err != nil {
			return nil, false, err
		}
		transformed := make([]string, len(values))
		for index, oldValue := range values {
			plannedValue, ok := graph.KeychainMapping[oldValue]
			if !ok {
				return nil, false, fmt.Errorf("existing entitlement %s has no whole-IPA planned value", key)
			}
			transformed[index] = plannedValue
		}
		result := signingResignListWithStringType(value, transformed)
		return result, !signingResignEntitlementValuesEqual(value, result), nil
	}
	if key == signingResignParentEntitlement || key == signingResignAssociatedAppClipEntitlement {
		values, err := signingResignAppClipRelationshipList(value, key)
		if err != nil {
			return nil, false, err
		}
		transformed := make([]string, len(values))
		for index, old := range values {
			next, ok := graph.AppClipMapping[old]
			if !ok {
				return nil, false, fmt.Errorf("existing entitlement %s has no paired relationship mapping", key)
			}
			transformed[index] = next
		}
		result := signingResignListWithStringType(value, transformed)
		return result, !signingResignEntitlementValuesEqual(value, result), nil
	}
	return nil, false, fmt.Errorf("entitlement %s is not supported by entitlement rebasing", key)
}
