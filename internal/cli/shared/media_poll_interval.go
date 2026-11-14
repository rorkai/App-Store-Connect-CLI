package shared

import (
	"context"
	"flag"
	"io"
	"os"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// minMediaPollInterval keeps --poll-interval from polling faster than the
// default schedule's starting rate.
const minMediaPollInterval = time.Second

// MediaPollIntervalFlag is the --poll-interval flag shared by media uploads
// that wait for App Store Connect processing.
type MediaPollIntervalFlag struct {
	fs    *flag.FlagSet
	value *time.Duration
}

// BindMediaPollIntervalFlag registers --poll-interval on fs.
func BindMediaPollIntervalFlag(fs *flag.FlagSet) *MediaPollIntervalFlag {
	return &MediaPollIntervalFlag{
		fs:    fs,
		value: fs.Duration("poll-interval", 0, "Fixed wait between processing status checks, at least 1s (e.g. 15s); by default checks start every 2s and slow to every 15s after about 30s"),
	}
}

// Apply validates the flag and, when set, returns a context whose media
// processing waits poll at the requested fixed interval.
func (f *MediaPollIntervalFlag) Apply(ctx context.Context) (context.Context, error) {
	return f.applyTo(ctx, os.Stderr)
}

// ApplyWith is Apply with the "Error:" line written to d's stderr.
func (f *MediaPollIntervalFlag) ApplyWith(ctx context.Context, d *Deps) (context.Context, error) {
	return f.applyTo(ctx, d.IO.ErrOut)
}

func (f *MediaPollIntervalFlag) applyTo(ctx context.Context, stderr io.Writer) (context.Context, error) {
	set := false
	f.fs.Visit(func(fl *flag.Flag) { set = set || fl.Name == "poll-interval" })
	if !set {
		return ctx, nil
	}
	if *f.value < minMediaPollInterval {
		return ctx, usageErrorfTo(stderr, "--poll-interval must be at least %s", minMediaPollInterval)
	}
	return asc.WithMediaPollInterval(ctx, *f.value), nil
}
