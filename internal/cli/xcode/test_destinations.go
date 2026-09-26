package xcode

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var loadSimulatorListJSON = func(ctx context.Context) ([]byte, error) {
	if xcodeCommandGOOS != "darwin" {
		return nil, fmt.Errorf("xcode test-destinations is supported on macOS only; current platform is %s", xcodeCommandGOOS)
	}
	return localxcode.SimctlListJSON(ctx)
}

// SetSimulatorListLoaderForTesting replaces the simctl list reader.
func SetSimulatorListLoaderForTesting(fn func(context.Context) ([]byte, error)) func() {
	previous := loadSimulatorListJSON
	if fn == nil {
		loadSimulatorListJSON = previous
	} else {
		loadSimulatorListJSON = fn
	}
	return func() { loadSimulatorListJSON = previous }
}

// XcodeTestDestinationsCommand lists destinations that can be pasted into xcode test --destination.
func XcodeTestDestinationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode test-destinations", flag.ExitOnError)
	platform := fs.String("platform", "", "Platform: iOS, watchOS, tvOS, visionOS, or macOS")
	availableOnly := fs.Bool("available-only", false, "Omit simulators marked unavailable")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "test-destinations",
		ShortUsage: "asc xcode test-destinations [--platform PLATFORM] [--available-only] [flags]",
		ShortHelp:  "List simulator and Mac destinations for xcode test.",
		LongHelp: `List destinations that can be pasted into asc xcode test --destination.

This is a read-only simctl list. It never boots, creates, erases, or deletes a simulator.
macOS is the local host destination and does not require simctl. Create, boot, and delete
remain outside this command. The simctl read uses the configured ASC timeout,
defaulting to 30 seconds.

Examples:
  asc xcode test-destinations --output json
  asc xcode test-destinations --platform iOS --available-only`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("xcode test-destinations does not accept positional arguments")
			}
			normalized, err := normalizeDestinationPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			readCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			result, err := listTestDestinations(readCtx, normalized, *availableOnly)
			if err != nil {
				return fmt.Errorf("xcode test-destinations: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func listTestDestinations(ctx context.Context, platform string, availableOnly bool) (*asc.XcodeTestDestinationsResult, error) {
	if platform == "macOS" && xcodeCommandGOOS != "darwin" {
		return nil, fmt.Errorf("xcode test-destinations is supported on macOS only; current platform is %s", xcodeCommandGOOS)
	}
	result := &asc.XcodeTestDestinationsResult{Destinations: []asc.XcodeTestDestination{}}
	if platform == "" || platform != "macOS" {
		body, err := loadSimulatorListJSON(ctx)
		if err != nil {
			return nil, err
		}
		parsed, err := parseSimctlList(body, platform, availableOnly)
		if err != nil {
			return nil, err
		}
		result.Destinations = append(result.Destinations, parsed...)
	}
	if platform == "" || platform == "macOS" {
		result.Destinations = append(result.Destinations, asc.XcodeTestDestination{
			Name:              "My Mac",
			Runtime:           "macOS",
			State:             "available",
			DestinationString: "platform=macOS",
			Available:         true,
		})
	}
	sort.SliceStable(result.Destinations, func(i, j int) bool {
		if result.Destinations[i].Runtime != result.Destinations[j].Runtime {
			return result.Destinations[i].Runtime < result.Destinations[j].Runtime
		}
		if result.Destinations[i].Name != result.Destinations[j].Name {
			return result.Destinations[i].Name < result.Destinations[j].Name
		}
		return result.Destinations[i].UDID < result.Destinations[j].UDID
	})
	return result, nil
}

func normalizeDestinationPlatform(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	switch strings.ToLower(trimmed) {
	case "ios":
		return "iOS", nil
	case "watchos":
		return "watchOS", nil
	case "tvos":
		return "tvOS", nil
	case "visionos", "xros":
		return "visionOS", nil
	case "macos":
		return "macOS", nil
	default:
		return "", fmt.Errorf("xcode test-destinations: --platform must be one of iOS, watchOS, tvOS, visionOS, macOS (got %q)", trimmed)
	}
}

type simctlListDocument struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

type simctlDevice struct {
	UDID        string `json:"udid"`
	Name        string `json:"name"`
	State       string `json:"state"`
	IsAvailable *bool  `json:"isAvailable"`
}

func parseSimctlList(body []byte, platform string, availableOnly bool) ([]asc.XcodeTestDestination, error) {
	var document simctlListDocument
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("parse simctl list: %w", err)
	}
	if document.Devices == nil {
		return nil, fmt.Errorf("parse simctl list: devices is missing")
	}
	out := make([]asc.XcodeTestDestination, 0)
	for runtimeName, devices := range document.Devices {
		runtimePlatform, runtimeVersion, ok := simulatorRuntime(runtimeName)
		if !ok {
			continue
		}
		if platform != "" && !strings.EqualFold(runtimePlatform, platform) {
			continue
		}
		for _, device := range devices {
			available := device.IsAvailable == nil || *device.IsAvailable
			if availableOnly && !available {
				continue
			}
			udid := strings.TrimSpace(device.UDID)
			name := strings.TrimSpace(device.Name)
			if udid == "" || name == "" {
				continue
			}
			out = append(out, asc.XcodeTestDestination{
				Name:              name,
				UDID:              udid,
				Runtime:           runtimeVersion,
				State:             strings.TrimSpace(device.State),
				DestinationString: fmt.Sprintf("platform=%s Simulator,id=%s", runtimePlatform, udid),
				Available:         available,
			})
		}
	}
	return out, nil
}

func simulatorRuntime(identifier string) (platform, version string, ok bool) {
	const marker = "SimRuntime."
	index := strings.LastIndex(identifier, marker)
	if index < 0 {
		return "", "", false
	}
	rest := identifier[index+len(marker):]
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	platform = parts[0]
	switch strings.ToLower(platform) {
	case "ios":
		platform = "iOS"
	case "watchos":
		platform = "watchOS"
	case "tvos":
		platform = "tvOS"
	case "visionos", "xros":
		platform = "visionOS"
	default:
		return "", "", false
	}
	return platform, strings.ReplaceAll(parts[1], "-", "."), true
}
