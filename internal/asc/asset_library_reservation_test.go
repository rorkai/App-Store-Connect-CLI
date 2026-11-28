package asc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAssetLibraryUploadJournalFailureStopsNextMutation(t *testing.T) {
	for _, kind := range []string{"image", "video"} {
		for _, failed := range []AssetLibraryUploadCheckpoint{AssetLibraryCheckpointReserved, AssetLibraryCheckpointTransferred} {
			t.Run(kind+"/"+string(failed), func(t *testing.T) {
				journalErr := errors.New("journal disk full")
				resourceType := "appAssetLibraryImages"
				if kind == "video" {
					resourceType = "appAssetLibraryVideos"
				}
				var transfers atomic.Int32
				var reserved atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !reserved.Load() {
						t.Error("transfer before durable reservation")
					}
					data, _ := io.ReadAll(r.Body)
					if r.Method != "PUT" || string(data) != "abc" {
						t.Errorf("transfer %s %q", r.Method, data)
					}
					transfers.Add(1)
					w.WriteHeader(http.StatusNoContent)
				}))
				defer server.Close()
				calls := 0
				client := newTestClient(t, func(r *http.Request) {
					calls++
					if calls != 1 || r.Method != "POST" {
						t.Fatalf("request after journal failure: %s %s", r.Method, r.URL)
					}
				}, &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"data":{"id":"reserved-1","type":%q,"attributes":{"state":"AWAITING_UPLOAD","uploadOperations":[{"method":"PUT","url":%q,"length":3,"offset":0}]}}}`, resourceType, server.URL))), Header: make(http.Header)})
				checkpoints := []AssetLibraryUploadCheckpoint{}
				ctx := WithAssetLibraryUploadJournal(context.Background(), func(phase AssetLibraryUploadCheckpoint, id, typ string) error {
					if id != "reserved-1" || typ != resourceType {
						t.Fatalf("reservation %s %s", id, typ)
					}
					checkpoints = append(checkpoints, phase)
					if phase == failed {
						return journalErr
					}
					reserved.Store(true)
					return nil
				})
				file := createTempAssetFile(t, []byte("abc"))
				defer file.Close()
				var id string
				var err error
				if kind == "image" {
					r, e := client.UploadAssetLibraryImageWithCategory(ctx, io.Discard, "library", "a.png", file, 3, "CREATIVE_ASSETS", "", nil)
					id, err = r.ImageID, e
				} else {
					r, e := client.UploadAssetLibraryVideoWithCategory(ctx, io.Discard, "library", "a.mp4", file, 3, "CREATIVE_ASSETS", "", "", nil)
					id, err = r.VideoID, e
				}
				wantTransfers := int32(0)
				wantCheckpoints := 1
				if failed == AssetLibraryCheckpointTransferred {
					wantTransfers = 1
					wantCheckpoints = 2
				}
				if !errors.Is(err, journalErr) || id != "reserved-1" || calls != 1 || len(checkpoints) != wantCheckpoints || transfers.Load() != wantTransfers {
					t.Fatalf("id=%s checkpoints=%v calls=%d transfers=%d err=%v", id, checkpoints, calls, transfers.Load(), err)
				}
			})
		}
	}
}
