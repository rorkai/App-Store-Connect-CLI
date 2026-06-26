package devices

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const defaultLocalUDIDCommandTimeout = 10 * time.Second

var errExistingDeviceFound = errors.New("existing device found")

var (
	localUDIDGOOS           = runtime.GOOS
	localUDIDCommandTimeout = defaultLocalUDIDCommandTimeout
	runLocalUDIDCommand     = func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, "ioreg", "-rd1", "-c", "IOPlatformExpertDevice").CombinedOutput()
	}
)

// DevicesCommand returns the devices command with subcommands.
func DevicesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("devices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "devices",
		ShortUsage: "asc devices <subcommand> [flags]",
		ShortHelp:  "Manage devices in App Store Connect.",
		LongHelp: `Manage devices in App Store Connect.

Examples:
  asc devices list
  asc devices get --id "DEVICE_ID"
  asc devices local-udid
  asc devices register --name "iPhone 15" --udid "UDID" --platform IOS
  asc devices update --id "DEVICE_ID" --status DISABLED`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			DevicesListCommand(),
			DevicesGetCommand(),
			DevicesLocalUDIDCommand(),
			DevicesRegisterCommand(),
			DevicesUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// DevicesListCommand returns the devices list subcommand.
func DevicesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	name := fs.String("name", "", "Filter by device name(s), comma-separated")
	platform := fs.String("platform", "", "Filter by platform(s), comma-separated: "+strings.Join(devicePlatformList(), ", "))
	status := fs.String("status", "", "Filter by status: ENABLED, DISABLED")
	udid := fs.String("udid", "", "Filter by UDID(s), comma-separated")
	ids := fs.String("id", "", "Filter by device ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by id, -id, name, -name, platform, -platform, status, -status, udid, -udid")
	fields := fs.String("fields", "", "Fields to include: addedDate, deviceClass, model, name, platform, status, udid")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc devices list [flags]",
		ShortHelp:  "List devices in App Store Connect.",
		LongHelp: `List devices in App Store Connect.

Examples:
  asc devices list
  asc devices list --platform IOS
  asc devices list --status ENABLED
  asc devices list --udid "UDID1,UDID2"
  asc devices list --fields "name,udid,platform,status"
  asc devices list --limit 50
  asc devices list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("devices list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("devices list: %w", err)
			}
			if err := shared.ValidateSort(*sort, "id", "-id", "name", "-name", "platform", "-platform", "status", "-status", "udid", "-udid"); err != nil {
				return fmt.Errorf("devices list: %w", err)
			}

			platformValues, err := normalizeDevicePlatforms(shared.SplitCSV(*platform))
			if err != nil {
				return fmt.Errorf("devices list: %w", err)
			}

			statusValue, err := normalizeDeviceStatus(*status)
			if err != nil {
				return fmt.Errorf("devices list: %w", err)
			}

			fieldsValue, err := normalizeDeviceFields(*fields)
			if err != nil {
				return fmt.Errorf("devices list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("devices list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.DevicesOption{
				asc.WithDevicesNames(shared.SplitCSV(*name)),
				asc.WithDevicesUDIDs(shared.SplitCSV(*udid)),
				asc.WithDevicesIDs(shared.SplitCSV(*ids)),
				asc.WithDevicesLimit(*limit),
				asc.WithDevicesNextURL(*next),
			}
			if len(platformValues) > 0 {
				opts = append(opts, asc.WithDevicesPlatforms(platformValues))
			}
			if statusValue != "" {
				opts = append(opts, asc.WithDevicesStatus(statusValue))
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithDevicesSort(*sort))
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithDevicesFields(fieldsValue))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithDevicesLimit(200))
				firstPage, err := client.GetDevices(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("devices list: failed to fetch: %w", err)
				}

				devices, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetDevices(ctx, asc.WithDevicesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("devices list: %w", err)
				}

				return shared.PrintOutput(devices, *output.Output, *output.Pretty)
			}

			devices, err := client.GetDevices(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("devices list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(devices, *output.Output, *output.Pretty)
		},
	}
}

// DevicesGetCommand returns the devices get subcommand.
func DevicesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Device ID")
	fields := fs.String("fields", "", "Fields to include: addedDate, deviceClass, model, name, platform, status, udid")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc devices get --id DEVICE_ID",
		ShortHelp:  "Get a device by ID.",
		LongHelp: `Get a device by ID.

Examples:
  asc devices get --id "DEVICE_ID"
  asc devices get --id "DEVICE_ID" --fields "name,udid,platform,status"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			fieldsValue, err := normalizeDeviceFields(*fields)
			if err != nil {
				return fmt.Errorf("devices get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("devices get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			device, err := client.GetDevice(requestCtx, idValue, fieldsValue)
			if err != nil {
				return fmt.Errorf("devices get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(device, *output.Output, *output.Pretty)
		},
	}
}

// DevicesLocalUDIDCommand returns the devices local-udid subcommand.
func DevicesLocalUDIDCommand() *ffcli.Command {
	fs := flag.NewFlagSet("local-udid", flag.ExitOnError)

	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "local-udid",
		ShortUsage: "asc devices local-udid [flags]",
		ShortHelp:  "Get the local macOS hardware UDID.",
		LongHelp: `Get the local macOS hardware UDID.

Examples:
  asc devices local-udid
  asc devices local-udid --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			localUDID, err := localMacUDID()
			if err != nil {
				return fmt.Errorf("devices local-udid: %w", err)
			}

			result := &asc.DeviceLocalUDIDResult{
				UDID:     localUDID,
				Platform: "MAC_OS",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// DevicesRegisterCommand returns the devices register subcommand.
func DevicesRegisterCommand() *ffcli.Command {
	fs := flag.NewFlagSet("register", flag.ExitOnError)

	name := fs.String("name", "", "Device name")
	udid := fs.String("udid", "", "Device UDID (required unless --udid-from-system)")
	udidFromSystem := fs.Bool("udid-from-system", false, "Use local macOS hardware UUID as UDID (macOS only)")
	platform := fs.String("platform", "", "Device platform: "+strings.Join(devicePlatformList(), ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "register",
		ShortUsage: "asc devices register --name NAME --udid UDID --platform " + strings.Join(devicePlatformList(), "|"),
		ShortHelp:  "Register a new device.",
		LongHelp: `Register a new device.

Examples:
  asc devices register --name "iPhone 15" --udid "UDID" --platform IOS
  asc devices register --name "My Mac" --udid-from-system --platform MAC_OS`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError()
			}

			udidValue := strings.TrimSpace(*udid)
			if *udidFromSystem && udidValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --udid and --udid-from-system are mutually exclusive")
				return flag.ErrHelp
			}
			if *udidFromSystem {
				localUDID, err := localMacUDID()
				if err != nil {
					return fmt.Errorf("devices register: %w", err)
				}
				udidValue = localUDID
			}
			if udidValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --udid is required")
				return shared.MissingRequiredUsageError()
			}

			platformValue := strings.TrimSpace(*platform)
			if *udidFromSystem && platformValue == "" {
				platformValue = "MAC_OS"
			}
			if platformValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --platform is required")
				return shared.MissingRequiredUsageError()
			}
			if *udidFromSystem && strings.ToUpper(platformValue) != "MAC_OS" {
				fmt.Fprintln(os.Stderr, "Error: --udid-from-system requires --platform MAC_OS")
				return flag.ErrHelp
			}

			platformValue, err := normalizeDevicePlatform(platformValue)
			if err != nil {
				return fmt.Errorf("devices register: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("devices register: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			existingDevice, err := findExistingDeviceByNormalizedUDID(requestCtx, client, udidValue, platformValue)
			if err != nil {
				return fmt.Errorf("devices register: failed to check existing devices: %w", err)
			}
			if existingDevice != nil {
				return shared.PrintOutput(existingDevice, *output.Output, *output.Pretty)
			}

			attrs := asc.DeviceCreateAttributes{
				Name:     nameValue,
				UDID:     udidValue,
				Platform: asc.DevicePlatform(platformValue),
			}

			device, err := client.CreateDevice(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("devices register: failed to register: %w", err)
			}

			return shared.PrintOutput(device, *output.Output, *output.Pretty)
		},
	}
}

func findExistingDeviceByNormalizedUDID(ctx context.Context, client *asc.Client, udidValue, platformValue string) (*asc.DeviceResponse, error) {
	targetUDID := normalizeDeviceUDIDForComparison(udidValue)
	if targetUDID == "" {
		return nil, nil
	}

	firstPage, err := client.GetDevices(
		ctx,
		asc.WithDevicesPlatforms([]string{platformValue}),
		asc.WithDevicesFields([]string{"name", "udid", "platform", "status"}),
		asc.WithDevicesLimit(200),
	)
	if err != nil {
		return nil, err
	}

	var found *asc.DeviceResponse
	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetDevices(ctx, asc.WithDevicesNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		devices, ok := page.(*asc.DevicesResponse)
		if !ok || devices == nil {
			return nil
		}
		for _, device := range devices.Data {
			if normalizeDeviceUDIDForComparison(device.Attributes.UDID) == targetUDID {
				found = &asc.DeviceResponse{Data: device}
				return errExistingDeviceFound
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errExistingDeviceFound) {
		return nil, err
	}
	return found, nil
}

func normalizeDeviceUDIDForComparison(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, ":", "")
	return strings.ToUpper(value)
}

// DevicesUpdateCommand returns the devices update subcommand.
func DevicesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Device ID")
	name := fs.String("name", "", "Device name")
	status := fs.String("status", "", "Device status: ENABLED, DISABLED")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc devices update --id DEVICE_ID [--name NAME] [--status ENABLED|DISABLED]",
		ShortHelp:  "Update a device.",
		LongHelp: `Update a device by ID.

Examples:
  asc devices update --id "DEVICE_ID" --name "My iPhone"
  asc devices update --id "DEVICE_ID" --status DISABLED`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			nameValue := strings.TrimSpace(*name)
			statusRaw := strings.TrimSpace(*status)
			if nameValue == "" && statusRaw == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			statusValue, err := normalizeDeviceStatus(statusRaw)
			if err != nil {
				return fmt.Errorf("devices update: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("devices update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.DeviceUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if statusValue != "" {
				statusEnum := asc.DeviceStatus(statusValue)
				attrs.Status = &statusEnum
			}

			device, err := client.UpdateDevice(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("devices update: failed to update: %w", err)
			}

			return shared.PrintOutput(device, *output.Output, *output.Pretty)
		},
	}
}

func normalizeDevicePlatform(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	normalized := strings.ToUpper(trimmed)
	if slices.Contains(devicePlatformList(), normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--platform must be one of: %s", strings.Join(devicePlatformList(), ", "))
}

func normalizeDevicePlatforms(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		platform, err := normalizeDevicePlatform(value)
		if err != nil {
			return nil, err
		}
		if platform != "" {
			normalized = append(normalized, platform)
		}
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func normalizeDeviceStatus(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	normalized := strings.ToUpper(trimmed)
	if slices.Contains(deviceStatusList(), normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--status must be one of: %s", strings.Join(deviceStatusList(), ", "))
}

func normalizeDeviceFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range deviceFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(deviceFieldsList(), ", "))
		}
	}

	return fields, nil
}

func devicePlatformList() []string {
	return []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS"}
}

func deviceStatusList() []string {
	return []string{"ENABLED", "DISABLED"}
}

func deviceFieldsList() []string {
	return []string{"addedDate", "deviceClass", "model", "name", "platform", "status", "udid"}
}

func localMacUDID() (string, error) {
	if localUDIDGOOS != "darwin" {
		return "", fmt.Errorf("--udid-from-system is only supported on macOS")
	}

	ctx, cancel := context.WithTimeout(context.Background(), localUDIDCommandTimeout)
	defer cancel()

	output, err := runLocalUDIDCommand(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read local hardware UUID: %w", err)
	}

	for line := range strings.SplitSeq(string(output), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "\"IOPlatformUUID\" = "); ok {
			value := after
			value = strings.Trim(value, "\"")
			if value != "" {
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("unable to locate IOPlatformUUID in ioreg output")
}
