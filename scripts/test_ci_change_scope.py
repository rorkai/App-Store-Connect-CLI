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
        self.assertEqual(
            ci_change_scope.classify(["README.md", "docs/TESTING.md"]),
            "docs",
        )

    def test_go_source_under_documentation_requires_full_suite(self) -> None:
        for path in ("docs/embed.go", "guides/example.go"):
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

    def test_mintlify_content_uses_website_scope(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(["docs.json", "guides/getting-started.mdx"]),
            "website",
        )

    def test_telemetry_scope_requires_only_telemetry_files(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(["internal/telemetry/client.go"]),
            "telemetry",
        )
        self.assertEqual(
            ci_change_scope.classify(
                ["internal/telemetry/client.go", "commands/telemetry.mdx"]
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
        for path in ("main.go", "internal/asc/client.go", ".github/workflows/pr-checks.yml"):
            with self.subTest(path=path):
                self.assertEqual(ci_change_scope.classify([path]), "full")

    def test_renamed_code_keeps_source_and_destination_in_full_scope(self) -> None:
        self.assertEqual(
            ci_change_scope.classify(
                ["internal/asc/removed.go", "docs/removed.md"]
            ),
            "full",
        )

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
        self.assertTrue(ci_change_scope.affects_website(["guides/testflight.mdx"]))
        self.assertFalse(ci_change_scope.affects_website(["docs/TESTING.md"]))


if __name__ == "__main__":
    unittest.main()
