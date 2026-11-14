package asc

import "testing"

func TestReviewSubmissionRowsRemoveTerminalControls(t *testing.T) {
	resp := &ReviewSubmissionsResponse{
		Data: []ReviewSubmissionResource{{
			ID: "SUBMISSION\x1b[31mID\u202e",
			Attributes: ReviewSubmissionAttributes{
				SubmissionState: "READY_FOR_REVIEW\u009b2K",
			},
		}},
	}

	headers, rows := reviewSubmissionsRows(resp)
	assertRowsAreInert(t, headers, rows)
}

func TestReviewSubmissionItemRowsRemoveTerminalControls(t *testing.T) {
	resp := &ReviewSubmissionItemsResponse{
		Data: []ReviewSubmissionItemResource{{
			ID: "ITEM\x1b]0;pwned\x07ID",
			Attributes: ReviewSubmissionItemAttributes{
				State: "ACCEPTED\u202e",
			},
		}},
	}

	headers, rows := reviewSubmissionItemsRows(resp)
	assertRowsAreInert(t, headers, rows)
}

func TestBuildIconsRowsRemoveTerminalControls(t *testing.T) {
	resp := &BuildIconsResponse{
		Data: []Resource[BuildIconAttributes]{{
			ID: "ICON\x1b[31mID",
			Attributes: BuildIconAttributes{
				Name:     hostileText,
				IconType: IconAssetType("APP_STORE\u202e\x1b[0m"),
			},
		}},
	}

	headers, rows := buildIconsRows(resp)
	assertRowsAreInert(t, headers, rows)
}

func TestCustomerReviewResponseRowsRemoveTerminalControls(t *testing.T) {
	resp := &CustomerReviewResponseResponse{
		Data: CustomerReviewResponseResource{
			ID: "RESPONSE\x1b[31mID",
			Attributes: CustomerReviewResponseV1Attributes{
				ResponseBody: hostileText,
				State:        "PUBLISHED\u202e",
				LastModified: "2026-01-01T00:00:00Z",
			},
		},
	}

	headers, rows := customerReviewResponseRows(resp)
	assertRowsAreInert(t, headers, rows)
}
