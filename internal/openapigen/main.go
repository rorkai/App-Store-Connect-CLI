// Command openapigen generates parts of the App Store Connect client in
// internal/asc from the OpenAPI spec and a bindings file. See
// docs/openapi/README.md.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	specPath := flag.String("spec", "../../docs/openapi/latest.json", "OpenAPI spec")
	bindingsPath := flag.String("bindings", "openapi_bindings.json", "bindings file")
	dir := flag.String("dir", ".", "client package directory")
	check := flag.Bool("check", false, "fail if generated files are out of date instead of writing them")
	adopt := flag.String("adopt", "", "comma-separated tags (or \"all\") whose hand-written methods and types to replace")
	dryRun := flag.Bool("dry-run", false, "with -adopt, only report what would be generated")
	flag.Parse()
	if err := run(*specPath, *bindingsPath, *dir, *check, *adopt, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "openapigen:", err)
		os.Exit(1)
	}
}

func run(specPath, bindingsPath, dir string, check bool, adopt string, dryRun bool) error {
	spec, err := LoadSpec(specPath)
	if err != nil {
		return err
	}
	bindings, err := LoadBindings(bindingsPath)
	if os.IsNotExist(err) {
		bindings, err = &Bindings{}, nil
	}
	if err != nil {
		return err
	}
	if adopt != "" {
		if check {
			return fmt.Errorf("-adopt and -check cannot be combined")
		}
		tags := strings.Split(adopt, ",")
		if adopt == "all" {
			tags = spec.Tags()
		}
		if err := Adopt(spec, bindings, dir, tags, dryRun, os.Stdout); err != nil || dryRun {
			return err
		}
		data, err := bindings.Encode()
		if err != nil {
			return err
		}
		if err := os.WriteFile(bindingsPath, data, 0o644); err != nil {
			return err
		}
	}
	return writeAll(spec, bindings, dir, check)
}

// writeAll generates every family and deletes generated files no family
// produces. With check it changes nothing and reports what is out of date.
func writeAll(spec *Spec, bindings *Bindings, dir string, check bool) error {
	want := map[string][]byte{}
	for _, fam := range bindings.Families {
		src, err := Generate(spec, fam)
		if err != nil {
			return err
		}
		want[filepath.Join(dir, fam.File)] = src
	}
	existing, err := filepath.Glob(filepath.Join(dir, "*_gen.go"))
	if err != nil {
		return err
	}
	var stale []string
	for _, path := range existing {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, ok := want[path]; !ok && bytes.HasPrefix(data, []byte(header)) {
			stale = append(stale, path)
		}
	}
	var outdated []string
	for path, src := range want {
		if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, src) {
			outdated = append(outdated, path)
		}
	}
	sort.Strings(outdated)
	if check {
		if len(stale)+len(outdated) > 0 {
			return fmt.Errorf("generated client files are out of date (run make generate-client): %s", strings.Join(append(outdated, stale...), ", "))
		}
		return nil
	}
	for _, path := range outdated {
		if err := os.WriteFile(path, want[path], 0o644); err != nil {
			return err
		}
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
