// Command openapigen generates parts of the App Store Connect client in
// internal/asc from the OpenAPI spec and the bindings directory. See
// docs/openapi/README.md.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	specPath := flag.String("spec", "../../docs/openapi/latest.json", "OpenAPI spec")
	bindingsDir := flag.String("bindings", "openapi_bindings", "directory with one bindings file per family")
	dir := flag.String("dir", ".", "client package directory")
	check := flag.Bool("check", false, "fail if generated files are out of date instead of writing them")
	adopt := flag.String("adopt", "", "comma-separated tags (or \"all\") whose hand-written methods and types to replace")
	dryRun := flag.Bool("dry-run", false, "with -adopt, only report what would be generated")
	flag.Parse()
	if err := run(*specPath, *bindingsDir, *dir, *check, *adopt, *dryRun, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "openapigen:", err)
		os.Exit(1)
	}
}

func run(specPath, bindingsDir, dir string, check bool, adopt string, dryRun bool, report io.Writer) error {
	spec, err := LoadSpec(specPath)
	if err != nil {
		return err
	}
	bindings, err := LoadBindings(bindingsDir)
	if err != nil {
		return err
	}
	if adopt == "" {
		return writeAll(spec, bindings, dir, check)
	}
	if check {
		return fmt.Errorf("-adopt and -check cannot be combined")
	}
	tags := strings.Split(adopt, ",")
	if adopt == "all" {
		tags = spec.Tags()
	}
	edits, adopted, err := Adopt(spec, bindings, dir, tags, report)
	if err != nil || dryRun {
		return err
	}
	// Render everything before touching a file, so a failure leaves the
	// hand-written code in place.
	writes, stale, err := plan(spec, bindings, dir)
	if err != nil {
		return err
	}
	for _, fam := range adopted {
		data, err := fam.Encode()
		if err != nil {
			return err
		}
		writes[bindingsFile(bindingsDir, fam.Tag)] = data
	}
	if len(adopted) > 0 {
		if err := os.MkdirAll(bindingsDir, 0o755); err != nil {
			return err
		}
	}
	for path, src := range edits {
		if src == nil {
			stale = append(stale, path)
		} else {
			writes[path] = src
		}
	}
	return apply(writes, stale)
}

// writeAll generates every family and deletes generated files no family
// produces. With check it changes nothing and reports what is out of date.
func writeAll(spec *Spec, bindings *Bindings, dir string, check bool) error {
	writes, stale, err := plan(spec, bindings, dir)
	if err != nil {
		return err
	}
	if !check {
		return apply(writes, stale)
	}
	if len(writes)+len(stale) > 0 {
		var paths []string
		for path := range writes {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		return fmt.Errorf("generated client files are out of date (run make generate-client): %s", strings.Join(append(paths, stale...), ", "))
	}
	return nil
}

// plan renders every family and returns the generated files that differ from
// disk and the generated files no family produces any more.
func plan(spec *Spec, bindings *Bindings, dir string) (map[string][]byte, []string, error) {
	writes := map[string][]byte{}
	for _, fam := range bindings.Families {
		src, err := Generate(spec, fam)
		if err != nil {
			return nil, nil, err
		}
		path := filepath.Join(dir, fam.File)
		if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, src) {
			writes[path] = src
		}
	}
	existing, err := filepath.Glob(filepath.Join(dir, "*_gen.go"))
	if err != nil {
		return nil, nil, err
	}
	var stale []string
	for _, path := range existing {
		if bindings.produces(dir, path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		if bytes.HasPrefix(data, []byte(header)) {
			stale = append(stale, path)
		}
	}
	return writes, stale, nil
}

func apply(writes map[string][]byte, removes []string) error {
	for path, src := range writes {
		if err := os.WriteFile(path, src, 0o644); err != nil {
			return err
		}
	}
	for _, path := range removes {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
