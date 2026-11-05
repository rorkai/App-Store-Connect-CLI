#!/usr/bin/env python3
"""Tests for conservative CI change-scope classification."""

import unittest

import ci_change_scope


class ChangeScopeTest(unittest.TestCase):
    def test_empty_change_list_requires_full_suite(self) -> None:
        self.assertEqual(ci_change_scope.classify([]), "full")

    def test_wall_source_is_the_only_wall_only_change(self) -> None:
        self.assertEqual(ci_change_scope.classify(["docs/wall-of-apps.json"]), "wall")
        self.assertEqual(
            ci_change_scope.classify(["docs/wall-of-apps.json", "README.md"]),
            "full",
        )

    def test_repository_documentation_uses_docs_scope(self) -> None:
        for paths in (
            ["README.md", "docs/TESTING.md"],
            [".agents/skills/develop-asc-change/SKILL.md"],
            [".agents/skills/develop-asc-change/references/test-matrix.md"],
            [".agents/skills/develop-asc-change/agents/openai.yaml"],
            [".agents/skills/develop-asc-change/SKILL.md", "docs/TESTING.md"],
        ):
            with self.subTest(paths=paths):
                self.assertEqual(ci_change_scope.classify(paths), "docs")

    def test_go_source_under_documentation_requires_full_suite(self) -> None:
        for path in ("docs/embed.go", "guides/example.go", ".agents/skills/example/nested/main.go"):
            with self.subTest(path=path):
                self.assertEqual(ci_change_scope.classify([path]), "full")

    def test_openapi_snapshot_requires_schema_drift_tests(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(["docs/openapi/latest.json"]),
            "full",
        )

    def test_embedded_guides_require_runtime_docs_tests(self) -> None:
        for path in ("docs/API_NOTES.md", "docs/WORKFLOWS.md"):
            with self.subTest(path=path):
                self.assertEqual(ci_change_scope.classify([path]), "full")

    def test_website_content_uses_website_scope(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(
                ["website/navigation.json", "website/src/content/docs/guides/testflight.mdx"]
            ),
            "website",
        )

    def test_telemetry_scope_requires_only_telemetry_files(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(["internal/telemetry/client.go"]),
            "telemetry",
        )
        self.assertEqual(
            ci_change_scope.classify(
                ["internal/telemetry/client.go", "website/src/content/docs/commands/telemetry.mdx"]
            ),
            "full",
        )

    def test_telemetry_cli_changes_require_command_coverage(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(["internal/cli/telemetry/telemetry.go"]),
            "full",
        )

    def test_mixed_specialized_changes_require_full_suite(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(
                ["docs/wall-of-apps.json", "internal/telemetry/client.go"]
            ),
            "full",
        )

    def test_general_go_and_ci_changes_require_full_suite(self) -> None:
        for path in (
            "main.go", "internal/asc/client.go", ".github/workflows/pr-checks.yml", "docs/THIRD_PARTY_NOTICES.md",
            ".agents/example.py",
            ".agents/skills/example/scripts/check.py",
            ".agents/skills/example/SKILL.md.py",
            ".agents/skills/example/agents/openai.yaml.py",
            ".agents/skills/example/config.yaml",
            ".agents/skills-extra/example/SKILL.md",
        ):
            with self.subTest(path=path):
                self.assertEqual(ci_change_scope.classify([path]), "full")

    def test_renamed_code_keeps_source_and_destination_in_full_scope(self) -> None:
        for paths in (
            ["internal/asc/removed.go", "docs/removed.md"],
            ["internal/asc/client.go", ".agents/skills/example/SKILL.md"],
        ):
            with self.subTest(paths=paths):
                self.assertEqual(ci_change_scope.classify(paths), "full")

    def test_ci_bootstrap_files_require_full_scope(self) -> None:
        for path in (
            ".github/workflows/pr-checks.yml",
            "scripts/ci_change_scope.py",
            "scripts/test_ci_change_scope.py",
            "scripts/test_ci_workflows.py",
        ):
            with self.subTest(path=path):
                self.assertEqual(ci_change_scope.classify([path]), "full")

    def test_dedicated_workflow_impact_matches_owned_paths(self) -> None:
        self.assertTrue(ci_change_scope.affects_website(["website/src/content/docs/guides/testflight.mdx"]))
        self.assertFalse(ci_change_scope.affects_website(["docs/TESTING.md"]))


if __name__ == "__main__":
    unittest.main()
