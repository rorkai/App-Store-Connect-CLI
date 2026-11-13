package cmdtest

import "testing"

func TestTestFlightListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"testflight", "testers", "apps", "list"}, wantErr: "testflight testers apps list: --next"},
		{args: []string{"testflight", "testers", "groups", "list"}, wantErr: "testflight testers groups list: --next"},
		{args: []string{"testflight", "testers", "builds", "list"}, wantErr: "testflight testers builds list: --next"},
		{args: []string{"testflight", "testers", "metrics"}, wantErr: "testflight testers metrics: --next"},
		{args: []string{"testflight", "metrics", "app-testers"}, wantErr: "testflight metrics app-testers: --next"},
		{args: []string{"testflight", "review", "view"}, wantErr: "testflight review view: --next"},
	})
}

func TestTestFlightListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"testflight", "groups", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaGroups?cursor=AQ&limit=200",
			dataType: "betaGroups",
		},
		{
			args:     []string{"testflight", "testers", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=AQ&limit=200",
			dataType: "betaTesters",
		},
		{
			args:     []string{"testflight", "groups", "links", "view", "--group-id", "group-1", "--type", "betaTesters"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/relationships/betaTesters?cursor=AQ&limit=200",
			dataType: "betaTesters",
		},
		{
			args:     []string{"testflight", "testers", "links", "view", "--tester-id", "tester-1", "--type", "apps"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/relationships/apps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"testflight", "testers", "apps", "list", "--tester-id", "tester-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/apps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"testflight", "testers", "groups", "list", "--tester-id", "tester-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/betaGroups?cursor=AQ&limit=200",
			dataType: "betaGroups",
		},
		{
			args:     []string{"testflight", "testers", "builds", "list", "--tester-id", "tester-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/builds?cursor=AQ&limit=200",
			dataType: "builds",
		},
		{
			args:     []string{"testflight", "agreements", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaLicenseAgreements?cursor=AQ&limit=200",
			dataType: "betaLicenseAgreements",
		},
		{
			args:       []string{"testflight", "testers", "metrics"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/metrics/betaTesterUsages?cursor=AQ&limit=2",
			dataType:   "betaTesterUsages",
			singlePage: true,
		},
		{
			args:       []string{"testflight", "review", "view"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/betaAppReviewDetails?cursor=AQ&limit=2",
			dataType:   "betaAppReviewDetails",
			singlePage: true,
		},
		{
			args:       []string{"testflight", "review", "view"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/buildBetaDetails?cursor=AQ&limit=2",
			dataType:   "buildBetaDetails",
			singlePage: true,
		},
		{
			args:       []string{"testflight", "recruitment", "options"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/betaRecruitmentCriterionOptions?cursor=AQ&limit=2",
			dataType:   "betaRecruitmentCriterionOptions",
			singlePage: true,
		},
		{
			args:     []string{"testflight", "review", "submissions", "list", "--build-id", "build-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaAppReviewSubmissions?cursor=AQ&limit=200",
			dataType: "betaAppReviewSubmissions",
		},
	})
}
