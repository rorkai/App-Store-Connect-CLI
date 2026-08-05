package telemetry

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	internalWorkerArg      = "--asc-internal-telemetry-worker"
	internalWorkerEnvVar   = "ASC_INTERNAL_TELEMETRY_WORKER"
	workerResourceName     = "telemetry-worker"
	workerDiagnosticName   = "telemetry-worker.log"
	maintenanceWorkerBatch = 16
	workerLockHandoff      = 100 * time.Millisecond
)

type eventSender func(Event, string) error

var startMaintenanceWorker = startDetachedMaintenanceWorker

func RunMaintenanceWorkerIfRequested(args []string) bool {
	if !isMaintenanceWorkerInvocation(args) {
		return false
	}
	if err := runDefaultMaintenanceWorker(); err != nil {
		debugf("telemetry worker failed: %v", err)
	}
	return true
}

func isMaintenanceWorkerInvocation(args []string) bool {
	return len(args) == 1 && args[0] == internalWorkerArg && os.Getenv(internalWorkerEnvVar) == "1"
}

func runDefaultMaintenanceWorker() error {
	store, err := defaultSpoolStore()
	if err != nil {
		return err
	}
	workerPath := filepath.Join(filepath.Dir(store.path), workerResourceName)
	return runMaintenanceWorker(workerPath, store, sendHTTPEventToEndpoint)
}

func runMaintenanceWorker(workerPath string, store spoolStore, deliver eventSender) error {
	unlock, err := lockTelemetryFile(workerPath, workerLockHandoff, "worker")
	if errors.Is(err, errTelemetryLockBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unlock()

	return flushSpoolWhileEnabled(store, deliver, func() (bool, error) {
		return telemetryDeliveryAllowed(store)
	})
}

func flushSpool(store spoolStore, deliver eventSender) error {
	return flushSpoolWhileEnabled(store, deliver, func() (bool, error) { return true, nil })
}

func flushSpoolWhileEnabled(
	store spoolStore,
	deliver eventSender,
	deliveryAllowed func() (bool, error),
) error {
	for {
		records, err := store.snapshot(maintenanceWorkerBatch)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}

		delivered := make(map[string]struct{}, len(records))
		hadFailure := false
		for _, record := range records {
			allowed, err := deliveryAllowed()
			if err != nil {
				return err
			}
			if !allowed {
				return nil
			}
			if err := deliver(record.Event, record.Endpoint); err != nil {
				if isPermanentDeliveryError(err) {
					delivered[record.Event.EventID] = struct{}{}
					debugf("telemetry event permanently rejected: %v", err)
					continue
				}
				hadFailure = true
				debugf("telemetry send failed: %v", err)
				continue
			}
			delivered[record.Event.EventID] = struct{}{}
		}
		if err := store.removeDelivered(delivered); err != nil {
			return err
		}
		if hadFailure {
			return nil
		}
	}
}

func telemetryDeliveryAllowed(store spoolStore) (bool, error) {
	if reason := environmentOptOutReason(); reason != "" {
		purgeSpoolAfterOptOut(store, reason)
		return false, nil
	}
	state, err := loadCurrentState()
	if err != nil {
		debugf("telemetry worker disabled: %v", err)
		return false, nil
	}
	if enabled, reason := enabledFromState(state); !enabled {
		purgeSpoolAfterOptOut(store, reason)
		return false, nil
	}
	return true, nil
}

func purgeSpoolAfterOptOut(store spoolStore, reason string) {
	if err := store.purge(); err != nil {
		debugf("telemetry spool purge failed after %s: %v", reason, err)
		return
	}
	debugf("telemetry spool purged after %s", reason)
}

func startDetachedMaintenanceWorker() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	command := exec.Command(executable, internalWorkerArg)
	command.Env = setEnvironmentValue(os.Environ(), internalWorkerEnvVar, "1")
	configureDetachedProcess(command)

	var diagnostic *os.File
	if debugEnabled() {
		store, pathErr := defaultSpoolStore()
		if pathErr != nil {
			debugf("telemetry worker diagnostics unavailable: %v", pathErr)
		} else if err := ensureSecureTelemetryDirectory(filepath.Dir(store.path)); err != nil {
			debugf("telemetry worker diagnostics unavailable: %v", err)
		} else {
			diagnosticPath := filepath.Join(filepath.Dir(store.path), workerDiagnosticName)
			diagnostic, err = os.OpenFile(diagnosticPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				debugf("telemetry worker diagnostics unavailable: %v", err)
			} else if err := diagnostic.Chmod(0o600); err != nil {
				_ = diagnostic.Close()
				diagnostic = nil
				debugf("telemetry worker diagnostics unavailable: %v", err)
			} else {
				command.Stderr = diagnostic
				debugf("telemetry worker diagnostics: %s", diagnosticPath)
			}
		}
	}

	if err := command.Start(); err != nil {
		if diagnostic != nil {
			_ = diagnostic.Close()
		}
		return fmt.Errorf("start worker: %w", err)
	}
	if diagnostic != nil {
		_ = diagnostic.Close()
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release worker: %w", err)
	}
	return nil
}

func setEnvironmentValue(environment []string, key, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != key {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}
