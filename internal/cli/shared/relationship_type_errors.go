package shared

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// RelationshipTypeValues renders the accepted --type values so flag help and
// both --type usage errors always agree on the list a links command accepts.
func RelationshipTypeValues(values []string) string {
	return strings.Join(values, ", ")
}

// RelationshipTypeFlagUsage renders the --type flag help for a links command
// so the help text enumerates every accepted relationship type.
func RelationshipTypeFlagUsage(values []string) string {
	return "Relationship type (required); must be one of: " + RelationshipTypeValues(values)
}

// MissingRelationshipTypeUsageError prints the missing --type diagnostic with
// every accepted relationship type and returns the shared missing-required
// usage error so the command exits with usage semantics.
func MissingRelationshipTypeUsageError(values []string) error {
	fmt.Fprintf(os.Stderr, "Error: --type is required; must be one of: %s\n", RelationshipTypeValues(values))
	return MissingRequiredUsageError("--type")
}

// PrintInvalidRelationshipTypeError prints the rejected --type value alongside
// every accepted relationship type. Callers return their own usage error so
// each links command keeps its existing error classification.
func PrintInvalidRelationshipTypeError(value string, values []string) {
	fmt.Fprintf(
		os.Stderr,
		"Error: --type %q is not a valid relationship type; must be one of: %s\n",
		value,
		RelationshipTypeValues(values),
	)
}

// RelationshipParent identifies the resource whose relationship linkages a
// links command requested, so a 404 can name what App Store Connect could not
// find.
type RelationshipParent struct {
	// ResourceType is Apple's resource type as it appears in a 404 detail,
	// such as "builds" or "betaGroups".
	ResourceType string
	// Label is the human-readable resource name used in the message, such as
	// "build" or "beta group".
	Label string
	// ID is the parent resource ID the request addressed. It is empty when a
	// next-page URL, not an ID flag, addressed the request.
	ID string
	// Hint is optional flag guidance appended to the unknown-parent message.
	Hint string
}

// DescribeRelationshipLookupFailure names the resource App Store Connect could
// not find so a 404 says whether the parent resource ID is unknown or the
// relationship linkage is missing. A 404 that names neither resource, and
// every other failure, is returned unchanged.
func DescribeRelationshipLookupFailure(err error, relationshipType string, parent RelationshipParent) error {
	if err == nil || !asc.IsNotFound(err) {
		return err
	}
	if parent.ResourceType != "" && asc.IsMissingResourceOfType(err, parent.ResourceType) {
		return NewErrorWithCause(errors.New(unknownRelationshipParentMessage(parent)), err)
	}
	if !namesRelationshipResource(err, relationshipType) {
		return err
	}
	if parent.ID == "" {
		return fmt.Errorf("%s relationship was not found: %w", relationshipType, err)
	}
	return fmt.Errorf("%s relationship was not found for %s %q: %w", relationshipType, parent.Label, parent.ID, err)
}

// unknownRelationshipParentMessage describes a parent resource App Store
// Connect does not know. Without an ID the request came from a next-page URL,
// so the message blames that URL instead of an ID flag.
func unknownRelationshipParentMessage(parent RelationshipParent) string {
	if parent.ID == "" {
		return fmt.Sprintf("the %s referenced by the requested page URL was not found", parent.Label)
	}
	message := fmt.Sprintf("%s %q was not found", parent.Label, parent.ID)
	if hint := strings.TrimSpace(parent.Hint); hint != "" {
		return message + "; " + hint
	}
	return message
}

// namesRelationshipResource reports whether Apple's 404 detail names the
// resource behind relationshipType, which Apple spells either exactly like the
// relationship or as its plural. Any other 404 keeps its original message so
// an unrelated failure is not relabeled as a missing relationship.
func namesRelationshipResource(err error, relationshipType string) bool {
	return asc.IsMissingResourceOfType(err, relationshipType) ||
		asc.IsMissingResourceOfType(err, relationshipType+"s")
}
