package asc

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

// TestScalarQueryParametersSendAndOmit pins, for the builder lines that send
// one string, integer or boolean parameter, which values send the parameter
// and which leave it out: a string is trimmed and omitted when blank, an
// integer is omitted unless positive, a boolean is sent as true or false.
func TestScalarQueryParametersSendAndOmit(t *testing.T) {
	stringParams := []struct {
		param string
		via   string
		send  func(string) url.Values
	}{
		{"filter[name]", "buildBetaGroupsQuery WithBetaGroupsName", func(v string) url.Values { return builtQuery(t, buildBetaGroupsQuery, WithBetaGroupsName(v)) }},
		{"filter[publicLink]", "buildBetaGroupsQuery WithBetaGroupsPublicLink", func(v string) url.Values { return builtQuery(t, buildBetaGroupsQuery, WithBetaGroupsPublicLink(v)) }},
		{"sort", "buildBetaGroupsQuery WithBetaGroupsSort", func(v string) url.Values { return builtQuery(t, buildBetaGroupsQuery, WithBetaGroupsSort(v)) }},
		{"sort", "buildBetaTestersQuery WithBetaTestersSort", func(v string) url.Values {
			return builtQuery(t, func(q *betaTestersQuery) string {
				query, err := buildBetaTestersQuery("", q)
				if err != nil {
					t.Fatalf("buildBetaTestersQuery() error: %v", err)
				}
				return query
			}, WithBetaTestersSort(v))
		}},
		{"granularity", "buildGCMatchmakingQueueSizesQuery WithGCMatchmakingMetricsGranularity", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingQueueSizesQuery, WithGCMatchmakingMetricsGranularity(v))
		}},
		{"granularity", "buildGCMatchmakingQueueRequestsQuery WithGCMatchmakingMetricsGranularity", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingQueueRequestsQuery, WithGCMatchmakingMetricsGranularity(v))
		}},
		{"filter[result]", "buildGCMatchmakingQueueRequestsQuery WithGCMatchmakingMetricsFilterResult", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingQueueRequestsQuery, WithGCMatchmakingMetricsFilterResult(v))
		}},
		{"filter[gameCenterDetail]", "buildGCMatchmakingQueueRequestsQuery WithGCMatchmakingMetricsFilterGameCenterDetail", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingQueueRequestsQuery, WithGCMatchmakingMetricsFilterGameCenterDetail(v))
		}},
		{"granularity", "buildGCMatchmakingQueueSessionsQuery WithGCMatchmakingMetricsGranularity", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingQueueSessionsQuery, WithGCMatchmakingMetricsGranularity(v))
		}},
		{"granularity", "buildGCMatchmakingRuleMetricsQuery WithGCMatchmakingMetricsGranularity", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingRuleMetricsQuery, WithGCMatchmakingMetricsGranularity(v))
		}},
		{"filter[result]", "buildGCMatchmakingRuleMetricsQuery WithGCMatchmakingMetricsFilterResult", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingRuleMetricsQuery, WithGCMatchmakingMetricsFilterResult(v))
		}},
		{"filter[gameCenterMatchmakingQueue]", "buildGCMatchmakingRuleMetricsQuery WithGCMatchmakingMetricsFilterQueue", func(v string) url.Values {
			return builtQuery(t, buildGCMatchmakingRuleMetricsQuery, WithGCMatchmakingMetricsFilterQueue(v))
		}},
		{"sort", "buildAppsQuery WithAppsSort", func(v string) url.Values { return builtQuery(t, buildAppsQuery, WithAppsSort(v)) }},
		{"sort", "buildAppTagsQuery WithAppTagsSort", func(v string) url.Values { return builtQuery(t, buildAppTagsQuery, WithAppTagsSort(v)) }},
		{"sort", "buildNominationsQuery WithNominationsSort", func(v string) url.Values { return builtQuery(t, buildNominationsQuery, WithNominationsSort(v)) }},
		{"sort", "buildBuildUploadsQuery WithBuildUploadsSort", func(v string) url.Values { return builtQuery(t, buildBuildUploadsQuery, WithBuildUploadsSort(v)) }},
		{"sort", "buildDevicesQuery WithDevicesSort", func(v string) url.Values { return builtQuery(t, buildDevicesQuery, WithDevicesSort(v)) }},
		{"sort", "buildFeedbackQuery WithFeedbackSort", func(v string) url.Values { return builtQuery(t, buildFeedbackQuery, WithFeedbackSort(v)) }},
		{"sort", "buildCrashQuery WithCrashSort", func(v string) url.Values { return builtQuery(t, buildCrashQuery, WithCrashSort(v)) }},
		{"sort", "buildCiBuildRunsQuery WithCiBuildRunsSort", func(v string) url.Values { return builtQuery(t, buildCiBuildRunsQuery, WithCiBuildRunsSort(v)) }},
		{"filter[app]", "buildCiProductsQuery WithCiProductsAppID", func(v string) url.Values { return builtQuery(t, buildCiProductsQuery, WithCiProductsAppID(v)) }},
		{"sort", "buildBackgroundAssetsQuery WithBackgroundAssetsSort", func(v string) url.Values {
			return builtQuery(t, buildBackgroundAssetsQuery, WithBackgroundAssetsSort(v))
		}},
		{"sort", "buildBackgroundAssetVersionsQuery WithBackgroundAssetVersionsSort", func(v string) url.Values {
			return builtQuery(t, buildBackgroundAssetVersionsQuery, WithBackgroundAssetVersionsSort(v))
		}},
		{"sort", "buildReviewQuery WithReviewSort", func(v string) url.Values { return reviewQueryValues(t, WithReviewSort(v)) }},
		{"filter[reviewTerritory]", "buildReviewQuery WithReviewTerritory", func(v string) url.Values { return reviewQueryValues(t, WithReviewTerritory(v)) }},
	}
	for _, tt := range stringParams {
		for value, want := range map[string][]string{"": nil, "   ": nil, " value ": {"value"}} {
			if got := tt.send(value)[tt.param]; !reflect.DeepEqual(got, want) {
				t.Errorf("%s(%q) sent %s=%q, want %q", tt.via, value, tt.param, got, want)
			}
		}
	}

	intParams := []struct {
		param string
		via   string
		send  func(int) url.Values
	}{
		{"limit[images]", "buildSubscriptionVersionsQuery WithSubscriptionVersionsImageLimit", func(v int) url.Values {
			return builtQuery(t, buildSubscriptionVersionsQuery, WithSubscriptionVersionsImageLimit(v))
		}},
		{"limit[localizations]", "buildSubscriptionVersionsQuery WithSubscriptionVersionsLocalizationLimit", func(v int) url.Values {
			return builtQuery(t, buildSubscriptionVersionsQuery, WithSubscriptionVersionsLocalizationLimit(v))
		}},
		{"limit[images]", "buildSubscriptionVersionQuery WithSubscriptionVersionImageLimit", func(v int) url.Values {
			return builtQuery(t, buildSubscriptionVersionQuery, WithSubscriptionVersionImageLimit(v))
		}},
		{"limit[localizations]", "buildSubscriptionVersionQuery WithSubscriptionVersionLocalizationLimit", func(v int) url.Values {
			return builtQuery(t, buildSubscriptionVersionQuery, WithSubscriptionVersionLocalizationLimit(v))
		}},
		{"limit[versions]", "buildSubscriptionsQuery WithSubscriptionsVersionLimit", func(v int) url.Values {
			return builtQuery(t, buildSubscriptionsQuery, WithSubscriptionsVersionLimit(v))
		}},
		{"limit[versions]", "buildSubscriptionQuery WithSubscriptionVersionLimit", func(v int) url.Values { return builtQuery(t, buildSubscriptionQuery, WithSubscriptionVersionLimit(v)) }},
	}
	for _, tt := range intParams {
		for value, want := range map[int][]string{-1: nil, 0: nil, 5: {"5"}} {
			if got := tt.send(value)[tt.param]; !reflect.DeepEqual(got, want) {
				t.Errorf("%s(%d) sent %s=%q, want %q", tt.via, value, tt.param, got, want)
			}
		}
	}

	boolParams := []struct {
		param string
		via   string
		send  func(bool) url.Values
	}{
		{"filter[expired]", "GetBuilds WithBuildsExpired", func(v bool) url.Values { return buildsRequestQuery(t, WithBuildsExpired(v)) }},
		{"exists[publishedResponse]", "buildReviewQuery WithPublishedResponseExists", func(v bool) url.Values { return reviewQueryValues(t, WithPublishedResponseExists(v)) }},
	}
	for _, tt := range boolParams {
		for value, want := range map[bool][]string{false: {"false"}, true: {"true"}} {
			if got := tt.send(value)[tt.param]; !reflect.DeepEqual(got, want) {
				t.Errorf("%s(%t) sent %s=%q, want %q", tt.via, value, tt.param, got, want)
			}
		}
	}
}

// builtQuery applies the options to an empty query and parses what the
// builder sends.
func builtQuery[Q any, O ~func(*Q)](t *testing.T, build func(*Q) string, opts ...O) url.Values {
	t.Helper()
	query := new(Q)
	for _, opt := range opts {
		opt(query)
	}
	values, err := url.ParseQuery(build(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	return values
}

func reviewQueryValues(t *testing.T, opts ...ReviewOption) url.Values {
	t.Helper()
	values, err := url.ParseQuery(buildReviewQuery(opts))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	return values
}

// buildsRequestQuery returns the query GetBuilds sends, which it builds inside
// the method.
func buildsRequestQuery(t *testing.T, opts ...BuildsOption) url.Values {
	t.Helper()
	var sent url.Values
	client := newTestClient(t, func(req *http.Request) { sent = req.URL.Query() }, jsonResponse(http.StatusOK, `{"data":[]}`))
	if _, err := client.GetBuilds(context.Background(), "123", opts...); err != nil {
		t.Fatalf("GetBuilds() error: %v", err)
	}
	return sent
}
