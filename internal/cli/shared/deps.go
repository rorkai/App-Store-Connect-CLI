package shared

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// IO holds the streams a command reads and writes, and whether its stdin,
// stdout and stderr are terminals. When In is the process stdin, whether it is
// a terminal is checked again on the stdin in use when it is read.
type IO struct {
	In        io.Reader
	Out       io.Writer
	ErrOut    io.Writer
	StdinTTY  bool
	StdoutTTY bool
	StderrTTY bool
}

// Deps holds what a command takes from the process: its App Store Connect
// client, its streams, the clock, the request and upload timeouts, and the
// timeout settings a command reads with a default of its own. After is the
// timer a command waits on, as time.After. Getwd is the working directory a
// command reads explicitly, as os.Getwd; relative paths a command passes to
// the operating system or to filepath.Abs still resolve against the process
// directory.
type Deps struct {
	Client         func() (*asc.Client, error)
	IO             *IO
	Now            func() time.Time
	Timeout        func(context.Context) (context.Context, context.CancelFunc)
	UploadTimeout  func(context.Context) (context.Context, context.CancelFunc)
	ResolveTimeout func(setting TimeoutSetting, defaultTimeout time.Duration) time.Duration

	After func(time.Duration) <-chan time.Time

	Getwd func() (string, error)
}

// DefaultDeps returns the dependencies of a normal run: the shared client
// factory, the process streams, the system clock and timers, the timeouts
// from the environment and the config file, and the process working
// directory.
func DefaultDeps() *Deps {
	return &Deps{
		Client: GetASCClient,
		IO: &IO{
			In:        processStdin{},
			Out:       processStdout{},
			ErrOut:    processStderr{},
			StdinTTY:  isTerminal(int(os.Stdin.Fd())),
			StdoutTTY: isTerminal(int(os.Stdout.Fd())),
			StderrTTY: isTerminal(int(os.Stderr.Fd())),
		},
		Now:            time.Now,
		Timeout:        ContextWithTimeout,
		UploadTimeout:  ContextWithUploadTimeout,
		ResolveTimeout: ResolveTimeoutSetting,

		After: time.After,

		Getwd: os.Getwd,
	}
}

// stdinTerminal reports whether s.In is a terminal. The process stdin reader
// reads whatever os.Stdin is when it is called, so its terminal state is taken
// from that stream at the same moment, as the package functions do.
func (s *IO) stdinTerminal() bool {
	if _, ok := s.In.(processStdin); ok {
		return isTerminal(int(os.Stdin.Fd()))
	}
	return s.StdinTTY
}

// The process streams are resolved on each call, so a command built before
// os.Stdin, os.Stdout or os.Stderr is replaced uses the replacement.
type (
	processStdin  struct{}
	processStdout struct{}
	processStderr struct{}
)

func (processStdin) Read(p []byte) (int, error) { return os.Stdin.Read(p) }

func (processStdout) Write(p []byte) (int, error) { return os.Stdout.Write(p) }

func (processStderr) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

// DownloadTimeout is ContextWithDownloadTimeout on d: a download uses the
// upload budget.
func (d *Deps) DownloadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return d.UploadTimeout(ctx)
}

// ResolvedTimeout is ContextWithResolvedTimeout on d.
func (d *Deps) ResolvedTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, d.ResolveTimeout(RequestTimeoutSetting, defaultTimeout))
}

// ResolvedUploadTimeout is ContextWithResolvedUploadTimeout on d.
func (d *Deps) ResolvedUploadTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, d.ResolveTimeout(UploadTimeoutSetting, defaultTimeout))
}

// MediaUploadTimeout is ContextWithMediaUploadTimeout on d.
func (d *Deps) MediaUploadTimeout(ctx context.Context, bytes int64) (context.Context, context.CancelFunc) {
	return d.ResolvedUploadTimeout(ctx, MediaUploadTimeoutForWork(1, bytes))
}

// AvailabilityTimeout is ContextWithAvailabilityTimeout on d.
func (d *Deps) AvailabilityTimeout(ctx context.Context, allTerritories bool) (context.Context, context.CancelFunc) {
	if allTerritories {
		return d.ResolvedTimeout(ctx, bulkAvailabilityTimeout)
	}
	return d.Timeout(ctx)
}

// NewFlagSet returns a flag set that writes usage and parse errors to d's
// stderr.
func (d *Deps) NewFlagSet(name string, errorHandling flag.ErrorHandling) *flag.FlagSet {
	fs := flag.NewFlagSet(name, errorHandling)
	fs.SetOutput(d.IO.ErrOut)
	return fs
}

// BindOutputFlags registers --output and --pretty. Unless ASC_DEFAULT_OUTPUT
// or a CI environment decides otherwise, --output defaults to table when d's
// stdout is a terminal and to json when it is not.
func (d *Deps) BindOutputFlags(fs *flag.FlagSet) OutputFlags {
	return BindOutputFlagsWith(fs, "output", d.DefaultOutputFormat(), "Output format: json, table, markdown")
}

// DefaultOutputFormat is DefaultOutputFormat decided from d's stdout, with an
// invalid ASC_DEFAULT_OUTPUT reported to d's stderr.
func (d *Deps) DefaultOutputFormat() string {
	return defaultOutputFormatFor(d.IO.StdoutTTY, d.IO.ErrOut)
}

// PrintOutput writes data to d's stdout in the given format, and the notice
// that more pages exist to d's stderr.
func (d *Deps) PrintOutput(data any, format string, pretty bool) error {
	return printOutputTo(d.IO.Out, d.IO.ErrOut, data, format, pretty)
}

// PrintOutputWithRenderers is PrintOutputWithRenderers with JSON written to
// d's stdout and the more-pages notice to d's stderr. The renderers must
// write to d.IO.Out themselves.
func (d *Deps) PrintOutputWithRenderers(data any, format string, pretty bool, tableRenderer, markdownRenderer func() error) error {
	return printOutputWithRenderersTo(d.IO.Out, d.IO.ErrOut, data, format, pretty, tableRenderer, markdownRenderer)
}

// PrintOutputRows is PrintOutputRows on d's stdout and stderr.
func (d *Deps) PrintOutputRows(data any, format string, pretty bool, headers []string, rows [][]string) error {
	return printOutputRowsTo(d.IO.Out, d.IO.ErrOut, data, format, pretty, headers, rows)
}

// RenderSection is RenderSection on d's stdout, with the title bold when d's
// stderr is a terminal.
func (d *Deps) RenderSection(title string, headers []string, rows [][]string, markdown bool) {
	renderSectionTo(d.IO.Out, d.IO.StderrTTY, title, headers, rows, markdown)
}

// PrintStreamPage is PrintStreamPage on d's stdout.
func (d *Deps) PrintStreamPage(data any) error {
	return asc.PrintJSONTo(d.IO.Out, data)
}

// PrintResolvedPrices is PrintResolvedPrices on d's stdout and stderr.
func (d *Deps) PrintResolvedPrices(result *ResolvedPricesResult, format string, pretty bool) error {
	return printResolvedPricesTo(d.IO.Out, d.IO.ErrOut, result, format, pretty)
}

// RenderLocalizationUploadResult is RenderLocalizationUploadResult on d's
// stdout.
func (d *Deps) RenderLocalizationUploadResult(result *asc.LocalizationUploadResult, markdown bool) error {
	return renderLocalizationUploadResultTo(d.IO.Out, result, markdown)
}

// UsageError is UsageError with the "Error:" line written to d's stderr.
func (d *Deps) UsageError(message string) error {
	return usageErrorTo(d.IO.ErrOut, message)
}

// UsageErrorf is UsageErrorf with the "Error:" line written to d's stderr.
func (d *Deps) UsageErrorf(format string, args ...any) error {
	return d.UsageError(fmt.Sprintf(format, args...))
}

// RequiredFlag is RequiredFlag with the "Error:" line written to d's stderr.
func (d *Deps) RequiredFlag(name string) error {
	return requiredFlagTo(d.IO.ErrOut, name)
}

// UpdateFlagRequired is UpdateFlagRequired with the "Error:" line written to
// d's stderr.
func (d *Deps) UpdateFlagRequired(fs *flag.FlagSet, updateFlags ...any) error {
	return updateFlagRequiredTo(d.IO.ErrOut, fs, updateFlags...)
}

// RejectNextFlagConflicts is RejectNextFlagConflicts with the "Error:" line
// written to d's stderr.
func (d *Deps) RejectNextFlagConflicts(fs *flag.FlagSet, next, command string, names ...string) error {
	return rejectNextFlagConflicts(d.IO.ErrOut, fs, next, command, names...)
}
