package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"
)

type keywordScoreSourcePayload struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

type keywordScoreSignalPayload struct {
	AppID                     string  `json:"appId"`
	Name                      string  `json:"name"`
	Subtitle                  string  `json:"subtitle"`
	PublisherName             string  `json:"publisherName"`
	UserRatingCount           int64   `json:"userRatingCount"`
	ReleaseDate               string  `json:"releaseDate,omitempty"`
	CurrentVersionReleaseDate string  `json:"currentVersionReleaseDate,omitempty"`
	NormalizedRatingCount     float64 `json:"normalizedRatingCount"`
	NormalizedRatingsPerDay   float64 `json:"normalizedRatingsPerDay"`
	KeywordMatch              string  `json:"keywordMatch"`
	AppScore                  float64 `json:"appScore"`
}

type keywordScoreRowPayload struct {
	Keyword            string                      `json:"keyword"`
	Status             string                      `json:"status"`
	DifficultyScore    *float64                    `json:"difficultyScore"`
	MinDifficultyScore *float64                    `json:"minDifficultyScore"`
	IsBrandKeyword     *bool                       `json:"isBrandKeyword"`
	AppCount           *int                        `json:"appCount"`
	KeywordMatch       string                      `json:"keywordMatch,omitempty"`
	Rank               *int                        `json:"rank,omitempty"`
	Fallback           bool                        `json:"fallback,omitempty"`
	AverageAppScore    *float64                    `json:"averageAppScore,omitempty"`
	MinimumAppScore    *float64                    `json:"minimumAppScore,omitempty"`
	NormalizedAppCount *float64                    `json:"normalizedAppCount,omitempty"`
	RawSignals         []keywordScoreSignalPayload `json:"rawSignals,omitempty"`
	Error              string                      `json:"error,omitempty"`
}

type keywordScorePayload struct {
	SchemaVersion string                      `json:"schemaVersion"`
	AppID         string                      `json:"appId,omitempty"`
	Country       string                      `json:"country"`
	Genre         string                      `json:"genre,omitempty"`
	Sources       []keywordScoreSourcePayload `json:"sources"`
	Summary       struct {
		Keywords    int `json:"keywords"`
		Scored      int `json:"scored"`
		Unavailable int `json:"unavailable"`
		WithRank    int `json:"withRank"`
	} `json:"summary"`
	Rows []keywordScoreRowPayload `json:"rows"`
}

func TestOptimizeKeywordsScoreUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing keywords",
			args:    []string{"optimize", "keywords", "score"},
			wantErr: "--keywords is required",
		},
		{
			name:    "keyword length",
			args:    []string{"optimize", "keywords", "score", "--keywords", "a"},
			wantErr: "must be between 2 and 60 characters",
		},
		{
			name:    "invalid genre",
			args:    []string{"optimize", "keywords", "score", "--keywords", "focus timer", "--genre", "bad genre"},
			wantErr: "--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES",
		},
		{
			name:    "positional argument",
			args:    []string{"optimize", "keywords", "score", "--keywords", "focus timer", "extra"},
			wantErr: "optimize keywords score does not accept positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_APP_ID", "")
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Errorf("unexpected request before input validation: %s", req.URL.String())
				return nil, errors.New("unexpected request")
			})

			stdout, stderr, runErr := runCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestOptimizeKeywordsScoreJSONShipsRawSignalsAndDegradesPopularity(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	searchBody := `{"resultCount":5,"results":[
		{"trackId":111,"trackName":"Focus Timer","sellerName":"Alpha Labs","averageUserRating":4.6,"userRatingCount":8000},
		{"trackId":222,"trackName":"Deep Work","sellerName":"Beta Labs","averageUserRating":4.2,"userRatingCount":4000},
		{"trackId":333,"trackName":"Timer Buddy","sellerName":"Gamma Labs","averageUserRating":3.9,"userRatingCount":900},
		{"trackId":444,"trackName":"Pomodoro","sellerName":"Delta Labs","averageUserRating":4.1,"userRatingCount":250},
		{"trackId":1234567890,"trackName":"My Focus App","sellerName":"Mine","averageUserRating":4.9,"userRatingCount":30}
	]}`
	lookupBody := `{"resultCount":1,"results":[
		{"trackId":111,"trackName":"Focus Timer","sellerName":"Alpha Labs","averageUserRating":4.6,"userRatingCount":8000,"releaseDate":"2020-01-15T08:00:00Z","currentVersionReleaseDate":"2026-08-01T08:00:00Z"}
	]}`

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "itunes.apple.com" {
			t.Errorf("host = %q, want the public iTunes host", req.URL.Host)
			return nil, errors.New("unexpected public API host")
		}
		switch req.URL.Path {
		case "/search":
			if got := req.URL.Query().Get("term"); got != "focus timer" {
				t.Errorf("term = %q, want the normalized keyword", got)
				return nil, errors.New("unexpected search term")
			}
			return jsonResponse(http.StatusOK, searchBody)
		case "/lookup":
			if got := req.URL.Query().Get("entity"); got != "software" {
				t.Errorf("entity = %q, want software", got)
				return nil, errors.New("unexpected lookup entity")
			}
			return jsonResponse(http.StatusOK, lookupBody)
		default:
			t.Errorf("unexpected path %q", req.URL.Path)
			return nil, errors.New("unexpected path")
		}
	})

	stdout, stderr, runErr := runCommand(t, []string{
		"optimize", "keywords", "score",
		"--keywords", " Focus Timer ",
		"--app", "1234567890",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload keywordScorePayload
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if payload.SchemaVersion != "1" || payload.AppID != "1234567890" || payload.Country != "US" {
		t.Fatalf("unexpected report identity: %+v", payload)
	}
	if len(payload.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(payload.Rows))
	}

	row := payload.Rows[0]
	if row.Keyword != "focus timer" || row.Status != "available" {
		t.Fatalf("row = %+v", row)
	}
	if row.DifficultyScore == nil || *row.DifficultyScore < 1 || *row.DifficultyScore > 100 {
		t.Fatalf("difficulty must be reported within 1-100: %+v", row.DifficultyScore)
	}
	if row.MinDifficultyScore == nil || row.IsBrandKeyword == nil || row.AppCount == nil || *row.AppCount != 5 {
		t.Fatalf("row is missing computed fields: %+v", row)
	}
	if row.Rank == nil || *row.Rank != 5 {
		t.Fatalf("rank = %+v, want 5", row.Rank)
	}
	if row.KeywordMatch != "titleExactPhrase" {
		t.Fatalf("keywordMatch = %q", row.KeywordMatch)
	}
	if len(row.RawSignals) != 5 {
		t.Fatalf("rawSignals = %d, want one entry per sampled competitor", len(row.RawSignals))
	}

	// The score must be reproducible from the raw signals that ship with it.
	leader := row.RawSignals[0]
	if leader.AppID != "111" || leader.PublisherName != "Alpha Labs" || leader.UserRatingCount != 8000 {
		t.Fatalf("leader raw signals = %+v", leader)
	}
	if leader.ReleaseDate == "" || leader.CurrentVersionReleaseDate == "" {
		t.Fatalf("hydrated competitor must carry both release dates: %+v", leader)
	}
	if leader.NormalizedRatingCount != 0.8 {
		t.Fatalf("normalizedRatingCount = %v, want 0.8 for 8000 ratings", leader.NormalizedRatingCount)
	}
	if leader.KeywordMatch != "titleExactPhrase" || leader.AppScore <= 0 {
		t.Fatalf("leader match signals = %+v", leader)
	}
	// The lookup batch only covered the leader, so the rest degrade visibly.
	if row.RawSignals[1].ReleaseDate != "" {
		t.Fatalf("unhydrated competitor must report an empty release date: %+v", row.RawSignals[1])
	}

	sources := map[string]keywordScoreSourcePayload{}
	for _, source := range payload.Sources {
		sources[source.Name] = source
	}
	popularity, ok := sources["search_term_popularity"]
	if !ok || popularity.Status != "unavailable" {
		t.Fatalf("popularity source = %+v, want unavailable", popularity)
	}
	if !strings.Contains(popularity.Error, "--genre") {
		t.Fatalf("popularity error = %q, want the missing flag named", popularity.Error)
	}
	if got := sources["public_search"]; got.Status != "available" {
		t.Fatalf("competition source = %+v", got)
	}
	if got := sources["app_rank"]; got.Status != "available" || got.Count != 1 {
		t.Fatalf("rank source = %+v", got)
	}
	if got := sources["competitor_metadata"]; got.Status != "available" || got.Count != 1 ||
		!strings.Contains(got.Error, "lookup returned incomplete required release metadata") ||
		!strings.Contains(got.Error, "4 of 5 requested app IDs") {
		t.Fatalf("metadata source = %+v, want partial coverage to be explicit", got)
	}

	var raw struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal raw rows: %v", err)
	}
	if string(raw.Rows[0]["popularity"]) != "null" {
		t.Fatalf("popularity must serialize as an explicit null: %s", raw.Rows[0]["popularity"])
	}
}

func TestOptimizeKeywordsScoreFallsBackOnThinResultWindows(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/lookup" {
			return jsonResponse(http.StatusOK, `{"resultCount":0,"results":[]}`)
		}
		return jsonResponse(http.StatusOK, `{"resultCount":2,"results":[
			{"trackId":111,"trackName":"Only One","sellerName":"Solo","averageUserRating":5,"userRatingCount":3},
			{"trackId":222,"trackName":"Only Two","sellerName":"Duo","averageUserRating":5,"userRatingCount":4}
		]}`)
	})

	stdout, _, runErr := runCommand(t, []string{
		"optimize", "keywords", "score", "--keywords", "very rare keyword", "--output", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}

	var payload keywordScorePayload
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	row := payload.Rows[0]
	if !row.Fallback {
		t.Fatalf("row = %+v, want fallback for a thin result window", row)
	}
	if row.DifficultyScore == nil || *row.DifficultyScore != 1 || row.MinDifficultyScore == nil || *row.MinDifficultyScore != 1 {
		t.Fatalf("fallback difficulty = %+v / %+v, want 1 and 1", row.DifficultyScore, row.MinDifficultyScore)
	}
	if row.AverageAppScore == nil || *row.AverageAppScore <= 0 || row.MinimumAppScore == nil || *row.MinimumAppScore <= 0 {
		t.Fatalf("fallback aggregates must preserve observed app scores: %+v", row)
	}
	if row.NormalizedAppCount == nil || *row.NormalizedAppCount != 0 {
		t.Fatalf("fallback normalized app count = %+v, want 0", row.NormalizedAppCount)
	}
}

func TestOptimizeKeywordsScoreDetectsBrandKeywords(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/lookup" {
			return jsonResponse(http.StatusOK, `{"resultCount":0,"results":[]}`)
		}
		switch req.URL.Query().Get("term") {
		case "acme notes":
			return jsonResponse(http.StatusOK, `{"resultCount":5,"results":[
				{"trackId":111,"trackName":"Acme Notes","sellerName":"Acme Notes Inc","averageUserRating":4.8,"userRatingCount":50000},
				{"trackId":222,"trackName":"Notes Pro","sellerName":"Beta Labs","averageUserRating":4.2,"userRatingCount":4000},
				{"trackId":333,"trackName":"Jot","sellerName":"Gamma Labs","averageUserRating":3.9,"userRatingCount":900},
				{"trackId":444,"trackName":"Memo","sellerName":"Delta Labs","averageUserRating":4.1,"userRatingCount":250},
				{"trackId":555,"trackName":"Scratch","sellerName":"Zeta Labs","averageUserRating":4.0,"userRatingCount":100}
			]}`)
		default:
			return jsonResponse(http.StatusOK, `{"resultCount":5,"results":[
				{"trackId":611,"trackName":"Note Taker","sellerName":"Alpha Labs","averageUserRating":4.8,"userRatingCount":50000},
				{"trackId":622,"trackName":"Notes Pro","sellerName":"Beta Labs","averageUserRating":4.2,"userRatingCount":4000},
				{"trackId":633,"trackName":"Jot","sellerName":"Gamma Labs","averageUserRating":3.9,"userRatingCount":900},
				{"trackId":644,"trackName":"Memo","sellerName":"Delta Labs","averageUserRating":4.1,"userRatingCount":250},
				{"trackId":655,"trackName":"Scratch","sellerName":"Zeta Labs","averageUserRating":4.0,"userRatingCount":100}
			]}`)
		}
	})

	stdout, _, runErr := runCommand(t, []string{
		"optimize", "keywords", "score", "--keywords", "acme notes,note taking", "--output", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}

	var payload keywordScorePayload
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if len(payload.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(payload.Rows))
	}
	if payload.Rows[0].IsBrandKeyword == nil || !*payload.Rows[0].IsBrandKeyword {
		t.Fatalf("acme notes must be reported as a brand keyword: %+v", payload.Rows[0].IsBrandKeyword)
	}
	if payload.Rows[1].IsBrandKeyword == nil || *payload.Rows[1].IsBrandKeyword {
		t.Fatalf("note taking must not be reported as a brand keyword: %+v", payload.Rows[1].IsBrandKeyword)
	}
}
