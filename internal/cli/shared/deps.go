package shared

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

	operatorStderr io.Writer
}

// Deps holds what a command takes from the process: its App Store Connect
// client, its streams, the clock, the request and upload timeouts, and the
// timeout settings a command reads with a default of its own. After is the
// timer a command waits on, as time.After. Getwd is the working directory a
// command reads explicitly, as os.Getwd; relative paths a command passes to
// the operating system or to filepath.Abs still resolve against the process
// directory. HTTPTransport is the transport of the clients a command builds
// for services other than the App Store Connect API. ClientWithTimeout
// returns an App Store Connect client whose HTTP timeout is the given
// duration, for commands whose single requests may run longer than the
// request timeout.
type Deps struct {
	Client         func() (*asc.Client, error)
	IO             *IO
	Now            func() time.Time
	Timeout        func(context.Context) (context.Context, context.CancelFunc)
	UploadTimeout  func(context.Context) (context.Context, context.CancelFunc)
	ResolveTimeout func(setting TimeoutSetting, defaultTimeout time.Duration) time.Duration

	After func(time.Duration) <-chan time.Time

	Getwd func() (string, error)

	// LookupEnv reads the environment variables a command takes as input,
	// such as ASC_APP_ID, as os.LookupEnv. Credentials, the profile and the
	// config are still read from the process environment. nil reads as an
	// empty environment, so a test that does not care about the environment
	// need not set it and never sees the process environment.
	LookupEnv func(string) (string, bool)

	HTTPTransport func() http.RoundTripper

	ClientWithTimeout func(time.Duration) (*asc.Client, error)
}

// DefaultDeps returns the dependencies of a normal run: the shared client
// factory, the process streams, the system clock and timers, the timeouts
// from the environment and the config file, the process working directory,
// and the process HTTP transport.
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

		LookupEnv: os.LookupEnv,

		HTTPTransport: processHTTPTransport,

		ClientWithTimeout: GetASCClientWithTimeout,
	}
}

// processHTTPTransport returns http.DefaultTransport when a client is built,
// the transport an http.Client without one sends through.
func processHTTPTransport() http.RoundTripper {
	return http.DefaultTransport
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

// ChildStderr returns the stream a child process writes its stderr to: the
// process stderr file itself when s.ErrOut writes to it, so a terminal stays a
// terminal for the commands the child runs, and s.ErrOut otherwise, which then
// receives the child's output through a pipe.
func (s *IO) ChildStderr() io.Writer {
	stderr := s.OperatorStderr()
	if _, ok := stderr.(processStderr); ok {
		return os.Stderr
	}
	return stderr
}

// OperatorStderr preserves instructions that need a person's attention.
func (s *IO) OperatorStderr() io.Writer {
	if s.operatorStderr != nil {
		return s.operatorStderr
	}
	return s.ErrOut
}

// WithQuietDiagnostics discards CLI diagnostics while retaining operator and child output.
func (s *IO) WithQuietDiagnostics() *IO {
	streams := *s
	streams.operatorStderr = s.OperatorStderr()
	streams.ErrOut = io.Discard
	streams.StderrTTY = false
	return &streams
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

// DownloadTimeout bounds a streamed download, including the body copy. Client
// and request timeouts also cover that copy, so the short request budget aborts
// a large report mid-transfer. Downloads use d's upload budget.
func (d *Deps) DownloadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return d.UploadTimeout(ctx)
}

// ResolvedTimeout is ContextWithResolvedTimeout on d.
func (d *Deps) ResolvedTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, d.ResolveTimeout(RequestTimeoutSetting, defaultTimeout))
}

// ResolvedUploadTimeout returns a context bounded by ASC_UPLOAD_TIMEOUT when
// configured, or by defaultTimeout otherwise.
func (d *Deps) ResolvedUploadTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, d.ResolveTimeout(UploadTimeoutSetting, defaultTimeout))
}

// MediaUploadTimeout bounds one media file's upload and processing wait by
// ASC_UPLOAD_TIMEOUT, or by MediaUploadTimeoutForWork when it is unset.
func (d *Deps) MediaUploadTimeout(ctx context.Context, bytes int64) (context.Context, context.CancelFunc) {
	return d.ResolvedUploadTimeout(ctx, MediaUploadTimeoutForWork(1, bytes))
}

// AvailabilityTimeout returns the request context for availability updates on
// d: all-territories updates fan out one PATCH per territory, so they get the
// longer bulk timeout.
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

// DefaultOutputFormat returns the default output format for CLI commands.
// It checks ASC_DEFAULT_OUTPUT first and reports an invalid value to d's stderr.
// When unset, an interactive stdout defaults to table output while CI and
// non-interactive contexts default to JSON. Valid ASC_DEFAULT_OUTPUT values are
// "json", "table", "markdown", and "md".
func (d *Deps) DefaultOutputFormat() string {
	return defaultOutputFormatFor(d.IO.StdoutTTY, d.IO.ErrOut)
}

// PrintOutput writes data to d's stdout in the given format, and the notice
// that more pages exist to d's stderr.
func (d *Deps) PrintOutput(data any, format string, pretty bool) error {
	return printOutputTo(d.IO.Out, d.IO.ErrOut, data, format, pretty)
}

// PrintOutputWithRenderers writes data as JSON to d's stdout, or calls the
// table or markdown renderer for those formats, then writes the more-pages
// notice to d's stderr. The renderers must write to d.IO.Out themselves.
func (d *Deps) PrintOutputWithRenderers(data any, format string, pretty bool, tableRenderer, markdownRenderer func() error) error {
	return printOutputWithRenderersTo(d.IO.Out, d.IO.ErrOut, data, format, pretty, tableRenderer, markdownRenderer)
}

// PrintOutputRows prints data to d's stdout as JSON, or as one table of
// headers and rows.
func (d *Deps) PrintOutputRows(data any, format string, pretty bool, headers []string, rows [][]string) error {
	return printOutputRowsTo(d.IO.Out, d.IO.ErrOut, data, format, pretty, headers, rows)
}

// RenderSection renders a titled section as markdown or table output on d's
// stdout, with the title bold when d's stderr is a terminal.
func (d *Deps) RenderSection(title string, headers []string, rows [][]string, markdown bool) {
	renderSectionTo(d.IO.Out, d.IO.StderrTTY, title, headers, rows, markdown)
}

// PrintStreamPage writes a single page of data as a JSON line to d's stdout.
// Used with --stream --paginate to emit results page-by-page as NDJSON
// instead of buffering all pages in memory.
func (d *Deps) PrintStreamPage(data any) error {
	return asc.PrintJSONTo(d.IO.Out, data)
}

// PrintResolvedPrices renders resolved price rows on d's stdout and stderr
// without registering global raw renderers.
func (d *Deps) PrintResolvedPrices(result *ResolvedPricesResult, format string, pretty bool) error {
	return printResolvedPricesTo(d.IO.Out, d.IO.ErrOut, result, format, pretty)
}

// RenderLocalizationUploadResult renders both the batch summary and per-locale
// details on d's stdout so interactive output includes retry artifact
// information.
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

// InvalidInputError reports input that was passed correctly on the command
// line but whose content is refused, such as a private key file with open
// permissions or a malformed key file. Like UsageError it writes the "Error:"
// line to d's stderr and maps to the usage exit code 2 with the same usage
// classification. Unlike UsageError it does not wrap flag.ErrHelp, so the
// command's usage page is not printed after a message that already names the
// problem and, where the caller adds them, the commands that fix it.
func (d *Deps) InvalidInputError(message string) error {
	trimmed := strings.TrimSpace(SanitizeTerminal(message))
	if trimmed != "" {
		fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", trimmed)
	}
	return NewReportedUsageError(classifyUsageMessage(trimmed), trimmed)
}

// InvalidInputErrorf is InvalidInputError with a formatted message.
func (d *Deps) InvalidInputErrorf(format string, args ...any) error {
	return d.InvalidInputError(fmt.Sprintf(format, args...))
}

// RequiredFlag is RequiredFlag with the "Error:" line written to d's stderr.
func (d *Deps) RequiredFlag(name string) error {
	return requiredFlagTo(d.IO.ErrOut, name)
}

// UpdateFlagRequired reports an update command run without any of its update
// flags. It prints "Error: at least one update flag is required (--a, --b)" to
// d's stderr and returns MissingRequiredUsageError.
//
// updateFlags are the variables the command's flags in fs are bound to (the
// pointers fs.String and friends return, or the flag.Value passed to fs.Var),
// the same ones the command checks. The message names their flags in the order
// given, so it always matches the flags the command has. A variable that no
// flag in fs is bound to is a programming error and panics, as redefining a
// flag does in the flag package.
func (d *Deps) UpdateFlagRequired(fs *flag.FlagSet, updateFlags ...any) error {
	return updateFlagRequiredTo(d.IO.ErrOut, fs, updateFlags...)
}

// RejectNextFlagConflicts rejects explicitly provided query flags when --next
// supplies the complete continuation request, including its query string. The
// "Error:" line goes to d's stderr.
func (d *Deps) RejectNextFlagConflicts(fs *flag.FlagSet, next, command string, names ...string) error {
	return rejectNextFlagConflicts(d.IO.ErrOut, fs, next, command, names...)
}
