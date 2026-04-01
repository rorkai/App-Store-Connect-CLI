package ascbin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ResolveOptions struct {
	BundledPath    string
	SystemOverride string
	PreferBundled  bool
	LookPath       func(string) (string, error)
}

type Resolution struct {
	Path            string   `json:"path"`
	Source          string   `json:"source"`
	Checked         []string `json:"checked"`
	BundledEligible bool     `json:"bundledEligible"`
}

func Resolve(opts ResolveOptions) (Resolution, error) {
	var checked []string
	if override := strings.TrimSpace(opts.SystemOverride); override != "" {
		checked = append(checked, override)
		if statFile(override) == nil {
			return Resolution{
				Path:            override,
				Source:          "system-override",
				Checked:         checked,
				BundledEligible: fileExists(opts.BundledPath),
			}, nil
		}
		return Resolution{}, fmt.Errorf("system override not found: %s", override)
	}

	bundled := strings.TrimSpace(opts.BundledPath)
	if opts.PreferBundled && bundled != "" {
		checked = append(checked, bundled)
		if statFile(bundled) == nil {
			return Resolution{
				Path:            bundled,
				Source:          "bundled",
				Checked:         checked,
				BundledEligible: true,
			}, nil
		}
	}

	if opts.LookPath != nil {
		system, err := opts.LookPath("asc")
		if err == nil {
			checked = append(checked, system)
			return Resolution{
				Path:            system,
				Source:          "path",
				Checked:         checked,
				BundledEligible: fileExists(bundled),
			}, nil
		}
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, execErrNotFound) {
			return Resolution{}, err
		}
	}

	if !opts.PreferBundled && bundled != "" {
		checked = append(checked, bundled)
		if statFile(bundled) == nil {
			return Resolution{
				Path:            bundled,
				Source:          "bundled-fallback",
				Checked:         checked,
				BundledEligible: true,
			}, nil
		}
	}

	return Resolution{}, fmt.Errorf("could not resolve asc binary from %v", checked)
}

var execErrNotFound = errors.New("not found")

var statFile = func(path string) error {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}

func fileExists(path string) bool {
	return path != "" && statFile(path) == nil
}
