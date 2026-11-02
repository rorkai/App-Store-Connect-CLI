package asc

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestOutputRegistry(t *testing.T) {
	t.Run("render by registry scenarios", runOutputRegistryRenderByRegistryScenarios)
	t.Run("helper registrations", runOutputRegistryHelperRegistrations)
	t.Run("panic scenarios", runOutputRegistryPanicScenarios)
}

// Every registered renderer must still print its headers when the response is
// empty, so an empty list never renders as a blank table.
func TestPrintTableAndMarkdown_EmptyResponsesKeepHeaders(t *testing.T) {
	ensureOutputRegistryPopulated()
	errorsWhenEmpty := map[reflect.Type]bool{
		typeForPtr[DiagnosticLogsResponse]():      true,
		typeForPtr[PerfPowerMetricsResponse]():    true,
		typeForPtr[PerformanceOverviewResponse](): true,
	}

	var types []reflect.Type
	for typ := range outputRegistry {
		types = append(types, typ)
	}
	for typ := range directRenderRegistry {
		types = append(types, typ)
	}
	slices.SortFunc(types, func(a, b reflect.Type) int { return strings.Compare(a.String(), b.String()) })

	for _, typ := range types {
		empty := reflect.New(typ.Elem()).Interface()
		var headers []string
		var err error
		captureStdout(t, func() error {
			err = renderByRegistry(empty, func(h []string, _ [][]string) {
				if len(h) == 0 {
					t.Errorf("%s: empty response rendered without headers", typ)
					return
				}
				headers = append(headers, h[0])
			})
			return nil
		})
		if errorsWhenEmpty[typ] {
			if err == nil {
				t.Errorf("%s: expected an error for an empty response", typ)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: render error: %v", typ, err)
			continue
		}
		if len(headers) == 0 {
			t.Errorf("%s: empty response rendered no table", typ)
			continue
		}
		for name, print := range map[string]func(any) error{"table": PrintTable, "markdown": PrintMarkdown} {
			output := captureStdout(t, func() error { return print(empty) })
			for _, header := range headers {
				if !strings.Contains(output, header) {
					t.Errorf("%s %s: missing header %q in %q", typ, name, header, output)
				}
			}
		}
	}
}

// Types that CLI commands pass to the printer. Without a registration,
// --output table/markdown silently falls back to JSON.
func TestCommandOutputTypesHaveTableRenderers(t *testing.T) {
	ensureOutputRegistryPopulated()

	for _, typ := range []reflect.Type{
		typeKey[SubscriptionPricePointsResponse](),
		typeKey[SubscriptionPricePointResponse](),
		typeKey[SubscriptionIntroductoryOffersResponse](),
		typeKey[SubscriptionIntroductoryOfferResponse](),
		typeKey[SubscriptionPromotionalOffersResponse](),
		typeKey[SubscriptionPromotionalOfferResponse](),
		typeKey[SubscriptionPromotionalOfferPricesResponse](),
		typeKey[SubscriptionAppStoreReviewScreenshotResponse](),
		typeKey[AppInfoLocalizationResponse](),
		typeKey[BuildUploadResponse](),
		typeKey[BuildUploadFileResponse](),
		typeKey[BetaBuildUsagesResponse](),
		typeKey[BetaCrashLogResponse](),
		typeKey[BetaFeedbackCrashSubmissionResponse](),
		typeKey[BetaFeedbackScreenshotSubmissionResponse](),
		typeKey[BetaRecruitmentCriterionCompatibleBuildCheckResponse](),
		typeKey[TerritoryAvailabilityResponse](),
		typeKey[CertificatePassTypeIDLinkageResponse](),
		typeKey[ProfileBundleIDLinkageResponse](),
		typeKey[AppStoreVersionDeleteResult](),
		typeKey[GameCenterAchievementImageResponse](),
		typeKey[GameCenterLeaderboardImageResponse](),
		typeKey[GameCenterLeaderboardSetImageResponse](),
		typeKey[GameCenterLeaderboardSetMemberLocalizationsResponse](),
		typeKey[GameCenterLeaderboardSetMemberLocalizationResponse](),
		typeKey[GameCenterLeaderboardSetMemberLocalizationDeleteResult](),
		typeKey[GameCenterLeaderboardSetMembersUpdateResult](),
	} {
		if !isRegistryTypeRegistered(typ) {
			t.Errorf("no table renderer registered for %s", typ)
			continue
		}
		if err := renderByRegistry(reflect.New(typ.Elem()).Interface(), func([]string, [][]string) {}); err != nil {
			t.Errorf("render zero %s: %v", typ, err)
		}
	}
}

func runOutputRegistryRenderByRegistryScenarios(t *testing.T) {
	t.Run("fallback to JSON for unregistered types", func(t *testing.T) {
		type unregistered struct {
			Value string `json:"value"`
		}
		output := captureStdout(t, func() error {
			return renderByRegistry(&unregistered{Value: "test"}, RenderTable)
		})
		if output == "" {
			t.Fatal("expected JSON fallback output")
		}
		if !strings.Contains(output, "test") {
			t.Fatalf("expected JSON output to contain 'test', got: %s", output)
		}
	})

	t.Run("rows handler renders value and typed nil zero value", func(t *testing.T) {
		type registered struct {
			Value string
		}

		key := typeKey[registered]()
		cleanupRegistryTypes(t, key)

		registerRows(func(v *registered) ([]string, [][]string) {
			return []string{"value"}, [][]string{{v.Value}}
		})

		cases := []struct {
			name    string
			data    *registered
			wantRow []string
		}{
			{name: "regular value", data: &registered{Value: "from-registry"}, wantRow: []string{"from-registry"}},
			{name: "typed nil", data: (*registered)(nil), wantRow: []string{""}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var gotHeaders []string
				var gotRows [][]string
				err := renderByRegistry(tc.data, func(headers []string, rows [][]string) {
					gotHeaders = headers
					gotRows = rows
				})
				if err != nil {
					t.Fatalf("renderByRegistry returned error: %v", err)
				}
				assertSingleRowEquals(t, gotHeaders, gotRows, []string{"value"}, tc.wantRow)
			})
		}
	})

	t.Run("rows error handler propagates error and typed nil zero value", func(t *testing.T) {
		t.Run("propagates error", func(t *testing.T) {
			type registered struct{}

			key := typeKey[registered]()
			cleanupRegistryTypes(t, key)

			wantErr := errors.New("rows renderer failed")
			registerRowsErr(func(*registered) ([]string, [][]string, error) {
				return nil, nil, wantErr
			})

			renderCalls := 0
			err := renderByRegistry(&registered{}, func([]string, [][]string) {
				renderCalls++
			})
			if !errors.Is(err, wantErr) {
				t.Fatalf("renderByRegistry error = %v, want %v", err, wantErr)
			}
			if renderCalls != 0 {
				t.Fatalf("expected render callback not to run on rows error, got %d calls", renderCalls)
			}
		})

		t.Run("typed nil zero value", func(t *testing.T) {
			type registered struct {
				Value string
			}

			key := typeKey[registered]()
			cleanupRegistryTypes(t, key)

			registerRowsErr(func(v *registered) ([]string, [][]string, error) {
				return []string{"value"}, [][]string{{v.Value}}, nil
			})

			var gotHeaders []string
			var gotRows [][]string
			err := renderByRegistry((*registered)(nil), func(headers []string, rows [][]string) {
				gotHeaders = headers
				gotRows = rows
			})
			if err != nil {
				t.Fatalf("renderByRegistry returned error: %v", err)
			}
			assertSingleRowEquals(t, gotHeaders, gotRows, []string{"value"}, []string{""})
		})
	})

	t.Run("direct handler renders value and typed nil zero value", func(t *testing.T) {
		type registered struct {
			Value string
		}

		key := typeKey[registered]()
		cleanupRegistryTypes(t, key)

		registerDirect(func(v *registered, render func([]string, [][]string)) error {
			render([]string{"value"}, [][]string{{v.Value}})
			return nil
		})

		cases := []struct {
			name    string
			data    *registered
			wantRow []string
		}{
			{name: "regular value", data: &registered{Value: "from-direct"}, wantRow: []string{"from-direct"}},
			{name: "typed nil", data: (*registered)(nil), wantRow: []string{""}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var gotHeaders []string
				var gotRows [][]string
				err := renderByRegistry(tc.data, func(headers []string, rows [][]string) {
					gotHeaders = headers
					gotRows = rows
				})
				if err != nil {
					t.Fatalf("renderByRegistry returned error: %v", err)
				}
				assertSingleRowEquals(t, gotHeaders, gotRows, []string{"value"}, tc.wantRow)
			})
		}
	})

	t.Run("direct handler propagates error", func(t *testing.T) {
		type registered struct{}

		key := typeKey[registered]()
		cleanupRegistryTypes(t, key)

		wantErr := errors.New("direct renderer failed")
		registerDirect(func(*registered, func([]string, [][]string)) error {
			return wantErr
		})

		renderCalls := 0
		err := renderByRegistry(&registered{}, func([]string, [][]string) {
			renderCalls++
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("renderByRegistry error = %v, want %v", err, wantErr)
		}
		if renderCalls != 0 {
			t.Fatalf("expected render callback not to run on direct error, got %d calls", renderCalls)
		}
	})

	t.Run("direct renderer preferred when both handlers exist", func(t *testing.T) {
		type registered struct{}

		key := typeKey[registered]()
		cleanupRegistryTypes(t, key)

		rowsHandlerCalled := seedRowsAndDirectHandlersForTest(
			key,
			renderWithRows([]string{"source"}, [][]string{{"direct"}}),
		)

		var gotHeaders []string
		var gotRows [][]string
		err := renderByRegistry(&registered{}, func(headers []string, rows [][]string) {
			gotHeaders = headers
			gotRows = rows
		})
		if err != nil {
			t.Fatalf("renderByRegistry returned error: %v", err)
		}
		if rowsHandlerCalled() {
			t.Fatal("expected direct handler precedence over rows handler")
		}
		assertSingleRowEquals(t, gotHeaders, gotRows, []string{"source"}, []string{"direct"})
	})

	t.Run("direct renderer error still bypasses rows handler", func(t *testing.T) {
		type registered struct{}

		key := typeKey[registered]()
		cleanupRegistryTypes(t, key)

		wantErr := errors.New("direct failed")
		rowsHandlerCalled := seedRowsAndDirectHandlersForTest(key, func(any, func([]string, [][]string)) error {
			return wantErr
		})

		renderCalls := 0
		err := renderByRegistry(&registered{}, func([]string, [][]string) {
			renderCalls++
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("renderByRegistry error = %v, want %v", err, wantErr)
		}
		if rowsHandlerCalled() {
			t.Fatal("expected rows handler not to run when direct handler exists")
		}
		if renderCalls != 0 {
			t.Fatalf("expected render callback not to run on direct error, got %d calls", renderCalls)
		}
	})
}

func runOutputRegistryHelperRegistrations(t *testing.T) {
	ensureOutputRegistryPopulated()

	t.Run("registry sanity", func(t *testing.T) {
		t.Run("registries are non-empty", func(t *testing.T) {
			if len(outputRegistry) == 0 {
				t.Fatal("output registry is empty; lazy population may not have run")
			}
			if len(directRenderRegistry) == 0 {
				t.Fatal("direct render registry is empty; lazy population may not have run")
			}
		})

		t.Run("all handlers non-nil", func(t *testing.T) {
			for typ, fn := range outputRegistry {
				if fn == nil {
					t.Errorf("nil rows handler registered for type %s", typ)
				}
			}
			for typ, fn := range directRenderRegistry {
				if fn == nil {
					t.Errorf("nil direct handler registered for type %s", typ)
				}
			}
		})

		t.Run("expected minimum total registrations", func(t *testing.T) {
			// Total registered types across both registries should be ~471.
			total := len(outputRegistry) + len(directRenderRegistry)
			const minExpected = 460
			if total < minExpected {
				t.Errorf("expected at least %d registered types, got %d (rows: %d, direct: %d)",
					minExpected, total, len(outputRegistry), len(directRenderRegistry))
			}
		})
	})

	t.Run("built-in helper registrations", func(t *testing.T) {
		t.Run("single linkage", func(t *testing.T) {
			handler := requireOutputHandlerFor[AppStoreVersionSubmissionLinkageResponse](
				t,
				"AppStoreVersionSubmissionLinkageResponse",
			)

			headers, rows, err := handler(&AppStoreVersionSubmissionLinkageResponse{
				Data: ResourceData{
					Type: ResourceType("appStoreVersionSubmissions"),
					ID:   "submission-123",
				},
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertRowContains(t, headers, rows, "submission-123")
		})

		t.Run("id/state", func(t *testing.T) {
			handler := requireOutputHandlerFor[BackgroundAssetVersionAppStoreReleaseResponse](
				t,
				"BackgroundAssetVersionAppStoreReleaseResponse",
			)

			headers, rows, err := handler(&BackgroundAssetVersionAppStoreReleaseResponse{
				Data: Resource[BackgroundAssetVersionAppStoreReleaseAttributes]{
					ID:         "release-1",
					Attributes: BackgroundAssetVersionAppStoreReleaseAttributes{State: "READY_FOR_SALE"},
				},
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertRowContains(t, headers, rows, "release-1", "READY_FOR_SALE")
		})

		t.Run("id/bool", func(t *testing.T) {
			handler := requireOutputHandlerFor[AlternativeDistributionDomainDeleteResult](
				t,
				"AlternativeDistributionDomainDeleteResult",
			)

			headers, rows, err := handler(&AlternativeDistributionDomainDeleteResult{
				ID:      "domain-1",
				Deleted: true,
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertRowContains(t, headers, rows, "domain-1", "true")
		})
	})

	t.Run("single-resource helper", func(t *testing.T) {
		type helperAttrs struct {
			Name string `json:"name"`
		}

		registerSingleResourceRowsAdapter(func(v *Response[helperAttrs]) ([]string, [][]string) {
			if len(v.Data) == 0 {
				return []string{"ID", "Name"}, nil
			}
			return []string{"ID", "Name"}, [][]string{{v.Data[0].ID, v.Data[0].Attributes.Name}}
		})

		key := typeKey[SingleResponse[helperAttrs]]()
		cleanupRegistryTypes(t, key)

		handler := requireOutputHandler(t, key, "SingleResponse helper")

		t.Run("regular payload", func(t *testing.T) {
			headers, rows, err := handler(&SingleResponse[helperAttrs]{
				Data: Resource[helperAttrs]{
					ID:         "helper-id",
					Attributes: helperAttrs{Name: "helper-name"},
				},
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertSingleRowEquals(t, headers, rows, []string{"ID", "Name"}, []string{"helper-id", "helper-name"})
		})

		t.Run("typed nil pointer", func(t *testing.T) {
			headers, rows, err := handler((*SingleResponse[helperAttrs])(nil))
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertSingleRowEquals(t, headers, rows, []string{"ID", "Name"}, []string{"", ""})
		})

		t.Run("rows+single-resource helper registers both handlers", func(t *testing.T) {
			type attrs struct {
				Name string `json:"name"`
			}

			registerRowsWithSingleResourceAdapter(func(v *Response[attrs]) ([]string, [][]string) {
				if len(v.Data) == 0 {
					return []string{"ID", "Name"}, nil
				}
				return []string{"ID", "Name"}, [][]string{{v.Data[0].ID, v.Data[0].Attributes.Name}}
			})

			listKey := typeKey[Response[attrs]]()
			singleKey := typeKey[SingleResponse[attrs]]()
			cleanupRegistryTypes(t, listKey, singleKey)

			listHandler := requireOutputHandler(t, listKey, "list handler from rows+single-resource helper")
			singleHandler := requireOutputHandler(t, singleKey, "single handler from rows+single-resource helper")

			listHeaders, listRows, err := listHandler(&Response[attrs]{
				Data: []Resource[attrs]{{ID: "list-id", Attributes: attrs{Name: "list-name"}}},
			})
			if err != nil {
				t.Fatalf("list handler returned error: %v", err)
			}
			assertSingleRowEquals(t, listHeaders, listRows, []string{"ID", "Name"}, []string{"list-id", "list-name"})

			singleHeaders, singleRows, err := singleHandler(&SingleResponse[attrs]{
				Data: Resource[attrs]{ID: "single-id", Attributes: attrs{Name: "single-name"}},
			})
			if err != nil {
				t.Fatalf("single handler returned error: %v", err)
			}
			assertSingleRowEquals(t, singleHeaders, singleRows, []string{"ID", "Name"}, []string{"single-id", "single-name"})
		})
	})

	t.Run("single-to-list helper behavior", func(t *testing.T) {
		t.Run("single-to-list adapter handles payload and typed nil", func(t *testing.T) {
			type single struct {
				Data string
			}
			type list struct {
				Data []string
			}

			registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
				if len(v.Data) == 0 {
					return []string{"value"}, nil
				}
				return []string{"value"}, [][]string{{v.Data[0]}}
			})

			key := typeKey[single]()
			cleanupRegistryTypes(t, key)
			handler := requireOutputHandler(t, key, "single-to-list helper")

			headers, rows, err := handler(&single{Data: "converted"})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertSingleRowEquals(t, headers, rows, []string{"value"}, []string{"converted"})

			headers, rows, err = handler((*single)(nil))
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertSingleRowEquals(t, headers, rows, []string{"value"}, []string{""})
		})

		t.Run("rows+single-to-list helper registers both handlers", func(t *testing.T) {
			type single struct {
				Data string
			}
			type list struct {
				Data []string
			}

			registerRowsWithSingleToListAdapter[single, list](func(v *list) ([]string, [][]string) {
				if len(v.Data) == 0 {
					return []string{"value"}, nil
				}
				return []string{"value"}, [][]string{{v.Data[0]}}
			})

			singleKey := typeKey[single]()
			listKey := typeKey[list]()
			cleanupRegistryTypes(t, singleKey, listKey)

			singleHandler := requireOutputHandler(t, singleKey, "single handler from rows+single-to-list helper")
			listHandler := requireOutputHandler(t, listKey, "list handler from rows+single-to-list helper")

			singleHeaders, singleRows, err := singleHandler(&single{Data: "single-value"})
			if err != nil {
				t.Fatalf("single handler returned error: %v", err)
			}
			assertSingleRowEquals(t, singleHeaders, singleRows, []string{"value"}, []string{"single-value"})

			listHeaders, listRows, err := listHandler(&list{Data: []string{"list-value"}})
			if err != nil {
				t.Fatalf("list handler returned error: %v", err)
			}
			assertSingleRowEquals(t, listHeaders, listRows, []string{"value"}, []string{"list-value"})
		})

		t.Run("rows+single-to-list helper avoids partial registration on adapter panic", func(t *testing.T) {
			type single struct {
				Value string
			}
			type list struct {
				Data []string
			}

			singleKey := typeKey[single]()
			listKey := typeKey[list]()
			cleanupRegistryTypes(t, singleKey, listKey)

			expectPanicContainsAll(t, func() {
				registerRowsWithSingleToListAdapter[single, list](func(v *list) ([]string, [][]string) {
					return []string{"value"}, nil
				})
			}, "requires Data field")

			assertRegistryTypeAbsent(t, singleKey)
			assertRegistryTypeAbsent(t, listKey)
		})

		t.Run("copies links when compatible", func(t *testing.T) {
			type single struct {
				Data  ResourceData
				Links Links
			}
			type list struct {
				Data  []ResourceData
				Links Links
			}

			registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
				if len(v.Data) == 0 {
					return []string{"id", "self"}, nil
				}
				return []string{"id", "self"}, [][]string{{v.Data[0].ID, v.Links.Self}}
			})

			key := typeKey[single]()
			cleanupRegistryTypes(t, key)
			handler := requireOutputHandler(t, key, "single-to-list links helper")

			headers, rows, err := handler(&single{
				Data: ResourceData{ID: "item-1", Type: ResourceType("items")},
				Links: Links{
					Self: "https://example.test/items/1",
				},
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertSingleRowEquals(t, headers, rows, []string{"id", "self"}, []string{"item-1", "https://example.test/items/1"})
		})

		t.Run("leaves target links zero when source has no links", func(t *testing.T) {
			type single struct {
				Data string
			}
			type list struct {
				Data  []string
				Links Links
			}

			registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
				if len(v.Data) == 0 {
					return []string{"value", "self"}, nil
				}
				return []string{"value", "self"}, [][]string{{v.Data[0], v.Links.Self}}
			})

			key := typeKey[single]()
			cleanupRegistryTypes(t, key)
			handler := requireOutputHandler(t, key, "single-to-list missing-source-links helper")

			headers, rows, err := handler(&single{Data: "converted"})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			assertSingleRowEquals(t, headers, rows, []string{"value", "self"}, []string{"converted", ""})
		})

		t.Run("disables link copy when link types differ", func(t *testing.T) {
			type sourceLinks struct {
				Self string
			}
			type targetLinks struct {
				Related string
			}
			type single struct {
				Data  string
				Links sourceLinks
			}
			type list struct {
				Data  []string
				Links targetLinks
			}

			fields := validateSingleToListAdapterTypes[single, list]()
			if fields.copyLinks {
				t.Fatal("expected copyLinks=false when source and target Links types are not assignable")
			}
		})
	})
}

func runOutputRegistryPanicScenarios(t *testing.T) {
	t.Run("helper nil callback panics", func(t *testing.T) {
		t.Run("single linkage nil extractor", func(t *testing.T) {
			type linkage struct{}
			key := typeKey[linkage]()
			cleanupRegistryTypes(t, key)

			expectNilRegistryPanic(t, "linkage extractor", func() {
				registerSingleLinkageRows[linkage](nil)
			})
			assertRegistryTypeAbsent(t, key)
		})

		t.Run("id/state nil extractor", func(t *testing.T) {
			type state struct{}
			key := typeKey[state]()
			cleanupRegistryTypes(t, key)

			expectNilRegistryPanic(t, "id/state extractor", func() {
				registerIDStateRows[state](nil, func(id, value string) ([]string, [][]string) {
					return []string{"id", "state"}, [][]string{{id, value}}
				})
			})
			assertRegistryTypeAbsent(t, key)
		})

		t.Run("id/state nil rows", func(t *testing.T) {
			type state struct{}
			key := typeKey[state]()
			cleanupRegistryTypes(t, key)

			expectNilRegistryPanic(t, "id/state rows function", func() {
				registerIDStateRows[state](func(*state) (string, string) {
					return "id", "value"
				}, nil)
			})
			assertRegistryTypeAbsent(t, key)
		})

		t.Run("id/bool nil rows", func(t *testing.T) {
			type idBool struct{}
			key := typeKey[idBool]()
			cleanupRegistryTypes(t, key)

			expectNilRegistryPanic(t, "id/bool rows function", func() {
				registerIDBoolRows[idBool](func(*idBool) (string, bool) {
					return "id", true
				}, nil)
			})
			assertRegistryTypeAbsent(t, key)
		})

		t.Run("id/bool nil extractor", func(t *testing.T) {
			type idBool struct{}
			key := typeKey[idBool]()
			cleanupRegistryTypes(t, key)

			expectNilRegistryPanic(t, "id/bool extractor", func() {
				registerIDBoolRows[idBool](nil, func(id string, deleted bool) ([]string, [][]string) {
					return []string{"id", "deleted"}, [][]string{{id, fmt.Sprintf("%t", deleted)}}
				})
			})
			assertRegistryTypeAbsent(t, key)
		})

		t.Run("single-resource nil rows", func(t *testing.T) {
			type helperAttrs struct {
				Name string `json:"name"`
			}
			key := typeKey[SingleResponse[helperAttrs]]()
			cleanupRegistryTypes(t, key)

			expectNilRowsFunctionPanic(t, func() {
				registerSingleResourceRowsAdapter[helperAttrs](nil)
			})
			assertRegistryTypeAbsent(t, key)
		})
	})

	t.Run("single-to-list helper validation panics", func(t *testing.T) {
		t.Run("missing data field", func(t *testing.T) {
			type single struct {
				Value string
			}
			type list struct {
				Data []string
			}

			expectPanicContainsAll(t, func() {
				registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
					return []string{"value"}, [][]string{{v.Data[0]}}
				})
			}, "requires Data field")
		})

		t.Run("source not struct", func(t *testing.T) {
			type single string
			type list struct {
				Data []string
			}

			expectPanicContainsAll(t, func() {
				registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
					return nil, nil
				})
			}, "source type must be a struct", typeFor[single]().String())
		})

		t.Run("target not struct", func(t *testing.T) {
			type single struct {
				Data string
			}
			type list []string

			expectPanicContainsAll(t, func() {
				registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
					return nil, nil
				})
			}, "target type must be a struct", typeFor[list]().String())
		})

		t.Run("target data not slice", func(t *testing.T) {
			type single struct {
				Data string
			}
			type list struct {
				Data string
			}

			expectPanicContainsAll(t, func() {
				registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
					return []string{"value"}, [][]string{{v.Data}}
				})
			}, "target Data field must be a slice")
		})

		t.Run("data type mismatch", func(t *testing.T) {
			type single struct {
				Data int
			}
			type list struct {
				Data []string
			}

			expectPanicContainsAll(t, func() {
				registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
					return []string{"value"}, nil
				})
			}, "Data type mismatch source=int target=string")
		})

		t.Run("nil rows function", func(t *testing.T) {
			type single struct {
				Data string
			}
			type list struct {
				Data []string
			}

			singleKey := typeKey[single]()
			cleanupRegistryTypes(t, singleKey)

			expectNilRowsFunctionPanic(t, func() {
				registerSingleToListRowsAdapter[single, list](nil)
			})

			assertRegistryTypeAbsent(t, singleKey)
		})
	})

	t.Run("registrar panics", func(t *testing.T) {
		t.Run("registerRows duplicate and conflict", func(t *testing.T) {
			type duplicate struct{}
			preregisterRowsForConflict[duplicate](t, "value")
			expectDuplicateRegistrationPanic(t, func() {
				registerRowsForConflict[duplicate]("value")
			})

			type conflict struct{}
			preregisterDirectForConflict[conflict](t)
			expectDuplicateRegistrationPanic(t, func() {
				registerRowsForConflict[conflict]("value")
			})
		})

		t.Run("registerRows nil function", func(t *testing.T) {
			type nilRows struct{}
			key := typeKey[nilRows]()
			cleanupRegistryTypes(t, key)

			expectNilRowsFunctionPanic(t, func() {
				registerRows[nilRows](nil)
			})

			assertRegistryTypeAbsent(t, key)
		})

		t.Run("registerRowsErr duplicate conflict and nil", func(t *testing.T) {
			type conflictDirect struct{}
			preregisterDirectForConflict[conflictDirect](t)
			expectDuplicateRegistrationPanic(t, func() {
				registerRowsErrForConflict[conflictDirect]()
			})

			type conflictRows struct{}
			preregisterRowsForConflict[conflictRows](t, "value")
			expectDuplicateRegistrationPanic(t, func() {
				registerRowsErrForConflict[conflictRows]()
			})

			type duplicate struct{}
			preregisterRowsErrForConflict[duplicate](t)
			expectDuplicateRegistrationPanic(t, func() {
				registerRowsErrForConflict[duplicate]()
			})

			type nilRowsErr struct{}
			key := typeKey[nilRowsErr]()
			cleanupRegistryTypes(t, key)
			expectNilRowsFunctionPanic(t, func() {
				registerRowsErr[nilRowsErr](nil)
			})
			assertRegistryTypeAbsent(t, key)
		})

		t.Run("registerDirect duplicate conflict and nil", func(t *testing.T) {
			type conflictRows struct{}
			preregisterRowsForConflict[conflictRows](t, "value")
			expectDuplicateRegistrationPanic(t, func() {
				registerDirectForConflict[conflictRows]()
			})

			type conflictRowsErr struct{}
			preregisterRowsErrForConflict[conflictRowsErr](t)
			expectDuplicateRegistrationPanic(t, func() {
				registerDirectForConflict[conflictRowsErr]()
			})

			type duplicate struct{}
			preregisterDirectForConflict[duplicate](t)
			expectDuplicateRegistrationPanic(t, func() {
				registerDirectForConflict[duplicate]()
			})

			type nilDirect struct{}
			key := typeKey[nilDirect]()
			cleanupRegistryTypes(t, key)
			expectNilDirectRenderFunctionPanic(t, func() {
				registerDirect[nilDirect](nil)
			})
			assertRegistryTypeAbsent(t, key)
		})
	})

	t.Run("registry precheck panics", func(t *testing.T) {
		t.Run("types available duplicate types", func(t *testing.T) {
			type duplicate struct{}
			key := typeKey[duplicate]()
			cleanupRegistryTypes(t, key)

			expectDuplicateRegistrationPanic(t, func() {
				ensureRegistryTypesAvailable(key, key)
			})

			assertRegistryTypeAbsent(t, key)
		})

		t.Run("type available nil type", func(t *testing.T) {
			expectInvalidNilRegistrationTypePanic(t, func() {
				ensureRegistryTypeAvailable(nil)
			})
		})

		t.Run("types available nil type", func(t *testing.T) {
			expectInvalidNilRegistrationTypePanic(t, func() {
				ensureRegistryTypesAvailable(nil)
			})
		})

		t.Run("types available nil before duplicate", func(t *testing.T) {
			expectInvalidNilRegistrationTypePanic(t, func() {
				ensureRegistryTypesAvailable(nil, nil)
			})
		})
	})
}

func expectPanicContainsAll(t *testing.T, fn func(), wants ...string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing all %q", wants)
		}
		got := fmt.Sprint(r)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Fatalf("panic %q does not contain %q", r, want)
			}
		}
	}()
	fn()
}

func expectDuplicateRegistrationPanic(t *testing.T, fn func()) {
	t.Helper()
	expectPanicContainsAll(t, fn, "duplicate registration")
}

func expectInvalidNilRegistrationTypePanic(t *testing.T, fn func()) {
	t.Helper()
	expectPanicContainsAll(t, fn, "invalid nil registration type")
}

func expectNilRegistryPanic(t *testing.T, kind string, fn func()) {
	t.Helper()
	expectPanicContainsAll(t, fn, "nil "+kind)
}

func expectNilRowsFunctionPanic(t *testing.T, fn func()) {
	t.Helper()
	expectNilRegistryPanic(t, "rows function", fn)
}

func expectNilDirectRenderFunctionPanic(t *testing.T, fn func()) {
	t.Helper()
	expectNilRegistryPanic(t, "direct render function", fn)
}

func renderWithRows(headers []string, rows [][]string) directRenderFunc {
	return func(_ any, render func([]string, [][]string)) error {
		render(headers, rows)
		return nil
	}
}

func seedRowsAndDirectHandlersForTest(typ reflect.Type, directFn directRenderFunc) func() bool {
	rowsHandlerCalled := false
	outputRegistry[typ] = func(any) ([]string, [][]string, error) {
		rowsHandlerCalled = true
		return []string{"source"}, [][]string{{"rows"}}, nil
	}
	directRenderRegistry[typ] = directFn

	return func() bool {
		return rowsHandlerCalled
	}
}

func assertRowContains(t *testing.T, headers []string, rows [][]string, expected ...string) {
	t.Helper()
	if len(headers) == 0 || len(rows) == 0 {
		t.Fatalf("expected non-empty headers/rows, got headers=%v rows=%v", headers, rows)
	}
	const minColumns = 2
	if len(rows[0]) < minColumns {
		t.Fatalf("expected at least %d columns in row, got row=%v", minColumns, rows[0])
	}
	joined := strings.Join(rows[0], " ")
	for _, want := range expected {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected row to contain %q, got row=%v", want, rows[0])
		}
	}
}

func assertSingleRowEquals(t *testing.T, headers []string, rows [][]string, wantHeaders []string, wantRow []string) {
	t.Helper()
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("unexpected headers: got=%v want=%v", headers, wantHeaders)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row, got %d (%v)", len(rows), rows)
	}
	if !reflect.DeepEqual(rows[0], wantRow) {
		t.Fatalf("unexpected row: got=%v want=%v", rows[0], wantRow)
	}
}

func cleanupRegistryTypes(t *testing.T, types ...reflect.Type) {
	t.Helper()
	t.Cleanup(func() {
		for _, typ := range types {
			delete(outputRegistry, typ)
			delete(directRenderRegistry, typ)
		}
	})
}

func assertRegistryTypeAbsent(t *testing.T, typ reflect.Type) {
	t.Helper()
	if _, exists := outputRegistry[typ]; exists {
		t.Fatalf("registry type %v should be absent from output registry", typ)
	}
	if _, exists := directRenderRegistry[typ]; exists {
		t.Fatalf("registry type %v should be absent from direct render registry", typ)
	}
}

func requireOutputHandler(t *testing.T, typ reflect.Type, label string) rowsFunc {
	t.Helper()
	handler, ok := outputRegistry[typ]
	if !ok || handler == nil {
		t.Fatalf("expected %s handler for type %v", label, typ)
	}
	return handler
}

func requireOutputHandlerFor[T any](t *testing.T, label string) rowsFunc {
	t.Helper()
	return requireOutputHandler(t, typeKey[T](), label)
}

func registerRowsForConflict[T any](headers ...string) {
	if len(headers) == 0 {
		headers = []string{"value"}
	}

	registerRows(func(*T) ([]string, [][]string) {
		return headers, nil
	})
}

func preregisterRowsForConflict[T any](t *testing.T, headers ...string) reflect.Type {
	t.Helper()

	return preregisterConflictType[T](t, func() {
		registerRowsForConflict[T](headers...)
	})
}

func registerRowsErrForConflict[T any]() {
	registerRowsErr(func(*T) ([]string, [][]string, error) {
		return nil, nil, nil
	})
}

func preregisterRowsErrForConflict[T any](t *testing.T) reflect.Type {
	t.Helper()

	return preregisterConflictType[T](t, registerRowsErrForConflict[T])
}

func registerDirectForConflict[T any]() {
	registerDirect(func(*T, func([]string, [][]string)) error {
		return nil
	})
}

func preregisterDirectForConflict[T any](t *testing.T) reflect.Type {
	t.Helper()

	return preregisterConflictType[T](t, registerDirectForConflict[T])
}

func preregisterConflictType[T any](t *testing.T, register func()) reflect.Type {
	t.Helper()

	key := typeKey[T]()
	cleanupRegistryTypes(t, key)
	register()
	return key
}

func typeKey[T any]() reflect.Type {
	return reflect.TypeFor[*T]()
}
