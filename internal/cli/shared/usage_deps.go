package shared

import (
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// stderrTerminal reports whether s.ErrOut is a terminal. The process stderr
// writer writes to whatever os.Stderr is when it is called, so its terminal
// state is taken from that stream at the same moment, as the package
// functions do.
func (s *IO) stderrTerminal() bool {
	if _, ok := s.ErrOut.(processStderr); ok {
		return isTerminal(int(os.Stderr.Fd()))
	}
	return s.StderrTTY
}

// Bold returns s wrapped in ANSI bold codes when d's stderr is a terminal.
func (d *Deps) Bold(s string) string {
	return boldFor(d.IO.stderrTerminal(), s)
}

// DefaultUsageFunc is DefaultUsageFunc with bold headings decided from d's
// stderr, where help is written.
func (d *Deps) DefaultUsageFunc(c *ffcli.Command) string {
	return defaultUsage(d.IO.stderrTerminal(), c)
}

// VisibleUsageFunc renders command help while omitting deprecated and removed
// commands from nested subcommand listings, with bold headings decided from d's
// stderr. Root-level deprecated commands are already hidden elsewhere; this
// keeps nested canonical help focused on current surfaces.
func (d *Deps) VisibleUsageFunc(c *ffcli.Command) string {
	return visibleUsage(d.IO.stderrTerminal(), c)
}
