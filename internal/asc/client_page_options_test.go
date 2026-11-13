package asc

import "testing"

func TestLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "positive sets limit", limit: 50, want: 50},
		{name: "zero keeps default", limit: 0, want: 7},
		{name: "negative keeps default", limit: -1, want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &devicesQuery{listQuery: listQuery{limit: 7}}
			Limit[DevicesOption](tt.limit)(query)
			if query.limit != tt.want {
				t.Fatalf("limit = %d, want %d", query.limit, tt.want)
			}
		})
	}
}

func TestNextURL(t *testing.T) {
	const previous = "https://api.appstoreconnect.apple.com/v1/devices?cursor=previous"
	tests := []struct {
		name string
		next string
		want string
	}{
		{name: "trims URL", next: "  https://api.appstoreconnect.apple.com/v1/devices?cursor=abc\n", want: "https://api.appstoreconnect.apple.com/v1/devices?cursor=abc"},
		{name: "empty keeps previous", next: "", want: previous},
		{name: "blank keeps previous", next: " \t ", want: previous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &devicesQuery{listQuery: listQuery{nextURL: previous}}
			NextURL[DevicesOption](tt.next)(query)
			if query.nextURL != tt.want {
				t.Fatalf("nextURL = %q, want %q", query.nextURL, tt.want)
			}
		})
	}
}
