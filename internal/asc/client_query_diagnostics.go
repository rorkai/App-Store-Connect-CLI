package asc

import (
	"net/url"
	"strings"
)

type perfPowerMetricsQuery struct {
	platforms   []string
	metricTypes []string
	deviceTypes []string
}

type diagnosticSignaturesQuery struct {
	listQuery
	diagnosticTypes []string
	fields          []string
}

type diagnosticLogsQuery struct {
	listQuery
}

func buildPerfPowerMetricsQuery(query *perfPowerMetricsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[metricType]", query.metricTypes)
	addCSV(values, "filter[deviceType]", query.deviceTypes)
	return values.Encode()
}

func buildDiagnosticSignaturesQuery(query *diagnosticSignaturesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[diagnosticType]", query.diagnosticTypes)
	addCSV(values, "fields[diagnosticSignatures]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildDiagnosticLogsQuery(query *diagnosticLogsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// PerfPowerMetricsOption is a functional option for performance/power metrics.
type PerfPowerMetricsOption func(*perfPowerMetricsQuery)

// DiagnosticSignaturesOption is a functional option for diagnostic signatures.
type DiagnosticSignaturesOption func(*diagnosticSignaturesQuery)

// DiagnosticLogsOption is a functional option for diagnostic logs.
type DiagnosticLogsOption func(*diagnosticLogsQuery)

// WithPerfPowerMetricsPlatforms filters metrics by platform(s).
func WithPerfPowerMetricsPlatforms(platforms []string) PerfPowerMetricsOption {
	return func(q *perfPowerMetricsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithPerfPowerMetricsMetricTypes filters metrics by metric type(s).
func WithPerfPowerMetricsMetricTypes(types []string) PerfPowerMetricsOption {
	return func(q *perfPowerMetricsQuery) {
		q.metricTypes = normalizeUpperList(types)
	}
}

// WithPerfPowerMetricsDeviceTypes filters metrics by device type(s).
func WithPerfPowerMetricsDeviceTypes(types []string) PerfPowerMetricsOption {
	return func(q *perfPowerMetricsQuery) {
		q.deviceTypes = normalizeList(types)
	}
}

// WithDiagnosticSignaturesLimit sets the max number of diagnostic signatures to return.
func WithDiagnosticSignaturesLimit(limit int) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithDiagnosticSignaturesNextURL uses a next page URL directly.
func WithDiagnosticSignaturesNextURL(next string) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithDiagnosticSignaturesDiagnosticTypes filters diagnostic signatures by type.
func WithDiagnosticSignaturesDiagnosticTypes(types []string) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		q.diagnosticTypes = normalizeUpperList(types)
	}
}

// WithDiagnosticSignaturesFields sets fields[diagnosticSignatures] for diagnostic signatures.
func WithDiagnosticSignaturesFields(fields []string) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithDiagnosticLogsLimit sets the max number of diagnostic logs to return.
func WithDiagnosticLogsLimit(limit int) DiagnosticLogsOption {
	return func(q *diagnosticLogsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}
