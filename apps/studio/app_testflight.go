package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// GetTestFlight fetches beta groups and tester counts concurrently.
func (a *App) GetTestFlight(appID string) (TestFlightResponse, error) {
	if strings.TrimSpace(appID) == "" {
		return TestFlightResponse{Error: "app ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return TestFlightResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	// 1. Fetch groups
	out, err := a.runASCCombinedOutput(ctx, ascPath, "testflight", "groups", "list", "--app", appID, "--paginate", "--output", "json")
	if err != nil {
		return TestFlightResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawGroup struct {
		ID         string `json:"id"`
		Attributes struct {
			Name            string `json:"name"`
			IsInternalGroup bool   `json:"isInternalGroup"`
			PublicLink      string `json:"publicLink"`
			FeedbackEnabled bool   `json:"feedbackEnabled"`
			CreatedDate     string `json:"createdDate"`
		} `json:"attributes"`
		Relationships struct {
			BetaTesters struct {
				Links struct {
					Related string `json:"related"`
				} `json:"links"`
			} `json:"betaTesters"`
		} `json:"relationships"`
	}
	var groupEnv struct {
		Data []rawGroup `json:"data"`
	}
	if json.Unmarshal(out, &groupEnv) != nil {
		return TestFlightResponse{Error: "failed to parse groups"}, nil
	}

	groups := make([]BetaGroup, len(groupEnv.Data))
	for i, g := range groupEnv.Data {
		groups[i] = BetaGroup{
			ID:              g.ID,
			Name:            g.Attributes.Name,
			IsInternal:      g.Attributes.IsInternalGroup,
			PublicLink:      g.Attributes.PublicLink,
			FeedbackEnabled: g.Attributes.FeedbackEnabled,
			CreatedDate:     g.Attributes.CreatedDate,
		}
	}

	runWithConcurrency(boundedStudioConcurrency(len(groupEnv.Data)), len(groupEnv.Data), func(i int) {
		out, err := a.runASCCombinedOutput(ctx, ascPath, "testflight", "testers", "list",
			"--group", groupEnv.Data[i].ID, "--limit", "1", "--output", "json")
		if err != nil {
			return
		}
		var env struct {
			Meta struct {
				Paging struct {
					Total int `json:"total"`
				} `json:"paging"`
			} `json:"meta"`
		}
		if json.Unmarshal(out, &env) == nil {
			groups[i].TesterCount = env.Meta.Paging.Total
		}
	})

	return TestFlightResponse{Groups: groups}, nil
}

// GetTestFlightTesters fetches ALL testers for a specific group (paginated).
func (a *App) GetTestFlightTesters(groupID string) (TestFlightResponse, error) {
	if strings.TrimSpace(groupID) == "" {
		return TestFlightResponse{Error: "group ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return TestFlightResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 120*time.Second)
	defer cancel()

	out, err := a.runASCCombinedOutput(ctx, ascPath, "testflight", "testers", "list",
		"--group", groupID, "--paginate", "--output", "json")
	if err != nil {
		return TestFlightResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawTester struct {
		Attributes struct {
			Email      string `json:"email"`
			FirstName  string `json:"firstName"`
			LastName   string `json:"lastName"`
			InviteType string `json:"inviteType"`
			State      string `json:"state"`
		} `json:"attributes"`
	}
	var env struct {
		Data []rawTester `json:"data"`
	}
	if json.Unmarshal(out, &env) != nil {
		return TestFlightResponse{Error: "failed to parse testers"}, nil
	}

	testers := make([]BetaTester, 0, len(env.Data))
	for _, t := range env.Data {
		testers = append(testers, BetaTester{
			Email:      t.Attributes.Email,
			FirstName:  t.Attributes.FirstName,
			LastName:   t.Attributes.LastName,
			InviteType: t.Attributes.InviteType,
			State:      t.Attributes.State,
		})
	}
	return TestFlightResponse{Testers: testers}, nil
}

// GetFeedback fetches TestFlight feedback list, then enriches each with detail view concurrently.
func (a *App) GetFeedback(appID string) (FeedbackResponse, error) {
	if strings.TrimSpace(appID) == "" {
		return FeedbackResponse{Error: "app ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return FeedbackResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 60*time.Second)
	defer cancel()

	// Fetch feedback list with screenshots
	out, err := a.runASCCombinedOutput(ctx, ascPath, "testflight", "feedback", "list",
		"--app", appID, "--include-screenshots", "--sort", "-createdDate", "--paginate", "--output", "json")
	if err != nil {
		return FeedbackResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawScreenshot struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	type rawFeedback struct {
		ID         string `json:"id"`
		Attributes struct {
			Comment      string          `json:"comment"`
			Email        string          `json:"email"`
			DeviceModel  string          `json:"deviceModel"`
			OSVersion    string          `json:"osVersion"`
			AppPlatform  string          `json:"appPlatform"`
			CreatedDate  string          `json:"createdDate"`
			DeviceFamily string          `json:"deviceFamily"`
			Screenshots  []rawScreenshot `json:"screenshots"`
		} `json:"attributes"`
	}
	var listEnv struct {
		Data []rawFeedback `json:"data"`
		Meta struct {
			Paging struct {
				Total int `json:"total"`
			} `json:"paging"`
		} `json:"meta"`
	}
	if json.Unmarshal(out, &listEnv) != nil {
		return FeedbackResponse{Error: "failed to parse feedback list"}, nil
	}

	items := make([]FeedbackItem, len(listEnv.Data))
	for i, fb := range listEnv.Data {
		var shots []FeedbackScreenshot
		for _, s := range fb.Attributes.Screenshots {
			shots = append(shots, FeedbackScreenshot{URL: s.URL, Width: s.Width, Height: s.Height})
		}
		items[i] = FeedbackItem{
			ID:           fb.ID,
			Comment:      fb.Attributes.Comment,
			Email:        fb.Attributes.Email,
			DeviceModel:  fb.Attributes.DeviceModel,
			DeviceFamily: fb.Attributes.DeviceFamily,
			OSVersion:    fb.Attributes.OSVersion,
			AppPlatform:  fb.Attributes.AppPlatform,
			CreatedDate:  fb.Attributes.CreatedDate,
			Screenshots:  shots,
		}
	}
	runWithConcurrency(boundedStudioConcurrency(len(listEnv.Data)), len(listEnv.Data), func(i int) {
		out, err := a.runASCCombinedOutput(ctx, ascPath, "testflight", "feedback", "view",
			"--submission-id", listEnv.Data[i].ID, "--output", "json")
		if err != nil {
			return
		}
		var env struct {
			Data struct {
				Attributes struct {
					Locale         string `json:"locale"`
					TimeZone       string `json:"timeZone"`
					ConnectionType string `json:"connectionType"`
					Battery        int    `json:"batteryPercentage"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			return
		}
		items[i].Locale = env.Data.Attributes.Locale
		items[i].TimeZone = env.Data.Attributes.TimeZone
		items[i].ConnectionType = env.Data.Attributes.ConnectionType
		items[i].Battery = env.Data.Attributes.Battery
	})

	return FeedbackResponse{Feedback: items, Total: listEnv.Meta.Paging.Total}, nil
}
