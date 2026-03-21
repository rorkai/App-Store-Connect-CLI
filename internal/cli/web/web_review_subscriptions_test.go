package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewSubscriptionsListCommandOutputsJSON(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet {
						t.Fatalf("unexpected method: %s", req.Method)
					}
					if req.URL.Path != "/iris/v1/apps/app-1/subscriptionGroups" {
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					body := `{
						"data": [{
							"id": "group-1",
							"type": "subscriptionGroups",
							"attributes": {"referenceName": "Premium"},
							"relationships": {
								"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
							}
						}],
						"included": [{
							"id": "sub-1",
							"type": "subscriptions",
							"attributes": {
								"productId": "com.example.monthly",
								"name": "Monthly",
								"state": "READY_TO_SUBMIT",
								"isAppStoreReviewInProgress": false,
								"submitWithNextAppStoreVersion": true
							}
						}]
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionsListOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.AppID != "app-1" {
		t.Fatalf("expected app-1, got %#v", payload)
	}
	if payload.AttachedCount != 1 || len(payload.Subscriptions) != 1 {
		t.Fatalf("unexpected list output: %#v", payload)
	}
	if payload.Subscriptions[0].ID != "sub-1" || !payload.Subscriptions[0].SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected subscription output: %#v", payload.Subscriptions[0])
	}
}

func TestWebReviewSubscriptionsListCommandTableIncludesGroupID(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet {
						t.Fatalf("unexpected method: %s", req.Method)
					}
					if req.URL.Path != "/iris/v1/apps/app-1/subscriptionGroups" {
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					body := `{
						"data": [{
							"id": "group-1",
							"type": "subscriptionGroups",
							"attributes": {"referenceName": "Premium"},
							"relationships": {
								"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
							}
						}],
						"included": [{
							"id": "sub-1",
							"type": "subscriptions",
							"attributes": {
								"productId": "com.example.monthly",
								"name": "Monthly",
								"state": "READY_TO_SUBMIT",
								"isAppStoreReviewInProgress": false,
								"submitWithNextAppStoreVersion": true
							}
						}]
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Group ID") {
		t.Fatalf("expected table header to include Group ID, got %q", stdout)
	}
	if !strings.Contains(stdout, "group-1") {
		t.Fatalf("expected table output to include group ID, got %q", stdout)
	}
	if !strings.Contains(stdout, "Premium") {
		t.Fatalf("expected table output to include group reference name, got %q", stdout)
	}
}

func TestReviewSubscriptionAttachSkipReasonReadyToSubmitDoesNotClaimAlreadyAttached(t *testing.T) {
	reason := reviewSubscriptionAttachSkipReason(webcore.ReviewSubscription{State: "READY_TO_SUBMIT"})
	if strings.Contains(reason, "already attached") {
		t.Fatalf("expected READY_TO_SUBMIT skip reason to avoid already-attached wording, got %q", reason)
	}
	if !strings.Contains(reason, "READY_TO_SUBMIT") {
		t.Fatalf("expected READY_TO_SUBMIT skip reason to mention current state, got %q", reason)
	}
}

func TestWebReviewSubscriptionsAttachCommandRefreshesState(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						attached := "false"
						if listCalls > 1 {
							attached = "true"
						}
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
								}
							}],
							"included": [{
								"id": "sub-1",
								"type": "subscriptions",
								"attributes": {
									"productId": "com.example.monthly",
									"name": "Monthly",
									"state": "READY_TO_SUBMIT",
									"isAppStoreReviewInProgress": false,
									"submitWithNextAppStoreVersion": ` + attached + `
								}
							}]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						body := `{
							"data": {
								"id": "submission-1",
								"type": "subscriptionSubmissions",
								"attributes": {"submitWithNextAppStoreVersion": true},
								"relationships": {
									"subscription": {"data": {"type": "subscriptions", "id": "sub-1"}}
								}
							}
						}`
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Operation != "attach" || !payload.Changed || payload.SubmissionID != "submission-1" {
		t.Fatalf("unexpected attach output: %#v", payload)
	}
	if !payload.Subscription.SubmitWithNextAppStoreVersion {
		t.Fatalf("expected refreshed attached subscription, got %#v", payload.Subscription)
	}
	if listCalls != 2 {
		t.Fatalf("expected list before and after attach, got %d calls", listCalls)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Attaching subscription to next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsAttachCommandOnlyMarksChangedWhenRefreshShowsAttached(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
								}
							}],
							"included": [{
								"id": "sub-1",
								"type": "subscriptions",
								"attributes": {
									"productId": "com.example.monthly",
									"name": "Monthly",
									"state": "READY_TO_SUBMIT",
									"isAppStoreReviewInProgress": false,
									"submitWithNextAppStoreVersion": false
								}
							}]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						body := `{
							"data": {
								"id": "submission-1",
								"type": "subscriptionSubmissions",
								"attributes": {"submitWithNextAppStoreVersion": true},
								"relationships": {
									"subscription": {"data": {"type": "subscriptions", "id": "sub-1"}}
								}
							}
						}`
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Changed {
		t.Fatalf("expected attach to stay unchanged until refresh shows attached, got %#v", payload)
	}
	if payload.Subscription.SubmitWithNextAppStoreVersion {
		t.Fatalf("expected refreshed subscription to remain unattached, got %#v", payload.Subscription)
	}
	if listCalls != 2 {
		t.Fatalf("expected list before and after attach, got %d calls", listCalls)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Attaching subscription to next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsRemoveCommandRefreshesState(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						attached := "true"
						if listCalls > 1 {
							attached = "false"
						}
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
								}
							}],
							"included": [{
								"id": "sub-1",
								"type": "subscriptions",
								"attributes": {
									"productId": "com.example.monthly",
									"name": "Monthly",
									"state": "READY_TO_SUBMIT",
									"isAppStoreReviewInProgress": false,
									"submitWithNextAppStoreVersion": ` + attached + `
								}
							}]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodDelete && req.URL.Path == "/iris/v1/subscriptionSubmissions/sub-1":
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader("")),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsRemoveCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Operation != "remove" || !payload.Changed {
		t.Fatalf("unexpected remove output: %#v", payload)
	}
	if payload.Subscription.SubmitWithNextAppStoreVersion {
		t.Fatalf("expected refreshed detached subscription, got %#v", payload.Subscription)
	}
	if listCalls != 2 {
		t.Fatalf("expected list before and after remove, got %d calls", listCalls)
	}
}

func TestWebReviewSubscriptionsAttachRequiresConfirm(t *testing.T) {
	cmd := WebReviewSubscriptionsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected missing confirm error")
		}
	})
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirm guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewSubscriptionsAttachFailsFastForMissingMetadata(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/app-1/subscriptionGroups" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					body := `{
						"data": [{
							"id": "group-1",
							"type": "subscriptionGroups",
							"attributes": {"referenceName": "Premium"},
							"relationships": {
								"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
							}
						}],
						"included": [{
							"id": "sub-1",
							"type": "subscriptions",
							"attributes": {
								"productId": "com.example.monthly",
								"name": "Monthly",
								"state": "MISSING_METADATA",
								"isAppStoreReviewInProgress": false,
								"submitWithNextAppStoreVersion": false
							}
						}]
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected missing-metadata preflight error")
		}
		var reported shared.ReportedError
		if !errors.As(err, &reported) {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	if !strings.Contains(stderr, "is MISSING_METADATA") {
		t.Fatalf("expected missing metadata preflight explanation, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc validate subscriptions --app "app-1"`) {
		t.Fatalf("expected validate subscriptions hint, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc subscriptions images create --subscription-id "sub-1" --file "./image.png"`) {
		t.Fatalf("expected promotional image hint, got %q", stderr)
	}
	wantLabels := []string{"Loading review subscriptions"}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsAttachFailsFastForNonReadyState(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	postCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
								}
							}],
							"included": [{
								"id": "sub-1",
								"type": "subscriptions",
								"attributes": {
									"productId": "com.example.monthly",
									"name": "Monthly",
									"state": "DEVELOPER_ACTION_NEEDED",
									"isAppStoreReviewInProgress": false,
									"submitWithNextAppStoreVersion": false
								}
							}]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						postCalls++
						body := `{
							"data": {
								"id": "submission-1",
								"type": "subscriptionSubmissions",
								"attributes": {"submitWithNextAppStoreVersion": true},
								"relationships": {
									"subscription": {"data": {"type": "subscriptions", "id": "sub-1"}}
								}
							}
						}`
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected non-ready preflight error")
		}
		var reported shared.ReportedError
		if !errors.As(err, &reported) {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	if postCalls != 0 {
		t.Fatalf("expected attach preflight to block POST, got %d calls", postCalls)
	}
	if !strings.Contains(stderr, "DEVELOPER_ACTION_NEEDED") {
		t.Fatalf("expected non-ready state explanation, got %q", stderr)
	}
	if !strings.Contains(stderr, "READY_TO_SUBMIT") {
		t.Fatalf("expected READY_TO_SUBMIT guidance, got %q", stderr)
	}
	wantLabels := []string{"Loading review subscriptions"}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsAttachGroupFailsFastWhenNoSubscriptionsAreReadyToSubmit(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	postCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [
										{"type": "subscriptions", "id": "sub-1"},
										{"type": "subscriptions", "id": "sub-2"}
									]}
								}
							}],
							"included": [
								{"id": "sub-1", "type": "subscriptions", "attributes": {"productId": "com.example.monthly", "name": "Monthly", "state": "DEVELOPER_ACTION_NEEDED", "submitWithNextAppStoreVersion": false}},
								{"id": "sub-2", "type": "subscriptions", "attributes": {"productId": "com.example.annual", "name": "Annual", "state": "APPROVED", "submitWithNextAppStoreVersion": false}}
							]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						postCalls++
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"submission-1","type":"subscriptionSubmissions"}}`)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachGroupCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--group-id", "group-1",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected no-ready attach-group preflight error")
		}
		var reported shared.ReportedError
		if !errors.As(err, &reported) {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	if postCalls != 0 {
		t.Fatalf("expected attach-group preflight to block POST, got %d calls", postCalls)
	}
	if !strings.Contains(stderr, "no READY_TO_SUBMIT subscriptions") {
		t.Fatalf("expected no-ready preflight explanation, got %q", stderr)
	}
	wantLabels := []string{"Loading review subscriptions"}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsAttachGroupCommandRefreshesReadySubscriptions(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	postIDs := []string{}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						sub1Attached := "false"
						sub2Attached := "false"
						if listCalls > 1 {
							sub1Attached = "true"
							sub2Attached = "true"
						}
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [
										{"type": "subscriptions", "id": "sub-1"},
										{"type": "subscriptions", "id": "sub-2"},
										{"type": "subscriptions", "id": "sub-3"}
									]}
								}
							}],
							"included": [
								{"id": "sub-1", "type": "subscriptions", "attributes": {"productId": "com.example.monthly", "name": "Monthly", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub1Attached + `}},
								{"id": "sub-2", "type": "subscriptions", "attributes": {"productId": "com.example.annual", "name": "Annual", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub2Attached + `}},
								{"id": "sub-3", "type": "subscriptions", "attributes": {"productId": "com.example.legacy", "name": "Legacy", "state": "MISSING_METADATA", "submitWithNextAppStoreVersion": false}}
							]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						var payload struct {
							Data struct {
								Relationships map[string]struct {
									Data struct {
										ID string `json:"id"`
									} `json:"data"`
								} `json:"relationships"`
							} `json:"data"`
						}
						if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
							t.Fatalf("decode body: %v", err)
						}
						postIDs = append(postIDs, payload.Data.Relationships["subscription"].Data.ID)
						body := `{"data":{"id":"submission-` + postIDs[len(postIDs)-1] + `","type":"subscriptionSubmissions","attributes":{"submitWithNextAppStoreVersion":true}}}`
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachGroupCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--group-id", "group-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionGroupMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Operation != "attach-group" || payload.ChangedCount != 2 || payload.SkippedCount != 1 {
		t.Fatalf("unexpected attach-group output: %#v", payload)
	}
	if len(postIDs) != 2 || postIDs[0] != "sub-1" || postIDs[1] != "sub-2" {
		t.Fatalf("expected attach requests for sub-1 and sub-2, got %v", postIDs)
	}
	if len(payload.Changed) != 2 {
		t.Fatalf("expected two changed subscriptions, got %#v", payload.Changed)
	}
	if len(payload.Skipped) != 1 || !strings.Contains(payload.Skipped[0].Reason, "MISSING_METADATA") {
		t.Fatalf("expected missing metadata skip, got %#v", payload.Skipped)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Attaching subscription group to next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsAttachGroupCommandOnlyCountsRefreshedAttachedSubscriptions(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	postIDs := []string{}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						sub1Attached := "false"
						sub2Attached := "false"
						if listCalls > 1 {
							sub1Attached = "true"
						}
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [
										{"type": "subscriptions", "id": "sub-1"},
										{"type": "subscriptions", "id": "sub-2"},
										{"type": "subscriptions", "id": "sub-3"}
									]}
								}
							}],
							"included": [
								{"id": "sub-1", "type": "subscriptions", "attributes": {"productId": "com.example.monthly", "name": "Monthly", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub1Attached + `}},
								{"id": "sub-2", "type": "subscriptions", "attributes": {"productId": "com.example.annual", "name": "Annual", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub2Attached + `}},
								{"id": "sub-3", "type": "subscriptions", "attributes": {"productId": "com.example.legacy", "name": "Legacy", "state": "MISSING_METADATA", "submitWithNextAppStoreVersion": false}}
							]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						var payload struct {
							Data struct {
								Relationships map[string]struct {
									Data struct {
										ID string `json:"id"`
									} `json:"data"`
								} `json:"relationships"`
							} `json:"data"`
						}
						if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
							t.Fatalf("decode body: %v", err)
						}
						postIDs = append(postIDs, payload.Data.Relationships["subscription"].Data.ID)
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"submission-1","type":"subscriptionSubmissions","attributes":{"submitWithNextAppStoreVersion":true}}}`)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachGroupCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--group-id", "group-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionGroupMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.ChangedCount != 1 {
		t.Fatalf("expected only one changed subscription after refresh gating, got %#v", payload)
	}
	if len(payload.Changed) != 1 || payload.Changed[0].ID != "sub-1" || !payload.Changed[0].SubmitWithNextAppStoreVersion {
		t.Fatalf("expected only sub-1 in changed output, got %#v", payload.Changed)
	}
	if payload.SkippedCount != 2 {
		t.Fatalf("expected one original skip plus one unchanged-after-refresh skip, got %#v", payload)
	}
	if len(postIDs) != 2 || postIDs[0] != "sub-1" || postIDs[1] != "sub-2" {
		t.Fatalf("expected attach requests for sub-1 and sub-2, got %v", postIDs)
	}
	reasons := []string{}
	for _, skipped := range payload.Skipped {
		reasons = append(reasons, skipped.Reason)
	}
	if !strings.Contains(strings.Join(reasons, " | "), "MISSING_METADATA") {
		t.Fatalf("expected missing metadata skip, got %#v", payload.Skipped)
	}
	if !strings.Contains(strings.Join(reasons, " | "), "still shows not attached") {
		t.Fatalf("expected unchanged-after-refresh skip, got %#v", payload.Skipped)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Attaching subscription group to next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsAttachGroupCommandIsIdempotentAfterReadySubscriptionsAlreadyAttached(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	postCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [
										{"type": "subscriptions", "id": "sub-1"},
										{"type": "subscriptions", "id": "sub-2"},
										{"type": "subscriptions", "id": "sub-3"}
									]}
								}
							}],
							"included": [
								{"id": "sub-1", "type": "subscriptions", "attributes": {"productId": "com.example.monthly", "name": "Monthly", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": true}},
								{"id": "sub-2", "type": "subscriptions", "attributes": {"productId": "com.example.annual", "name": "Annual", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": true}},
								{"id": "sub-3", "type": "subscriptions", "attributes": {"productId": "com.example.legacy", "name": "Legacy", "state": "MISSING_METADATA", "submitWithNextAppStoreVersion": false}}
							]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
						postCalls++
						body := `{"data":{"id":"submission-1","type":"subscriptionSubmissions","attributes":{"submitWithNextAppStoreVersion":true}}}`
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsAttachGroupCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--group-id", "group-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionGroupMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Operation != "attach-group" || payload.ChangedCount != 0 || payload.SkippedCount != 3 {
		t.Fatalf("unexpected idempotent attach-group output: %#v", payload)
	}
	if postCalls != 0 {
		t.Fatalf("expected no POSTs on idempotent rerun, got %d", postCalls)
	}
	if listCalls != 2 {
		t.Fatalf("expected list before and after idempotent attach-group, got %d calls", listCalls)
	}
	if len(payload.Skipped) != 3 {
		t.Fatalf("expected three skipped subscriptions, got %#v", payload.Skipped)
	}
	if payload.Skipped[0].Reason != "already attached" || payload.Skipped[1].Reason != "already attached" {
		t.Fatalf("expected already-attached skips, got %#v", payload.Skipped)
	}
	if !strings.Contains(payload.Skipped[2].Reason, "MISSING_METADATA") {
		t.Fatalf("expected missing metadata skip, got %#v", payload.Skipped)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsRemoveGroupCommandRefreshesAttachedSubscriptions(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	deletePaths := []string{}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						sub1Attached := "true"
						sub2Attached := "true"
						if listCalls > 1 {
							sub1Attached = "false"
							sub2Attached = "false"
						}
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [
										{"type": "subscriptions", "id": "sub-1"},
										{"type": "subscriptions", "id": "sub-2"},
										{"type": "subscriptions", "id": "sub-3"}
									]}
								}
							}],
							"included": [
								{"id": "sub-1", "type": "subscriptions", "attributes": {"productId": "com.example.monthly", "name": "Monthly", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub1Attached + `}},
								{"id": "sub-2", "type": "subscriptions", "attributes": {"productId": "com.example.annual", "name": "Annual", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub2Attached + `}},
								{"id": "sub-3", "type": "subscriptions", "attributes": {"productId": "com.example.legacy", "name": "Legacy", "state": "MISSING_METADATA", "submitWithNextAppStoreVersion": false}}
							]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/iris/v1/subscriptionSubmissions/"):
						deletePaths = append(deletePaths, req.URL.Path)
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader("")),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsRemoveGroupCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--group-id", "group-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionGroupMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Operation != "remove-group" || payload.ChangedCount != 2 || payload.SkippedCount != 1 {
		t.Fatalf("unexpected remove-group output: %#v", payload)
	}
	if len(deletePaths) != 2 ||
		deletePaths[0] != "/iris/v1/subscriptionSubmissions/sub-1" ||
		deletePaths[1] != "/iris/v1/subscriptionSubmissions/sub-2" {
		t.Fatalf("expected delete requests for sub-1 and sub-2, got %v", deletePaths)
	}
	if len(payload.Skipped) != 1 || !strings.Contains(payload.Skipped[0].Reason, "not attached") {
		t.Fatalf("expected not attached skip, got %#v", payload.Skipped)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Removing subscription group from next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsRemoveCommandOnlyMarksChangedWhenRefreshShowsDetached(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
								}
							}],
							"included": [{
								"id": "sub-1",
								"type": "subscriptions",
								"attributes": {
									"productId": "com.example.monthly",
									"name": "Monthly",
									"state": "READY_TO_SUBMIT",
									"isAppStoreReviewInProgress": false,
									"submitWithNextAppStoreVersion": true
								}
							}]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodDelete && req.URL.Path == "/iris/v1/subscriptionSubmissions/sub-1":
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader("")),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsRemoveCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--subscription-id", "sub-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.Changed {
		t.Fatalf("expected remove to stay unchanged until refresh shows detached, got %#v", payload)
	}
	if !payload.Subscription.SubmitWithNextAppStoreVersion {
		t.Fatalf("expected refreshed subscription to remain attached, got %#v", payload.Subscription)
	}
	if listCalls != 2 {
		t.Fatalf("expected list before and after remove, got %d calls", listCalls)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Removing subscription from next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestWebReviewSubscriptionsRemoveGroupCommandOnlyCountsRefreshedDetachedSubscriptions(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	listCalls := 0
	deletePaths := []string{}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
						listCalls++
						sub1Attached := "true"
						sub2Attached := "true"
						if listCalls > 1 {
							sub1Attached = "false"
						}
						body := `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"attributes": {"referenceName": "Premium"},
								"relationships": {
									"subscriptions": {"data": [
										{"type": "subscriptions", "id": "sub-1"},
										{"type": "subscriptions", "id": "sub-2"},
										{"type": "subscriptions", "id": "sub-3"}
									]}
								}
							}],
							"included": [
								{"id": "sub-1", "type": "subscriptions", "attributes": {"productId": "com.example.monthly", "name": "Monthly", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub1Attached + `}},
								{"id": "sub-2", "type": "subscriptions", "attributes": {"productId": "com.example.annual", "name": "Annual", "state": "READY_TO_SUBMIT", "submitWithNextAppStoreVersion": ` + sub2Attached + `}},
								{"id": "sub-3", "type": "subscriptions", "attributes": {"productId": "com.example.legacy", "name": "Legacy", "state": "MISSING_METADATA", "submitWithNextAppStoreVersion": false}}
							]
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil

					case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/iris/v1/subscriptionSubmissions/"):
						deletePaths = append(deletePaths, req.URL.Path)
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader("")),
							Request:    req,
						}, nil

					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewSubscriptionsRemoveGroupCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--group-id", "group-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewSubscriptionGroupMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.ChangedCount != 1 {
		t.Fatalf("expected only one changed subscription after refresh gating, got %#v", payload)
	}
	if len(payload.Changed) != 1 || payload.Changed[0].ID != "sub-1" || payload.Changed[0].SubmitWithNextAppStoreVersion {
		t.Fatalf("expected only sub-1 in changed output as detached, got %#v", payload.Changed)
	}
	if payload.SkippedCount != 2 {
		t.Fatalf("expected one original skip plus one unchanged-after-refresh skip, got %#v", payload)
	}
	if len(deletePaths) != 2 ||
		deletePaths[0] != "/iris/v1/subscriptionSubmissions/sub-1" ||
		deletePaths[1] != "/iris/v1/subscriptionSubmissions/sub-2" {
		t.Fatalf("expected delete requests for sub-1 and sub-2, got %v", deletePaths)
	}
	reasons := []string{}
	for _, skipped := range payload.Skipped {
		reasons = append(reasons, skipped.Reason)
	}
	if !strings.Contains(strings.Join(reasons, " | "), "not attached") {
		t.Fatalf("expected not-attached skip, got %#v", payload.Skipped)
	}
	if !strings.Contains(strings.Join(reasons, " | "), "still shows attached") {
		t.Fatalf("expected unchanged-after-refresh skip, got %#v", payload.Skipped)
	}
	wantLabels := []string{
		"Loading review subscriptions",
		"Removing subscription group from next app version",
		"Refreshing review subscriptions",
	}
	if strings.Join(*labels, "|") != strings.Join(wantLabels, "|") {
		t.Fatalf("expected labels %v, got %v", wantLabels, *labels)
	}
}

func TestCollectReviewSubscriptionGroupChangesMarksNotFoundAfterRefresh(t *testing.T) {
	refreshedGroup := []webcore.ReviewSubscription{
		{ID: "sub-1", SubmitWithNextAppStoreVersion: true},
	}

	changed, skipped := collectReviewSubscriptionGroupChanges(
		refreshedGroup,
		[]string{" sub-1 ", "sub-missing"},
		true,
		reviewSubscriptionAttachUnchangedAfterRefreshReason(),
	)

	if len(changed) != 1 || changed[0].ID != "sub-1" {
		t.Fatalf("expected sub-1 to be marked changed, got %#v", changed)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected one skipped subscription, got %#v", skipped)
	}
	if skipped[0].Subscription.ID != "sub-missing" {
		t.Fatalf("expected missing subscription id in skip output, got %#v", skipped[0])
	}
	if skipped[0].Reason != "subscription was not found after refresh" {
		t.Fatalf("unexpected skip reason: %#v", skipped[0])
	}
}

func TestBuildReviewSubscriptionGroupMutationRowsIncludeChangedAndSkippedDetails(t *testing.T) {
	payload := reviewSubscriptionGroupMutationOutput{
		AppID:        "app-1",
		GroupID:      "group-1",
		Operation:    "attach-group",
		ChangedCount: 2,
		SkippedCount: 1,
		Changed: []webcore.ReviewSubscription{
			{ID: "sub-1", ProductID: "com.example.monthly", State: "READY_TO_SUBMIT", SubmitWithNextAppStoreVersion: true},
			{ID: "sub-2", Name: "   ", ProductID: "   ", State: " ", SubmitWithNextAppStoreVersion: false},
		},
		Skipped: []reviewSubscriptionMutationSkip{
			{
				Subscription: webcore.ReviewSubscription{ID: "sub-3", Name: "Legacy"},
				Reason:       "state is MISSING_METADATA",
			},
		},
	}

	rows := buildReviewSubscriptionGroupMutationRows(payload)
	if len(rows) != 9 {
		t.Fatalf("expected 6 summary + 2 changed + 1 skipped rows, got %#v", rows)
	}
	if rows[4][2] != "2" || rows[5][2] != "1" {
		t.Fatalf("expected changed/skipped counts in summary rows, got %#v", rows[4:6])
	}
	if !strings.Contains(rows[6][2], "id=sub-1") || !strings.Contains(rows[6][2], "name=com.example.monthly") || !strings.Contains(rows[6][2], "attached=true") {
		t.Fatalf("expected changed row to include product-id fallback and attached state, got %#v", rows[6])
	}
	if !strings.Contains(rows[7][2], "id=sub-2") || !strings.Contains(rows[7][2], "name=sub-2") || !strings.Contains(rows[7][2], "state=n/a") {
		t.Fatalf("expected changed row to include subscription-id fallback and n/a state, got %#v", rows[7])
	}
	if !strings.Contains(rows[8][2], "id=sub-3") || !strings.Contains(rows[8][2], "name=Legacy") || !strings.Contains(rows[8][2], "reason=state is MISSING_METADATA") {
		t.Fatalf("expected skipped row details, got %#v", rows[8])
	}
}

func TestBuildReviewSubscriptionMutationRowsFallbacks(t *testing.T) {
	rows := buildReviewSubscriptionMutationRows(reviewSubscriptionMutationOutput{
		AppID:        "app-1",
		Operation:    "attach",
		Changed:      false,
		SubmissionID: "   ",
		Subscription: webcore.ReviewSubscription{
			ID:                            "sub-1",
			ProductID:                     "   ",
			Name:                          "",
			GroupReferenceName:            "",
			State:                         "",
			SubmitWithNextAppStoreVersion: false,
			IsAppStoreReviewInProgress:    false,
		},
	})

	if len(rows) != 11 {
		t.Fatalf("expected 11 mutation rows, got %#v", rows)
	}
	if rows[3][2] != "n/a" {
		t.Fatalf("expected empty submission id to render as n/a, got %#v", rows[3])
	}
	if rows[6][2] != "sub-1" {
		t.Fatalf("expected subscription name fallback to subscription id, got %#v", rows[6])
	}
	if rows[9][2] != "false" || rows[10][2] != "false" {
		t.Fatalf("expected boolean fields to render false values, got %#v %#v", rows[9], rows[10])
	}
}

func TestReviewSubscriptionGroupLabelFallsBackToGroupID(t *testing.T) {
	name := reviewSubscriptionGroupLabel(
		[]webcore.ReviewSubscription{
			{GroupReferenceName: "   "},
			{GroupReferenceName: "Premium"},
		},
		"group-1",
	)
	if name != "Premium" {
		t.Fatalf("expected first non-empty group label, got %q", name)
	}

	fallback := reviewSubscriptionGroupLabel(
		[]webcore.ReviewSubscription{
			{GroupReferenceName: "   "},
		},
		"  group-2  ",
	)
	if fallback != "group-2" {
		t.Fatalf("expected fallback group id when names missing, got %q", fallback)
	}
}
