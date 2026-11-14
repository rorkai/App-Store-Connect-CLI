// Command openapigen generates parts of the App Store Connect client in
// internal/asc from the OpenAPI spec, and with -target web parts of the web
// interface client in internal/web from the web interface schema, each from
// the bindings directory of its package. See docs/openapi/README.md.
package main

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	target := flag.String("target", "asc", "client to generate: asc (internal/asc) or web (internal/web)")
	specPath := flag.String("spec", "", "OpenAPI spec (default: the target's schema)")
	bindingsDir := flag.String("bindings", "openapi_bindings", "directory with one bindings file per family")
	dir := flag.String("dir", ".", "client package directory")
	check := flag.Bool("check", false, "fail if generated files are out of date instead of writing them")
	adopt := flag.String("adopt", "", "comma-separated tags (or \"all\") whose hand-written methods and types to replace")
	dryRun := flag.Bool("dry-run", false, "with -adopt, only report what would be generated")
	flag.Parse()
	var err error
	switch *target {
	case "asc":
		err = run(cmp.Or(*specPath, "../../docs/openapi/latest.json"), *bindingsDir, *dir, *check, *adopt, *dryRun, os.Stdout)
	case "web":
		err = runWeb(cmp.Or(*specPath, "../../docs/openapi/web/latest.json"), *bindingsDir, *dir, *check, *adopt, *dryRun, os.Stdout)
	default:
		err = fmt.Errorf("unknown target %q", *target)
	}
	if err != nil {
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
	return writeAdopted(edits, adopted, bindingsDir, func() (map[string][]byte, []string, error) {
		return plan(spec, bindings, dir)
	})
}

// writeAdopted writes the bindings of the adopted families, the generated
// files planned after adoption, and the hand-written files adoption shrank.
func writeAdopted(edits map[string][]byte, adopted []*Family, bindingsDir string, planFiles func() (map[string][]byte, []string, error)) error {
	// Encode sorts a family, so encode before rendering to generate in the
	// order the bindings files keep.
	families := map[string][]byte{}
	for _, fam := range adopted {
		data, err := fam.Encode()
		if err != nil {
			return err
		}
		families[bindingsFile(bindingsDir, fam.Tag)] = data
	}
	// Render everything before touching a file, so a failure leaves the
	// hand-written code in place.
	writes, stale, err := planFiles()
	if err != nil {
		return err
	}
	for path, data := range families {
		writes[path] = data
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
	return writeOrCheck(writes, stale, err, check)
}

func writeOrCheck(writes map[string][]byte, stale []string, err error, check bool) error {
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
	return planTarget(bindings, dir, header, func(fam *Family) ([]byte, error) { return Generate(spec, fam) })
}

// planTarget is plan for the target whose generated files start with
// fileHeader and whose families generate renders.
func planTarget(bindings *Bindings, dir, fileHeader string, generate func(*Family) ([]byte, error)) (map[string][]byte, []string, error) {
	writes := map[string][]byte{}
	for _, fam := range bindings.Families {
		src, err := generate(fam)
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
		if bytes.HasPrefix(data, []byte(fileHeader)) {
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
