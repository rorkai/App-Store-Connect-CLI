package cmdtest

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

var libraryReuseContexts = []struct{ path, resource, relationship string }{
	{"localizations", "appStoreVersionLocalizations", "appStoreVersionLocalization"},
	{"product-pages custom-pages localizations", "appCustomProductPageLocalizations", "appCustomProductPageLocalization"},
	{"product-pages experiments treatments localizations", "appStoreVersionExperimentTreatmentLocalizations", "appStoreVersionExperimentTreatmentLocalization"},
	{"app-events localizations", "appEventLocalizations", "appEventLocalization"},
}

func libraryReuseJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func libraryReuseCalls(reg *asctest.Registry) []string {
	calls := []string{}
	for _, req := range reg.Requests() {
		calls = append(calls, req.Method+" "+req.URL.Path)
	}
	return calls
}

func libraryReuseDeletedIDs(reg *asctest.Registry) []string {
	calls := []string{}
	for _, req := range reg.Requests() {
		if req.Method == http.MethodDelete {
			calls = append(calls, strings.TrimPrefix(req.URL.Path, "/v1/appAssetLibraryPlacements/"))
		}
	}
	return calls
}

func TestLibraryReuseCreateContexts(t *testing.T) {
	for _, c := range libraryReuseContexts {
		t.Run(c.resource, func(t *testing.T) {
			setupAuth(t)
			kind := "APP_PREVIEW"
			group := "IPHONE_DUO_PROFILE"
			if c.resource == "appEventLocalizations" {
				kind = "EVENT_DETAILS_PAGE_ASSET"
				group = "DEFAULT_PROFILE"
			}
			want := map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacements", "attributes": map[string]any{"placementType": kind, "placementGroup": group}, "relationships": map[string]any{"video": map[string]any{"data": map[string]any{"type": "appAssetLibraryVideos", "id": "video"}}, c.relationship: map[string]any{"data": map[string]any{"type": c.resource, "id": "loc"}}}}}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(libraryReuseJSON(t, want)),
				asctest.JSON(201, `{"data":{"id":"placement","type":"appAssetLibraryPlacements","attributes":{"state":"ACTIVE"}}}`))
			args := append(strings.Fields(c.path), "placements", "create", "--localization-id", "loc", "--video-id", "video", "--placement-type", kind)
			if group != "DEFAULT_PROFILE" {
				args = append(args, "--placement-group", group)
			}
			stdout, stderr, err := runLibraryReuse(t, args...)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["videoId"] != "video" || receipt["placementGroup"] != group || receipt["created"] != true {
				t.Fatalf("output=%s stderr=%s error=%v", stdout, stderr, err)
			}
		})
	}
}

func TestLibraryReuseOrderContexts(t *testing.T) {
	for _, c := range libraryReuseContexts[:3] {
		t.Run(c.resource, func(t *testing.T) {
			setupAuth(t)
			group := "IPHONE_DUO_PROFILE"
			want := map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacementOrderingRequests", "attributes": map[string]any{"placementGroup": group}, "relationships": map[string]any{"orderedPlacements": map[string]any{"data": []any{map[string]any{"type": "appAssetLibraryPlacements", "id": "second"}, map[string]any{"type": "appAssetLibraryPlacements", "id": "first"}}}, c.relationship: map[string]any{"data": map[string]any{"type": c.resource, "id": "loc"}}}}}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.POST("/v1/appAssetLibraryPlacementOrderingRequests").JSONBody(libraryReuseJSON(t, want)),
				asctest.JSON(201, `{"data":{"type":"appAssetLibraryPlacementOrderingRequests","id":"order"},"included":[{"type":"appAssetLibraryPlacements","id":"second"}]}`))
			args := append(strings.Fields(c.path), "placements", "reorder", "--localization-id", "loc", "--placement-group", group, "--placement-ids", "second,first")
			stdout, stderr, err := runLibraryReuse(t, args...)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["id"] != "order" || receipt["reordered"] != true || !reflect.DeepEqual(receipt["placementIds"], []any{"second", "first"}) {
				t.Fatalf("output=%s stderr=%s err=%v", stdout, stderr, err)
			}
		})
	}
}

func TestLibraryReuseUsageBeforeAuth(t *testing.T) {
	for _, args := range [][]string{
		{"create", "--localization-id", "loc", "--image-id", "image", "--video-id", "video", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"create", "--localization-id", "loc", "--video-id", "video", "--placement-type", "APP_SCREENSHOT", "--placement-group", "IPHONE_DUO_PROFILE"},
		{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_PREVIEW", "--placement-group", "IPHONE_DUO_PROFILE"},
		{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_SCREENSHOT"},
		{"reorder", "--localization-id", "loc", "--placement-group", "IPHONE_DUO_PROFILE", "--placement-ids", "a,a"},
		{"reorder", "--localization-id", "loc", "--placement-group", "IPHONE_DUO_PROFILE", "--placement-ids", "a,"},
		{"reorder", "--localization-id", "loc", "--placement-ids", "a,b"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := runLibraryReuse(t, append([]string{"localizations", "placements"}, args...)...)
			if !isUsageClassError(err) || stdout != "" || stderr == "" {
				t.Fatalf("expected usage: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

func runLibraryReuse(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("dev")
	var configure func(*ffcli.Command)
	configure = func(c *ffcli.Command) {
		if c.FlagSet != nil {
			c.FlagSet.Init(c.FlagSet.Name(), flag.ContinueOnError)
			c.FlagSet.SetOutput(io.Discard)
		}
		for _, child := range c.Subcommands {
			configure(child)
		}
	}
	configure(root)
	var err error
	stdout, stderr := captureOutput(t, func() {
		if err = root.Parse(args); err == nil {
			err = root.Run(context.Background())
		}
	})
	return stdout, stderr, err
}

func TestLibraryReuseNewReadsPreserveEnvelope(t *testing.T) {
	for _, c := range libraryReuseContexts[2:] {
		t.Run(c.resource, func(t *testing.T) {
			setupAuth(t)
			kind := "APP_SCREENSHOT"
			group := "IPHONE_DUO_PROFILE"
			if c.resource == "appEventLocalizations" {
				kind = "EVENT_CARD_ASSET"
				group = "DEFAULT_PROFILE"
			}
			const body = `{"data":[],"included":[],"links":{"self":"retained"},"future":{"nested":null}}`
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/"+c.resource+"/loc/placements", "filter[placementGroup]="+group, "filter[placementType]="+kind, "include=image,video"),
				asctest.JSON(200, body))
			args := append(strings.Fields(c.path), "placements", "list", "--localization-id", "loc", "--placement-type", kind, "--placement-group", group, "--include", "image,video")
			stdout, stderr, err := runLibraryReuse(t, args...)
			var got, want any
			_ = json.Unmarshal([]byte(stdout), &got)
			_ = json.Unmarshal([]byte(body), &want)
			if err != nil || stderr != "" || !reflect.DeepEqual(got, want) {
				t.Fatalf("changed envelope: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestLibraryReuseScreenshotImageAndCreativeVideo(t *testing.T) {
	for _, tc := range []struct{ mediaFlag, mediaKind, kind, group string }{
		{"--image-id", "image", "APP_SCREENSHOT", "IPHONE_DUO_PROFILE"},
		{"--image-id", "image", "IMESSAGE_APP_SCREENSHOT", "IMESSAGE_IPHONE_DUO_PROFILE"},
		{"--video-id", "video", "PRODUCT_PAGE_HEADER_ASSET", "DEFAULT_PROFILE"},
		{"--video-id", "video", "APP_STORE_SEARCH_RESULTS_ASSET", "DEFAULT_PROFILE"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			setupAuth(t)
			mediaType := "appAssetLibraryImages"
			if tc.mediaKind == "video" {
				mediaType = "appAssetLibraryVideos"
			}
			want := map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacements", "attributes": map[string]any{"placementType": tc.kind, "placementGroup": tc.group}, "relationships": map[string]any{tc.mediaKind: map[string]any{"data": map[string]any{"type": mediaType, "id": "asset"}}, "appStoreVersionLocalization": map[string]any{"data": map[string]any{"type": "appStoreVersionLocalizations", "id": "loc"}}}}}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(libraryReuseJSON(t, want)),
				asctest.JSON(201, `{"data":{"type":"appAssetLibraryPlacements","id":"placement"}}`))
			stdout, stderr, err := runLibraryReuse(t, "localizations", "placements", "create", "--localization-id", "loc", tc.mediaFlag, "asset", "--placement-type", tc.kind, "--placement-group", tc.group)
			if err != nil || stderr != "" || stdout == "" {
				t.Fatalf("failed=%s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestLibraryReuseOrderRefusesFalseSuccess(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, response := range []struct {
		status int
		body   string
	}{{403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`}, {201, `{"data":{}}`}, {201, `{"data":{"type":"appAssetLibraryPlacements","id":"wrong"}}`}} {
		reg.Register(asctest.POST("/v1/appAssetLibraryPlacementOrderingRequests").JSONBody(`{"data":{"type":"appAssetLibraryPlacementOrderingRequests","attributes":{"placementGroup":"IPHONE_DUO_PROFILE"},"relationships":{"appStoreVersionLocalization":{"data":{"type":"appStoreVersionLocalizations","id":"loc"}},"orderedPlacements":{"data":[{"type":"appAssetLibraryPlacements","id":"first"},{"type":"appAssetLibraryPlacements","id":"second"}]}}}}`),
			asctest.JSON(response.status, response.body))
		stdout, _, err := runLibraryReuse(t, "localizations", "placements", "reorder", "--localization-id", "loc", "--placement-group", "IPHONE_DUO_PROFILE", "--placement-ids", "first,second")
		if err == nil || stdout != "" {
			t.Fatalf("false ordering success: %s %v", stdout, err)
		}
	}
}

// Event examples must be runnable for the event contract, including its group.
func TestLibraryReuseEventHelpExamples(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, query := range [][]string{
		{"filter[placementType]=EVENT_CARD_ASSET"},
		{"filter[placementType]=EVENT_DETAILS_PAGE_ASSET", "include=image,video"},
		{"limit=200"},
		{"filter[state]=FAILED", "sort=-lastModifiedDate"},
	} {
		reg.Register(asctest.GET("/v1/appEventLocalizations/ID/placements", query...), asctest.JSON(200, `{"data":[]}`))
	}
	for _, operation := range []string{"list"} {
		command := RootCommand("dev")
		for _, name := range []string{"app-events", "localizations", "placements", operation} {
			var found *ffcli.Command
			for _, child := range command.Subcommands {
				if child.Name == name {
					found = child
					break
				}
			}
			if found == nil {
				t.Fatalf("missing command %s", name)
			}
			command = found
		}
		examples := 0
		for _, line := range strings.Split(command.LongHelp, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "asc ") {
				continue
			}
			examples++
			_, stderr, err := runLibraryReuse(t, strings.Fields(line)[1:]...)
			if err != nil || stderr != "" {
				t.Errorf("unusable help example %q: %s %v", line, stderr, err)
			}
		}
		if examples == 0 {
			t.Fatalf("%s help has no runnable examples", operation)
		}
	}
}

func TestLibraryReuseAtomicSwapContexts(t *testing.T) {
	for _, c := range libraryReuseContexts {
		t.Run(c.resource, func(t *testing.T) {
			setupAuth(t)
			kind := "APP_PREVIEW"
			if c.resource == "appEventLocalizations" {
				kind = "EVENT_CARD_ASSET"
			}
			group := "IPHONE_67"
			if c.resource == "appEventLocalizations" {
				group = "DEFAULT_PROFILE"
			}
			want := map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacements", "attributes": map[string]any{"placementType": kind, "placementGroup": group}, "relationships": map[string]any{"video": map[string]any{"data": map[string]any{"type": "appAssetLibraryVideos", "id": "video"}}, c.relationship: map[string]any{"data": map[string]any{"type": c.resource, "id": "loc"}}, "placementToSwapOut": map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacements", "id": "old"}}}}}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/appAssetLibraryPlacements/old"),
				asctest.JSON(200, `{"data":{"type":"appAssetLibraryPlacements","id":"old","attributes":{"placementType":"`+kind+`","placementGroup":"`+group+`"}}}`))
			reg.Register(asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(libraryReuseJSON(t, want)),
				asctest.JSON(201, `{"data":{"type":"appAssetLibraryPlacements","id":"new","attributes":{"state":"ACTIVE"}}}`))
			args := append(strings.Fields(c.path), "placements", "swap", "--localization-id", "loc", "--placement-id", "old", "--video-id", "video", "--placement-type", kind, "--confirm")
			stdout, stderr, err := runLibraryReuse(t, args...)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["id"] != "new" || receipt["swapped"] != true || receipt["previousPlacementId"] != "old" || receipt["videoId"] != "video" || receipt["placementGroup"] != group {
				t.Fatalf("swap receipt=%s %s %v", stdout, stderr, err)
			}
			if calls, want := libraryReuseCalls(reg), []string{"GET /v1/appAssetLibraryPlacements/old", "POST /v1/appAssetLibraryPlacements"}; !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want %v", calls, want)
			}
		})
	}
}

func TestLibraryReuseSwapStopsWhenExistingPlacementCannotBeRead(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appAssetLibraryPlacements/old"),
		asctest.JSON(404, `{"errors":[{"status":"404","code":"NOT_FOUND","detail":"missing"}]}`))
	stdout, _, err := runLibraryReuse(t, "localizations", "placements", "swap", "--localization-id", "loc", "--placement-id", "old", "--image-id", "image", "--placement-type", "APP_SCREENSHOT", "--confirm")
	if err == nil || stdout != "" || !strings.Contains(err.Error(), "placements swap: read placement old") {
		t.Fatalf("stdout=%q err=%v", stdout, err)
	}
}

func TestLibraryReuseBulkRemovalContexts(t *testing.T) {
	for _, c := range libraryReuseContexts {
		t.Run(c.resource, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/first"), asctest.JSON(204, ""))
			reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/second"), asctest.JSON(204, ""))
			args := append(strings.Fields(c.path), "placements", "delete", "--placement-ids", "first,second", "--confirm")
			stdout, stderr, err := runLibraryReuse(t, args...)
			calls := libraryReuseDeletedIDs(reg)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["deleted"] != true || !reflect.DeepEqual(receipt["placementIds"], []any{"first", "second"}) || !reflect.DeepEqual(receipt["deletedPlacementIds"], []any{"first", "second"}) || !reflect.DeepEqual(calls, []string{"first", "second"}) {
				t.Fatalf("remove receipt=%s %s %v calls=%v", stdout, stderr, err, calls)
			}
		})
	}
}

func TestLibraryReuseSequentialRemovalPartialReceipt(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/first"), asctest.JSON(204, ""))
	reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/second"),
		asctest.JSON(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`))
	stdout, stderr, err := runLibraryReuse(t, "localizations", "placements", "delete", "--placement-ids", "first,second,third", "--confirm")
	calls := libraryReuseDeletedIDs(reg)
	var receipt map[string]any
	if err == nil || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["deleted"] != false || receipt["failedPlacementId"] != "second" || !reflect.DeepEqual(receipt["deletedPlacementIds"], []any{"first"}) || !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("partial=%s stderr=%s err=%v calls=%v", stdout, stderr, err, calls)
	}
}

func TestLibraryReuseSwapBulkUsageBeforeHTTP(t *testing.T) {
	setupAuth(t)
	asctest.NewRegistry(t).Install()
	for _, args := range [][]string{
		{"swap", "--localization-id", "loc", "--placement-id", "old", "--image-id", "image", "--video-id", "video", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--confirm"},
		{"swap", "--localization-id", "loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--confirm"},
		{"swap", "--localization-id", "loc", "--placement-id", "old", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"swap", "--localization-id", "loc", "--placement-id", "../old", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--confirm"},
		{"delete", "--id", "one", "--placement-ids", "first,second", "--confirm"},
		{"delete", "--placement-ids", "first,first", "--confirm"},
		{"delete", "--placement-ids", "first,", "--confirm"},
		{"delete", "--placement-ids", "first,second"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := runLibraryReuse(t, append([]string{"localizations", "placements"}, args...)...)
			if !isUsageClassError(err) || stdout != "" || stderr == "" {
				t.Fatalf("usage=%s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestLibraryReuseSwapBulkFailureDoesNotFallBack(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, operation := range []string{"swap", "delete"} {
		responses := []struct {
			status int
			body   string
		}{{403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`}}
		if operation == "swap" {
			responses = append(responses, struct {
				status int
				body   string
			}{201, `{"data":{}}`})
		}
		for _, response := range responses {
			before := len(reg.Requests())
			if operation == "swap" {
				// Swap reads the existing placement's group before its one mutation.
				reg.Register(asctest.GET("/v1/appAssetLibraryPlacements/old"),
					asctest.JSON(200, `{"data":{"type":"appAssetLibraryPlacements","id":"old","attributes":{"placementGroup":"DEFAULT_PROFILE"}}}`))
				reg.Register(asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(libraryReuseHeaderSwapBody),
					asctest.JSON(response.status, response.body))
			} else {
				reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/one"), asctest.JSON(response.status, response.body))
			}
			args := []string{"localizations", "placements", operation}
			if operation == "swap" {
				args = append(args, "--localization-id", "loc", "--placement-id", "old", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--confirm")
			} else {
				args = append(args, "--placement-ids", "one,two", "--confirm")
			}
			stdout, _, err := runLibraryReuse(t, args...)
			calls := 0
			for _, req := range reg.Requests()[before:] {
				if req.Method != http.MethodGet {
					calls++
				}
			}
			if err == nil || calls != 1 {
				t.Fatalf("failed mutation: %s %v calls=%d", stdout, err, calls)
			}
			if operation == "swap" && stdout != "" {
				t.Fatalf("false swap receipt %s", stdout)
			}
			if operation == "delete" {
				var receipt map[string]any
				if json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["deleted"] != false || receipt["failedPlacementId"] != "one" {
					t.Fatalf("partial receipt=%s", stdout)
				}
			}
		}
	}
}

const libraryReuseHeaderSwapBody = `{"data":{"type":"appAssetLibraryPlacements","attributes":{"placementGroup":"DEFAULT_PROFILE","placementType":"PRODUCT_PAGE_HEADER_ASSET"},"relationships":{"appStoreVersionLocalization":{"data":{"type":"appStoreVersionLocalizations","id":"loc"}},"image":{"data":{"type":"appAssetLibraryImages","id":"image"}},"placementToSwapOut":{"data":{"type":"appAssetLibraryPlacements","id":"old"}}}}}`

func TestLibraryReuseSwapAndBulkHumanReceipts(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for range []string{"table", "markdown"} {
		reg.Register(asctest.GET("/v1/appAssetLibraryPlacements/old"),
			asctest.JSON(200, `{"data":{"type":"appAssetLibraryPlacements","id":"old","attributes":{"placementGroup":"DEFAULT_PROFILE"}}}`))
		reg.Register(asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(libraryReuseHeaderSwapBody),
			asctest.JSON(201, `{"data":{"type":"appAssetLibraryPlacements","id":"new"}}`))
		reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/one"), asctest.JSON(204, ""))
		reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/two"), asctest.JSON(204, ""))
	}
	for _, format := range []string{"table", "markdown"} {
		for _, args := range [][]string{
			{"swap", "--localization-id", "loc", "--placement-id", "old", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--confirm"},
			{"delete", "--placement-ids", "one,two", "--confirm"},
		} {
			args = append(append([]string{"localizations", "placements"}, args...), "--output", format)
			stdout, stderr, err := runLibraryReuse(t, args...)
			if err != nil || stderr != "" || !strings.Contains(stdout, "true") {
				t.Fatalf("human receipt=%s %s %v", stdout, stderr, err)
			}
		}
	}
}

// Official 4.5.1 ordering supports version, CPP, and PPO parents, not events.
// A public event probe also rejected appEventLocalization; reject locally.
func TestLibraryReuseEventReorderRejectedBeforeHTTP(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	_, _, err := runLibraryReuse(t, "app-events", "localizations", "placements", "reorder", "--localization-id", "loc", "--placement-group", "DEFAULT_PROFILE", "--placement-ids", "one,two")
	requests := len(reg.Requests())
	if err == nil || requests != 0 {
		t.Fatalf("event reorder must be unavailable before HTTP: err=%v requests=%d", err, requests)
	}
}
