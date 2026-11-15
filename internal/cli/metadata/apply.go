package metadata

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newMetadataApplyCommand returns the canonical apply alias for metadata push.
func newMetadataApplyCommand(d *shared.Deps) *ffcli.Command {
	return newMetadataMutationCommand(d, metadataMutationCommandConfig{
		name:      "apply",
		verbTitle: "Apply",
	})
}
