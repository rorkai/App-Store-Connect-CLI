package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's 409 body when the capability is already enabled on the bundle ID.
const capabilityExists409 = `{"errors":[{"id":"3c7e1f92-6b4d-4a58-9c2e-8d1f0a3b5c74","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"The capability is already enabled for this bundle ID.","source":{"pointer":"/data/attributes/capabilityType"}}]}`

// Apple's 409 for a capability type it will not accept. Not an existence
// conflict, so it must keep failing without a read-back.
const capabilityTypeRejected409 = `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.TYPE","title":"An attribute value has invalid type","detail":"The capability type is not supported for this bundle ID."}]}`

const existingCapabilities = `{"data":[{"type":"bundleIdCapabilities","id":"cap-icloud","attributes":{"capabilityType":"ICLOUD","settings":[]}},{"type":"bundleIdCapabilities","id":"cap-push","attributes":{"capabilityType":"PUSH_NOTIFICATIONS"}}],"links":{}}`

const noCapabilities = `{"data":[],"links":{}}`

// Apple's 409 body when the item is already on the review submission.
const reviewItemExists409 = `{"errors":[{"id":"9e4b2a17-3d8c-4f61-a5b0-7c2e1d9f8a43","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The appStoreVersion is already included in a review submission.","source":{"pointer":"/data/relationships/appStoreVersion"}}]}`

// A 409 that is not an existence conflict and must keep failing.
const reviewItemState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"The review submission is no longer editable."}]}`

const existingReviewItems = `{"data":[{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{}}`

// An item list whose appStoreVersion relationship is present but null, the
// shape Apple uses for a type the item does not carry.
// The requested version is on the submission, but as a REMOVED (detached)
// historical item, which is not proof the item is present.
const reviewItemsRemovedVersion = `{"data":[{"type":"reviewSubmissionItems","id":"item-removed","attributes":{"state":"REMOVED"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{}}`

const reviewItemsOtherVersion = `{"data":[{"type":"reviewSubmissionItems","id":"item-9","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-other"}},"inAppPurchaseVersion":{"data":null}}}],"links":{}}`

func TestCapabilitiesAddIfExistsSkipReturnsExistingCapability(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"bundle-ids", "capabilities", "add", "--bundle", "bundle-1",
		"--capability", "ICLOUD", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/bundleIdCapabilities":
			return jsonResponse(http.StatusConflict, capabilityExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return jsonResponse(http.StatusOK, existingCapabilities)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET", seen)
	}
	if !strings.Contains(stdout, `"id":"cap-icloud"`) || !strings.Contains(stdout, "ICLOUD") {
		t.Fatalf("stdout = %q, want the existing ICLOUD capability", stdout)
	}
	if strings.Contains(stdout, "cap-push") {
		t.Fatalf("stdout = %q, must report only the matching capability", stdout)
	}
	if !strings.Contains(stderr, "already enabled") || !strings.Contains(stderr, "cap-icloud") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want the already-exists diagnostic", stderr)
	}
}

func TestCapabilitiesAddIfExistsUpdatePatchesSettings(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"bundle-ids", "capabilities", "add", "--bundle", "bundle-1",
		"--capability", "ICLOUD",
		"--settings", `[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]`,
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/bundleIdCapabilities":
			return jsonResponse(http.StatusConflict, capabilityExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return jsonResponse(http.StatusOK, existingCapabilities)
		case req.Method == http.MethodPatch && req.Path == "/v1/bundleIdCapabilities/cap-icloud":
			if !strings.Contains(req.Body, "ICLOUD_VERSION") || !strings.Contains(req.Body, "XCODE_6") {
				t.Fatalf("PATCH body = %s, want the supplied settings", req.Body)
			}
			if strings.Contains(req.Body, "capabilityType") {
				t.Fatalf("PATCH body = %s, must not resend the create-only capabilityType", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"bundleIdCapabilities","id":"cap-icloud","attributes":{"capabilityType":"ICLOUD","settings":[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, read-back, PATCH", seen)
	}
	if !strings.Contains(stdout, "XCODE_6") {
		t.Fatalf("stdout = %q, want the PATCH response", stdout)
	}
	if !strings.Contains(stderr, "updated it in place") {
		t.Fatalf("stderr = %q, want the update outcome", stderr)
	}
}

// With no --settings there is nothing the PATCH can carry, so update resolves
// like skip instead of sending an empty PATCH.
func TestCapabilitiesAddIfExistsUpdateWithoutSettingsDoesNotPatch(t *testing.T) {
	_, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"bundle-ids", "capabilities", "add", "--bundle", "bundle-1",
		"--capability", "ICLOUD", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/bundleIdCapabilities":
			return jsonResponse(http.StatusConflict, capabilityExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return jsonResponse(http.StatusOK, existingCapabilities)
		default:
			t.Fatalf("unexpected request %s %s; an empty PATCH must not be sent", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET only", seen)
	}
	if !strings.Contains(stderr, "left unchanged") {
		t.Fatalf("stderr = %q, want the left-unchanged outcome", stderr)
	}
}

func TestCapabilitiesAddDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"bundle-ids", "capabilities", "add", "--bundle", "bundle-1",
		"--capability", "ICLOUD", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/bundleIdCapabilities" {
			return jsonResponse(http.StatusConflict, capabilityExists409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	if !strings.Contains(runErr.Error(), "The capability is already enabled for this bundle ID.") {
		t.Fatalf("run error = %v, want Apple's detail preserved", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestCapabilitiesAddIfExistsStillFailsForRejectedType(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"bundle-ids", "capabilities", "add", "--bundle", "bundle-1",
		"--capability", "ICLOUD", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/bundleIdCapabilities" {
			return jsonResponse(http.StatusConflict, capabilityTypeRejected409)
		}
		t.Fatalf("unexpected request %s %s; a rejected type must not read back", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the rejected-type conflict", runErr)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestCapabilitiesAddIfExistsSkipStillFailsWhenReadBackFindsNothing(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"bundle-ids", "capabilities", "add", "--bundle", "bundle-1",
		"--capability", "ICLOUD", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/bundleIdCapabilities":
			return jsonResponse(http.StatusConflict, capabilityExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return jsonResponse(http.StatusOK, noCapabilities)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original conflict", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET", seen)
	}
}

func TestReviewItemsAddIfExistsSkipReturnsExistingItem(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"review", "items-add", "--submission", "submission-1",
		"--item-type", "appStoreVersions", "--item-id", "version-1",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusConflict, reviewItemExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/reviewSubmissions/submission-1/items":
			// include= is what materializes the relationship; fields= alone
			// returns items with links only.
			if !strings.Contains(req.Query, "include=appStoreVersion") {
				t.Fatalf("read-back query = %q, want include=appStoreVersion", req.Query)
			}
			return jsonResponse(http.StatusOK, existingReviewItems)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET", seen)
	}
	if !strings.Contains(stdout, `"id":"item-1"`) {
		t.Fatalf("stdout = %q, want the existing item", stdout)
	}
	if !strings.Contains(stderr, "already on submission submission-1") || !strings.Contains(stderr, "item-1") {
		t.Fatalf("stderr = %q, want the already-exists diagnostic", stderr)
	}
}

// update has no meaning for a submission item, so it is a usage error listing
// only the modes this command supports.
// A REMOVED item is detached from the submission, so it must not satisfy the
// read-back: the conflict is surfaced instead of a false success.
func TestReviewItemsAddIfExistsSkipIgnoresRemovedItems(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "items-add", "--submission", "submission-1",
		"--item-type", "appStoreVersions", "--item-id", "version-1",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusConflict, reviewItemExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/reviewSubmissions/submission-1/items":
			return jsonResponse(http.StatusOK, reviewItemsRemovedVersion)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original conflict; a REMOVED item is not proof of presence", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET", seen)
	}
}

func TestReviewItemsAddRejectsUpdateMode(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "items-add", "--submission", "submission-1",
		"--item-type", "appStoreVersions", "--item-id", "version-1",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s; the usage error must precede any HTTP request", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil {
		t.Fatal("run error = nil, want a usage error")
	}
	if !strings.Contains(runErr.Error(), "--if-exists must be one of fail, skip") {
		t.Fatalf("run error = %v, want a usage error listing only fail and skip", runErr)
	}
	if strings.Contains(runErr.Error(), "fail, skip, update") {
		t.Fatalf("run error = %v, must not advertise update", runErr)
	}
	if len(seen) != 0 {
		t.Fatalf("requests = %+v, want none", seen)
	}
}

func TestReviewItemsAddDefaultIfExistsFailPreservesConflict(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "items-add", "--submission", "submission-1",
		"--item-type", "appStoreVersions", "--item-id", "version-1",
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/reviewSubmissionItems" {
			return jsonResponse(http.StatusConflict, reviewItemExists409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestReviewItemsAddIfExistsStillFailsForStateConflict(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "items-add", "--submission", "submission-1",
		"--item-type", "appStoreVersions", "--item-id", "version-1",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/reviewSubmissionItems" {
			return jsonResponse(http.StatusConflict, reviewItemState409)
		}
		t.Fatalf("unexpected request %s %s; a state conflict must not read back", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the state conflict", runErr)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

// The submission carries a different version, and a null relationship for a
// type the item does not have. Neither may be mistaken for the requested item.
func TestReviewItemsAddIfExistsSkipStillFailsWhenReadBackFindsAnotherItem(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "items-add", "--submission", "submission-1",
		"--item-type", "appStoreVersions", "--item-id", "version-1",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusConflict, reviewItemExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/reviewSubmissions/submission-1/items":
			return jsonResponse(http.StatusOK, reviewItemsOtherVersion)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original conflict when the item is not there", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET", seen)
	}
}
