#!/usr/bin/env python3
from __future__ import annotations

import tempfile
import unittest
from contextlib import redirect_stderr
from io import StringIO
from pathlib import Path
from unittest import mock

import check_agent_skills


class CheckAgentSkillsTests(unittest.TestCase):
    def make_skill(
        self,
        root: Path,
        *,
        link: str = "references/details.md",
        metadata: str | None = None,
    ) -> Path:
        skill_dir = root / ".agents" / "skills" / "sample-skill"
        (skill_dir / "agents").mkdir(parents=True)
        (skill_dir / "references").mkdir()
        (skill_dir / "references" / "details.md").write_text("# Details\n")
        (skill_dir / "SKILL.md").write_text(
            "---\n"
            "name: sample-skill\n"
            "description: Validate a representative skill when its workflow is requested.\n"
            "---\n\n"
            "# Sample\n\n"
            f"Read [details]({link}).\n",
            encoding="utf-8",
        )
        (skill_dir / "agents" / "openai.yaml").write_text(
            metadata
            if metadata is not None
            else (
                'interface:\n'
                '  display_name: "Sample Skill"\n'
                '  short_description: "Validate a representative skill"\n'
                '  default_prompt: "Use $sample-skill for this workflow."\n'
            ),
            encoding="utf-8",
        )
        return skill_dir

    def test_validate_skill_accepts_complete_skill(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            skill_dir = self.make_skill(root)
            with (
                mock.patch.object(check_agent_skills, "ROOT", root),
                mock.patch.object(check_agent_skills, "AGENTS_PATH", root / "AGENTS.md"),
            ):
                errors = check_agent_skills.validate_skill(skill_dir, "$sample-skill")
        self.assertEqual(errors, [])

    def test_validate_skill_rejects_missing_reference(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            skill_dir = self.make_skill(root, link="references/missing.md")
            with (
                mock.patch.object(check_agent_skills, "ROOT", root),
                mock.patch.object(check_agent_skills, "AGENTS_PATH", root / "AGENTS.md"),
            ):
                errors = check_agent_skills.validate_skill(skill_dir, "$sample-skill")
        self.assertTrue(any("missing local link target" in error for error in errors))

    def test_validate_skill_rejects_interface_fields_in_another_mapping(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            skill_dir = self.make_skill(
                root,
                metadata=(
                    "dependencies:\n"
                    '  display_name: "Sample Skill"\n'
                    '  short_description: "Validate a representative skill"\n'
                    '  default_prompt: "Use $sample-skill for this workflow."\n'
                    "interface:\n"
                ),
            )
            with (
                mock.patch.object(check_agent_skills, "ROOT", root),
                mock.patch.object(check_agent_skills, "AGENTS_PATH", root / "AGENTS.md"),
            ):
                errors = check_agent_skills.validate_skill(skill_dir, "$sample-skill")
        self.assertTrue(any("missing quoted display_name" in error for error in errors))

    def test_validate_skill_rejects_empty_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            skill_dir = self.make_skill(root, metadata="")
            with (
                mock.patch.object(check_agent_skills, "ROOT", root),
                mock.patch.object(check_agent_skills, "AGENTS_PATH", root / "AGENTS.md"),
            ):
                errors = check_agent_skills.validate_skill(skill_dir, "$sample-skill")
        self.assertTrue(any("missing interface mapping" in error for error in errors))

    def test_main_reports_missing_agents_file_without_traceback(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            skills_root = root / ".agents" / "skills"
            skills_root.mkdir(parents=True)
            stderr = StringIO()
            with (
                mock.patch.object(check_agent_skills, "ROOT", root),
                mock.patch.object(check_agent_skills, "SKILLS_ROOT", skills_root),
                mock.patch.object(check_agent_skills, "AGENTS_PATH", root / "AGENTS.md"),
                mock.patch.object(check_agent_skills, "EXPECTED_SKILLS", set()),
                redirect_stderr(stderr),
            ):
                exit_code = check_agent_skills.main()
        self.assertEqual(exit_code, 1)
        self.assertIn("AGENTS.md: file is missing", stderr.getvalue())

    def test_parse_frontmatter_rejects_extra_nested_fields(self) -> None:
        text = "---\nname: sample\nmetadata:\n  owner: team\n---\nbody\n"
        with self.assertRaisesRegex(ValueError, "unsupported frontmatter line"):
            check_agent_skills.parse_frontmatter(text, Path("SKILL.md"))

    def test_yaml_string_requires_quoted_value(self) -> None:
        self.assertEqual(
            check_agent_skills.yaml_string('  display_name: "Sample"\n', "display_name"),
            "Sample",
        )
        self.assertIsNone(
            check_agent_skills.yaml_string("  display_name: Sample\n", "display_name")
        )


if __name__ == "__main__":
    unittest.main()
