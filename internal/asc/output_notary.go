package asc

import "strconv"

// NotarizationStapleResult is the computed output for a successful local
// ticket stapling and validation operation.
type NotarizationStapleResult struct {
	FilePath  string `json:"filePath"`
	Operation string `json:"operation"`
	Stapled   bool   `json:"stapled"`
	Validated bool   `json:"validated"`
}

// NotarizationValidateResult is the computed output for a successful local
// ticket validation operation.
type NotarizationValidateResult struct {
	FilePath  string `json:"filePath"`
	Operation string `json:"operation"`
	Validated bool   `json:"validated"`
}

func notarizationStapleResultRows(result *NotarizationStapleResult) ([]string, [][]string) {
	return []string{"Operation", "File Path", "Stapled", "Validated"}, [][]string{{
		result.Operation,
		result.FilePath,
		strconv.FormatBool(result.Stapled),
		strconv.FormatBool(result.Validated),
	}}
}

func notarizationValidateResultRows(result *NotarizationValidateResult) ([]string, [][]string) {
	return []string{"Operation", "File Path", "Validated"}, [][]string{{
		result.Operation,
		result.FilePath,
		strconv.FormatBool(result.Validated),
	}}
}

func notarySubmissionStatusRows(resp *NotarySubmissionStatusResponse) ([]string, [][]string) {
	headers := []string{"ID", "Status", "Name", "Created"}
	rows := [][]string{{
		resp.Data.ID,
		string(resp.Data.Attributes.Status),
		compactWhitespace(resp.Data.Attributes.Name),
		resp.Data.Attributes.CreatedDate,
	}}
	return headers, rows
}

func notarySubmissionsListRows(resp *NotarySubmissionsListResponse) ([]string, [][]string) {
	headers := []string{"ID", "Status", "Name", "Created"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.Status),
			compactWhitespace(item.Attributes.Name),
			item.Attributes.CreatedDate,
		})
	}
	return headers, rows
}

// NotarizationLogDownloadResult is the receipt for a developer log saved to a
// file. It omits the log URL, which is presigned.
type NotarizationLogDownloadResult struct {
	ID           string `json:"id"`
	OutputPath   string `json:"outputPath"`
	BytesWritten int64  `json:"bytesWritten"`
}

func notarizationLogDownloadResultRows(result *NotarizationLogDownloadResult) ([]string, [][]string) {
	headers := []string{"ID", "Output Path", "Bytes Written"}
	rows := [][]string{{result.ID, result.OutputPath, strconv.FormatInt(result.BytesWritten, 10)}}
	return headers, rows
}

func notarySubmissionLogsRows(resp *NotarySubmissionLogsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Developer Log URL"}
	rows := [][]string{{resp.Data.ID, resp.Data.Attributes.DeveloperLogURL}}
	return headers, rows
}
