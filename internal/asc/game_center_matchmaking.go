package asc

import (
	"encoding/json"
	"net/url"
	"strings"
)

// GameCenterMatchmakingQueueAttributes represents a matchmaking queue resource.
type GameCenterMatchmakingQueueAttributes struct {
	ReferenceName               string   `json:"referenceName"`
	ClassicMatchmakingBundleIDs []string `json:"classicMatchmakingBundleIds,omitempty"`
}

// GameCenterMatchmakingQueueRelationships describes relationships for matchmaking queues.
type GameCenterMatchmakingQueueRelationships struct {
	RuleSet           *Relationship `json:"ruleSet,omitempty"`
	ExperimentRuleSet *Relationship `json:"experimentRuleSet,omitempty"`
}

// GameCenterMatchmakingQueueUpdateData is the data portion of a queue update request.
type GameCenterMatchmakingQueueUpdateData struct {
	Type          ResourceType                                `json:"type"`
	ID            string                                      `json:"id"`
	Attributes    *GameCenterMatchmakingQueueUpdateAttributes `json:"attributes,omitempty"`
	Relationships *GameCenterMatchmakingQueueRelationships    `json:"relationships,omitempty"`
}

// GameCenterMatchmakingRuleSetAttributes represents a matchmaking rule set resource.
type GameCenterMatchmakingRuleSetAttributes struct {
	ReferenceName       string `json:"referenceName"`
	RuleLanguageVersion int    `json:"ruleLanguageVersion"`
	MinPlayers          int    `json:"minPlayers"`
	MaxPlayers          int    `json:"maxPlayers"`
}

// GameCenterMatchmakingRuleAttributes represents a matchmaking rule resource.
type GameCenterMatchmakingRuleAttributes struct {
	ReferenceName string  `json:"referenceName"`
	Description   string  `json:"description"`
	Type          string  `json:"type"`
	Expression    string  `json:"expression"`
	Weight        float64 `json:"weight,omitempty"`
}

// GameCenterMatchmakingRuleRelationships describes relationships for matchmaking rules.
type GameCenterMatchmakingRuleRelationships struct {
	RuleSet *Relationship `json:"ruleSet,omitempty"`
}

// GameCenterMatchmakingRulesResponse is the response from rule list endpoints.
type GameCenterMatchmakingRulesResponse = Response[GameCenterMatchmakingRuleAttributes]

// GameCenterMatchmakingTeamAttributes represents a matchmaking team resource.
type GameCenterMatchmakingTeamAttributes struct {
	ReferenceName string `json:"referenceName"`
	MinPlayers    int    `json:"minPlayers"`
	MaxPlayers    int    `json:"maxPlayers"`
}

// GameCenterMatchmakingTeamRelationships describes relationships for matchmaking teams.
type GameCenterMatchmakingTeamRelationships struct {
	RuleSet *Relationship `json:"ruleSet,omitempty"`
}

// GameCenterMatchmakingTeamsResponse is the response from team list endpoints.
type GameCenterMatchmakingTeamsResponse = Response[GameCenterMatchmakingTeamAttributes]

// GameCenterMetricsDataPoint represents a metrics data point.
type GameCenterMetricsDataPoint struct {
	Start  string         `json:"start,omitempty"`
	End    string         `json:"end,omitempty"`
	Values map[string]any `json:"values,omitempty"`
}

// GameCenterMetricsDimension represents a metrics dimension entry.
type GameCenterMetricsDimension struct {
	Data any `json:"data,omitempty"`
}

// GameCenterMetricsData represents a metrics row.
type GameCenterMetricsData struct {
	DataPoints  []GameCenterMetricsDataPoint          `json:"dataPoints"`
	Dimensions  map[string]GameCenterMetricsDimension `json:"dimensions,omitempty"`
	Granularity any                                   `json:"granularity,omitempty"`
}

// GameCenterMetricsResponse represents a generic metrics response.
type GameCenterMetricsResponse struct {
	Data  []GameCenterMetricsData `json:"data"`
	Links Links                   `json:"links"`
	Meta  json.RawMessage         `json:"meta,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *GameCenterMetricsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *GameCenterMetricsResponse) GetData() any {
	return r.Data
}

// GameCenterMatchmakingQueueSizesResponse is the response for matchmaking queue sizes metrics.
type GameCenterMatchmakingQueueSizesResponse = GameCenterMetricsResponse

// GameCenterMatchmakingQueueRequestsResponse is the response for matchmaking queue requests metrics.
type GameCenterMatchmakingQueueRequestsResponse = GameCenterMetricsResponse

// GameCenterMatchmakingQueueSessionsResponse is the response for matchmaking sessions metrics.
type GameCenterMatchmakingQueueSessionsResponse = GameCenterMetricsResponse

// GameCenterMatchmakingQueueExperimentSizesResponse is the response for experiment queue sizes metrics.
type GameCenterMatchmakingQueueExperimentSizesResponse = GameCenterMetricsResponse

// GameCenterMatchmakingQueueExperimentRequestsResponse is the response for experiment queue requests metrics.
type GameCenterMatchmakingQueueExperimentRequestsResponse = GameCenterMetricsResponse

// GameCenterMatchmakingBooleanRuleResultsResponse is the response for boolean rule results metrics.
type GameCenterMatchmakingBooleanRuleResultsResponse = GameCenterMetricsResponse

// GameCenterMatchmakingNumberRuleResultsResponse is the response for number rule results metrics.
type GameCenterMatchmakingNumberRuleResultsResponse = GameCenterMetricsResponse

// GameCenterMatchmakingRuleErrorsResponse is the response for rule errors metrics.
type GameCenterMatchmakingRuleErrorsResponse = GameCenterMetricsResponse

// GCMatchmakingMetricsOption is a functional option for matchmaking metrics queries.
type GCMatchmakingMetricsOption func(*gcMatchmakingMetricsQuery)

type gcMatchmakingMetricsQuery struct {
	listQuery
	granularity                      string
	sort                             []string
	groupBy                          []string
	filterResult                     string
	filterGameCenterDetail           string
	filterGameCenterMatchmakingQueue string
}

// WithGCMatchmakingMetricsGranularity sets the granularity value.
func WithGCMatchmakingMetricsGranularity(granularity string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		q.granularity = strings.TrimSpace(granularity)
	}
}

// WithGCMatchmakingMetricsSort sets the sort fields.
func WithGCMatchmakingMetricsSort(sort []string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		q.sort = normalizeList(sort)
	}
}

// WithGCMatchmakingMetricsGroupBy sets the groupBy fields.
func WithGCMatchmakingMetricsGroupBy(groupBy []string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		q.groupBy = normalizeList(groupBy)
	}
}

// WithGCMatchmakingMetricsFilterResult sets the result filter.
func WithGCMatchmakingMetricsFilterResult(value string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		q.filterResult = strings.TrimSpace(value)
	}
}

// WithGCMatchmakingMetricsFilterGameCenterDetail sets the game center detail filter.
func WithGCMatchmakingMetricsFilterGameCenterDetail(value string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		q.filterGameCenterDetail = strings.TrimSpace(value)
	}
}

// WithGCMatchmakingMetricsFilterQueue sets the matchmaking queue filter.
func WithGCMatchmakingMetricsFilterQueue(value string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		q.filterGameCenterMatchmakingQueue = strings.TrimSpace(value)
	}
}

func buildGCMatchmakingQueueSizesQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.granularity) != "" {
		values.Set("granularity", strings.TrimSpace(query.granularity))
	}
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildGCMatchmakingQueueRequestsQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.granularity) != "" {
		values.Set("granularity", strings.TrimSpace(query.granularity))
	}
	addCSV(values, "groupBy", query.groupBy)
	if strings.TrimSpace(query.filterResult) != "" {
		values.Set("filter[result]", strings.TrimSpace(query.filterResult))
	}
	if strings.TrimSpace(query.filterGameCenterDetail) != "" {
		values.Set("filter[gameCenterDetail]", strings.TrimSpace(query.filterGameCenterDetail))
	}
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildGCMatchmakingQueueSessionsQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.granularity) != "" {
		values.Set("granularity", strings.TrimSpace(query.granularity))
	}
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildGCMatchmakingRuleMetricsQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.granularity) != "" {
		values.Set("granularity", strings.TrimSpace(query.granularity))
	}
	addCSV(values, "groupBy", query.groupBy)
	if strings.TrimSpace(query.filterResult) != "" {
		values.Set("filter[result]", strings.TrimSpace(query.filterResult))
	}
	if strings.TrimSpace(query.filterGameCenterMatchmakingQueue) != "" {
		values.Set("filter[gameCenterMatchmakingQueue]", strings.TrimSpace(query.filterGameCenterMatchmakingQueue))
	}
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterMatchmakingRuleSetTestAttributes represents a rule set test result.
type GameCenterMatchmakingRuleSetTestAttributes struct {
	MatchmakingResults []any `json:"matchmakingResults,omitempty"`
}
