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

// IO holds the streams a command reads and writes, and whether its stdout and
// stderr are terminals.
type IO struct {
	In        io.Reader
	Out       io.Writer
	ErrOut    io.Writer
	StdoutTTY bool
	StderrTTY bool
}

// Deps holds what a command takes from the process: its App Store Connect
// client, its streams, the clock, and the request timeout.
type Deps struct {
	Client  func() (*asc.Client, error)
	IO      *IO
	Now     func() time.Time
	Timeout func(context.Context) (context.Context, context.CancelFunc)
}

// DefaultDeps returns the dependencies of a normal run: the shared client
// factory, the process streams, the system clock, and the request timeout.
func DefaultDeps() *Deps {
	return &Deps{
		Client: GetASCClient,
		IO: &IO{
			In:        processStdin{},
			Out:       processStdout{},
			ErrOut:    processStderr{},
			StdoutTTY: isTerminal(int(os.Stdout.Fd())),
			StderrTTY: isTerminal(int(os.Stderr.Fd())),
		},
		Now:     time.Now,
		Timeout: ContextWithTimeout,
	}
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
	return BindOutputFlagsWith(fs, "output", defaultOutputFormatFor(d.IO.StdoutTTY, d.IO.ErrOut), "Output format: json, table, markdown")
}

// PrintOutput writes data to d's stdout in the given format, and the notice
// that more pages exist to d's stderr.
func (d *Deps) PrintOutput(data any, format string, pretty bool) error {
	return printOutputTo(d.IO.Out, d.IO.ErrOut, data, format, pretty)
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

// RejectNextFlagConflicts is RejectNextFlagConflicts with the "Error:" line
// written to d's stderr.
func (d *Deps) RejectNextFlagConflicts(fs *flag.FlagSet, next, command string, names ...string) error {
	return rejectNextFlagConflicts(d.IO.ErrOut, fs, next, command, names...)
}
