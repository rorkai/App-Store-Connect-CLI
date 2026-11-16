package shots

import (
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// textBoxFlagValues are the raw --text-box-* flag values and whether each was
// set explicitly.
type textBoxFlagValues struct {
	color      string
	colorSet   bool
	padding    int
	paddingSet bool
	radius     int
	radiusSet  bool
}

// textBoxFlags are the validated --text-box flags. Explicit flags override the
// matching --overlay-config entry key by key.
type textBoxFlags struct {
	enabled bool
	color   string
	padding *int
	radius  *int
	// used records whether resolve drew a box for any input, so detail
	// flags that no matched overlay entry enables are rejected rather than
	// ignored. It is shared by every copy of the settings in one run.
	used *bool
}

func parseTextBoxFlags(d *shared.Deps, enabled bool, values textBoxFlagValues) (textBoxFlags, error) {
	flags := textBoxFlags{enabled: enabled, used: new(bool)}
	if values.colorSet {
		color := strings.TrimSpace(values.color)
		if err := screenshots.ValidateTextBoxColor(color); err != nil {
			return textBoxFlags{}, shared.WithDiagnostic(d.UsageError("--text-box-color "+err.Error()), shared.DiagnosticInvalidInput, "--text-box-color")
		}
		flags.color = color
	}
	if values.paddingSet {
		if values.padding < 0 {
			return textBoxFlags{}, shared.WithDiagnostic(d.UsageError("--text-box-padding must be >= 0"), shared.DiagnosticInvalidInput, "--text-box-padding")
		}
		padding := values.padding
		flags.padding = &padding
	}
	if values.radiusSet {
		if values.radius < 0 {
			return textBoxFlags{}, shared.WithDiagnostic(d.UsageError("--text-box-radius must be >= 0"), shared.DiagnosticInvalidInput, "--text-box-radius")
		}
		radius := values.radius
		flags.radius = &radius
	}
	return flags, nil
}

// firstDetailFlag names the first explicitly set detail flag for diagnostics.
func (flags textBoxFlags) firstDetailFlag() string {
	switch {
	case flags.color != "":
		return "--text-box-color"
	case flags.padding != nil:
		return "--text-box-padding"
	default:
		return "--text-box-radius"
	}
}

// resolve merges the flags over the overlay entry matched for one input. The
// box is drawn when --text-box is set or the entry sets textBox: true.
func (flags textBoxFlags) resolve(entry screenshots.OverlayEntry) *screenshots.TextBoxOptions {
	box := screenshots.OverlayTextBox(entry)
	if box == nil {
		if !flags.enabled {
			return nil
		}
		// --text-box turns the box on for every input; the entry may still
		// style it.
		box = &screenshots.TextBoxOptions{
			Color:   strings.TrimSpace(entry.TextBoxColor),
			Padding: entry.TextBoxPadding,
			Radius:  entry.TextBoxRadius,
		}
	}
	if flags.color != "" {
		box.Color = flags.color
	}
	if flags.padding != nil {
		box.Padding = flags.padding
	}
	if flags.radius != nil {
		box.Radius = flags.radius
	}
	if flags.used != nil {
		*flags.used = true
	}
	return box
}

// requireUsed rejects --text-box-color, --text-box-padding, and
// --text-box-radius when every framed input resolved without a box. Call it
// after resolving every input.
func (flags textBoxFlags) requireUsed(d *shared.Deps) error {
	detailSet := flags.color != "" || flags.padding != nil || flags.radius != nil
	if !detailSet || (flags.used != nil && *flags.used) {
		return nil
	}
	parameter := flags.firstDetailFlag()
	return shared.WithDiagnostic(d.UsageError(parameter+" has no effect: no framed input has a title or keyword with a text box enabled by --text-box or its --overlay-config entry"), shared.DiagnosticConflictingInput, parameter)
}
