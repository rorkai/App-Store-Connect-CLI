package asc

import "encoding/json"

// PerfPowerMetricType represents a performance/power metric category.
type PerfPowerMetricType string

const (
	PerfPowerMetricTypeDisk        PerfPowerMetricType = "DISK"
	PerfPowerMetricTypeHang        PerfPowerMetricType = "HANG"
	PerfPowerMetricTypeBattery     PerfPowerMetricType = "BATTERY"
	PerfPowerMetricTypeLaunch      PerfPowerMetricType = "LAUNCH"
	PerfPowerMetricTypeMemory      PerfPowerMetricType = "MEMORY"
	PerfPowerMetricTypeAnimation   PerfPowerMetricType = "ANIMATION"
	PerfPowerMetricTypeTermination PerfPowerMetricType = "TERMINATION"
	PerfPowerMetricTypeStorage     PerfPowerMetricType = "STORAGE"
)

// DiagnosticSignatureType represents a diagnostic signature category.
type DiagnosticSignatureType string

const (
	DiagnosticSignatureTypeDiskWrites DiagnosticSignatureType = "DISK_WRITES"
	DiagnosticSignatureTypeHangs      DiagnosticSignatureType = "HANGS"
	DiagnosticSignatureTypeLaunches   DiagnosticSignatureType = "LAUNCHES"
)

// DiagnosticInsightDirection describes diagnostic insight direction.
type DiagnosticInsightDirection string

const (
	DiagnosticInsightDirectionUp        DiagnosticInsightDirection = "UP"
	DiagnosticInsightDirectionDown      DiagnosticInsightDirection = "DOWN"
	DiagnosticInsightDirectionUndefined DiagnosticInsightDirection = "UNDEFINED"
)

// DiagnosticInsightType describes the insight category.
type DiagnosticInsightType string

const (
	DiagnosticInsightTypeTrend DiagnosticInsightType = "TREND"
)

// DiagnosticInsightReferenceVersion describes a reference version for insight.
type DiagnosticInsightReferenceVersion struct {
	Version string  `json:"version,omitempty"`
	Value   float64 `json:"value,omitempty"`
}

// DiagnosticInsight describes insight details for diagnostic signatures.
type DiagnosticInsight struct {
	InsightType       DiagnosticInsightType               `json:"insightType,omitempty"`
	Direction         DiagnosticInsightDirection          `json:"direction,omitempty"`
	ReferenceVersions []DiagnosticInsightReferenceVersion `json:"referenceVersions,omitempty"`
}

// DiagnosticSignatureAttributes describes diagnostic signature metadata.
type DiagnosticSignatureAttributes struct {
	DiagnosticType DiagnosticSignatureType `json:"diagnosticType,omitempty"`
	Signature      string                  `json:"signature,omitempty"`
	Weight         float64                 `json:"weight,omitempty"`
	Insight        *DiagnosticInsight      `json:"insight,omitempty"`
}

// DiagnosticSignaturesResponse is the response from diagnostic signatures endpoints.
type DiagnosticSignaturesResponse = Response[DiagnosticSignatureAttributes]

// PerfPowerMetricsResponse wraps raw Xcode metrics JSON.
type PerfPowerMetricsResponse struct {
	Data json.RawMessage `json:"-"`
}

// MarshalJSON preserves raw API JSON for metrics responses.
func (r PerfPowerMetricsResponse) MarshalJSON() ([]byte, error) {
	if len(r.Data) == 0 {
		return []byte("null"), nil
	}
	return r.Data, nil
}

// DiagnosticLogsResponse wraps raw diagnostic logs JSON.
type DiagnosticLogsResponse struct {
	Data json.RawMessage `json:"-"`
}

// MarshalJSON preserves raw API JSON for diagnostic logs responses.
func (r DiagnosticLogsResponse) MarshalJSON() ([]byte, error) {
	if len(r.Data) == 0 {
		return []byte("null"), nil
	}
	return r.Data, nil
}
