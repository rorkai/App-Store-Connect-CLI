package assetlibrary

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLibraryUploadContextCoversProcessing(t *testing.T) {
	video := func(ctx context.Context) (context.Context, context.CancelFunc) { return libraryVideoUploadContext(ctx) }
	image := func(ctx context.Context) (context.Context, context.CancelFunc) {
		return libraryImageUploadContext(ctx, 3<<19)
	}
	tests := []struct {
		name    string
		context func(context.Context) (context.Context, context.CancelFunc)
		env     string
		want    time.Duration
	}{
		// Apple commonly needs 10 to 15 minutes to process a library video,
		// so the general 5 minute upload default would expire mid-processing.
		{name: "default", context: video, want: 30 * time.Minute},
		{name: "explicit ASC_UPLOAD_TIMEOUT", context: video, env: "7m", want: 7 * time.Minute},
		// Image processing can also outlast 5 minutes, so a 1.5 MiB image gets
		// the screenshot upload default: 10m + 1m per file + 30s per MiB.
		{name: "image default scales with its size", context: image, want: 12 * time.Minute},
		{name: "image explicit ASC_UPLOAD_TIMEOUT", context: image, env: "7m", want: 7 * time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			t.Setenv("ASC_UPLOAD_TIMEOUT", test.env)
			t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

			start := time.Now()
			ctx, cancel := test.context(context.Background())
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
