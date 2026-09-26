package metadata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestApplyMetadataPlanDoesNotFailWhenCancellationFollowsFinalAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = metadataFetchRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appInfoLocalizations/loc-en" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: &metadataCancelOnEOFBody{
				reader: strings.NewReader(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"name":"New name"}}}`),
				cancel: cancel,
			},
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	client := newMetadataFetchClient(t)

	actions, err := applyMetadataPlan(
		ctx,
		client,
		"appinfo-1",
		"version-1",
		"1.2.3",
		map[string]appInfoLocalPatch{
			"en-US": {
				localization: AppInfoLocalization{Name: "New name"},
				setFields:    map[string]string{"name": "New name"},
			},
		},
		nil,
		[]asc.Resource[asc.AppInfoLocalizationAttributes]{
			{ID: "loc-en", Attributes: asc.AppInfoLocalizationAttributes{Locale: "en-US", Name: "Old name"}},
		},
		nil,
		false,
		metadataIfExistsOptions{},
	)
	if err != nil {
		t.Fatalf("final completed action should succeed despite trailing cancellation: %v", err)
	}
	if ctx.Err() != context.Canceled || len(actions) != 1 || actions[0].Status != "succeeded" {
		t.Fatalf("unexpected final-action result: ctx=%v actions=%+v", ctx.Err(), actions)
	}
}

type metadataCancelOnEOFBody struct {
	reader io.Reader
	cancel context.CancelFunc
	once   sync.Once
}

func (b *metadataCancelOnEOFBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if err == io.EOF {
		b.once.Do(b.cancel)
	}
	return n, err
}

func (b *metadataCancelOnEOFBody) Close() error {
	return nil
}
