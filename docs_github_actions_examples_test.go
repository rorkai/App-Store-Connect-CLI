package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// refNameInjectionPayload is a valid Git ref name (no space, `~`, `^`, `:`, `?`,
// `*`, `[`, or `\`) that appends a redirect if it ever reaches shell source.
const refNameInjectionPayload = "v1.0;>injected"

var actionsExpressionPattern = regexp.MustCompile(`\$\{\{\s*([^{}]+?)\s*\}\}`)

type docActionStep struct {
	File string
	Name string
	Env  map[string]string
	Run  string
}

func TestGitHubActionsDocExamplesKeepRefNameOutOfShellSource(t *testing.T) {
	sawUntrustedEnv := false
	for _, file := range githubActionsDocFiles(t) {
		steps := gitHubActionsDocSteps(t, file)
		for _, step := range steps {
			if match := actionsExpressionPattern.FindString(step.Run); match != "" {
				t.Errorf("%s: step %q interpolates %s inside run source; pass it through env instead", file, step.Name, match)
			}
			for name, value := range step.Env {
				if hasUntrustedActionsExpression(value) {
					sawUntrustedEnv = true
					if referencesShellVariable(step.Run, name) && !referencesQuotedShellVariable(step.Run, name) {
						t.Errorf("%s: step %q sets env %s but does not reference it as a quoted shell variable", file, step.Name, name)
					}
				}
			}
		}
	}
	if !sawUntrustedEnv {
		t.Fatal("expected at least one documented step to route an untrusted GitHub value through env")
	}
}

func TestGitHubActionsDocExamplesTreatRefNameAsInertArgument(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is not installed; shell-safety of the documented examples cannot be executed here: %v", err)
	}

	for _, file := range githubActionsDocFiles(t) {
		for _, step := range gitHubActionsDocSteps(t, file) {
			if !strings.Contains(step.Run, "asc ") || !stepReferencesUntrustedEnv(step) {
				continue
			}
			script, env := renderDocStepScript(t, file, step)

			t.Run(file+"/"+step.Name, func(t *testing.T) {
				workDir := t.TempDir()
				argsFile := filepath.Join(workDir, "asc-args.txt")
				stubDir := writeASCStub(t, argsFile)

				cmd := exec.Command(bashPath, "-c", script)
				cmd.Dir = workDir
				cmd.Env = append(os.Environ(), env...)
				cmd.Env = append(cmd.Env, "PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("run documented step: %v\n%s", err, output)
				}

				if entries, err := os.ReadDir(workDir); err == nil {
					for _, entry := range entries {
						if entry.Name() == "injected" {
							t.Fatalf("ref name executed shell syntax: %q was created", entry.Name())
						}
					}
				}

				args := recordedASCArgs(t, argsFile)
				if len(args) == 0 {
					t.Fatalf("expected the documented step to invoke asc")
				}
				sawFullRef := false
				for _, arg := range args {
					if !strings.Contains(arg, "v1.0") {
						continue
					}
					if !strings.Contains(arg, refNameInjectionPayload) {
						t.Fatalf("asc argument %q lost part of the ref; the ref must arrive as one inert argument containing %q", arg, refNameInjectionPayload)
					}
					sawFullRef = true
				}
				if !sawFullRef {
					t.Fatalf("expected asc to receive the ref name; got args %q", args)
				}
			})
		}
	}
}

func githubActionsDocFiles(t *testing.T) []string {
	t.Helper()

	files := make([]string, 0)
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != "." && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "dist") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".mdx" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		if strings.Contains(content, "${{") && strings.Contains(content, "run:") {
			files = append(files, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("discover GitHub Actions documentation: %v", err)
	}
	return files
}

func hasUntrustedActionsExpression(value string) bool {
	for _, match := range actionsExpressionPattern.FindAllStringSubmatch(value, -1) {
		expression := strings.TrimSpace(match[1])
		if strings.HasPrefix(expression, "github.ref") ||
			strings.HasPrefix(expression, "github.event.inputs.") ||
			strings.HasPrefix(expression, "inputs.") ||
			strings.HasPrefix(expression, "secrets.") {
			return true
		}
	}
	return false
}

func stepReferencesUntrustedEnv(step docActionStep) bool {
	for name, value := range step.Env {
		if hasRefOrInputActionsExpression(value) && referencesQuotedShellVariable(step.Run, name) {
			return true
		}
	}
	return false
}

func hasRefOrInputActionsExpression(value string) bool {
	for _, match := range actionsExpressionPattern.FindAllStringSubmatch(value, -1) {
		expression := strings.TrimSpace(match[1])
		if strings.HasPrefix(expression, "github.ref") ||
			strings.HasPrefix(expression, "github.event.inputs.") ||
			strings.HasPrefix(expression, "inputs.") {
			return true
		}
	}
	return false
}

// referencesQuotedShellVariable reports whether script expands name at least
// once and never expands it outside double quotes, so word splitting and glob
// expansion cannot act on the value.
func referencesQuotedShellVariable(script string, name string) bool {
	found := false
	inSingle := false
	inDouble := false

	for i := 0; i < len(script); i++ {
		switch c := script[i]; {
		case c == '\\' && !inSingle:
			i++
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '$' && !inSingle:
			rest := script[i+1:]
			braced := strings.HasPrefix(rest, "{"+name+"}")
			bare := strings.HasPrefix(rest, name) && !isShellNameByte(byteAt(rest, len(name)))
			if !braced && !bare {
				continue
			}
			if !inDouble {
				return false
			}
			found = true
		}
	}

	return found
}

func referencesShellVariable(script string, name string) bool {
	return strings.Contains(script, "$"+name) || strings.Contains(script, "${"+name+"}")
}

func isShellNameByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func byteAt(value string, index int) byte {
	if index >= len(value) {
		return 0
	}
	return value[index]
}

func gitHubActionsDocSteps(t *testing.T, file string) []docActionStep {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}

	var steps []docActionStep
	for _, language := range []string{"yaml", "yml"} {
		for _, block := range fencedBlocks(string(data), language) {
			if !strings.Contains(block, "run:") {
				continue
			}
			var root yaml.Node
			if err := yaml.Unmarshal([]byte(block), &root); err != nil {
				t.Fatalf("%s: parse yaml example: %v\n%s", file, err, block)
			}
			collectDocActionSteps(file, &root, &steps)
		}
	}
	return steps
}

func fencedBlocks(content string, language string) []string {
	var blocks []string
	var current []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inBlock = false
				continue
			}
			inBlock = strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(line, "```")), language)
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}
	return blocks
}

func collectDocActionSteps(file string, node *yaml.Node, steps *[]docActionStep) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		step := docActionStep{File: file}
		hasRun := false
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			value := node.Content[i+1]
			switch key {
			case "name":
				step.Name = value.Value
			case "run":
				step.Run = value.Value
				hasRun = true
			case "env":
				if value.Kind != yaml.MappingNode {
					continue
				}
				step.Env = make(map[string]string, len(value.Content)/2)
				for j := 0; j+1 < len(value.Content); j += 2 {
					step.Env[value.Content[j].Value] = value.Content[j+1].Value
				}
			}
		}
		if hasRun {
			*steps = append(*steps, step)
		}
	}
	for _, child := range node.Content {
		collectDocActionSteps(file, child, steps)
	}
}

// renderDocStepScript expands GitHub Actions expressions the way the runner
// does: expression results are substituted into the workflow text before the
// shell ever sees it.
func renderDocStepScript(t *testing.T, file string, step docActionStep) (string, []string) {
	t.Helper()

	env := make([]string, 0, len(step.Env))
	for name, value := range step.Env {
		env = append(env, name+"="+expandActionsExpressions(t, file, step, value))
	}
	return expandActionsExpressions(t, file, step, step.Run), env
}

func expandActionsExpressions(t *testing.T, file string, step docActionStep, value string) string {
	t.Helper()

	return actionsExpressionPattern.ReplaceAllStringFunc(value, func(match string) string {
		expression := strings.TrimSpace(actionsExpressionPattern.FindStringSubmatch(match)[1])
		if expression == "" {
			t.Fatalf("%s: step %q contains an empty GitHub Actions expression", file, step.Name)
		}
		return refNameInjectionPayload
	})
}

func writeASCStub(t *testing.T, argsFile string) string {
	t.Helper()

	stubDir := t.TempDir()
	stub := "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\n' \"$arg\" >> " + argsFile + "; done\n"
	stubPath := filepath.Join(stubDir, "asc")
	if err := os.WriteFile(stubPath, []byte(stub), 0o700); err != nil {
		t.Fatalf("write asc stub: %v", err)
	}
	return stubDir
}

func recordedASCArgs(t *testing.T, argsFile string) []string {
	t.Helper()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read recorded asc args: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}
