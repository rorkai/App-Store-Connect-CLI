package shots

import (
	"context"
	"flag"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

type frameDevicesOutput struct {
	Default string                          `json:"default"`
	Devices []screenshots.FrameDeviceOption `json:"devices"`
}

// NewShotsFramesListDevicesCommand returns the screenshots list-frame-devices subcommand.
func NewShotsFramesListDevicesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list-frame-devices", flag.ExitOnError)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list-frame-devices",
		ShortUsage: "asc screenshots list-frame-devices [--output json]",
		ShortHelp:  "List supported frame devices and the default.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result := frameDevicesOutput{
				Default: string(screenshots.DefaultFrameDevice()),
				Devices: screenshots.FrameDeviceOptions(),
			}
			rows := make([][]string, 0, len(result.Devices))
			for _, device := range result.Devices {
				rows = append(rows, []string{device.ID, device.Family, strconv.FormatBool(device.Default), strings.Join(device.FrameColors, ", ")})
			}
			return d.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"ID", "Family", "Default", "Frame Colors"}, rows)
		},
	}
}
