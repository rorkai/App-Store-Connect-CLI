package testflight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type testFlightSyncStub struct {
	app              *asc.AppResponse
	groups           *asc.BetaGroupsResponse
	buildsByGroup    map[string]*asc.BuildsResponse
	testersByGroup   map[string]*asc.BetaTestersResponse
	buildsErrByGroup map[string]error
	testerErrByGroup map[string]error
}

func (s *testFlightSyncStub) GetApp(ctx context.Context, appID string) (*asc.AppResponse, error) {
	return s.app, nil
}

func (s *testFlightSyncStub) GetBetaGroups(ctx context.Context, appID string, opts ...asc.BetaGroupsOption) (*asc.BetaGroupsResponse, error) {
	return s.groups, nil
}

func (s *testFlightSyncStub) GetBetaGroupBuilds(ctx context.Context, groupID string, opts ...asc.BetaGroupBuildsOption) (*asc.BuildsResponse, error) {
	if err, ok := s.buildsErrByGroup[groupID]; ok && err != nil {
		return nil, err
	}
	if resp, ok := s.buildsByGroup[groupID]; ok && resp != nil {
		return resp, nil
	}
	return &asc.BuildsResponse{}, nil
}

func (s *testFlightSyncStub) GetBetaGroupTesters(ctx context.Context, groupID string, opts ...asc.BetaGroupTestersOption) (*asc.BetaTestersResponse, error) {
	if err, ok := s.testerErrByGroup[groupID]; ok && err != nil {
		return nil, err
	}
	if resp, ok := s.testersByGroup[groupID]; ok && resp != nil {
		return resp, nil
	}
	return &asc.BetaTestersResponse{}, nil
}

func TestPullTestFlightConfig_IncludesBuildsAndTesters(t *testing.T) {
	stub := testFlightSyncStub{
		app: &asc.AppResponse{
			Data: asc.Resource[asc.AppAttributes]{
				ID: "app-1",
				Attributes: asc.AppAttributes{
					Name:     "Demo",
					BundleID: "com.example.demo",
				},
			},
		},
		groups: &asc.BetaGroupsResponse{
			Data: []asc.Resource[asc.BetaGroupAttributes]{
				{
					ID: "group-1",
					Attributes: asc.BetaGroupAttributes{
						Name:            "Alpha",
						IsInternalGroup: true,
						FeedbackEnabled: true,
					},
				},
				{
					ID: "group-2",
					Attributes: asc.BetaGroupAttributes{
						Name:                   "Beta",
						IsInternalGroup:        false,
						PublicLinkEnabled:      true,
						PublicLinkLimitEnabled: true,
						PublicLinkLimit:        100,
						FeedbackEnabled:        true,
					},
				},
			},
		},
		buildsByGroup: map[string]*asc.BuildsResponse{
			"group-1": {
				Data: []asc.Resource[asc.BuildAttributes]{
					{
						ID: "build-1",
						Attributes: asc.BuildAttributes{
							Version:         "1.0.0",
							UploadedDate:    "2026-01-20T00:00:00Z",
							ProcessingState: "PROCESSING",
						},
					},
				},
			},
			"group-2": {
				Data: []asc.Resource[asc.BuildAttributes]{
					{
						ID: "build-1",
						Attributes: asc.BuildAttributes{
							Version:         "1.0.0",
							UploadedDate:    "2026-01-20T00:00:00Z",
							ProcessingState: "PROCESSING",
						},
					},
					{
						ID: "build-2",
						Attributes: asc.BuildAttributes{
							Version:         "1.1.0",
							UploadedDate:    "2026-01-21T00:00:00Z",
							ProcessingState: "VALID",
						},
					},
				},
			},
		},
		testersByGroup: map[string]*asc.BetaTestersResponse{
			"group-1": {
				Data: []asc.Resource[asc.BetaTesterAttributes]{
					{
						ID: "tester-1",
						Attributes: asc.BetaTesterAttributes{
							FirstName: "Ada",
							LastName:  "Lovelace",
							Email:     "ada@example.com",
							State:     asc.BetaTesterStateInvited,
						},
					},
				},
			},
			"group-2": {
				Data: []asc.Resource[asc.BetaTesterAttributes]{
					{
						ID: "tester-1",
						Attributes: asc.BetaTesterAttributes{
							FirstName: "Ada",
							LastName:  "Lovelace",
							Email:     "ada@example.com",
							State:     asc.BetaTesterStateInvited,
						},
					},
					{
						ID: "tester-2",
						Attributes: asc.BetaTesterAttributes{
							FirstName: "Grace",
							LastName:  "Hopper",
							Email:     "grace@example.com",
							State:     asc.BetaTesterStateAccepted,
						},
					},
				},
			},
		},
	}

	config, err := pullTestFlightConfig(context.Background(), &stub, "app-1", testFlightPullOptions{
		includeBuilds:  true,
		includeTesters: true,
	})
	if err != nil {
		t.Fatalf("pullTestFlightConfig() error: %v", err)
	}

	if config.App.Name != "Demo" {
		t.Fatalf("expected app name Demo, got %q", config.App.Name)
	}
	if len(config.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(config.Groups))
	}
	if len(config.Builds) != 2 {
		t.Fatalf("expected 2 builds, got %d", len(config.Builds))
	}
	if len(config.Testers) != 2 {
		t.Fatalf("expected 2 testers, got %d", len(config.Testers))
	}

	groupByID := make(map[string]TestFlightGroupConfig)
	for _, group := range config.Groups {
		groupByID[group.ID] = group
	}
	if got := strings.Join(groupByID["group-1"].Builds, ","); got != "build-1" {
		t.Fatalf("expected group-1 builds build-1, got %q", got)
	}
	if len(groupByID["group-1"].BuildDetails) != 1 || groupByID["group-1"].BuildDetails[0].BuildNumber != "1.0.0" {
		t.Fatalf("expected group-1 buildDetails with build number 1.0.0, got %+v", groupByID["group-1"].BuildDetails)
	}
	if got := strings.Join(groupByID["group-2"].Builds, ","); got != "build-1,build-2" {
		t.Fatalf("expected group-2 builds build-1,build-2, got %q", got)
	}

	buildByID := make(map[string]TestFlightBuildConfig)
	for _, build := range config.Builds {
		buildByID[build.ID] = build
	}
	if got := strings.Join(buildByID["build-1"].Groups, ","); got != "group-1,group-2" {
		t.Fatalf("expected build-1 groups group-1,group-2, got %q", got)
	}

	testerByID := make(map[string]TestFlightTesterConfig)
	for _, tester := range config.Testers {
		testerByID[tester.ID] = tester
	}
	if testerByID["tester-1"].Name != "Ada Lovelace" {
		t.Fatalf("expected tester-1 name Ada Lovelace, got %q", testerByID["tester-1"].Name)
	}
	if got := strings.Join(testerByID["tester-1"].Groups, ","); got != "group-1,group-2" {
		t.Fatalf("expected tester-1 groups group-1,group-2, got %q", got)
	}
}

func TestPullTestFlightConfig_GroupFilter(t *testing.T) {
	stub := testFlightSyncStub{
		app: &asc.AppResponse{
			Data: asc.Resource[asc.AppAttributes]{
				ID: "app-1",
				Attributes: asc.AppAttributes{
					Name:     "Demo",
					BundleID: "com.example.demo",
				},
			},
		},
		groups: &asc.BetaGroupsResponse{
			Data: []asc.Resource[asc.BetaGroupAttributes]{
				{
					ID: "group-1",
					Attributes: asc.BetaGroupAttributes{
						Name: "Alpha",
					},
				},
				{
					ID: "group-2",
					Attributes: asc.BetaGroupAttributes{
						Name: "Beta",
					},
				},
			},
		},
		buildsByGroup: map[string]*asc.BuildsResponse{
			"group-2": {
				Data: []asc.Resource[asc.BuildAttributes]{
					{
						ID: "build-2",
						Attributes: asc.BuildAttributes{
							Version:         "1.1.0",
							UploadedDate:    "2026-01-21T00:00:00Z",
							ProcessingState: "VALID",
						},
					},
				},
			},
		},
		testersByGroup: map[string]*asc.BetaTestersResponse{
			"group-2": {
				Data: []asc.Resource[asc.BetaTesterAttributes]{
					{
						ID: "tester-2",
						Attributes: asc.BetaTesterAttributes{
							Email: "grace@example.com",
							State: asc.BetaTesterStateAccepted,
						},
					},
				},
			},
		},
	}

	config, err := pullTestFlightConfig(context.Background(), &stub, "app-1", testFlightPullOptions{
		includeBuilds:  true,
		includeTesters: true,
		groupFilter:    "beta",
	})
	if err != nil {
		t.Fatalf("pullTestFlightConfig() error: %v", err)
	}

	if len(config.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(config.Groups))
	}
	if config.Groups[0].ID != "group-2" {
		t.Fatalf("expected group-2, got %q", config.Groups[0].ID)
	}
	if len(config.Builds) != 1 || config.Builds[0].ID != "build-2" {
		t.Fatalf("expected build-2, got %+v", config.Builds)
	}
	if len(config.Testers) != 1 || config.Testers[0].ID != "tester-2" {
		t.Fatalf("expected tester-2, got %+v", config.Testers)
	}
}

func TestPullTestFlightConfig_WithoutIncludeBuildsOmitsBuildDetails(t *testing.T) {
	stub := testFlightSyncStub{
		app: &asc.AppResponse{
			Data: asc.Resource[asc.AppAttributes]{
				ID: "app-1",
				Attributes: asc.AppAttributes{
					Name:     "Demo",
					BundleID: "com.example.demo",
				},
			},
		},
		groups: &asc.BetaGroupsResponse{
			Data: []asc.Resource[asc.BetaGroupAttributes]{
				{ID: "group-1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}},
			},
		},
		buildsByGroup: map[string]*asc.BuildsResponse{
			"group-1": {
				Data: []asc.Resource[asc.BuildAttributes]{
					{ID: "build-1", Attributes: asc.BuildAttributes{Version: "1.0.0"}},
				},
			},
		},
	}

	config, err := pullTestFlightConfig(context.Background(), &stub, "app-1", testFlightPullOptions{
		includeBuilds: false,
	})
	if err != nil {
		t.Fatalf("pullTestFlightConfig() error: %v", err)
	}

	if len(config.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(config.Groups))
	}
	if config.Groups[0].Builds != nil {
		t.Fatalf("expected no group builds when include-builds=false, got %+v", config.Groups[0].Builds)
	}
	if config.Groups[0].BuildDetails != nil {
		t.Fatalf("expected no group buildDetails when include-builds=false, got %+v", config.Groups[0].BuildDetails)
	}
	if config.Builds != nil {
		t.Fatalf("expected no top-level builds when include-builds=false, got %+v", config.Builds)
	}
}

func TestPullTestFlightConfig_BuildFilterAlsoFiltersBuildDetails(t *testing.T) {
	stub := testFlightSyncStub{
		app: &asc.AppResponse{
			Data: asc.Resource[asc.AppAttributes]{
				ID: "app-1",
				Attributes: asc.AppAttributes{
					Name:     "Demo",
					BundleID: "com.example.demo",
				},
			},
		},
		groups: &asc.BetaGroupsResponse{
			Data: []asc.Resource[asc.BetaGroupAttributes]{
				{ID: "group-1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}},
			},
		},
		buildsByGroup: map[string]*asc.BuildsResponse{
			"group-1": {
				Data: []asc.Resource[asc.BuildAttributes]{
					{ID: "build-1", Attributes: asc.BuildAttributes{Version: "1.0.0", ProcessingState: "PROCESSING", UploadedDate: "2026-01-20T00:00:00Z"}},
					{ID: "build-2", Attributes: asc.BuildAttributes{Version: "1.1.0", ProcessingState: "VALID", UploadedDate: "2026-01-21T00:00:00Z"}},
				},
			},
		},
	}

	config, err := pullTestFlightConfig(context.Background(), &stub, "app-1", testFlightPullOptions{
		includeBuilds: true,
		buildFilters:  []string{"build-2"},
	})
	if err != nil {
		t.Fatalf("pullTestFlightConfig() error: %v", err)
	}

	if len(config.Builds) != 1 || config.Builds[0].ID != "build-2" {
		t.Fatalf("expected only filtered build build-2, got %+v", config.Builds)
	}
	if len(config.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(config.Groups))
	}
	group := config.Groups[0]
	if got := strings.Join(group.Builds, ","); got != "build-2" {
		t.Fatalf("expected filtered group builds build-2, got %q", got)
	}
	if len(group.BuildDetails) != 1 {
		t.Fatalf("expected one filtered build detail, got %+v", group.BuildDetails)
	}
	if group.BuildDetails[0].ID != "build-2" || group.BuildDetails[0].BuildNumber != "1.1.0" {
		t.Fatalf("expected build detail for build-2, got %+v", group.BuildDetails[0])
	}
}

func TestResolveTestFlightOutputPath(t *testing.T) {
	cwd := t.TempDir()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})

	got, err := resolveTestFlightOutputPath("configs/testflight.yaml")
	if err != nil {
		t.Fatalf("resolveTestFlightOutputPath() error: %v", err)
	}
	// Resolve symlinks on cwd to match the resolved output path (macOS /var -> /private/var)
	cwdResolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatalf("EvalSymlinks() error: %v", err)
	}
	want := filepath.Join(cwdResolved, "configs", "testflight.yaml")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	if _, err := resolveTestFlightOutputPath("../outside.yaml"); err == nil {
		t.Fatalf("expected traversal error")
	}
}

func TestMarshalTestFlightConfigYAML(t *testing.T) {
	config := &TestFlightConfig{
		App: TestFlightAppConfig{
			ID:       "app-1",
			Name:     "Demo",
			BundleID: "com.example.demo",
		},
		Groups: []TestFlightGroupConfig{
			{
				ID:              "group-1",
				Name:            "Alpha",
				IsInternalGroup: true,
				FeedbackEnabled: true,
			},
		},
	}

	data, err := marshalTestFlightConfigYAML(config)
	if err != nil {
		t.Fatalf("marshalTestFlightConfigYAML() error: %v", err)
	}

	if !strings.Contains(string(data), "bundleId:") {
		t.Fatalf("expected bundleId in YAML, got: %s", string(data))
	}

	var decoded TestFlightConfig
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error: %v", err)
	}
	if decoded.App.BundleID != "com.example.demo" {
		t.Fatalf("expected bundleId to round-trip, got %q", decoded.App.BundleID)
	}
}

func manyGroupsSyncStub(groupCount int) *testFlightSyncStub {
	stub := &testFlightSyncStub{
		app: &asc.AppResponse{
			Data: asc.Resource[asc.AppAttributes]{
				ID: "app-1",
				Attributes: asc.AppAttributes{
					Name:     "Demo",
					BundleID: "com.example.demo",
				},
			},
		},
		groups:         &asc.BetaGroupsResponse{},
		buildsByGroup:  map[string]*asc.BuildsResponse{},
		testersByGroup: map[string]*asc.BetaTestersResponse{},
	}

	for i := 0; i < groupCount; i++ {
		groupID := fmt.Sprintf("group-%02d", i)
		stub.groups.Data = append(stub.groups.Data, asc.Resource[asc.BetaGroupAttributes]{
			ID: groupID,
			Attributes: asc.BetaGroupAttributes{
				Name:            fmt.Sprintf("Group %02d", i),
				FeedbackEnabled: true,
			},
		})
		stub.buildsByGroup[groupID] = &asc.BuildsResponse{
			Data: []asc.Resource[asc.BuildAttributes]{
				// Shared build exercises cross-group aggregation.
				{
					ID: "build-shared",
					Attributes: asc.BuildAttributes{
						Version:         "1.0.0",
						UploadedDate:    "2026-01-20T00:00:00Z",
						ProcessingState: "VALID",
					},
				},
				{
					ID: fmt.Sprintf("build-%02d", i),
					Attributes: asc.BuildAttributes{
						Version:         fmt.Sprintf("1.0.%d", i),
						UploadedDate:    "2026-01-21T00:00:00Z",
						ProcessingState: "VALID",
					},
				},
			},
		}
		stub.testersByGroup[groupID] = &asc.BetaTestersResponse{
			Data: []asc.Resource[asc.BetaTesterAttributes]{
				{
					ID: "tester-shared",
					Attributes: asc.BetaTesterAttributes{
						Email: "shared@example.com",
						State: asc.BetaTesterStateAccepted,
					},
				},
				{
					ID: fmt.Sprintf("tester-%02d", i),
					Attributes: asc.BetaTesterAttributes{
						Email: fmt.Sprintf("tester%02d@example.com", i),
						State: asc.BetaTesterStateInvited,
					},
				},
			},
		}
	}
	return stub
}

func TestPullTestFlightConfig_ParallelFetchPreservesDeterministicOutput(t *testing.T) {
	const groupCount = 9
	stub := manyGroupsSyncStub(groupCount)
	opts := testFlightPullOptions{
		includeBuilds:  true,
		includeTesters: true,
	}

	first, err := pullTestFlightConfig(context.Background(), stub, "app-1", opts)
	if err != nil {
		t.Fatalf("pullTestFlightConfig() error: %v", err)
	}

	if len(first.Groups) != groupCount {
		t.Fatalf("expected %d groups, got %d", groupCount, len(first.Groups))
	}
	// One shared build plus one distinct build per group.
	if len(first.Builds) != groupCount+1 {
		t.Fatalf("expected %d builds, got %d", groupCount+1, len(first.Builds))
	}
	if len(first.Testers) != groupCount+1 {
		t.Fatalf("expected %d testers, got %d", groupCount+1, len(first.Testers))
	}

	var sharedBuild *TestFlightBuildConfig
	for i := range first.Builds {
		if first.Builds[i].ID == "build-shared" {
			sharedBuild = &first.Builds[i]
			break
		}
	}
	if sharedBuild == nil {
		t.Fatalf("expected shared build in %#v", first.Builds)
	}
	if len(sharedBuild.Groups) != groupCount {
		t.Fatalf("expected shared build in %d groups, got %#v", groupCount, sharedBuild.Groups)
	}
	if !sort.StringsAreSorted(sharedBuild.Groups) {
		t.Fatalf("expected sorted shared build groups, got %#v", sharedBuild.Groups)
	}

	for run := 0; run < 4; run++ {
		again, err := pullTestFlightConfig(context.Background(), stub, "app-1", opts)
		if err != nil {
			t.Fatalf("pullTestFlightConfig() error on run %d: %v", run, err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("non-deterministic output on run %d:\nfirst = %#v\nagain = %#v", run, first, again)
		}
	}
}

func TestPullTestFlightConfig_BuildFetchErrorPropagates(t *testing.T) {
	stub := manyGroupsSyncStub(6)
	wantErr := errors.New("builds unavailable")
	stub.buildsErrByGroup = map[string]error{"group-03": wantErr}

	_, err := pullTestFlightConfig(context.Background(), stub, "app-1", testFlightPullOptions{
		includeBuilds: true,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("pullTestFlightConfig() error = %v, want %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "fetch beta group builds") {
		t.Fatalf("expected wrapped build fetch error, got %v", err)
	}
}

func TestPullTestFlightConfig_TesterFetchErrorPropagates(t *testing.T) {
	stub := manyGroupsSyncStub(6)
	wantErr := errors.New("testers unavailable")
	stub.testerErrByGroup = map[string]error{"group-05": wantErr}

	_, err := pullTestFlightConfig(context.Background(), stub, "app-1", testFlightPullOptions{
		includeTesters: true,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("pullTestFlightConfig() error = %v, want %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "fetch beta group testers") {
		t.Fatalf("expected wrapped tester fetch error, got %v", err)
	}
}
