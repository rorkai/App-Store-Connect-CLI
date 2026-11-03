package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// BetaRecruitmentCriteriaAttributes describes beta recruitment criteria metadata.
type BetaRecruitmentCriteriaAttributes struct {
	LastModifiedDate             string                        `json:"lastModifiedDate,omitempty"`
	DeviceFamilyOsVersionFilters []DeviceFamilyOsVersionFilter `json:"deviceFamilyOsVersionFilters,omitempty"`
}

// BetaRecruitmentCriteriaCreateAttributes describes create attributes.
type BetaRecruitmentCriteriaCreateAttributes struct {
	DeviceFamilyOsVersionFilters []DeviceFamilyOsVersionFilter `json:"deviceFamilyOsVersionFilters"`
}

// BetaRecruitmentCriteriaUpdateAttributes describes update attributes.
type BetaRecruitmentCriteriaUpdateAttributes struct {
	DeviceFamilyOsVersionFilters []DeviceFamilyOsVersionFilter `json:"deviceFamilyOsVersionFilters,omitempty"`
}

// BetaRecruitmentCriteriaResponse is the response from beta recruitment criteria endpoints.
type BetaRecruitmentCriteriaResponse = SingleResponse[BetaRecruitmentCriteriaAttributes]

// BetaRecruitmentCriteriaDeleteResult represents CLI output for deletions.
type BetaRecruitmentCriteriaDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BetaRecruitmentCriterionCompatibleBuildCheckAttributes describes compatible build check attributes.
type BetaRecruitmentCriterionCompatibleBuildCheckAttributes struct {
	HasCompatibleBuild bool `json:"hasCompatibleBuild,omitempty"`
}

// BetaRecruitmentCriterionCompatibleBuildCheckResponse is the response for compatible build checks.
type BetaRecruitmentCriterionCompatibleBuildCheckResponse = SingleResponse[BetaRecruitmentCriterionCompatibleBuildCheckAttributes]

// BetaRecruitmentCriteriaRelationships describes relationships for recruitment criteria.
type BetaRecruitmentCriteriaRelationships struct {
	BetaGroup *Relationship `json:"betaGroup,omitempty"`
}

// BetaRecruitmentCriteriaCreateData is the data portion of a criteria create request.
type BetaRecruitmentCriteriaCreateData struct {
	Type          ResourceType                            `json:"type"`
	Attributes    BetaRecruitmentCriteriaCreateAttributes `json:"attributes"`
	Relationships *BetaRecruitmentCriteriaRelationships   `json:"relationships"`
}

// BetaRecruitmentCriteriaCreateRequest is a request to create beta recruitment criteria.
type BetaRecruitmentCriteriaCreateRequest struct {
	Data BetaRecruitmentCriteriaCreateData `json:"data"`
}

// BetaRecruitmentCriteriaUpdateData is the data portion of a criteria update request.
type BetaRecruitmentCriteriaUpdateData struct {
	Type       ResourceType                             `json:"type"`
	ID         string                                   `json:"id"`
	Attributes *BetaRecruitmentCriteriaUpdateAttributes `json:"attributes,omitempty"`
}

// BetaRecruitmentCriteriaUpdateRequest is a request to update beta recruitment criteria.
type BetaRecruitmentCriteriaUpdateRequest struct {
	Data BetaRecruitmentCriteriaUpdateData `json:"data"`
}

// BetaRecruitmentCriterionOptionAttributes describes recruitment criteria options.
type BetaRecruitmentCriterionOptionAttributes struct {
	Identifier             string                                       `json:"identifier,omitempty"`
	Name                   string                                       `json:"name,omitempty"`
	Category               string                                       `json:"category,omitempty"`
	DeviceFamilyOsVersions []BetaRecruitmentCriterionOptionDeviceFamily `json:"deviceFamilyOsVersions,omitempty"`
}

// BetaRecruitmentCriterionOptionDeviceFamily describes device families and OS versions for options.
type BetaRecruitmentCriterionOptionDeviceFamily struct {
	DeviceFamily DeviceFamily `json:"deviceFamily,omitempty"`
	OSVersions   []string     `json:"osVersions,omitempty"`
}

// BetaRecruitmentCriterionOptionsResponse is the response from recruitment criteria options list.
type BetaRecruitmentCriterionOptionsResponse = Response[BetaRecruitmentCriterionOptionAttributes]

// BetaGroupPublicLinkUsagesResponse is the response from public link usage metrics.
// The metric endpoint returns data-point objects rather than JSON:API resource
// objects, so it has a dedicated envelope instead of Response[T].
type BetaGroupPublicLinkUsagesResponse struct {
	Data  []BetaGroupPublicLinkUsage `json:"data"`
	Links Links                      `json:"links"`
	Meta  json.RawMessage            `json:"meta,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *BetaGroupPublicLinkUsagesResponse) GetLinks() *Links {
	if r == nil {
		return nil
	}
	return &r.Links
}

// GetMeta returns the raw metadata field for pagination warnings.
func (r *BetaGroupPublicLinkUsagesResponse) GetMeta() json.RawMessage {
	if r == nil {
		return nil
	}
	return r.Meta
}

// GetData returns the metric data for aggregation.
func (r *BetaGroupPublicLinkUsagesResponse) GetData() any {
	if r == nil {
		return nil
	}
	return r.Data
}

// BetaGroupPublicLinkUsage is one metric series returned by the public link
// usage endpoint. The raw item is retained so JSON output remains faithful to
// Apple's response while table output can use typed metric fields.
type BetaGroupPublicLinkUsage struct {
	DataPoints []BetaGroupPublicLinkUsageDataPoint `json:"dataPoints,omitempty"`
	raw        json.RawMessage
}

func (m *BetaGroupPublicLinkUsage) UnmarshalJSON(data []byte) error {
	type metricAlias BetaGroupPublicLinkUsage
	var decoded metricAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.raw = append(decoded.raw[:0], data...)
	*m = BetaGroupPublicLinkUsage(decoded)
	return nil
}

func (m BetaGroupPublicLinkUsage) MarshalJSON() ([]byte, error) {
	if len(m.raw) > 0 {
		return m.raw, nil
	}
	type metricAlias BetaGroupPublicLinkUsage
	return json.Marshal(metricAlias(m))
}

// BetaGroupPublicLinkUsageDataPoint is one time interval in a metric series.
type BetaGroupPublicLinkUsageDataPoint struct {
	Start  string                         `json:"start,omitempty"`
	End    string                         `json:"end,omitempty"`
	Values BetaGroupPublicLinkUsageValues `json:"values,omitempty"`
}

// BetaGroupPublicLinkUsageValues contains the counts and ratios Apple reports for a data point.
type BetaGroupPublicLinkUsageValues struct {
	ViewCount               *int     `json:"viewCount,omitempty"`
	AcceptedCount           *int     `json:"acceptedCount,omitempty"`
	DidNotAcceptCount       *int     `json:"didNotAcceptCount,omitempty"`
	DidNotMeetCriteriaCount *int     `json:"didNotMeetCriteriaCount,omitempty"`
	NotRelevantRatio        *float64 `json:"notRelevantRatio,omitempty"`
	NotClearRatio           *float64 `json:"notClearRatio,omitempty"`
	NotInterestingRatio     *float64 `json:"notInterestingRatio,omitempty"`
}

// BetaGroupTesterUsagesResponse is the response from beta tester usage metrics.
// The metric endpoint returns data-point objects rather than JSON:API resource
// objects, so it has a dedicated envelope instead of Response[T].
type BetaGroupTesterUsagesResponse struct {
	Data     []BetaGroupTesterUsage `json:"data"`
	Links    Links                  `json:"links"`
	Included json.RawMessage        `json:"included,omitempty"`
	Meta     json.RawMessage        `json:"meta,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *BetaGroupTesterUsagesResponse) GetLinks() *Links {
	if r == nil {
		return nil
	}
	return &r.Links
}

// GetMeta returns the raw metadata field for pagination warnings.
func (r *BetaGroupTesterUsagesResponse) GetMeta() json.RawMessage {
	if r == nil {
		return nil
	}
	return r.Meta
}

// GetData returns the metric data for aggregation.
func (r *BetaGroupTesterUsagesResponse) GetData() any {
	if r == nil {
		return nil
	}
	return r.Data
}

// BetaGroupTesterUsage is one metric series returned by the group tester
// usage endpoint. The raw item is retained so JSON output remains faithful to
// Apple's response while table output can use typed metric fields.
type BetaGroupTesterUsage struct {
	Type       string                          `json:"type,omitempty"`
	ID         string                          `json:"id,omitempty"`
	DataPoints []BetaGroupTesterUsageDataPoint `json:"dataPoints,omitempty"`
	Dimensions *BetaGroupTesterUsageDimensions `json:"dimensions,omitempty"`
	raw        json.RawMessage
}

func (m *BetaGroupTesterUsage) UnmarshalJSON(data []byte) error {
	type metricAlias BetaGroupTesterUsage
	var decoded metricAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.raw = append(decoded.raw[:0], data...)
	*m = BetaGroupTesterUsage(decoded)
	return nil
}

func (m BetaGroupTesterUsage) MarshalJSON() ([]byte, error) {
	if len(m.raw) > 0 {
		return m.raw, nil
	}
	type metricAlias BetaGroupTesterUsage
	return json.Marshal(metricAlias(m))
}

// BetaGroupTesterUsageDataPoint is one time interval in a metric series.
type BetaGroupTesterUsageDataPoint struct {
	Start  string                     `json:"start,omitempty"`
	End    string                     `json:"end,omitempty"`
	Values BetaGroupTesterUsageValues `json:"values,omitempty"`
}

// BetaGroupTesterUsageValues contains the counts Apple reports for a data point.
type BetaGroupTesterUsageValues struct {
	CrashCount    *int `json:"crashCount,omitempty"`
	SessionCount  *int `json:"sessionCount,omitempty"`
	FeedbackCount *int `json:"feedbackCount,omitempty"`
}

// BetaGroupTesterUsageDimensions identifies the tester represented by a metric series.
type BetaGroupTesterUsageDimensions struct {
	BetaTesters *BetaGroupTesterUsageDimension `json:"betaTesters,omitempty"`
}

// BetaGroupTesterUsageDimension describes a grouped metric dimension.
type BetaGroupTesterUsageDimension struct {
	Data  *BetaGroupTesterUsageDimensionData  `json:"data,omitempty"`
	Links *BetaGroupTesterUsageDimensionLinks `json:"links,omitempty"`
}

// BetaGroupTesterUsageDimensionLinks contains links for a metric dimension.
type BetaGroupTesterUsageDimensionLinks struct {
	GroupBy string `json:"groupBy,omitempty"`
	Related string `json:"related,omitempty"`
}

// MetricDimensionData accepts both the string shape in Apple's schema and
// the resource identifier object returned by current metric responses.
type MetricDimensionData struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
	raw  json.RawMessage
}

// BetaGroupTesterUsageDimensionData is the historical name for MetricDimensionData.
type BetaGroupTesterUsageDimensionData = MetricDimensionData

func (d *MetricDimensionData) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		*d = MetricDimensionData{}
		return nil
	}

	var identifier string
	if err := json.Unmarshal(trimmed, &identifier); err == nil {
		d.ID = identifier
		d.Type = ""
		d.raw = append(d.raw[:0], trimmed...)
		return nil
	}

	var resource struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(trimmed, &resource); err != nil {
		return fmt.Errorf("betaTesters.data must be a string or object: %w", err)
	}
	d.Type = resource.Type
	d.ID = resource.ID
	d.raw = append(d.raw[:0], trimmed...)
	return nil
}

func (d MetricDimensionData) MarshalJSON() ([]byte, error) {
	if len(d.raw) > 0 {
		return d.raw, nil
	}
	if d.Type != "" {
		return json.Marshal(struct {
			Type string `json:"type"`
			ID   string `json:"id,omitempty"`
		}{Type: d.Type, ID: d.ID})
	}
	return json.Marshal(d.ID)
}
