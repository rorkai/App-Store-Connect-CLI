package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const releaseRehearsalWorkflowPath = ".github/workflows/release-rehearsal.yml"

type rehearsalWorkflow struct {
	On struct {
		WorkflowDispatch struct {
			Inputs map[string]rehearsalInput `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Jobs map[string]rehearsalJob `yaml:"jobs"`
}

type rehearsalInput struct {
	Required bool   `yaml:"required"`
	Type     string `yaml:"type"`
}

type rehearsalJob struct {
	RunsOn string            `yaml:"runs-on"`
	Env    map[string]string `yaml:"env"`
	Steps  []rehearsalStep   `yaml:"steps"`
}

type rehearsalStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	With map[string]any    `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

var forbiddenRehearsalCommands = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bgh\s+release\s+(create|upload|edit|delete)\b`),
	regexp.MustCompile(`(?i)\bgh\s+pr\s+(create|edit|merge)\b`),
	regexp.MustCompile(`(?i)\bgh\s+repo\s+fork\b`),
	regexp.MustCompile(`(?i)\bgh\s+api\b`),
	regexp.MustCompile(`(?i)\bgit\s+(push|tag|commit)\b`),
	regexp.MustCompile(`(?i)\b(npm|pnpm|yarn)\s+publish\b`),
	regexp.MustCompile(`(?i)\bgem\s+push\b`),
	regexp.MustCompile(`(?i)\bcargo\s+publish\b`),
	regexp.MustCompile(`(?i)\b(dotnet\s+nuget|nuget)\s+push\b`),
	regexp.MustCompile(`(?i)\bdocker\s+push\b`),
	regexp.MustCompile(`(?i)\bpod\s+trunk\s+push\b`),
	regexp.MustCompile(`(?i)\bwingetcreate(?:\.exe)?\s+(new|update|submit)\b`),
	regexp.MustCompile(`(?i)\bbrew\s+bump-formula-pr\b`),
	regexp.MustCompile(`(?i)\bswift\s+package-registry\s+publish\b`),
	regexp.MustCompile(`(?i)\bcurl\b.*(?:--request|-X)\s*(POST|PUT|PATCH|DELETE)\b`),
}

var forbiddenRehearsalActions = []string{
	"actions/create-release@",
	"actions/upload-artifact@",
	"actions/upload-release-asset@",
	"apple-actions/import-codesign-certs@",
	"docker/build-push-action@",
	"docker/login-action@",
	"goreleaser/goreleaser-action@",
	"homebrew/actions/bump-packages@",
	"ncipollo/release-action@",
	"pypa/gh-action-pypi-publish@",
	"rubygems/release-gem@",
	"softprops/action-gh-release@",
}

func TestReleaseRehearsalWorkflowContract(t *testing.T) {
	workflow := readReleaseRehearsalWorkflow(t)
	if err := validateReleaseRehearsalWorkflow([]byte(workflow)); err != nil {
		t.Fatal(err)
	}

	misorderedCheckout := replaceRehearsalFixture(t, workflow,
		"      - name: Checkout requested source\n        uses: actions/checkout@v7",
		"      - run: echo harmless\n\n      - name: Cache source\n        uses: actions/cache@v5")
	cases := map[string]string{
		"version not required": replaceRehearsalFixture(t, workflow,
			"version:\n        description: Candidate release version in x.y.z form\n        required: true",
			"version:\n        description: Candidate release version in x.y.z form\n        required: false"),
		"ref not string": replaceRehearsalFixture(t, workflow,
			"ref:\n        description: Candidate commit SHA or Git ref containing rehearsal tooling\n        required: true\n        type: string",
			"ref:\n        description: Candidate commit SHA or Git ref containing rehearsal tooling\n        required: true\n        type: boolean"),
		"checkout ignores ref": replaceRehearsalFixture(t, workflow,
			"ref: ${{ inputs.ref }}", "ref: main"),
		"version env ignores input": replaceRehearsalFixture(t, workflow,
			"      - name: Run non-publishing release rehearsal\n        env:\n          VERSION: ${{ inputs.version }}",
			"      - name: Run non-publishing release rehearsal\n        env:\n          VERSION: 9.9.9"),
		"tested sha not persisted": replaceRehearsalFixture(t, workflow,
			`printf 'TESTED_SHA=%s\n' "${TESTED_SHA}" >> "${GITHUB_ENV}"`,
			`printf 'TESTED_SHA=%s\n' "${TESTED_SHA}"`),
		"different expected sha": replaceRehearsalFixture(t, workflow,
			`--expected-sha "${TESTED_SHA}"`, `--expected-sha "${OTHER_SHA}"`),
		"misordered anonymous checkout": replaceRehearsalFixture(t, misorderedCheckout,
			"      - name: Set up Go",
			"      - uses: actions/checkout@v7\n        with:\n          ref: ${{ inputs.ref }}\n          fetch-depth: 0\n          persist-credentials: false\n\n      - name: Set up Go"),
		"write all": replaceRehearsalFixture(t, workflow,
			"permissions:\n  contents: read", "permissions: write-all"),
		"package write": replaceRehearsalFixture(t, workflow,
			"permissions:\n  contents: read", "permissions:\n  contents: read\n  packages: write"),
		"job identity token": replaceRehearsalFixture(t, workflow,
			"  rehearse:\n    runs-on:", "  rehearse:\n    permissions:\n      id-token: write\n    runs-on:"),
		"non-release platform": replaceRehearsalFixture(t, workflow,
			"runs-on: macos-latest", "runs-on: ubuntu-latest"),
		"release action": replaceRehearsalFixture(t, workflow,
			"uses: actions/setup-go@v7", "uses: actions/upload-artifact@v7"),
		"github release": replaceRehearsalFixture(t, workflow,
			"run: make tools", "run: gh release create 9.9.9"),
		"package publish": replaceRehearsalFixture(t, workflow,
			"run: make tools", "run: npm publish"),
		"package manager submission": replaceRehearsalFixture(t, workflow,
			"run: make tools", "run: wingetcreate submit manifests"),
		"publishing sibling job": replaceRehearsalFixture(t, workflow,
			"jobs:\n  rehearse:",
			"jobs:\n  publish:\n    runs-on: ubuntu-latest\n    steps:\n      - run: gh release create 9.9.9\n  rehearse:"),
		"missing rehearsal entrypoint": replaceRehearsalFixture(t, workflow,
			"python3 scripts/release_rehearsal.py", "python3 scripts/other.py"),
		"missing release notes": replaceRehearsalFixture(t, workflow,
			`cat "release/asc_${VERSION}_release-notes.md"`, "echo missing-release-notes"),
		"missing checksums": replaceRehearsalFixture(t, workflow,
			`cat "release/asc_${VERSION}_checksums.txt"`, "echo missing-checksums"),
		"summary not appended": replaceRehearsalFixture(t, workflow,
			`echo "All files remained local to this runner; no release assets were uploaded."
          } >> "${GITHUB_STEP_SUMMARY}"`,
			`echo "All files remained local to this runner; no release assets were uploaded."
          } >> /dev/null`),
	}

	for name, fixture := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateReleaseRehearsalWorkflow([]byte(fixture)); err == nil {
				t.Fatal("unsafe fixture passed the workflow contract")
			}
		})
	}
}

func validateReleaseRehearsalWorkflow(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse release rehearsal workflow: %w", err)
	}

	var workflow rehearsalWorkflow
	if err := root.Decode(&workflow); err != nil {
		return fmt.Errorf("decode release rehearsal workflow: %w", err)
	}
	for _, name := range []string{"version", "ref"} {
		input, ok := workflow.On.WorkflowDispatch.Inputs[name]
		if !ok || !input.Required || input.Type != "string" {
			return fmt.Errorf("workflow_dispatch input %q must be a required string", name)
		}
	}

	permissionBlocks := findYAMLMappingValues(&root, "permissions")
	if len(permissionBlocks) == 0 {
		return fmt.Errorf("workflow must declare explicit read-only permissions")
	}
	for _, block := range permissionBlocks {
		if block.Kind != yaml.MappingNode || len(block.Content) != 2 ||
			block.Content[0].Value != "contents" || block.Content[1].Value != "read" {
			return fmt.Errorf("every permissions block must be exactly contents: read")
		}
	}

	job, ok := workflow.Jobs["rehearse"]
	if !ok {
		return fmt.Errorf("missing rehearse job")
	}
	if job.RunsOn != "macos-latest" {
		return fmt.Errorf("rehearsal guardrails must run on the macOS release platform")
	}
	if job.Env["GIT_TERMINAL_PROMPT"] != "0" || job.Env["GH_PROMPT_DISABLED"] != "1" {
		return fmt.Errorf("rehearse job must disable interactive Git and GitHub prompts")
	}

	checkout, checkoutIndex, err := findRehearsalStep(job.Steps, "", "actions/checkout@")
	if err != nil {
		return err
	}
	if fmt.Sprint(checkout.With["ref"]) != "${{ inputs.ref }}" ||
		fmt.Sprint(checkout.With["fetch-depth"]) != "0" ||
		fmt.Sprint(checkout.With["persist-credentials"]) != "false" {
		return fmt.Errorf("checkout must use inputs.ref, full history, and no persisted credentials")
	}

	record, recordIndex, err := findRehearsalStep(job.Steps, "Record exact tested commit", "")
	if err != nil {
		return err
	}
	if record.Env["REQUESTED_REF"] != "${{ inputs.ref }}" ||
		record.Env["VERSION"] != "${{ inputs.version }}" ||
		!strings.Contains(record.Run, `TESTED_SHA="$(git rev-parse HEAD)"`) ||
		!strings.Contains(record.Run, `printf 'TESTED_SHA=%s\n' "${TESTED_SHA}" >> "${GITHUB_ENV}"`) {
		return fmt.Errorf("tested SHA must be derived from HEAD and explicitly persisted")
	}

	rehearse, rehearseIndex, err := findRehearsalStep(
		job.Steps,
		"Run non-publishing release rehearsal",
		"",
	)
	if err != nil {
		return err
	}
	if rehearse.Env["VERSION"] != "${{ inputs.version }}" ||
		!strings.Contains(rehearse.Run, "python3 scripts/release_rehearsal.py") ||
		!strings.Contains(rehearse.Run, `--version "${VERSION}"`) ||
		!strings.Contains(rehearse.Run, `--expected-sha "${TESTED_SHA}"`) {
		return fmt.Errorf("rehearsal step must bind the requested version and persisted tested SHA")
	}
	if checkoutIndex >= recordIndex || recordIndex >= rehearseIndex {
		return fmt.Errorf("checkout, tested-SHA persistence, and rehearsal execution must stay ordered")
	}

	summary, summaryIndex, err := findRehearsalStep(
		job.Steps,
		"Summarize local rehearsal outputs",
		"",
	)
	if err != nil {
		return err
	}
	if summary.Env["VERSION"] != "${{ inputs.version }}" ||
		!strings.Contains(summary.Run, `cat "release/asc_${VERSION}_release-notes.md"`) ||
		!strings.Contains(summary.Run, `cat "release/asc_${VERSION}_checksums.txt"`) ||
		!strings.Contains(summary.Run, `} >> "${GITHUB_STEP_SUMMARY}"`) {
		return fmt.Errorf("summary step must append release notes and checksums to GITHUB_STEP_SUMMARY")
	}
	if rehearseIndex >= summaryIndex {
		return fmt.Errorf("rehearsal outputs must be generated before they are summarized")
	}

	workflowText := strings.ToLower(string(data))
	if strings.Contains(workflowText, "secrets.") {
		return fmt.Errorf("release rehearsal must not consume repository secrets")
	}
	for jobName, currentJob := range workflow.Jobs {
		for _, step := range currentJob.Steps {
			uses := strings.ToLower(step.Uses)
			for _, forbidden := range forbiddenRehearsalActions {
				if strings.Contains(uses, forbidden) {
					return fmt.Errorf("release rehearsal job %q uses publishing action %q", jobName, step.Uses)
				}
			}
			for _, forbidden := range forbiddenRehearsalCommands {
				if forbidden.MatchString(step.Run) {
					return fmt.Errorf("release rehearsal job %q runs publishing command in step %q", jobName, step.Name)
				}
			}
		}
	}

	return nil
}

func findYAMLMappingValues(node *yaml.Node, key string) []*yaml.Node {
	if node == nil {
		return nil
	}
	var values []*yaml.Node
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == key {
				values = append(values, node.Content[index+1])
			}
			values = append(values, findYAMLMappingValues(node.Content[index+1], key)...)
		}
		return values
	}
	for _, child := range node.Content {
		values = append(values, findYAMLMappingValues(child, key)...)
	}
	return values
}

func findRehearsalStep(
	steps []rehearsalStep,
	name string,
	usesPrefix string,
) (rehearsalStep, int, error) {
	var match rehearsalStep
	matchIndex := -1
	matchCount := 0
	for index, step := range steps {
		if name != "" && step.Name != name {
			continue
		}
		if usesPrefix != "" && !strings.HasPrefix(step.Uses, usesPrefix) {
			continue
		}
		match = step
		matchIndex = index
		matchCount++
	}
	if matchCount != 1 {
		return rehearsalStep{}, -1, fmt.Errorf(
			"expected one matching workflow step, found %d",
			matchCount,
		)
	}
	return match, matchIndex, nil
}

func readReleaseRehearsalWorkflow(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(releaseRehearsalWorkflowPath)
	if err != nil {
		t.Fatalf("read release rehearsal workflow: %v", err)
	}
	return string(data)
}

func replaceRehearsalFixture(t *testing.T, workflow string, old string, replacement string) string {
	t.Helper()
	if strings.Count(workflow, old) != 1 {
		t.Fatalf("fixture source %q must occur exactly once", old)
	}
	return strings.Replace(workflow, old, replacement, 1)
}
