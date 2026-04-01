#!/usr/bin/env python3
from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import check_release_docs
import check_repo_docs
import check_website_commands
import check_website_docs
import doc_links


class RepoDocsChecksTest(unittest.TestCase):
    def test_repo_docs_accept_valid_local_links(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            docs = root / "docs"
            docs.mkdir()
            source = root / "README.md"
            target = docs / "guide.md"
            source.write_text("[Guide](docs/guide.md)\n[External](https://example.com)\n")
            target.write_text("# Guide\n")

            errors = check_repo_docs.check_files(root, [source])
            self.assertEqual(errors, [])

    def test_repo_docs_report_missing_local_links(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            source = root / "README.md"
            source.write_text("[Missing](docs/missing.md)\n")

            errors = check_repo_docs.check_files(root, [source])
            self.assertEqual(len(errors), 1)
            self.assertIn("missing local docs target", errors[0])

    def test_repo_docs_ignore_angle_bracket_external_links(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            source = root / "README.md"
            source.write_text("[External](<https://example.com/docs>)\n")

            errors = check_repo_docs.check_files(root, [source])
            self.assertEqual(errors, [])

    def test_repo_docs_reject_targets_outside_repo_root(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            root = tmp / "repo"
            root.mkdir()
            outside = tmp / "outside.md"
            outside.write_text("# Outside\n")
            source = root / "README.md"
            source.write_text("[Outside](../outside.md)\n")

            errors = check_repo_docs.check_files(root, [source])
            self.assertEqual(len(errors), 1)
            self.assertIn("escapes repository root", errors[0])


class WebsiteDocsChecksTest(unittest.TestCase):
    def test_website_docs_accept_valid_navigation_and_links(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "guides").mkdir()
            (website / "docs.json").write_text(
                json.dumps(
                    {
                        "navigation": {
                            "tabs": [{"groups": [{"pages": ["index", "guides/test"]}]}]
                        },
                        "redirects": [
                            {"source": "/old-test", "destination": "/guides/test"}
                        ],
                    }
                )
            )
            (website / "index.mdx").write_text('[Guide](/guides/test)\n<a href="/guides/test">Guide</a>\n')
            (website / "guides" / "test.mdx").write_text("# Test\n")

            page_ids, routes = check_website_docs.collect_site_state(website)
            self.assertEqual(check_website_docs.check_navigation(website, page_ids), [])
            self.assertEqual(check_website_docs.check_redirects(website, routes), [])
            self.assertEqual(check_website_docs.check_internal_links(website, routes), [])

    def test_website_docs_accept_nested_navigation_groups(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "guides").mkdir()
            (website / "docs.json").write_text(
                json.dumps(
                    {
                        "navigation": {
                            "tabs": [
                                {
                                    "groups": [
                                        {
                                            "group": "Guides",
                                            "pages": [
                                                {
                                                    "group": "Advanced",
                                                    "pages": ["guides/test"],
                                                }
                                            ],
                                        }
                                    ]
                                }
                            ]
                        }
                    }
                )
            )
            (website / "index.mdx").write_text("# Home\n")
            (website / "guides" / "test.mdx").write_text("# Test\n")

            page_ids, _ = check_website_docs.collect_site_state(website)
            self.assertEqual(check_website_docs.check_navigation(website, page_ids), [])

    def test_website_docs_report_missing_navigation_page(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "docs.json").write_text(
                json.dumps({"navigation": {"tabs": [{"groups": [{"pages": ["missing"]}]}]}})
            )
            (website / "index.mdx").write_text("# Home\n")

            page_ids, _ = check_website_docs.collect_site_state(website)
            errors = check_website_docs.check_navigation(website, page_ids)
            self.assertEqual(len(errors), 1)
            self.assertIn("missing page", errors[0])

    def test_website_docs_report_missing_route(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "docs.json").write_text(
                json.dumps({"navigation": {"tabs": [{"groups": [{"pages": ["index"]}]}]}})
            )
            (website / "index.mdx").write_text("[Missing](/guides/unknown)\n")

            _, routes = check_website_docs.collect_site_state(website)
            errors = check_website_docs.check_internal_links(website, routes)
            self.assertEqual(len(errors), 1)
            self.assertIn("missing website route", errors[0])

    def test_website_docs_report_missing_redirect_destination(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "docs.json").write_text(
                json.dumps(
                    {
                        "navigation": {"tabs": [{"groups": [{"pages": ["index"]}]}]},
                        "redirects": [
                            {"source": "/old-index", "destination": "/missing-page"}
                        ],
                    }
                )
            )
            (website / "index.mdx").write_text("# Home\n")

            _, routes = check_website_docs.collect_site_state(website)
            errors = check_website_docs.check_redirects(website, routes)
            self.assertEqual(len(errors), 1)
            self.assertIn("redirect destination", errors[0])

    def test_website_docs_ignore_angle_bracket_external_links(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "docs.json").write_text(
                json.dumps({"navigation": {"tabs": [{"groups": [{"pages": ["index"]}]}]}})
            )
            (website / "index.mdx").write_text("[External](<https://example.com/docs>)\n")

            _, routes = check_website_docs.collect_site_state(website)
            errors = check_website_docs.check_internal_links(website, routes)
            self.assertEqual(errors, [])

    def test_website_docs_reject_asset_targets_outside_website_root(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            website = tmp / "website"
            website.mkdir()
            (tmp / "shared.png").write_text("png\n")
            (website / "docs.json").write_text(
                json.dumps({"navigation": {"tabs": [{"groups": [{"pages": ["index"]}]}]}})
            )
            (website / "index.mdx").write_text("![Shared](../shared.png)\n")

            _, routes = check_website_docs.collect_site_state(website)
            errors = check_website_docs.check_internal_links(website, routes)
            self.assertEqual(len(errors), 1)
            self.assertIn("escapes website root", errors[0])


class WebsiteCommandChecksTest(unittest.TestCase):
    def test_website_command_checks_accept_valid_examples(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={"--profile": False, "--debug": True},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={"--output": False},
                subcommands={"list"},
            ),
            ("apps", "list"): check_website_commands.CommandSpec(
                path=("apps", "list"),
                usage="asc apps list [flags]",
                flags={"--output": False},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc --profile prod apps list --output json\n```\n"
            )
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(errors, [])

    def test_website_command_checks_reject_unknown_subcommand(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"reviews"},
            ),
            ("reviews",): check_website_commands.CommandSpec(
                path=("reviews",),
                usage="asc reviews [flags] | asc reviews <subcommand> [flags]",
                flags={"--app": False},
                subcommands={"list", "view"},
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc reviews get --id REVIEW_ID\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("unknown subcommand", errors[0])

    def test_website_command_checks_reject_misplaced_global_flag(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={"--profile": False},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={},
                subcommands={"list"},
            ),
            ("apps", "list"): check_website_commands.CommandSpec(
                path=("apps", "list"),
                usage="asc apps list [flags]",
                flags={},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc apps list --profile prod\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("must appear before", errors[0])

    def test_website_command_checks_do_not_swallow_token_after_boolean_root_flag(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={"--debug": True},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={},
                subcommands={"list"},
            ),
            ("apps", "list"): check_website_commands.CommandSpec(
                path=("apps", "list"),
                usage="asc apps list [flags]",
                flags={},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc apps list --debug bogus\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 2)
            self.assertIn("must appear before", errors[0])
            self.assertIn("unexpected positional argument", errors[1])

    def test_website_command_checks_accept_root_flags_without_top_level_command(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={"--profile": False, "--debug": True},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={},
                subcommands={"list"},
            ),
            ("apps", "list"): check_website_commands.CommandSpec(
                path=("apps", "list"),
                usage="asc apps list [flags]",
                flags={},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc --profile prod --debug\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(errors, [])

    def test_website_command_checks_consume_global_flag_value_before_subcommand_match(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={"--profile": False},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={},
                subcommands={"list"},
            ),
            ("apps", "list"): check_website_commands.CommandSpec(
                path=("apps", "list"),
                usage="asc apps list [flags]",
                flags={},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc --profile apps list\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("could not resolve top-level command", errors[0])

    def test_website_command_checks_reject_flags_after_positionals(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"demo"},
            ),
            ("demo",): check_website_commands.CommandSpec(
                path=("demo",),
                usage="asc demo [flags] <name>",
                flags={"--pretty": True},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc demo beta --pretty\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("appears after positional", errors[0])

    def test_website_command_checks_reject_missing_flag_value_before_next_flag(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={},
                subcommands={"list"},
            ),
            ("apps", "list"): check_website_commands.CommandSpec(
                path=("apps", "list"),
                usage="asc apps list [flags]",
                flags={"--output": False, "--paginate": True},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc apps list --output --paginate\n```\n")
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("missing value for flag '--output'", errors[0])

    def test_website_command_checks_parse_deprecated_replacement(self) -> None:
        help_text = """
DESCRIPTION
  DEPRECATED: use `asc analytics segments view`.

USAGE
  asc analytics segments get --segment-id "SEGMENT_ID"
"""
        self.assertEqual(
            check_website_commands.deprecation_replacement(help_text),
            "asc analytics segments view",
        )

    def test_website_command_checks_extract_single_line_command_with_quoted_value(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc app-events get --event-id \"EVENT_ID\"\n```\n")
            examples = check_website_commands.extract_examples(website)
            self.assertEqual(len(examples), 1)
            self.assertEqual(
                examples[0].tokens,
                ("asc", "app-events", "get", "--event-id", "EVENT_ID"),
            )

    def test_website_command_checks_ignore_usage_placeholder_after_flag(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text("```bash\nasc completion --shell <bash|zsh|fish>\n```\n")
            examples = check_website_commands.extract_examples(website)
            self.assertEqual(examples, [])

    def test_website_command_checks_parse_indented_fenced_blocks(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"apps"},
            ),
            ("apps",): check_website_commands.CommandSpec(
                path=("apps",),
                usage="asc apps list [flags]",
                flags={},
                subcommands={"list"},
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "<Accordion>\n"
                "    ```bash\n"
                "    asc apps nope\n"
                "    ```\n"
                "</Accordion>\n"
            )
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("unknown subcommand", errors[0])

    def test_website_command_checks_validate_inline_command_references(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"create"},
            ),
            ("submit", "create"): check_website_commands.CommandSpec(
                path=("submit", "create"),
                usage="asc submit create [flags]",
                flags={"--app": False, "--version": False, "--build": False, "--confirm": True},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "When you run `asc submit create`, the CLI starts a submission.\n"
            )
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("missing required flag", errors[0])

    def test_website_command_checks_ignore_deprecated_inline_alias_notes(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"versions"},
            ),
            ("versions",): check_website_commands.CommandSpec(
                path=("versions",),
                usage="asc versions <subcommand> [flags]",
                flags={},
                subcommands={"view"},
            ),
            ("versions", "view"): check_website_commands.CommandSpec(
                path=("versions", "view"),
                usage="asc versions view [flags]",
                flags={},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "`asc versions get` still works as a deprecated compatibility alias.\n"
            )
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(errors, [])

    def test_website_command_checks_reject_deprecated_inline_alias_examples(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"status", "cancel"},
            ),
        }

        original_hidden_alias = check_website_commands.hidden_deprecated_alias_replacement
        self.addCleanup(
            setattr,
            check_website_commands,
            "hidden_deprecated_alias_replacement",
            original_hidden_alias,
        )
        check_website_commands.hidden_deprecated_alias_replacement = (
            lambda _binary_path, _example, _root_flags: "asc publish appstore --submit"
        )
        original_path_help = check_website_commands.path_help
        self.addCleanup(
            setattr,
            check_website_commands,
            "path_help",
            original_path_help,
        )
        check_website_commands.path_help = (
            lambda _binary_path, path: (
                "DESCRIPTION\n"
                "  DEPRECATED: use `asc publish appstore --submit`.\n\n"
                "USAGE\n"
                "  asc submit create [flags]\n"
            )
            if path == ("submit", "create")
            else ""
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "Use `asc submit create --app 123456789 --version-id version-123 --build 42 --confirm` to submit.\n"
            )
            errors = check_website_commands.collect_errors(
                website,
                index,
                Path(tmpdir) / "asc-doc-check",
            )

        self.assertEqual(len(errors), 1)
        self.assertIn("deprecated alias", errors[0])
        self.assertIn("asc publish appstore --submit", errors[0])

    def test_website_command_checks_ignore_ellipsis_inline_references(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"sandbox"},
            ),
            ("sandbox",): check_website_commands.CommandSpec(
                path=("sandbox",),
                usage="asc sandbox <subcommand> [flags]",
                flags={},
                subcommands={"list"},
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "Use the current CLI surface under `asc sandbox ...`.\n"
            )
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(errors, [])

    def test_website_command_checks_require_submit_create_build_and_confirm(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"create"},
            ),
            ("submit", "create"): check_website_commands.CommandSpec(
                path=("submit", "create"),
                usage="asc submit create [flags]",
                flags={"--app": False, "--version": False, "--build": False, "--confirm": True},
                subcommands=set(),
            ),
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc submit create --app 123456789 --version 1.2.0\n```\n"
            )
            errors = check_website_commands.collect_errors(website, index)
            self.assertEqual(len(errors), 1)
            self.assertIn("missing required flag", errors[0])
            self.assertIn("--build", errors[0])
            self.assertIn("--confirm", errors[0])

    def test_token_command_path_skips_root_flags_before_command_lookup(self) -> None:
        path = check_website_commands.token_command_path(
            (
                "asc",
                "--api-debug",
                "--profile",
                "ci",
                "submit",
                "create",
                "--app",
                "123456789",
            ),
            {"--api-debug": True, "--profile": False},
        )
        self.assertEqual(path, ("submit", "create"))

    def test_website_command_checks_continue_validating_hidden_deprecated_alias_examples(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"preflight", "status", "cancel"},
            ),
        }

        original_hidden_alias = check_website_commands.hidden_deprecated_alias_replacement
        self.addCleanup(
            setattr,
            check_website_commands,
            "hidden_deprecated_alias_replacement",
            original_hidden_alias,
        )
        check_website_commands.hidden_deprecated_alias_replacement = (
            lambda _binary_path, _example, _root_flags: "asc publish appstore --submit"
        )
        original_path_help = check_website_commands.path_help
        self.addCleanup(
            setattr,
            check_website_commands,
            "path_help",
            original_path_help,
        )
        check_website_commands.path_help = (
            lambda _binary_path, path: (
                "USAGE\n"
                "  asc submit create [flags]\n\n"
                "FLAGS\n"
                "  --app          App Store Connect app ID\n"
                "  --version      App Store version string\n"
                "  --version-id   App Store version ID\n"
                "  --build        Build ID to attach\n"
                "  --confirm      Confirm submission (required) (default: false)\n"
            )
            if path == ("submit", "create")
            else ""
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc submit create --app 123456789 --version 1.2.0\n```\n"
            )
            errors = check_website_commands.collect_errors(
                website,
                index,
                Path(tmpdir) / "asc-doc-check",
            )

        self.assertEqual(len(errors), 1)
        self.assertIn("missing required flag", errors[0])
        self.assertIn("--build", errors[0])
        self.assertIn("--confirm", errors[0])

    def test_website_command_checks_reject_valid_hidden_deprecated_alias_examples(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"preflight", "status", "cancel"},
            ),
        }

        original_hidden_alias = check_website_commands.hidden_deprecated_alias_replacement
        self.addCleanup(
            setattr,
            check_website_commands,
            "hidden_deprecated_alias_replacement",
            original_hidden_alias,
        )
        check_website_commands.hidden_deprecated_alias_replacement = (
            lambda _binary_path, _example, _root_flags: "asc publish appstore --submit"
        )
        original_path_help = check_website_commands.path_help
        self.addCleanup(
            setattr,
            check_website_commands,
            "path_help",
            original_path_help,
        )
        check_website_commands.path_help = (
            lambda _binary_path, path: (
                "USAGE\n"
                "  asc submit create [flags]\n\n"
                "FLAGS\n"
                "  --app          App Store Connect app ID\n"
                "  --version      App Store version string\n"
                "  --version-id   App Store version ID\n"
                "  --build        Build ID to attach\n"
                "  --confirm      Confirm submission (required) (default: false)\n"
            )
            if path == ("submit", "create")
            else ""
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc submit create --app 123456789 --version 1.2.0 --build 42 --confirm\n```\n"
            )
            errors = check_website_commands.collect_errors(
                website,
                index,
                Path(tmpdir) / "asc-doc-check",
            )

        self.assertEqual(len(errors), 1)
        self.assertIn("deprecated alias", errors[0])
        self.assertIn("asc publish appstore --submit", errors[0])

    def test_website_command_checks_allow_hidden_alias_only_flags_before_deprecation_error(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"preflight", "status", "cancel"},
            ),
        }

        original_hidden_alias = check_website_commands.hidden_deprecated_alias_replacement
        self.addCleanup(
            setattr,
            check_website_commands,
            "hidden_deprecated_alias_replacement",
            original_hidden_alias,
        )
        check_website_commands.hidden_deprecated_alias_replacement = (
            lambda _binary_path, _example, _root_flags: "asc publish appstore --submit"
        )
        original_path_help = check_website_commands.path_help
        self.addCleanup(
            setattr,
            check_website_commands,
            "path_help",
            original_path_help,
        )
        check_website_commands.path_help = (
            lambda _binary_path, path: (
                "USAGE\n"
                "  asc submit create [flags]\n\n"
                "FLAGS\n"
                "  --app          App Store Connect app ID\n"
                "  --version      App Store version string\n"
                "  --version-id   App Store version ID\n"
                "  --build        Build ID to attach\n"
                "  --confirm      Confirm submission (required) (default: false)\n"
            )
            if path == ("submit", "create")
            else ""
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc submit create --app 123456789 --version-id version-123 --build 42 --confirm\n```\n"
            )
            errors = check_website_commands.collect_errors(
                website,
                index,
                Path(tmpdir) / "asc-doc-check",
            )

        self.assertEqual(len(errors), 1)
        self.assertIn("deprecated alias", errors[0])
        self.assertNotIn("unknown flag '--version-id'", errors[0])

    def test_hidden_submit_create_falls_back_to_alias_flags_when_help_hides_flags(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"status", "cancel"},
            ),
        }

        original_hidden_alias = check_website_commands.hidden_deprecated_alias_replacement
        self.addCleanup(
            setattr,
            check_website_commands,
            "hidden_deprecated_alias_replacement",
            original_hidden_alias,
        )
        check_website_commands.hidden_deprecated_alias_replacement = (
            lambda _binary_path, _example, _root_flags: "asc publish appstore --submit"
        )
        original_path_help = check_website_commands.path_help
        self.addCleanup(
            setattr,
            check_website_commands,
            "path_help",
            original_path_help,
        )
        check_website_commands.path_help = (
            lambda _binary_path, path: (
                "DESCRIPTION\n"
                "  DEPRECATED: use `asc publish appstore --submit`.\n\n"
                "USAGE\n"
                "  asc submit create [flags]\n"
            )
            if path == ("submit", "create")
            else ""
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc submit create --app 123456789 --version-id version-123 --build 42 --confirm\n```\n"
            )
            errors = check_website_commands.collect_errors(
                website,
                index,
                Path(tmpdir) / "asc-doc-check",
            )

        self.assertEqual(len(errors), 1)
        self.assertIn("deprecated alias", errors[0])
        self.assertNotIn("unknown flag '--version-id'", errors[0])
        self.assertNotIn("unknown flag '--build'", errors[0])
        self.assertNotIn("unknown flag '--confirm'", errors[0])

    def test_hidden_submit_preflight_falls_back_to_alias_flags_when_help_hides_flags(self) -> None:
        index = {
            (): check_website_commands.CommandSpec(
                path=(),
                usage="asc <subcommand> [flags]",
                flags={},
                subcommands={"submit"},
            ),
            ("submit",): check_website_commands.CommandSpec(
                path=("submit",),
                usage="asc submit <subcommand> [flags]",
                flags={},
                subcommands={"status", "cancel"},
            ),
        }

        original_hidden_alias = check_website_commands.hidden_deprecated_alias_replacement
        self.addCleanup(
            setattr,
            check_website_commands,
            "hidden_deprecated_alias_replacement",
            original_hidden_alias,
        )
        check_website_commands.hidden_deprecated_alias_replacement = (
            lambda _binary_path, _example, _root_flags: "asc validate"
        )
        original_path_help = check_website_commands.path_help
        self.addCleanup(
            setattr,
            check_website_commands,
            "path_help",
            original_path_help,
        )
        check_website_commands.path_help = (
            lambda _binary_path, path: (
                "DESCRIPTION\n"
                "  DEPRECATED: use `asc validate` for App Store submission readiness.\n\n"
                "USAGE\n"
                "  asc submit preflight [flags]\n"
            )
            if path == ("submit", "preflight")
            else ""
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            website = Path(tmpdir)
            (website / "index.mdx").write_text(
                "```bash\nasc submit preflight --app 123456789 --version 2.0 --output json\n```\n"
            )
            errors = check_website_commands.collect_errors(
                website,
                index,
                Path(tmpdir) / "asc-doc-check",
            )

        self.assertEqual(len(errors), 1)
        self.assertIn("deprecated alias", errors[0])
        self.assertNotIn("unknown flag '--app'", errors[0])
        self.assertNotIn("unknown flag '--version'", errors[0])
        self.assertNotIn("unknown flag '--output'", errors[0])

    def test_hidden_alias_flags_do_not_list_canonical_publish_appstore(self) -> None:
        self.assertNotIn(
            ("publish", "appstore"),
            check_website_commands.HIDDEN_DEPRECATED_ALIAS_FLAGS,
        )

class DocLinksTest(unittest.TestCase):
    def test_normalize_target_strips_angle_brackets_before_prefix_check(self) -> None:
        normalized = doc_links.normalize_target(
            "<https://example.com/docs>",
            allow_root_relative=True,
        )
        self.assertIsNone(normalized)


class ReleaseDocsChecksTest(unittest.TestCase):
    def test_release_docs_match_version_heading(self) -> None:
        changelog = "## Changelog\n\n### v1.2.3\n"
        self.assertTrue(check_release_docs.version_is_documented(changelog, "1.2.3"))
        self.assertTrue(check_release_docs.version_is_documented(changelog, "v1.2.3"))
        self.assertFalse(check_release_docs.version_is_documented(changelog, "1.2.4"))


class HookChecksTest(unittest.TestCase):
    def test_pre_commit_treats_root_level_mdx_and_mintlify_config_as_docs(self) -> None:
        hook = (
            Path(__file__).resolve().parents[1] / ".githooks" / "pre-commit"
        ).read_text()
        self.assertIn(
            'docs.json|.mintignore|.mintlify/*|*.mdx|cicd/*|commands/*|concepts/*|configuration/*|guides/*|resources/*)',
            hook,
        )

    def test_pre_commit_treats_docs_go_files_as_code(self) -> None:
        hook = (
            Path(__file__).resolve().parents[1] / ".githooks" / "pre-commit"
        ).read_text()
        needs_code_case = hook.split('case "$path" in')[4]
        docs_case = needs_code_case.index(
            'docs.json|.mintignore|.mintlify/*|*.mdx|cicd/*|commands/*|concepts/*|configuration/*|guides/*|resources/*|README.md|CONTRIBUTING.md|SUPPORT.md|docs/*|.github/PULL_REQUEST_TEMPLATE.md)'
        )
        go_case = needs_code_case.index("*.go|go.mod|go.sum|Makefile)")
        self.assertLess(go_case, docs_case)


if __name__ == "__main__":
    unittest.main()
