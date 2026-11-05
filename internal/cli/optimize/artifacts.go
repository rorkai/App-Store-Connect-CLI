package optimize

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	searchPlanReportArtifact           = "report.json"
	searchPlanMetadataArtifact         = "metadata-candidates.csv"
	searchPlanExactKeywordsArtifact    = "exact-keywords.json"
	searchPlanNegativeKeywordsArtifact = "negative-keywords.json"
)

func writeSearchPlanArtifacts(directory string, report SearchPlanReport) ([]string, error) {
	root, err := rootfs.New(strings.TrimSpace(directory))
	if err != nil {
		return nil, fmt.Errorf("prepare --out-dir: %w", err)
	}
	artifactNames := []string{
		searchPlanReportArtifact,
		searchPlanMetadataArtifact,
		searchPlanExactKeywordsArtifact,
		searchPlanNegativeKeywordsArtifact,
	}
	report.Artifacts = make([]string, 0, len(artifactNames))
	for _, name := range artifactNames {
		report.Artifacts = append(report.Artifacts, filepath.Join(strings.TrimSpace(directory), name))
	}

	reportData, err := canonicalSearchPlanJSON(report)
	if err != nil {
		return nil, err
	}
	metadataData, err := buildMetadataCandidatesCSV(report)
	if err != nil {
		return nil, err
	}
	exactData, err := buildExactKeywordArtifact(report.Rows)
	if err != nil {
		return nil, err
	}
	negativeData, err := buildNegativeKeywordArtifact(report.Rows)
	if err != nil {
		return nil, err
	}

	files := []struct {
		name string
		data []byte
	}{
		{name: searchPlanReportArtifact, data: reportData},
		{name: searchPlanMetadataArtifact, data: metadataData},
		{name: searchPlanExactKeywordsArtifact, data: exactData},
		{name: searchPlanNegativeKeywordsArtifact, data: negativeData},
	}
	for _, file := range files {
		if err := root.WriteFile(file.name, file.data, 0o644); err != nil {
			return nil, fmt.Errorf("write optimization artifact %q: %w", filepath.Join(directory, file.name), err)
		}
	}
	return report.Artifacts, nil
}

func canonicalSearchPlanJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildMetadataCandidatesCSV(report SearchPlanReport) ([]byte, error) {
	keywords := splitMetadataKeywords(report.Metadata.Keywords)
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		seen[normalizeSearchTerm(keyword)] = struct{}{}
	}
	for _, row := range report.Rows {
		if !slices.Contains(row.Actions, "metadata_candidate") {
			continue
		}
		term := strings.TrimSpace(row.Term)
		key := normalizeSearchTerm(term)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		candidate := strings.Join(append(append([]string(nil), keywords...), term), ",")
		if utf8.RuneCountInString(candidate) > 100 {
			continue
		}
		keywords = append(keywords, term)
		seen[key] = struct{}{}
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"locale", "keywords"}); err != nil {
		return nil, err
	}
	if err := writer.Write([]string{report.Locale, strings.Join(keywords, ",")}); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func splitMetadataKeywords(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		key := normalizeSearchTerm(trimmed)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

type bulkKeywordArtifact struct {
	Items []bulkKeywordArtifactItem `json:"items"`
}

type bulkKeywordArtifactItem struct {
	CorrelationID int                     `json:"correlationId"`
	Data          bulkKeywordArtifactData `json:"data"`
}

type bulkKeywordArtifactData struct {
	CampaignID int64  `json:"campaignId,omitempty"`
	AdGroupID  int64  `json:"adGroupId,omitempty"`
	Text       string `json:"text"`
	MatchType  string `json:"matchType"`
	Status     string `json:"status,omitempty"`
}

func buildExactKeywordArtifact(rows []SearchPlanRow) ([]byte, error) {
	payload := bulkKeywordArtifact{Items: []bulkKeywordArtifactItem{}}
	for _, row := range rows {
		if !slices.Contains(row.Actions, "promote_exact") || row.AdGroupID == nil {
			continue
		}
		payload.Items = append(payload.Items, bulkKeywordArtifactItem{
			CorrelationID: len(payload.Items),
			Data: bulkKeywordArtifactData{
				AdGroupID: *row.AdGroupID,
				Text:      row.Term,
				MatchType: "EXACT",
			},
		})
	}
	return canonicalSearchPlanJSON(payload)
}

func buildNegativeKeywordArtifact(rows []SearchPlanRow) ([]byte, error) {
	payload := bulkKeywordArtifact{Items: []bulkKeywordArtifactItem{}}
	for _, row := range rows {
		if !slices.Contains(row.Actions, "negative_candidate") || row.CampaignID == nil {
			continue
		}
		item := bulkKeywordArtifactItem{
			CorrelationID: len(payload.Items),
			Data: bulkKeywordArtifactData{
				CampaignID: *row.CampaignID,
				Text:       row.Term,
				MatchType:  "EXACT",
				Status:     "ENABLED",
			},
		}
		if row.AdGroupID != nil {
			item.Data.AdGroupID = *row.AdGroupID
		}
		payload.Items = append(payload.Items, item)
	}
	return canonicalSearchPlanJSON(payload)
}
