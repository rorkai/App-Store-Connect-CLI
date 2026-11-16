package publish

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type postUploadBuildDistributionFake struct {
	addErrors    []error
	getErrors    []error
	getResponses []*asc.BuildResponse
	addCalls     int
	getCalls     int
}

func (f *postUploadBuildDistributionFake) AddBetaGroupsToBuildWithNotify(_ context.Context, _ string, _ []string, _ bool) (asc.BuildBetaGroupsNotificationAction, error) {
	f.addCalls++
	if len(f.addErrors) == 0 {
		return asc.BuildBetaGroupsNotificationActionNone, nil
	}
	index := min(f.addCalls-1, len(f.addErrors)-1)
	return asc.BuildBetaGroupsNotificationActionNone, f.addErrors[index]
}

func (f *postUploadBuildDistributionFake) GetBuild(_ context.Context, buildID string, _ ...asc.BuildOption) (*asc.BuildResponse, error) {
	f.getCalls++
	if len(f.getErrors) > 0 {
		index := min(f.getCalls-1, len(f.getErrors)-1)
		if f.getErrors[index] != nil {
			return nil, f.getErrors[index]
		}
	}
	if len(f.getResponses) > 0 {
		index := min(f.getCalls-1, len(f.getResponses)-1)
		return f.getResponses[index], nil
	}
	return &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{ID: buildID}}, nil
}

func TestAddUploadedBuildBetaGroupsRetriesConfirmedPropagation404ThenSucceeds(t *testing.T) {
	fake := &postUploadBuildDistributionFake{addErrors: []error{postUploadBuildMissingError("build-1"), nil}}
	waits := 0
	var diagnostics bytes.Buffer

	result, err := addUploadedBuildBetaGroupsWithPolicy(
		context.Background(),
		fake,
		"build-1",
		postUploadBuildGroups(),
		shared.AddBuildBetaGroupsOptions{},
		postUploadBuildPropagationRetryPolicy{
			Backoffs:         []time.Duration{time.Second},
			DiagnosticWriter: &diagnostics,
			Wait: func(context.Context, time.Duration) error {
				waits++
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("addUploadedBuildBetaGroupsWithPolicy() error = %v", err)
	}
	if result == nil || len(result.AddedGroupIDs) != 1 || result.AddedGroupIDs[0] != "group-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if fake.addCalls != 2 || fake.getCalls != 1 || waits != 1 {
		t.Fatalf("calls = add:%d get:%d wait:%d, want 2/1/1", fake.addCalls, fake.getCalls, waits)
	}
	wantDiagnostic := "Uploaded TestFlight build build-1 is still propagating; retrying beta-group assignment (attempt 2/2) in 1s.\n"
	if diagnostics.String() != wantDiagnostic {
		t.Fatalf("diagnostic = %q, want %q", diagnostics.String(), wantDiagnostic)
	}
}

func TestPostUploadBuildPropagationClassifierMatchesCapturedProductionErrorOnly(t *testing.T) {
	captured := &asc.APIError{
		StatusCode: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Title:      "The specified resource does not exist",
		Detail:     "There is no resource of type 'builds' with id '09f4080c-6ee7-4e52-8103-e1241eaaa58a'",
	}
	if !isPostUploadBuildPropagationError(captured, "09f4080c-6ee7-4e52-8103-e1241eaaa58a") {
		t.Fatal("expected captured production build relationship 404 to be retryable")
	}
	if isPostUploadBuildPropagationError(captured, "different-build-id") {
		t.Fatal("did not expect a mismatched build ID to be retryable")
	}
	nearCollision := postUploadBuildMissingError("x09f4080c-6ee7-4e52-8103-e1241eaaa58ay")
	if isPostUploadBuildPropagationError(nearCollision, "09f4080c-6ee7-4e52-8103-e1241eaaa58a") {
		t.Fatal("did not expect a build ID contained inside another token to be retryable")
	}
	uppercase := postUploadBuildMissingError("09F4080C-6EE7-4E52-8103-E1241EAAA58A")
	if !isPostUploadBuildPropagationError(uppercase, "09f4080c-6ee7-4e52-8103-e1241eaaa58a") {
		t.Fatal("expected differently cased quoted build ID to be retryable")
	}
}

func TestAddUploadedBuildBetaGroupsExhaustsBoundedPropagationRetries(t *testing.T) {
	fake := &postUploadBuildDistributionFake{addErrors: []error{postUploadBuildMissingError("build-1")}}
	waits := 0

	_, err := addUploadedBuildBetaGroupsWithPolicy(
		context.Background(),
		fake,
		"build-1",
		postUploadBuildGroups(),
		shared.AddBuildBetaGroupsOptions{},
		postUploadBuildPropagationRetryPolicy{
			Backoffs: []time.Duration{time.Second, 2 * time.Second},
			Wait: func(context.Context, time.Duration) error {
				waits++
				return nil
			},
		},
	)
	if err == nil || !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("expected final not-found error, got %v", err)
	}
	if fake.addCalls != 3 || fake.getCalls != 3 || waits != 2 {
		t.Fatalf("calls = add:%d get:%d wait:%d, want 3/3/2", fake.addCalls, fake.getCalls, waits)
	}
}

func TestAddUploadedBuildBetaGroupsDoesNotRetryWhenBuildCannotBeConfirmed(t *testing.T) {
	fake := &postUploadBuildDistributionFake{
		addErrors: []error{postUploadBuildMissingError("invalid-build")},
		getErrors: []error{&asc.APIError{StatusCode: http.StatusNotFound, Code: "NOT_FOUND", Detail: "build does not exist"}},
	}
	waits := 0

	_, err := addUploadedBuildBetaGroupsWithPolicy(
		context.Background(),
		fake,
		"invalid-build",
		postUploadBuildGroups(),
		shared.AddBuildBetaGroupsOptions{},
		postUploadBuildPropagationRetryPolicy{
			Backoffs: []time.Duration{0, 0},
			Wait: func(context.Context, time.Duration) error {
				waits++
				return nil
			},
		},
	)
	if err == nil || !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("expected invalid build not-found error, got %v", err)
	}
	if fake.addCalls != 1 || fake.getCalls != 1 || waits != 0 {
		t.Fatalf("calls = add:%d get:%d wait:%d, want 1/1/0", fake.addCalls, fake.getCalls, waits)
	}
}

func TestAddUploadedBuildBetaGroupsDoesNotRetryMalformedBuildConfirmation(t *testing.T) {
	tests := []struct {
		name     string
		response *asc.BuildResponse
	}{
		{name: "nil response"},
		{name: "missing build ID", response: &asc.BuildResponse{}},
		{name: "mismatched build ID", response: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{ID: "build-2"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &postUploadBuildDistributionFake{
				addErrors:    []error{postUploadBuildMissingError("build-1")},
				getResponses: []*asc.BuildResponse{tt.response},
			}
			waits := 0
			_, err := addUploadedBuildBetaGroupsWithPolicy(
				context.Background(),
				fake,
				"build-1",
				postUploadBuildGroups(),
				shared.AddBuildBetaGroupsOptions{},
				postUploadBuildPropagationRetryPolicy{
					Backoffs: []time.Duration{0},
					Wait: func(context.Context, time.Duration) error {
						waits++
						return nil
					},
				},
			)
			if err == nil || !strings.Contains(err.Error(), "response did not contain the requested build") {
				t.Fatalf("expected failed build confirmation, got %v", err)
			}
			if fake.addCalls != 1 || fake.getCalls != 1 || waits != 0 {
				t.Fatalf("calls = add:%d get:%d wait:%d, want 1/1/0", fake.addCalls, fake.getCalls, waits)
			}
		})
	}
}

func TestAddUploadedBuildBetaGroupsStopsWhenConfirmedBuildProcessingFailed(t *testing.T) {
	for _, state := range []string{asc.BuildProcessingStateFailed, asc.BuildProcessingStateInvalid} {
		t.Run(state, func(t *testing.T) {
			fake := &postUploadBuildDistributionFake{
				addErrors: []error{postUploadBuildMissingError("build-1")},
				getResponses: []*asc.BuildResponse{{Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-1",
					Attributes: asc.BuildAttributes{
						ProcessingState: state,
					},
				}}},
			}
			waits := 0

			_, err := addUploadedBuildBetaGroupsWithPolicy(
				context.Background(),
				fake,
				"build-1",
				postUploadBuildGroups(),
				shared.AddBuildBetaGroupsOptions{},
				postUploadBuildPropagationRetryPolicy{
					Backoffs: []time.Duration{time.Minute},
					Wait: func(context.Context, time.Duration) error {
						waits++
						return nil
					},
				},
			)
			if err == nil || !strings.Contains(err.Error(), "build processing failed: "+state) {
				t.Fatalf("expected terminal processing failure, got %v", err)
			}
			if fake.addCalls != 1 || fake.getCalls != 1 || waits != 0 {
				t.Fatalf("calls = add:%d get:%d wait:%d, want 1/1/0", fake.addCalls, fake.getCalls, waits)
			}
		})
	}
}

func TestAddUploadedBuildBetaGroupsKeepsRetryingForNonterminalBuildStates(t *testing.T) {
	for _, state := range []string{asc.BuildProcessingStateProcessing, asc.BuildProcessingStateValid} {
		t.Run(state, func(t *testing.T) {
			fake := &postUploadBuildDistributionFake{
				addErrors: []error{postUploadBuildMissingError("build-1"), nil},
				getResponses: []*asc.BuildResponse{{Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-1",
					Attributes: asc.BuildAttributes{
						ProcessingState: state,
					},
				}}},
			}
			waits := 0

			_, err := addUploadedBuildBetaGroupsWithPolicy(
				context.Background(),
				fake,
				"build-1",
				postUploadBuildGroups(),
				shared.AddBuildBetaGroupsOptions{},
				postUploadBuildPropagationRetryPolicy{
					Backoffs: []time.Duration{0},
					Wait: func(context.Context, time.Duration) error {
						waits++
						return nil
					},
				},
			)
			if err != nil {
				t.Fatalf("expected propagation retry success, got %v", err)
			}
			if fake.addCalls != 2 || fake.getCalls != 1 || waits != 1 {
				t.Fatalf("calls = add:%d get:%d wait:%d, want 2/1/1", fake.addCalls, fake.getCalls, waits)
			}
		})
	}
}

func TestAddUploadedBuildBetaGroupsDoesNotRetryUnrelatedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "bad request", err: &asc.APIError{StatusCode: http.StatusBadRequest, Code: "PARAMETER_ERROR", Detail: "invalid relationship"}},
		{name: "server error", err: &asc.APIError{StatusCode: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE", Detail: "try later"}},
		{name: "different 404 resource", err: &asc.APIError{StatusCode: http.StatusNotFound, Code: "NOT_FOUND", Detail: "There is no resource of type 'betaGroups' with id 'group-1'"}},
		{name: "mismatched build 404", err: postUploadBuildMissingError("build-2")},
		{name: "notification partial failure", err: &asc.BuildBetaGroupsPartialError{BuildID: "build-1", Step: "checking notification state", Err: postUploadBuildMissingError("build-1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &postUploadBuildDistributionFake{addErrors: []error{tt.err}}
			waits := 0
			_, err := addUploadedBuildBetaGroupsWithPolicy(
				context.Background(),
				fake,
				"build-1",
				postUploadBuildGroups(),
				shared.AddBuildBetaGroupsOptions{},
				postUploadBuildPropagationRetryPolicy{
					Backoffs: []time.Duration{0},
					Wait: func(context.Context, time.Duration) error {
						waits++
						return nil
					},
				},
			)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected original error %v, got %v", tt.err, err)
			}
			if fake.addCalls != 1 || fake.getCalls != 0 || waits != 0 {
				t.Fatalf("calls = add:%d get:%d wait:%d, want 1/0/0", fake.addCalls, fake.getCalls, waits)
			}
		})
	}
}

func TestAddUploadedBuildBetaGroupsPreservesCancellationAndDeadline(t *testing.T) {
	tests := []struct {
		name    string
		context func() (context.Context, context.CancelFunc)
		want    error
	}{
		{
			name: "cancellation",
			context: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			want: context.Canceled,
		},
		{
			name: "deadline",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			want: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.context()
			defer cancel()
			fake := &postUploadBuildDistributionFake{addErrors: []error{postUploadBuildMissingError("build-1")}}
			_, err := addUploadedBuildBetaGroupsWithPolicy(
				ctx,
				fake,
				"build-1",
				postUploadBuildGroups(),
				shared.AddBuildBetaGroupsOptions{},
				postUploadBuildPropagationRetryPolicy{
					Backoffs: []time.Duration{time.Minute},
					Wait: func(ctx context.Context, delay time.Duration) error {
						return waitForPostUploadBuildPropagation(ctx, time.After, delay)
					},
				},
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if fake.addCalls != 1 || fake.getCalls != 1 {
				t.Fatalf("calls = add:%d get:%d, want 1/1", fake.addCalls, fake.getCalls)
			}
		})
	}
}

func postUploadBuildGroups() []shared.ResolvedBetaGroup {
	return []shared.ResolvedBetaGroup{{ID: "group-1", Name: "External"}}
}

func postUploadBuildMissingError(buildID string) error {
	return &asc.APIError{
		StatusCode: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Title:      "The specified resource does not exist",
		Detail:     "There is no resource of type 'builds' with id '" + buildID + "'",
	}
}
