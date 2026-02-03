package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/doctor"
)

// AuthDoctorCommand is an offline diagnostic for auth/config.
func AuthDoctorCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth doctor", flag.ExitOnError)

	profileFlag := fs.String("profile", "", "Profile name to check")
	local := fs.Bool("local", false, "Use repo-local ./.asc/config.json")
	jsonOut := fs.Bool("json", false, "Output machine-readable JSON")

	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc auth doctor [flags]",
		ShortHelp:  "Run offline auth/config diagnostics.",
		LongHelp: `Run offline auth/config diagnostics.

This command does not contact Apple. It validates config.json structure and checks
whether the selected credentials appear complete (key_id, issuer_id, private_key_path).

Examples:
  asc auth doctor
  asc auth doctor --local
  asc auth doctor --profile "Client"
  asc auth doctor --json`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx

			var cfgPath string
			var cfgLoadErr error
			var cfg *config.Config

			if *local {
				cfgPath, cfgLoadErr = config.LocalPath()
			} else {
				cfgPath, cfgLoadErr = config.Path()
			}

			if cfgLoadErr == nil && cfgPath != "" {
				if _, err := os.Stat(cfgPath); err != nil {
					cfgLoadErr = err
				} else {
					cfg, cfgLoadErr = config.LoadAt(cfgPath)
				}
			}

			profile := doctor.ResolveProfile(*profileFlag, "")
			if cfg != nil {
				profile = doctor.ResolveProfile(*profileFlag, cfg.DefaultKeyName)
			}

			report := doctor.BuildReport(cfgPath, cfg, cfgLoadErr, profile)

			if *jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetEscapeHTML(false)
				if err := enc.Encode(report); err != nil {
					return fmt.Errorf("auth doctor: failed to encode json: %w", err)
				}
			} else {
				for _, check := range report.Checks {
					prefix := "FAIL"
					if check.OK {
						prefix = "OK"
					}
					fmt.Printf("%s\t%s\t%s\n", prefix, check.Name, check.Message)
				}
				if report.OK {
					fmt.Println("OK\tauth.doctor\tall checks passed")
				} else {
					fmt.Println("FAIL\tauth.doctor\tone or more checks failed")
				}
			}

			if report.OK {
				return nil
			}
			return fmt.Errorf("auth doctor failed")
		},
	}
}
