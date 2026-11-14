#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("update-openapi-index.py")
SPEC = importlib.util.spec_from_file_location("update_openapi_index", MODULE_PATH)
assert SPEC and SPEC.loader
update_openapi_index = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(update_openapi_index)


class UpdateOpenAPIIndexTests(unittest.TestCase):
    def test_generate_lines_is_sorted_and_filters_non_operations(self) -> None:
        spec = {
            "paths": {
                "/z": {"post": {}, "parameters": []},
                "/a": {"get": {}},
            }
        }
        self.assertEqual(
            update_openapi_index.generate_lines(spec),
            ["GET /a", "POST /z"],
        )

    def run_main(self, root: Path, *args: str) -> tuple[int, str]:
        stdout = StringIO()
        with (
            mock.patch.object(update_openapi_index, "SPEC_PATH", root / "latest.json"),
            mock.patch.object(update_openapi_index, "OUT_PATH", root / "paths.txt"),
            mock.patch.object(update_openapi_index, "WEB_SPEC_PATH", root / "web.json"),
            mock.patch.object(update_openapi_index, "WEB_OUT_PATH", root / "web-paths.txt"),
            mock.patch.object(sys, "argv", ["update-openapi-index.py", *args]),
            redirect_stdout(stdout),
        ):
            exit_code = update_openapi_index.main()
        return exit_code, stdout.getvalue()

    def test_check_fails_for_stale_index_without_rewriting(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "latest.json").write_text(json.dumps({"paths": {"/v1/apps": {"get": {}}}}))
            (root / "paths.txt").write_text("GET /v1/builds\n")
            web_spec = {"paths": {"/iris/v1/apps": {"get": {}}}}
            (root / "web.json").write_text(update_openapi_index.canonical_text(web_spec))
            (root / "web-paths.txt").write_text("GET /iris/v1/apps\n")

            exit_code, stdout = self.run_main(root, "--check")

            self.assertEqual(exit_code, 1)
            self.assertEqual((root / "paths.txt").read_text(), "GET /v1/builds\n")
            self.assertIn("paths.txt is out of date", stdout)

    def test_web_schema_must_be_canonical_and_update_rewrites_it(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "latest.json").write_text(json.dumps({"paths": {"/v1/apps": {"get": {}}}}))
            (root / "paths.txt").write_text("GET /v1/apps\n")
            unsorted = json.dumps({"paths": {"/z": {"get": {}}, "/a": {"post": {}}}})
            (root / "web.json").write_text(unsorted)
            (root / "web-paths.txt").write_text("GET /z\nPOST /a\n")

            exit_code, stdout = self.run_main(root, "--check")
            self.assertEqual(exit_code, 1)
            self.assertIn("not in canonical form", stdout)
            self.assertEqual((root / "web.json").read_text(), unsorted)

            self.assertEqual(self.run_main(root)[0], 0)
            self.assertEqual(list(json.loads((root / "web.json").read_text())["paths"]), ["/a", "/z"])
            self.assertEqual(self.run_main(root, "--check")[0], 0)


if __name__ == "__main__":
    unittest.main()
