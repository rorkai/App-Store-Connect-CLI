#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path
from unittest.mock import Mock, patch


MODULE_PATH = Path(__file__).with_name("generate-command-docs.py")
SPEC = importlib.util.spec_from_file_location("generate_command_docs", MODULE_PATH)
assert SPEC and SPEC.loader
generate_command_docs = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(generate_command_docs)

CHECK_MODULE_PATH = Path(__file__).with_name("check-commands-docs.py")
CHECK_SPEC = importlib.util.spec_from_file_location("check_commands_docs", CHECK_MODULE_PATH)
assert CHECK_SPEC and CHECK_SPEC.loader
check_commands_docs = importlib.util.module_from_spec(CHECK_SPEC)
CHECK_SPEC.loader.exec_module(check_commands_docs)


HELP_WITH_SAMPLES = """DESCRIPTION
  asc is a fast, lightweight CLI for App Store Connect from Rork.

USAGE
  asc <subcommand> [flags]

GETTING STARTED
  asc search "upload a build" --output json   # find the command
  asc status --app APP_ID                     # release overview

UTILITY COMMANDS
  search:  Search asc commands and examples.

FLAGS
  --debug  Enable debug logging to stderr
"""


class ParseHelpTests(unittest.TestCase):
    @patch.object(generate_command_docs.subprocess, "run")
    def test_help_stdout_wins_over_go_download_diagnostics(self, run: Mock) -> None:
        run.return_value = Mock(stdout=HELP_WITH_SAMPLES, stderr="go: downloading example.com/module\n")

        self.assertEqual(generate_command_docs.run_help_text(), HELP_WITH_SAMPLES)

    @patch.object(check_commands_docs.subprocess, "run")
    def test_command_check_stdout_wins_over_go_download_diagnostics(self, run: Mock) -> None:
        run.return_value = Mock(stdout=HELP_WITH_SAMPLES, stderr="go: downloading example.com/module\n")

        self.assertEqual(check_commands_docs.run_help_text(), HELP_WITH_SAMPLES)

    def test_usage_comes_from_the_usage_section(self) -> None:
        usage, _, _ = generate_command_docs.parse_help(HELP_WITH_SAMPLES)
        self.assertEqual(usage, "asc <subcommand> [flags]")

    def test_sample_invocations_do_not_become_the_usage_pattern(self) -> None:
        # An unindented heading ends the USAGE section, so samples rendered
        # under it cannot be mistaken for the usage pattern even when the
        # USAGE section itself carries no recognizable invocation.
        help_text = HELP_WITH_SAMPLES.replace("  asc <subcommand> [flags]\n", "")
        usage, _, _ = generate_command_docs.parse_help(help_text)
        self.assertEqual(usage, "asc <subcommand> [flags]")

    def test_groups_and_flags_still_parse(self) -> None:
        _, flags, groups = generate_command_docs.parse_help(HELP_WITH_SAMPLES)
        self.assertEqual(flags, [("--debug", "Enable debug logging to stderr")])
        self.assertEqual(
            groups,
            [("UTILITY COMMANDS", [("search", "Search asc commands and examples.")])],
        )


class RenderTests(unittest.TestCase):
    def test_validate_url_check_is_present_in_high_signal_examples(self) -> None:
        rendered = generate_command_docs.render(
            "asc <subcommand> [flags]", [], []
        )

        self.assertIn(
            'asc validate --app "123456789" --version "1.2.3" --check-urls',
            rendered,
        )

    def test_xcode_nested_commands_are_present_in_high_signal_examples(self) -> None:
        rendered = generate_command_docs.render(
            "asc <subcommand> [flags]", [], []
        )

        self.assertIn(
            "asc xcode test-destinations --platform iOS --available-only --output json",
            rendered,
        )
        self.assertIn(
            "asc xcode test junit --xcresult ./Test.xcresult --report-file ./junit.xml --output json",
            rendered,
        )


if __name__ == "__main__":
    unittest.main()
