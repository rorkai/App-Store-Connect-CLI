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

    def test_check_fails_for_stale_index_without_rewriting(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            spec_path = root / "latest.json"
            out_path = root / "paths.txt"
            spec_path.write_text(json.dumps({"paths": {"/v1/apps": {"get": {}}}}))
            out_path.write_text("GET /v1/builds\n")

            stdout = StringIO()
            with (
                mock.patch.object(update_openapi_index, "SPEC_PATH", spec_path),
                mock.patch.object(update_openapi_index, "OUT_PATH", out_path),
                mock.patch.object(sys, "argv", ["update-openapi-index.py", "--check"]),
                redirect_stdout(stdout),
            ):
                exit_code = update_openapi_index.main()

            self.assertEqual(exit_code, 1)
            self.assertEqual(out_path.read_text(), "GET /v1/builds\n")
            self.assertIn("paths.txt is out of date", stdout.getvalue())


if __name__ == "__main__":
    unittest.main()
