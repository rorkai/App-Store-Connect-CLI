package testflight

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type concurrentSyncStub struct {
	testFlightSyncStub
	fetchBuilds  func(context.Context, string) (*asc.BuildsResponse, error)
	fetchTesters func(context.Context, string) (*asc.BetaTestersResponse, error)
}

func (s *concurrentSyncStub) GetBetaGroupBuilds(ctx context.Context, id string, opts ...asc.BetaGroupBuildsOption) (*asc.BuildsResponse, error) {
	return s.fetchBuilds(ctx, id)
}

func (s *concurrentSyncStub) GetBetaGroupTesters(ctx context.Context, id string, opts ...asc.BetaGroupTestersOption) (*asc.BetaTestersResponse, error) {
	return s.fetchTesters(ctx, id)
}

func syncConcurrencyStub(n int) concurrentSyncStub {
	s := concurrentSyncStub{testFlightSyncStub: testFlightSyncStub{app: &asc.AppResponse{}, groups: &asc.BetaGroupsResponse{}}}
	for i := 0; i < n; i++ {
		s.groups.Data = append(s.groups.Data, asc.Resource[asc.BetaGroupAttributes]{ID: fmt.Sprint(i)})
	}
	s.fetchBuilds = func(context.Context, string) (*asc.BuildsResponse, error) { return &asc.BuildsResponse{}, nil }
	s.fetchTesters = func(context.Context, string) (*asc.BetaTestersResponse, error) {
		return &asc.BetaTestersResponse{}, nil
	}
	return s
}

func TestPullTestFlightConfigConcurrentBoundAndCancellation(t *testing.T) {
	for _, phase := range []string{"builds", "testers"} {
		t.Run(phase, func(t *testing.T) {
			s := syncConcurrencyStub(9)
			started := make(chan string, 9)
			var active, maxActive atomic.Int32
			blocked := func(ctx context.Context, id string) error {
				now := active.Add(1)
				defer active.Add(-1)
				for {
					old := maxActive.Load()
					if now <= old || maxActive.CompareAndSwap(old, now) {
						break
					}
				}
				started <- id
				<-ctx.Done()
				return ctx.Err()
			}
			if phase == "builds" {
				s.fetchBuilds = func(ctx context.Context, id string) (*asc.BuildsResponse, error) { return nil, blocked(ctx, id) }
			} else {
				s.fetchTesters = func(ctx context.Context, id string) (*asc.BetaTestersResponse, error) { return nil, blocked(ctx, id) }
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := pullTestFlightConfig(ctx, &s, "app", testFlightPullOptions{includeBuilds: phase == "builds", includeTesters: phase == "testers"})
				done <- err
			}()
			timeout := time.NewTimer(time.Second)
			defer timeout.Stop()
			for i := 0; i < 4; i++ {
				select {
				case <-started:
				case <-timeout.C:
					cancel()
					<-done
					t.Fatal("independent reads did not overlap")
				}
			}
			select {
			case id := <-started:
				t.Fatalf("fifth read %s exceeded bound", id)
			default:
			}
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancellation was ignored")
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not stop reads")
			}
			if maxActive.Load() != 4 || active.Load() != 0 {
				t.Fatalf("maximum %d, remaining %d", maxActive.Load(), active.Load())
			}
			if len(started) != 0 {
				t.Fatal("reads continued after cancellation")
			}
		})
	}
}

func TestPullTestFlightConfigConcurrentPreservesOrderAndOverlapsPhases(t *testing.T) {
	s := syncConcurrencyStub(5)
	laterStarted := make(chan string, 10)
	s.fetchBuilds = func(ctx context.Context, id string) (*asc.BuildsResponse, error) {
		if id == "0" {
			// A slow first group must not hold back the fifth group or the testers phase.
			for seen := map[string]bool{}; !seen["build-4"] || !seen["tester-0"]; {
				select {
				case started := <-laterStarted:
					seen[started] = true
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		} else {
			laterStarted <- "build-" + id
		}
		return &asc.BuildsResponse{Data: []asc.Resource[asc.BuildAttributes]{{ID: "shared", Attributes: asc.BuildAttributes{Version: id}}}}, nil
	}
	s.fetchTesters = func(ctx context.Context, id string) (*asc.BetaTestersResponse, error) {
		laterStarted <- "tester-" + id
		return &asc.BetaTestersResponse{Data: []asc.Resource[asc.BetaTesterAttributes]{{ID: "shared", Attributes: asc.BetaTesterAttributes{Email: id}}}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	config, err := pullTestFlightConfig(ctx, &s, "app", testFlightPullOptions{includeBuilds: true, includeTesters: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Builds) != 1 || config.Builds[0].Version != "0" || len(config.Testers) != 1 || config.Testers[0].Email != "0" {
		t.Fatalf("first-group precedence changed: %+v", config)
	}
	if !reflect.DeepEqual(config.Builds[0].Groups, []string{"0", "1", "2", "3", "4"}) {
		t.Fatalf("memberships changed: %v", config.Builds[0].Groups)
	}
}

func TestPullTestFlightConfigOverlapsAppAndGroupReads(t *testing.T) {
	s := appOverlapStub{concurrentSyncStub: syncConcurrencyStub(1), groupsStarted: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := pullTestFlightConfig(ctx, &s, "app", testFlightPullOptions{}); err != nil {
		t.Fatal(err)
	}
}

type appOverlapStub struct {
	concurrentSyncStub
	groupsStarted chan struct{}
}

func (s *appOverlapStub) GetApp(ctx context.Context, appID string) (*asc.AppResponse, error) {
	select {
	case <-s.groupsStarted:
		return s.app, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("app read did not overlap group read: %w", ctx.Err())
	}
}

func (s *appOverlapStub) GetBetaGroups(ctx context.Context, appID string, opts ...asc.BetaGroupsOption) (*asc.BetaGroupsResponse, error) {
	close(s.groupsStarted)
	return s.groups, nil
}

func BenchmarkPullTestFlightConfigLatency(b *testing.B) {
	s := syncConcurrencyStub(8)
	s.fetchBuilds = func(context.Context, string) (*asc.BuildsResponse, error) {
		time.Sleep(2 * time.Millisecond)
		return &asc.BuildsResponse{}, nil
	}
	s.fetchTesters = func(context.Context, string) (*asc.BetaTestersResponse, error) {
		time.Sleep(2 * time.Millisecond)
		return &asc.BetaTestersResponse{}, nil
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := pullTestFlightConfig(context.Background(), &s, "app", testFlightPullOptions{includeBuilds: true, includeTesters: true}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPullTestFlightConfigConcurrentPreservesFirstError(t *testing.T) {
	s := syncConcurrencyStub(8)
	lastStarted := make(chan struct{})
	s.fetchBuilds = func(ctx context.Context, id string) (*asc.BuildsResponse, error) {
		if id == "3" {
			close(lastStarted)
		}
		if id == "0" {
			select {
			case <-lastStarted:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("first group failed")
		}
		if id == "1" {
			return nil, fmt.Errorf("second group failed")
		}
		return &asc.BuildsResponse{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := pullTestFlightConfig(ctx, &s, "app", testFlightPullOptions{includeBuilds: true, includeTesters: true})
	if err == nil || err.Error() != "fetch beta group builds: first group failed" {
		t.Fatalf("first error changed: %v", err)
	}
}

func TestPullTestFlightConfigConcurrentPaginatesEachGroup(t *testing.T) {
	s := syncConcurrencyStub(2)
	var builds, testers [2]atomic.Int32
	s.fetchBuilds = func(ctx context.Context, id string) (*asc.BuildsResponse, error) {
		i := int(id[0] - '0')
		n := builds[i].Add(1)
		resp := &asc.BuildsResponse{Data: []asc.Resource[asc.BuildAttributes]{{ID: fmt.Sprintf("build-%s-%d", id, n)}}}
		if n == 1 {
			resp.Links.Next = "https://api.appstoreconnect.apple.com/v1/betaGroups/" + id + "/builds?page=2"
		}
		return resp, nil
	}
	s.fetchTesters = func(ctx context.Context, id string) (*asc.BetaTestersResponse, error) {
		i := int(id[0] - '0')
		n := testers[i].Add(1)
		resp := &asc.BetaTestersResponse{Data: []asc.Resource[asc.BetaTesterAttributes]{{ID: fmt.Sprintf("tester-%s-%d", id, n)}}}
		if n == 1 {
			resp.Links.Next = "https://api.appstoreconnect.apple.com/v1/betaGroups/" + id + "/betaTesters?page=2"
		}
		return resp, nil
	}
	config, err := pullTestFlightConfig(context.Background(), &s, "app", testFlightPullOptions{includeBuilds: true, includeTesters: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Builds) != 4 || len(config.Testers) != 4 {
		t.Fatalf("pages omitted: %+v", config)
	}
	for i := 0; i < 2; i++ {
		if builds[i].Load() != 2 || testers[i].Load() != 2 {
			t.Fatalf("group %d page counts: %d/%d", i, builds[i].Load(), testers[i].Load())
		}
	}
	for _, build := range config.Builds {
		if len(build.Groups) != 1 || build.Groups[0] != build.ID[6:7] {
			t.Fatalf("build membership changed: %+v", build)
		}
	}
	for _, tester := range config.Testers {
		if len(tester.Groups) != 1 || tester.Groups[0] != tester.ID[7:8] {
			t.Fatalf("tester membership changed: %+v", tester)
		}
	}
}

func TestPullTestFlightConfigConcurrentFirstErrorCancelsPeers(t *testing.T) {
	s := syncConcurrencyStub(4)
	peersStarted := make(chan struct{}, 3)
	var active atomic.Int32
	s.fetchBuilds = func(ctx context.Context, id string) (*asc.BuildsResponse, error) {
		active.Add(1)
		defer active.Add(-1)
		if id == "0" {
			for i := 0; i < 3; i++ {
				select {
				case <-peersStarted:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, fmt.Errorf("first group failed")
		}
		peersStarted <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := pullTestFlightConfig(ctx, &s, "app", testFlightPullOptions{includeBuilds: true})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || err.Error() != "fetch beta group builds: first group failed" {
			t.Fatalf("error changed: %v", err)
		}
		if active.Load() != 0 {
			t.Fatal("failed pull left active peers")
		}
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("first group failure did not cancel stalled peers")
	}
}

func TestFetchTesterGroupMembershipsUsesBoundedPool(t *testing.T) {
	groupIDs := []string{"g0", "g1", "g2", "g3", "g4"}
	lastStarted := make(chan struct{})
	var active, maxActive atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		now := active.Add(1)
		defer active.Add(-1)
		for {
			old := maxActive.Load()
			if now <= old || maxActive.CompareAndSwap(old, now) {
				break
			}
		}
		groupID := strings.Split(req.URL.Path, "/")[3]
		switch groupID {
		case "g0":
			// A slow first group must not hold back the fifth group.
			select {
			case <-lastStarted:
			case <-req.Context().Done():
				return
			}
		case "g4":
			close(lastStarted)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":[{"type":"betaTesters","id":"shared"},{"type":"betaTesters","id":"tester-%s"}],"links":{}}`, groupID)
	}))
	defer server.Close()
	client := newBetaTesterCSVConflictClient(t, server)
	resolver := &betaGroupResolver{byID: map[string]string{}, sortedGroupIDs: groupIDs}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	membership, err := fetchTesterGroupMemberships(ctx, &shared.Deps{Timeout: shared.ContextWithTimeout}, client, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(membership["shared"], groupIDs) || !reflect.DeepEqual(membership["tester-g4"], []string{"g4"}) {
		t.Fatalf("memberships changed: %v", membership)
	}
	if maxActive.Load() > 4 {
		t.Fatalf("maximum in-flight reads %d exceeded bound", maxActive.Load())
	}
}
