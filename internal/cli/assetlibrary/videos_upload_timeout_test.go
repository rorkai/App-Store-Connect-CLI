package assetlibrary

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLibraryVideoUploadContextCoversVideoProcessing(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		// Apple commonly needs 10 to 15 minutes to process a library video,
		// so the general 5 minute upload default would expire mid-processing.
		{name: "default", want: 30 * time.Minute},
		{name: "explicit ASC_UPLOAD_TIMEOUT", env: "7m", want: 7 * time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			t.Setenv("ASC_UPLOAD_TIMEOUT", test.env)
			t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

			start := time.Now()
			ctx, cancel := libraryVideoUploadContext(context.Background())
			defer cancel()
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected a deadline")
			}
			if got := deadline.Sub(start); got < test.want-time.Second || got > test.want+time.Second {
				t.Fatalf("deadline in %s, want about %s", got, test.want)
			}
		})
	}
}
