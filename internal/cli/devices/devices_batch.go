package devices

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const maxDeviceBatchFileSize = 2 * 1024 * 1024

type deviceBatchRecord struct {
	Row      int
	UDID     string
	Name     string
	Platform string
}

// newDevicesRegisterBatchCommand returns the devices register-batch subcommand.
func newDevicesRegisterBatchCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("register-batch", flag.ExitOnError)

	filePath := fs.String("file", "", "Tab-separated device file (UDID, name, optional platform)")
	platform := fs.String("platform", "", "Default platform when a row omits it: "+strings.Join(devicePlatformList(), ", "))
	dryRun := fs.Bool("dry-run", false, "Validate and show the registration plan without creating devices")
	confirm := fs.Bool("confirm", false, "Confirm device registration mutations")
	continueOnError := fs.Bool("continue-on-error", true, "Continue registering after an API failure")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "register-batch",
		ShortUsage: "asc devices register-batch --file FILE [--platform PLATFORM] [flags]",
		ShortHelp:  "Register devices from a tab-separated file.",
		LongHelp: `Register devices from a tab-separated file.

Accepted rows contain a UDID, device name, and optional platform, separated by
tabs. A header using Device ID, Device Name, and Device Platform is optional.
Blank rows and rows whose first field begins with # are ignored. Use --platform
as the default for rows without a platform.

The entire file is validated before network access. Existing devices and
repeated UDIDs are skipped after removing hyphens and colons for comparison.

Examples:
  asc devices register-batch --file "./devices.txt" --confirm
  asc devices register-batch --file "./devices.txt" --platform IOS --confirm
  asc devices register-batch --file "./devices.txt" --dry-run
  asc devices register-batch --file "./devices.txt" --continue-on-error=false --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				return d.RequiredFlag("--file")
			}

			defaultPlatform, err := normalizeDevicePlatform(*platform)
			if err != nil {
				return fmt.Errorf("devices register-batch: %w", d.UsageError(err.Error()))
			}
			if err := shared.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			records, err := readDeviceBatchTSV(d, fileValue, defaultPlatform)
			if err != nil {
				return fmt.Errorf("devices register-batch: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("devices register-batch: %w", err)
			}

			existing, err := fetchDevicesByNormalizedUDID(ctx, d, client)
			if err != nil {
				return fmt.Errorf("devices register-batch: failed to check existing devices: %w", err)
			}

			summary := &asc.DeviceBatchRegistrationSummary{
				InputFile:       filepath.Clean(fileValue),
				DryRun:          *dryRun,
				ContinueOnError: *continueOnError,
				Total:           len(records),
				Results:         make([]asc.DeviceBatchRegistrationItem, 0, len(records)),
			}
			seenInput := make(map[string]int, len(records))

			var refused error
			for _, record := range records {
				normalizedUDID := normalizeDeviceUDIDForComparison(record.UDID)
				result := asc.DeviceBatchRegistrationItem{
					Row:      record.Row,
					Name:     record.Name,
					UDID:     record.UDID,
					Platform: record.Platform,
				}
				summary.Processed++

				if firstRow, duplicate := seenInput[normalizedUDID]; duplicate {
					result.Status = "skipped"
					result.Reason = fmt.Sprintf("duplicate UDID from line %d", firstRow)
					summary.Skipped++
					summary.Results = append(summary.Results, result)
					continue
				}
				seenInput[normalizedUDID] = record.Row

				if device, found := existing[normalizedUDID]; found {
					result.ID = device.ID
					result.Status = "skipped"
					result.Reason = "already registered"
					summary.Skipped++
					summary.Results = append(summary.Results, result)
					continue
				}

				if *dryRun {
					result.Status = "planned"
					summary.Planned++
					summary.Results = append(summary.Results, result)
					continue
				}

				requestCtx, cancel := d.Timeout(ctx)
				created, createErr := client.CreateDevice(requestCtx, asc.DeviceCreateAttributes{
					Name:     record.Name,
					UDID:     record.UDID,
					Platform: asc.DevicePlatform(record.Platform),
				})
				cancel()
				if createErr == nil && (created == nil || strings.TrimSpace(created.Data.ID) == "") {
					createErr = fmt.Errorf("registration returned an empty device ID")
				}
				if createErr != nil {
					result.Status = "failed"
					result.Error = createErr.Error()
					refused = shared.KeepReadOnlyRefusal(refused, createErr)
					summary.Failed++
					summary.Results = append(summary.Results, result)
					if !*continueOnError || ctx.Err() != nil {
						break
					}
					continue
				}

				result.ID = created.Data.ID
				result.Status = "registered"
				summary.Registered++
				summary.Results = append(summary.Results, result)
				existing[normalizedUDID] = created.Data
			}

			if err := d.PrintOutput(summary, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if summary.Failed > 0 {
				return shared.NewReportedError(shared.NewErrorWithCause(fmt.Errorf("devices register-batch: %d registration(s) failed", summary.Failed), refused))
			}
			return nil
		},
	}
}

func readDeviceBatchTSV(d *shared.Deps, path, defaultPlatform string) ([]deviceBatchRecord, error) {
	defaultPlatform, err := normalizeDevicePlatform(defaultPlatform)
	if err != nil {
		return nil, d.UsageError(err.Error())
	}

	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open --file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat --file: %w", err)
	}
	if info.Size() > maxDeviceBatchFileSize {
		return nil, fmt.Errorf("device file exceeds %d-byte limit", maxDeviceBatchFileSize)
	}
	return readDeviceBatchTSVFromReader(d, file, defaultPlatform)
}

func readDeviceBatchTSVFromReader(d *shared.Deps, input io.Reader, defaultPlatform string) ([]deviceBatchRecord, error) {
	limited := &io.LimitedReader{R: input, N: maxDeviceBatchFileSize + 1}
	reader := csv.NewReader(limited)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1

	records := make([]deviceBatchRecord, 0)
	firstRecord := true
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read tab-separated device file: %w", readErr)
		}
		line := 1
		if len(row) > 0 {
			line, _ = reader.FieldPos(0)
			row[0] = strings.TrimPrefix(row[0], "\ufeff")
		}
		if isIgnoredDeviceBatchRow(row) {
			continue
		}

		if firstRecord {
			isHeader, headerErr := parseDeviceBatchHeader(row, line)
			if headerErr != nil {
				return nil, headerErr
			}
			firstRecord = false
			if isHeader {
				continue
			}
		}

		record, err := parseDeviceBatchRow(d, row, line, defaultPlatform)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if limited.N == 0 {
		return nil, fmt.Errorf("device file exceeds %d-byte limit", maxDeviceBatchFileSize)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("device file contains no records")
	}
	return records, nil
}

func isIgnoredDeviceBatchRow(row []string) bool {
	allEmpty := true
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return true
	}
	return len(row) > 0 && strings.HasPrefix(strings.TrimSpace(row[0]), "#")
}

func parseDeviceBatchHeader(row []string, line int) (bool, error) {
	if len(row) == 0 {
		return false, nil
	}
	first := normalizeDeviceBatchHeader(row[0])
	if first != "deviceid" && first != "udid" {
		return false, nil
	}
	if len(row) < 2 || (normalizeDeviceBatchHeader(row[1]) != "devicename" && normalizeDeviceBatchHeader(row[1]) != "name") {
		return false, fmt.Errorf("line %d: device header must start with Device ID and Device Name", line)
	}
	if len(row) > 3 {
		return false, fmt.Errorf("line %d: device header must contain 2 or 3 tab-separated columns", line)
	}
	if len(row) == 3 {
		third := normalizeDeviceBatchHeader(row[2])
		if third != "deviceplatform" && third != "platform" {
			return false, fmt.Errorf("line %d: third header column must be Device Platform", line)
		}
	}
	return true, nil
}

func normalizeDeviceBatchHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "")
	return replacer.Replace(value)
}

func parseDeviceBatchRow(d *shared.Deps, row []string, line int, defaultPlatform string) (deviceBatchRecord, error) {
	if len(row) < 2 || len(row) > 3 {
		return deviceBatchRecord{}, fmt.Errorf("line %d: expected 2 or 3 tab-separated columns", line)
	}
	udid := strings.TrimSpace(row[0])
	if udid == "" || normalizeDeviceUDIDForComparison(udid) == "" {
		return deviceBatchRecord{}, fmt.Errorf("line %d: device UDID is required", line)
	}
	name := strings.TrimSpace(row[1])
	if name == "" {
		return deviceBatchRecord{}, fmt.Errorf("line %d: device name is required", line)
	}

	platform := defaultPlatform
	if len(row) == 3 && strings.TrimSpace(row[2]) != "" {
		platform = strings.TrimSpace(row[2])
	}
	if platform == "" {
		return deviceBatchRecord{}, fmt.Errorf("line %d: device platform is required (provide a third column or --platform)", line)
	}
	platform, err := normalizeDevicePlatform(platform)
	if err != nil {
		return deviceBatchRecord{}, fmt.Errorf("line %d: %w", line, d.UsageError(err.Error()))
	}

	return deviceBatchRecord{Row: line, UDID: udid, Name: name, Platform: platform}, nil
}

func fetchDevicesByNormalizedUDID(ctx context.Context, d *shared.Deps, client *asc.Client) (map[string]asc.Resource[asc.DeviceAttributes], error) {
	requestCtx, cancel := d.Timeout(ctx)
	firstPage, err := client.GetDevices(
		requestCtx,
		asc.WithDevicesFields([]string{"name", "udid", "platform", "status"}),
		asc.Limit[asc.DevicesOption](200),
	)
	cancel()
	if err != nil {
		return nil, err
	}

	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		return client.GetDevices(requestCtx, asc.NextURL[asc.DevicesOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	devices, ok := all.(*asc.DevicesResponse)
	if !ok || devices == nil {
		return nil, fmt.Errorf("unexpected devices response type")
	}

	byUDID := make(map[string]asc.Resource[asc.DeviceAttributes], len(devices.Data))
	for _, device := range devices.Data {
		normalized := normalizeDeviceUDIDForComparison(device.Attributes.UDID)
		if normalized != "" {
			byUDID[normalized] = device
		}
	}
	return byUDID, nil
}
