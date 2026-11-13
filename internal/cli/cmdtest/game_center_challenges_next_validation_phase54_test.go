package cmdtest

import "testing"

func TestGameCenterChallengesListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "challenges", "list"}, wantErr: "game-center challenges list: --next"},
		{args: []string{"game-center", "challenges", "versions", "list"}, wantErr: "game-center challenges versions list: --next"},
		{args: []string{"game-center", "challenges", "localizations", "list"}, wantErr: "game-center challenges localizations list: --next"},
		{args: []string{"game-center", "challenges", "releases", "list"}, wantErr: "game-center challenges releases list: --next"},
	})
}

func TestGameCenterChallengesListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "challenges", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterChallenges?cursor=AQ&limit=200",
			dataType: "gameCenterChallenges",
		},
		{
			args:     []string{"game-center", "challenges", "versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterChallenges/challenge-1/versions?cursor=AQ&limit=200",
			dataType: "gameCenterChallengeVersions",
		},
		{
			args:     []string{"game-center", "challenges", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterChallengeVersions/version-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterChallengeLocalizations",
		},
		{
			args:     []string{"game-center", "challenges", "releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/challengeReleases?cursor=AQ&limit=200",
			dataType: "gameCenterChallengeVersionReleases",
		},
	})
}
