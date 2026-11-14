package promotedpurchases

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// FixedProductTypeCreateConfig customizes a promoted-purchases create subcommand for a specific product type.
type FixedProductTypeCreateConfig struct {
	ShortUsage     string
	ShortHelp      string
	LongHelp       string
	ProductType    promotedPurchaseProductType
	ProductIDUsage string
	// Deps is what cmd was built on. It defaults to shared.DefaultDeps().
	Deps *shared.Deps
}

// ConfigureFixedProductTypeCreateCommand rewrites the create subcommand to enforce a fixed product type.
func ConfigureFixedProductTypeCreateCommand(cmd *ffcli.Command, cfg FixedProductTypeCreateConfig) {
	d := configDeps(cfg.Deps)
	createCmd := findDirectSubcommand(cmd, "create")
	if createCmd == nil || createCmd.FlagSet == nil || createCmd.Exec == nil {
		return
	}

	createCmd.ShortUsage = cfg.ShortUsage
	createCmd.ShortHelp = cfg.ShortHelp
	createCmd.LongHelp = cfg.LongHelp

	if productTypeFlag := createCmd.FlagSet.Lookup("product-type"); productTypeFlag != nil {
		productTypeFlag.Usage = fmt.Sprintf("Product type: %s (fixed for this command)", cfg.ProductType)
		shared.HideFlagFromHelp(productTypeFlag)
	}
	if productIDFlag := createCmd.FlagSet.Lookup("product-id"); productIDFlag != nil && strings.TrimSpace(cfg.ProductIDUsage) != "" {
		productIDFlag.Usage = cfg.ProductIDUsage
	}

	originalExec := createCmd.Exec
	createCmd.Exec = func(ctx context.Context, args []string) error {
		productTypeFlag := createCmd.FlagSet.Lookup("product-type")
		if productTypeFlag != nil {
			currentValue := strings.TrimSpace(productTypeFlag.Value.String())
			if currentValue == "" {
				if err := createCmd.FlagSet.Set("product-type", string(cfg.ProductType)); err != nil {
					return err
				}
			} else {
				normalized, err := normalizePromotedPurchaseProductType(currentValue)
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err)
					return flag.ErrHelp
				}
				if normalized != cfg.ProductType {
					fmt.Fprintf(d.IO.ErrOut, "Error: --product-type is fixed to %s for this command\n", cfg.ProductType)
					return flag.ErrHelp
				}
				if err := createCmd.FlagSet.Set("product-type", string(cfg.ProductType)); err != nil {
					return err
				}
			}
		}

		return originalExec(ctx, args)
	}
}

func configDeps(d *shared.Deps) *shared.Deps {
	if d != nil {
		return d
	}
	return shared.DefaultDeps()
}

func findDirectSubcommand(cmd *ffcli.Command, name string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	for _, sub := range cmd.Subcommands {
		if sub != nil && sub.Name == name {
			return sub
		}
	}
	return nil
}
