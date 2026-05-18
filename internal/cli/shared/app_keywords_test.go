package shared

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestBuildAppKeywordsResponse(t *testing.T) {
	keywords := []string{"alpha", "beta"}
	resp := BuildAppKeywordsResponse(keywords)
	if resp == nil {
		t.Fatal("expected non-nil response")
		return
	}
	if len(resp.Data) != len(keywords) {
		t.Fatalf("expected %d keywords, got %d", len(keywords), len(resp.Data))
	}
	for i, keyword := range keywords {
		entry := resp.Data[i]
		if entry.Type != asc.ResourceTypeAppKeywords {
			t.Fatalf("entry[%d] type = %q, want %q", i, entry.Type, asc.ResourceTypeAppKeywords)
		}
		if entry.ID != keyword {
			t.Fatalf("entry[%d] id = %q, want %q", i, entry.ID, keyword)
		}
	}
}
