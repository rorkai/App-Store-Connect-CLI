package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's territory catalog, which pricing availability create always fetches
// before the POST so every territory gets an entry.
const territoryCatalog = `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"GBR"}],"links":{}}`

// Apple's 409 body when the app already has an appAvailability, recorded live
// against app 6759231657 on 2026-09-15 (POST /v2/appAvailabilities), with the
// app ID swapped for the fixture's.
const availabilityExists409 = `{"errors":[{"id":"bf11b24f-795a-4d9e-a553-7015d4c934aa","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"An 'appAvailabilities' with a relationship to 'apps' with id 'app-1' already exists.","source":{"pointer":"/data/relationships/app"}}]}`

// The same live conflict when the request carried fewer territories than the
// catalog: Apple reports the existence error first, followed by one
// bootstrap-style "expects an included resource" error per omitted territory
// (174 of them live, abbreviated here). The existence error in errors[0] must
// win over the later bootstrap-style entries.
const availabilityExistsWithIncludeErrors409 = `{"errors":[{"id":"bf11b24f-795a-4d9e-a553-7015d4c934aa","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"An 'appAvailabilities' with a relationship to 'apps' with id 'app-1' already exists.","source":{"pointer":"/data/relationships/app"}},{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The relationship 'territoryAvailabilities.territory' expects an included resource with type 'territories' but no matching resource was included in the request."}]}`

// Apple's public-API bootstrap rejection. It carries the same 409 code as the
// existence conflict, so the command must keep classifying it first.
const availabilityBootstrap409 = `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The relationship 'territoryAvailabilities.territory' expects an included resource with type 'territories' but no matching resource was included in the request."}]}`

// A 409 that is not an existence conflict and must keep failing.
const availabilityState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"The app is not in a state that allows availability changes."}]}`

const existingAvailability = `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}},"links":{"self":"https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1"}}`

// Both territories already available, so the update path has nothing to PATCH.
const existingTerritoryAvailabilities = `{"data":[{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"type":"territoryAvailabilities","id":"ta-gbr","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}}}}],"links":{}}`

const availabilityNotFound404 = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAvailabilities' with id 'app-1'"}]}`

func availabilityCreateArgs(extra ...string) []string {
	args := []string{
		"pricing", "availability", "create",
		"--app", "app-1",
		"--territory", "USA,GBR",
		"--available", "true",
		"--available-in-new-territories", "true",
		"--output", "json",
	}
	return append(args, extra...)
}

func TestPricingAvailabilityCreateIfExistsSkipReturnsExistingRecord(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "skip"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityExists409)
			case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
				return jsonResponse(http.StatusOK, existingAvailability)
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want territories, POST, read-back", seen)
	}
	if !strings.Contains(stdout, `"id":"availability-1"`) {
		t.Fatalf("stdout = %q, want Apple's envelope for the existing record", stdout)
	}
	if !strings.Contains(stderr, "already has availability availability-1") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want the already-exists diagnostic", stderr)
	}
}

func TestPricingAvailabilityCreateIfExistsUpdateRoutesToTheEditPath(t *testing.T) {
	availabilityReads := 0
	_, stderr, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "update"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityExists409)
			case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
				availabilityReads++
				return jsonResponse(http.StatusOK, existingAvailability)
			case req.Method == http.MethodGet && req.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
				return jsonResponse(http.StatusOK, existingTerritoryAvailabilities)
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	// The edit path is what reads the availability a second time and then the
	// territory availabilities; both requested territories already match, so it
	// issues no PATCH.
	if availabilityReads != 2 {
		t.Fatalf("availability reads = %d, want the read-back plus the edit path's own read", availabilityReads)
	}
	for _, req := range seen {
		if req.Method == http.MethodPatch {
			t.Fatalf("unexpected PATCH %+v; both territories already matched", req)
		}
	}
	// Nothing changed, so the diagnostic must not claim an in-place update.
	if strings.Contains(stderr, "updated it in place") {
		t.Fatalf("stderr = %q, must not claim an update when every territory already matched", stderr)
	}
	if !strings.Contains(stderr, "every requested territory already matched; left unchanged (--if-exists update)") {
		t.Fatalf("stderr = %q, want the no-op update outcome", stderr)
	}
	if !strings.Contains(stderr, "Updated 0 territories") {
		t.Fatalf("stderr = %q, want the edit path's own territory summary", stderr)
	}
}

func TestPricingAvailabilityCreateIfExistsUpdatePatchesChangedTerritories(t *testing.T) {
	// USA is unavailable, so --available true must PATCH it through the edit
	// path; GBR already matches.
	territoriesBefore := `{"data":[{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"type":"territoryAvailabilities","id":"ta-gbr","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}}}}],"links":{}}`
	patched := false
	_, stderr, _, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "update"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityExists409)
			case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
				return jsonResponse(http.StatusOK, existingAvailability)
			case req.Method == http.MethodGet && req.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
				if patched {
					return jsonResponse(http.StatusOK, existingTerritoryAvailabilities)
				}
				return jsonResponse(http.StatusOK, territoriesBefore)
			case req.Method == http.MethodPatch && req.Path == "/v1/territoryAvailabilities/ta-usa":
				patched = true
				return jsonResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":true}}}`)
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if !patched {
		t.Fatal("expected the edit path to PATCH the USA territory availability")
	}
	if !strings.Contains(stderr, "updated it in place (--if-exists update)") {
		t.Fatalf("stderr = %q, want the update outcome", stderr)
	}
}

// Live, a create that omits catalog territories returns the existence error
// first plus bootstrap-style include errors after it. The existence error wins.
func TestPricingAvailabilityCreateIfExistsSkipHandlesExistenceWithTrailingIncludeErrors(t *testing.T) {
	stdout, stderr, _, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "skip"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityExistsWithIncludeErrors409)
			case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
				return jsonResponse(http.StatusOK, existingAvailability)
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if !strings.Contains(stdout, `"id":"availability-1"`) || !strings.Contains(stderr, "left unchanged (--if-exists skip)") {
		t.Fatalf("stdout = %q stderr = %q, want the existing record", stdout, stderr)
	}
}

func TestPricingAvailabilityCreateDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs(),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityExists409)
			default:
				t.Fatalf("unexpected request %s %s; fail must not read back", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	if !strings.Contains(runErr.Error(), "An 'appAvailabilities' with a relationship to 'apps' with id 'app-1' already exists.") {
		t.Fatalf("run error = %v, want Apple's detail preserved", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want territories and the POST only", seen)
	}
}

// Apple's bootstrap rejection shares the existence conflict's 409 code, so it
// must keep its own remediation and must not be swallowed by --if-exists.
func TestPricingAvailabilityCreateIfExistsKeepsBootstrapRejection(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "skip"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityBootstrap409)
			default:
				t.Fatalf("unexpected request %s %s; the bootstrap rejection must not read back", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr == nil {
		t.Fatal("run error = nil, want the bootstrap rejection")
	}
	if !strings.Contains(runErr.Error(), "asc web apps availability create") {
		t.Fatalf("run error = %v, want the bootstrap remediation preserved", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want territories and the POST only", seen)
	}
}

func TestPricingAvailabilityCreateIfExistsStillFailsForStateConflict(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "update"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityState409)
			default:
				t.Fatalf("unexpected request %s %s; a state conflict must not read back", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the state conflict", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want territories and the POST only", seen)
	}
}

func TestPricingAvailabilityCreateIfExistsSkipStillFailsWhenReadBackFindsNothing(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "skip"),
		func(req ifExistsRequest) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.Path == "/v1/territories":
				return jsonResponse(http.StatusOK, territoryCatalog)
			case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
				return jsonResponse(http.StatusConflict, availabilityExists409)
			case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
				return jsonResponse(http.StatusNotFound, availabilityNotFound404)
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			}
		})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original conflict when the read-back finds nothing", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want territories, POST, read-back", seen)
	}
}

func TestPricingAvailabilityCreateRejectsInvalidIfExistsBeforeHTTP(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs("--if-exists", "replace"),
		func(req ifExistsRequest) (*http.Response, error) {
			t.Fatalf("unexpected request %s %s; the usage error must precede any HTTP request", req.Method, req.Path)
			return nil, nil
		})
	if runErr == nil {
		t.Fatal("run error = nil, want a usage error")
	}
	if !strings.Contains(runErr.Error(), "--if-exists must be one of fail, skip, update") {
		t.Fatalf("run error = %v, want the supported-modes usage error", runErr)
	}
	if len(seen) != 0 {
		t.Fatalf("requests = %+v, want none", seen)
	}
}
