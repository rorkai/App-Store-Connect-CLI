package cmdtest

import "testing"

func TestVideoPreviewsSetPosterFrameUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "missing id",
			args:    []string{"video-previews", "set-poster-frame", "--time-code", "00:00:01:00"},
			wantErr: "--id is required",
		},
		{
			name:    "missing time code",
			args:    []string{"video-previews", "set-poster-frame", "--id", "PREVIEW_123"},
			wantErr: "--time-code is required",
		},
		{
			name:    "invalid time code",
			args:    []string{"video-previews", "set-poster-frame", "--id", "PREVIEW_123", "--time-code", "abc"},
			wantErr: "HH:MM:SS:FF or HH:MM:SS.mmm",
		},
		{
			name:    "rejects positional args",
			args:    []string{"video-previews", "set-poster-frame", "--id", "PREVIEW_123", "--time-code", "00:00:01:00", "extra-arg"},
			wantErr: `unexpected argument "extra-arg"`,
		},
	})
}
