package asc

import "testing"

func TestReviewSubmissionItemsRows_UsesExpandedRelationships(t *testing.T) {
	tests := []struct {
		name     string
		rel      *ReviewSubmissionItemRelationships
		wantType string
		wantID   string
	}{
		{
			name: "app custom product page version",
			rel: &ReviewSubmissionItemRelationships{
				AppCustomProductPageVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeAppCustomProductPageVersions, ID: "cppv-1"},
				},
			},
			wantType: string(ResourceTypeAppCustomProductPageVersions),
			wantID:   "cppv-1",
		},
		{
			name: "legacy app custom product page",
			rel: &ReviewSubmissionItemRelationships{
				AppCustomProductPage: &Relationship{
					Data: ResourceData{Type: ResourceTypeAppCustomProductPages, ID: "cpp-1"},
				},
			},
			wantType: string(ResourceTypeAppCustomProductPages),
			wantID:   "cpp-1",
		},
		{
			name: "app store version experiment v2",
			rel: &ReviewSubmissionItemRelationships{
				AppStoreVersionExperimentV2: &Relationship{
					Data: ResourceData{Type: ResourceTypeAppStoreVersionExperiments, ID: "exp-1"},
				},
			},
			wantType: string(ResourceTypeAppStoreVersionExperiments),
			wantID:   "exp-1",
		},
		{
			name: "background asset version",
			rel: &ReviewSubmissionItemRelationships{
				BackgroundAssetVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeBackgroundAssetVersions, ID: "bgv-1"},
				},
			},
			wantType: string(ResourceTypeBackgroundAssetVersions),
			wantID:   "bgv-1",
		},
		{
			name: "game center achievement version",
			rel: &ReviewSubmissionItemRelationships{
				GameCenterAchievementVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeGameCenterAchievementVersions, ID: "achv-1"},
				},
			},
			wantType: string(ResourceTypeGameCenterAchievementVersions),
			wantID:   "achv-1",
		},
		{
			name: "game center activity version",
			rel: &ReviewSubmissionItemRelationships{
				GameCenterActivityVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeGameCenterActivityVersions, ID: "actv-1"},
				},
			},
			wantType: string(ResourceTypeGameCenterActivityVersions),
			wantID:   "actv-1",
		},
		{
			name: "game center challenge version",
			rel: &ReviewSubmissionItemRelationships{
				GameCenterChallengeVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeGameCenterChallengeVersions, ID: "chv-1"},
				},
			},
			wantType: string(ResourceTypeGameCenterChallengeVersions),
			wantID:   "chv-1",
		},
		{
			name: "game center leaderboard set version",
			rel: &ReviewSubmissionItemRelationships{
				GameCenterLeaderboardSetVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeGameCenterLeaderboardSetVersions, ID: "lbsv-1"},
				},
			},
			wantType: string(ResourceTypeGameCenterLeaderboardSetVersions),
			wantID:   "lbsv-1",
		},
		{
			name: "game center leaderboard version",
			rel: &ReviewSubmissionItemRelationships{
				GameCenterLeaderboardVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeGameCenterLeaderboardVersions, ID: "lbv-1"},
				},
			},
			wantType: string(ResourceTypeGameCenterLeaderboardVersions),
			wantID:   "lbv-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.rel.ReviewSubmission = &Relationship{
				Data: ResourceData{Type: ResourceTypeReviewSubmissions, ID: "submission-1"},
			}
			resp := &ReviewSubmissionItemsResponse{
				Data: []ReviewSubmissionItemResource{
					{
						ID:            "item-1",
						Attributes:    ReviewSubmissionItemAttributes{State: "READY_FOR_REVIEW"},
						Relationships: test.rel,
					},
				},
			}

			headers, rows := reviewSubmissionItemsRows(resp)
			if len(headers) != 5 {
				t.Fatalf("expected 5 headers, got %d (%v)", len(headers), headers)
			}
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			if len(rows[0]) != 5 {
				t.Fatalf("expected 5 columns, got %d (%v)", len(rows[0]), rows[0])
			}
			if rows[0][2] != test.wantType {
				t.Fatalf("expected item type %q, got %q", test.wantType, rows[0][2])
			}
			if rows[0][3] != test.wantID {
				t.Fatalf("expected item id %q, got %q", test.wantID, rows[0][3])
			}
			if rows[0][4] != "submission-1" {
				t.Fatalf("expected submission id %q, got %q", "submission-1", rows[0][4])
			}
		})
	}
}
