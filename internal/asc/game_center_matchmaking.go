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

// GameCenterMatchmakingQueueCreateAttributes describes attributes for creating a queue.
type GameCenterMatchmakingQueueCreateAttributes struct {
	ReferenceName               string   `json:"referenceName"`
	ClassicMatchmakingBundleIDs []string `json:"classicMatchmakingBundleIds,omitempty"`
}

// GameCenterMatchmakingQueueUpdateAttributes describes attributes for updating a queue.
type GameCenterMatchmakingQueueUpdateAttributes struct {
	ClassicMatchmakingBundleIDs []string `json:"classicMatchmakingBundleIds,omitempty"`
}

// GameCenterMatchmakingQueueRelationships describes relationships for matchmaking queues.
type GameCenterMatchmakingQueueRelationships struct {
	RuleSet           *Relationship `json:"ruleSet,omitempty"`
	ExperimentRuleSet *Relationship `json:"experimentRuleSet,omitempty"`
}

// GameCenterMatchmakingQueueCreateData is the data portion of a queue create request.
type GameCenterMatchmakingQueueCreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterMatchmakingQueueCreateAttributes `json:"attributes"`
	Relationships *GameCenterMatchmakingQueueRelationships   `json:"relationships,omitempty"`
}

// GameCenterMatchmakingQueueCreateRequest is a request to create a queue.
type GameCenterMatchmakingQueueCreateRequest struct {
	Data GameCenterMatchmakingQueueCreateData `json:"data"`
}

// GameCenterMatchmakingQueueUpdateData is the data portion of a queue update request.
type GameCenterMatchmakingQueueUpdateData struct {
	Type          ResourceType                                `json:"type"`
	ID            string                                      `json:"id"`
	Attributes    *GameCenterMatchmakingQueueUpdateAttributes `json:"attributes,omitempty"`
	Relationships *GameCenterMatchmakingQueueRelationships    `json:"relationships,omitempty"`
}

// GameCenterMatchmakingQueueUpdateRequest is a request to update a queue.
type GameCenterMatchmakingQueueUpdateRequest struct {
	Data GameCenterMatchmakingQueueUpdateData `json:"data"`
}

// GameCenterMatchmakingQueuesResponse is the response from queue list endpoints.
type GameCenterMatchmakingQueuesResponse = Response[GameCenterMatchmakingQueueAttributes]

// GameCenterMatchmakingQueueResponse is the response from queue detail endpoints.
type GameCenterMatchmakingQueueResponse = SingleResponse[GameCenterMatchmakingQueueAttributes]

// GameCenterMatchmakingQueueDeleteResult represents CLI output for queue deletions.
type GameCenterMatchmakingQueueDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCMatchmakingQueuesOption is a functional option for GetGameCenterMatchmakingQueues.
type GCMatchmakingQueuesOption func(*gcMatchmakingQueuesQuery)

type gcMatchmakingQueuesQuery struct {
	listQuery
}

// WithGCMatchmakingQueuesLimit sets the max number of queues to return.
func WithGCMatchmakingQueuesLimit(limit int) GCMatchmakingQueuesOption {
	return func(q *gcMatchmakingQueuesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCMatchmakingQueuesNextURL uses a next page URL directly.
func WithGCMatchmakingQueuesNextURL(next string) GCMatchmakingQueuesOption {
	return func(q *gcMatchmakingQueuesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCMatchmakingQueuesQuery(query *gcMatchmakingQueuesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterMatchmakingRuleSetAttributes represents a matchmaking rule set resource.
type GameCenterMatchmakingRuleSetAttributes struct {
	ReferenceName       string `json:"referenceName"`
	RuleLanguageVersion int    `json:"ruleLanguageVersion"`
	MinPlayers          int    `json:"minPlayers"`
	MaxPlayers          int    `json:"maxPlayers"`
}

// GameCenterMatchmakingRuleSetCreateAttributes describes attributes for creating a rule set.
type GameCenterMatchmakingRuleSetCreateAttributes struct {
	ReferenceName       string `json:"referenceName"`
	RuleLanguageVersion int    `json:"ruleLanguageVersion"`
	MinPlayers          int    `json:"minPlayers"`
	MaxPlayers          int    `json:"maxPlayers"`
}

// GameCenterMatchmakingRuleSetUpdateAttributes describes attributes for updating a rule set.
type GameCenterMatchmakingRuleSetUpdateAttributes struct {
	MinPlayers *int `json:"minPlayers,omitempty"`
	MaxPlayers *int `json:"maxPlayers,omitempty"`
}

// GameCenterMatchmakingRuleSetCreateData is the data portion of a rule set create request.
type GameCenterMatchmakingRuleSetCreateData struct {
	Type       ResourceType                                 `json:"type"`
	Attributes GameCenterMatchmakingRuleSetCreateAttributes `json:"attributes"`
}

// GameCenterMatchmakingRuleSetCreateRequest is a request to create a rule set.
type GameCenterMatchmakingRuleSetCreateRequest struct {
	Data GameCenterMatchmakingRuleSetCreateData `json:"data"`
}

// GameCenterMatchmakingRuleSetUpdateData is the data portion of a rule set update request.
type GameCenterMatchmakingRuleSetUpdateData struct {
	Type       ResourceType                                  `json:"type"`
	ID         string                                        `json:"id"`
	Attributes *GameCenterMatchmakingRuleSetUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterMatchmakingRuleSetUpdateRequest is a request to update a rule set.
type GameCenterMatchmakingRuleSetUpdateRequest struct {
	Data GameCenterMatchmakingRuleSetUpdateData `json:"data"`
}

// GameCenterMatchmakingRuleSetsResponse is the response from rule set list endpoints.
type GameCenterMatchmakingRuleSetsResponse = Response[GameCenterMatchmakingRuleSetAttributes]

// GameCenterMatchmakingRuleSetResponse is the response from rule set detail endpoints.
type GameCenterMatchmakingRuleSetResponse = SingleResponse[GameCenterMatchmakingRuleSetAttributes]

// GameCenterMatchmakingRuleSetDeleteResult represents CLI output for rule set deletions.
type GameCenterMatchmakingRuleSetDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCMatchmakingRuleSetsOption is a functional option for GetGameCenterMatchmakingRuleSets.
type GCMatchmakingRuleSetsOption func(*gcMatchmakingRuleSetsQuery)

type gcMatchmakingRuleSetsQuery struct {
	listQuery
}

// WithGCMatchmakingRuleSetsLimit sets the max number of rule sets to return.
func WithGCMatchmakingRuleSetsLimit(limit int) GCMatchmakingRuleSetsOption {
	return func(q *gcMatchmakingRuleSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCMatchmakingRuleSetsNextURL uses a next page URL directly.
func WithGCMatchmakingRuleSetsNextURL(next string) GCMatchmakingRuleSetsOption {
	return func(q *gcMatchmakingRuleSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCMatchmakingRuleSetsQuery(query *gcMatchmakingRuleSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterMatchmakingRuleAttributes represents a matchmaking rule resource.
type GameCenterMatchmakingRuleAttributes struct {
	ReferenceName string  `json:"referenceName"`
	Description   string  `json:"description"`
	Type          string  `json:"type"`
	Expression    string  `json:"expression"`
	Weight        float64 `json:"weight,omitempty"`
}

// GameCenterMatchmakingRuleCreateAttributes describes attributes for creating a rule.
type GameCenterMatchmakingRuleCreateAttributes struct {
	ReferenceName string   `json:"referenceName"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Expression    string   `json:"expression"`
	Weight        *float64 `json:"weight,omitempty"`
}

// GameCenterMatchmakingRuleUpdateAttributes describes attributes for updating a rule.
type GameCenterMatchmakingRuleUpdateAttributes struct {
	Description *string  `json:"description,omitempty"`
	Expression  *string  `json:"expression,omitempty"`
	Weight      *float64 `json:"weight,omitempty"`
}

// GameCenterMatchmakingRuleRelationships describes relationships for matchmaking rules.
type GameCenterMatchmakingRuleRelationships struct {
	RuleSet *Relationship `json:"ruleSet,omitempty"`
}

// GameCenterMatchmakingRuleCreateData is the data portion of a rule create request.
type GameCenterMatchmakingRuleCreateData struct {
	Type          ResourceType                              `json:"type"`
	Attributes    GameCenterMatchmakingRuleCreateAttributes `json:"attributes"`
	Relationships *GameCenterMatchmakingRuleRelationships   `json:"relationships,omitempty"`
}

// GameCenterMatchmakingRuleCreateRequest is a request to create a rule.
type GameCenterMatchmakingRuleCreateRequest struct {
	Data GameCenterMatchmakingRuleCreateData `json:"data"`
}

// GameCenterMatchmakingRuleUpdateData is the data portion of a rule update request.
type GameCenterMatchmakingRuleUpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *GameCenterMatchmakingRuleUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterMatchmakingRuleUpdateRequest is a request to update a rule.
type GameCenterMatchmakingRuleUpdateRequest struct {
	Data GameCenterMatchmakingRuleUpdateData `json:"data"`
}

// GameCenterMatchmakingRulesResponse is the response from rule list endpoints.
type GameCenterMatchmakingRulesResponse = Response[GameCenterMatchmakingRuleAttributes]

// GameCenterMatchmakingRuleResponse is the response from rule detail endpoints.
type GameCenterMatchmakingRuleResponse = SingleResponse[GameCenterMatchmakingRuleAttributes]

// GameCenterMatchmakingRuleDeleteResult represents CLI output for rule deletions.
type GameCenterMatchmakingRuleDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCMatchmakingRulesOption is a functional option for GetGameCenterMatchmakingRules.
type GCMatchmakingRulesOption func(*gcMatchmakingRulesQuery)

type gcMatchmakingRulesQuery struct {
	listQuery
}

// WithGCMatchmakingRulesLimit sets the max number of rules to return.
func WithGCMatchmakingRulesLimit(limit int) GCMatchmakingRulesOption {
	return func(q *gcMatchmakingRulesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCMatchmakingRulesNextURL uses a next page URL directly.
func WithGCMatchmakingRulesNextURL(next string) GCMatchmakingRulesOption {
	return func(q *gcMatchmakingRulesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCMatchmakingRulesQuery(query *gcMatchmakingRulesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterMatchmakingTeamAttributes represents a matchmaking team resource.
type GameCenterMatchmakingTeamAttributes struct {
	ReferenceName string `json:"referenceName"`
	MinPlayers    int    `json:"minPlayers"`
	MaxPlayers    int    `json:"maxPlayers"`
}

// GameCenterMatchmakingTeamCreateAttributes describes attributes for creating a team.
type GameCenterMatchmakingTeamCreateAttributes struct {
	ReferenceName string `json:"referenceName"`
	MinPlayers    int    `json:"minPlayers"`
	MaxPlayers    int    `json:"maxPlayers"`
}

// GameCenterMatchmakingTeamUpdateAttributes describes attributes for updating a team.
type GameCenterMatchmakingTeamUpdateAttributes struct {
	MinPlayers *int `json:"minPlayers,omitempty"`
	MaxPlayers *int `json:"maxPlayers,omitempty"`
}

// GameCenterMatchmakingTeamRelationships describes relationships for matchmaking teams.
type GameCenterMatchmakingTeamRelationships struct {
	RuleSet *Relationship `json:"ruleSet,omitempty"`
}

// GameCenterMatchmakingTeamCreateData is the data portion of a team create request.
type GameCenterMatchmakingTeamCreateData struct {
	Type          ResourceType                              `json:"type"`
	Attributes    GameCenterMatchmakingTeamCreateAttributes `json:"attributes"`
	Relationships *GameCenterMatchmakingTeamRelationships   `json:"relationships,omitempty"`
}

// GameCenterMatchmakingTeamCreateRequest is a request to create a team.
type GameCenterMatchmakingTeamCreateRequest struct {
	Data GameCenterMatchmakingTeamCreateData `json:"data"`
}

// GameCenterMatchmakingTeamUpdateData is the data portion of a team update request.
type GameCenterMatchmakingTeamUpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *GameCenterMatchmakingTeamUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterMatchmakingTeamUpdateRequest is a request to update a team.
type GameCenterMatchmakingTeamUpdateRequest struct {
	Data GameCenterMatchmakingTeamUpdateData `json:"data"`
}

// GameCenterMatchmakingTeamsResponse is the response from team list endpoints.
type GameCenterMatchmakingTeamsResponse = Response[GameCenterMatchmakingTeamAttributes]

// GameCenterMatchmakingTeamResponse is the response from team detail endpoints.
type GameCenterMatchmakingTeamResponse = SingleResponse[GameCenterMatchmakingTeamAttributes]

// GameCenterMatchmakingTeamDeleteResult represents CLI output for team deletions.
type GameCenterMatchmakingTeamDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCMatchmakingTeamsOption is a functional option for GetGameCenterMatchmakingTeams.
type GCMatchmakingTeamsOption func(*gcMatchmakingTeamsQuery)

type gcMatchmakingTeamsQuery struct {
	listQuery
}

// WithGCMatchmakingTeamsLimit sets the max number of teams to return.
func WithGCMatchmakingTeamsLimit(limit int) GCMatchmakingTeamsOption {
	return func(q *gcMatchmakingTeamsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCMatchmakingTeamsNextURL uses a next page URL directly.
func WithGCMatchmakingTeamsNextURL(next string) GCMatchmakingTeamsOption {
	return func(q *gcMatchmakingTeamsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCMatchmakingTeamsQuery(query *gcMatchmakingTeamsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

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

// WithGCMatchmakingMetricsLimit sets the max number of groups to return.
func WithGCMatchmakingMetricsLimit(limit int) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCMatchmakingMetricsNextURL uses a next page URL directly.
func WithGCMatchmakingMetricsNextURL(next string) GCMatchmakingMetricsOption {
	return func(q *gcMatchmakingMetricsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCMatchmakingQueueSizesQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	addValue(values, "granularity", query.granularity)
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildGCMatchmakingQueueRequestsQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	addValue(values, "granularity", query.granularity)
	addCSV(values, "groupBy", query.groupBy)
	addValue(values, "filter[result]", query.filterResult)
	addValue(values, "filter[gameCenterDetail]", query.filterGameCenterDetail)
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildGCMatchmakingQueueSessionsQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	addValue(values, "granularity", query.granularity)
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildGCMatchmakingRuleMetricsQuery(query *gcMatchmakingMetricsQuery) string {
	values := url.Values{}
	addValue(values, "granularity", query.granularity)
	addCSV(values, "groupBy", query.groupBy)
	addValue(values, "filter[result]", query.filterResult)
	addValue(values, "filter[gameCenterMatchmakingQueue]", query.filterGameCenterMatchmakingQueue)
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterMatchmakingRuleSetTestAttributes represents a rule set test result.
type GameCenterMatchmakingRuleSetTestAttributes struct {
	MatchmakingResults []any `json:"matchmakingResults,omitempty"`
}

// GameCenterMatchmakingRuleSetTestResponse is the response for a rule set test.
type GameCenterMatchmakingRuleSetTestResponse = SingleResponse[GameCenterMatchmakingRuleSetTestAttributes]
