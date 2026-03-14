#!/usr/bin/env python3
"""Generate internal/cli/schema/schema_index.json from the OpenAPI snapshot.

This produces a compact JSON index of every API endpoint with pre-resolved
parameters, request attributes, and response schema names. The index is
embedded in the asc binary for runtime schema introspection (`asc schema`).

Usage:
    python3 scripts/generate-schema-index.py [--check]
"""

import argparse
import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SPEC_PATH = REPO_ROOT / "docs" / "openapi" / "latest.json"
EMBED_PATH = REPO_ROOT / "internal" / "cli" / "schema" / "schema_index.json"


def resolve_ref(schemas: dict, ref: str):
    if not ref or not ref.startswith("#/components/schemas/"):
        return None
    return schemas.get(ref.split("/")[-1])


def resolve_parameter_ref(parameters: dict, ref: str):
    if not ref or not ref.startswith("#/components/parameters/"):
        return None
    return parameters.get(ref.split("/")[-1])


def extract_fields(schemas: dict, obj, depth: int = 0) -> dict:
    if depth > 3 or not obj:
        return {}
    if "$ref" in obj:
        resolved = resolve_ref(schemas, obj["$ref"])
        return extract_fields(schemas, resolved, depth + 1) if resolved else {}
    result = {}
    for key, val in obj.get("properties", {}).items():
        if key in ("links", "meta", "included"):
            continue
        field_type = val.get("type", "")
        if "$ref" in val:
            field_type = val["$ref"].split("/")[-1]
        enum = val.get("enum") or val.get("items", {}).get("enum")
        if enum:
            result[key] = {"type": field_type or "string", "enum": enum}
        elif field_type:
            result[key] = field_type
        else:
            result[key] = "object"
    return result


def extract_request_attributes(schemas: dict, ref: str):
    resolved = resolve_ref(schemas, ref) if ref else None
    if not resolved:
        return None
    data = resolved.get("properties", {}).get("data", {})
    if "$ref" in data:
        data = resolve_ref(schemas, data["$ref"]) or data
    attrs = data.get("properties", {}).get("attributes", {})
    fields = extract_fields(schemas, attrs)
    return fields if fields else None


def compact_parameter(parameter_components: dict, parameter: dict):
    p = parameter
    if "$ref" in p:
        resolved = resolve_parameter_ref(parameter_components, p["$ref"])
        if not resolved:
            return None
        p = resolved

    name = p.get("name")
    in_value = p.get("in")
    if not name or not in_value:
        return None

    compact: dict = {"name": name, "in": in_value}
    schema = p.get("schema", {})
    enum = schema.get("enum") or schema.get("items", {}).get("enum")
    if enum:
        compact["enum"] = enum
    if p.get("required"):
        compact["required"] = True
    return compact


def compact_parameters(
    parameter_components: dict, path_params: list, operation_params: list
) -> list[dict]:
    seen: set[tuple[str, str]] = set()
    compact: list[dict] = []

    for parameter in (path_params or []) + (operation_params or []):
        if not isinstance(parameter, dict):
            continue
        pi = compact_parameter(parameter_components, parameter)
        if not pi:
            continue
        key = (pi["name"], pi["in"])
        if key in seen:
            continue
        seen.add(key)
        compact.append(pi)

    return compact


def build_index(spec: dict) -> list[dict]:
    schemas = spec.get("components", {}).get("schemas", {})
    parameter_components = spec.get("components", {}).get("parameters", {})
    index = []

    for path, methods in spec.get("paths", {}).items():
        path_params = methods.get("parameters", [])
        for method, details in methods.items():
            if method not in ("get", "post", "patch", "delete"):
                continue

            entry: dict = {"method": method.upper(), "path": path}

            params = compact_parameters(
                parameter_components, path_params, details.get("parameters", [])
            )
            if params:
                entry["parameters"] = params

            rb = (
                details.get("requestBody", {})
                .get("content", {})
                .get("application/json", {})
                .get("schema", {})
            )
            if rb and "$ref" in rb:
                ref = rb["$ref"]
                entry["requestSchema"] = ref.split("/")[-1]
                attrs = extract_request_attributes(schemas, ref)
                if attrs:
                    entry["requestAttributes"] = attrs

            for code in ("200", "201"):
                resp = details.get("responses", {}).get(code, {})
                rs = (
                    resp.get("content", {})
                    .get("application/json", {})
                    .get("schema", {})
                )
                if rs and "$ref" in rs:
                    entry["responseSchema"] = rs["$ref"].split("/")[-1]
                    break

            index.append(entry)

    index.sort(key=lambda e: (e["path"], e["method"]))
    return index


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Generate schema index from OpenAPI spec"
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Fail if schema_index.json differs from generated output",
    )
    args = parser.parse_args()

    if not SPEC_PATH.exists():
        print(f"OpenAPI spec not found: {SPEC_PATH}", file=sys.stderr)
        return 1

    with open(SPEC_PATH) as f:
        spec = json.load(f)

    index = build_index(spec)
    generated = json.dumps(index, separators=(",", ":"), ensure_ascii=False)

    if args.check:
        embed_current = EMBED_PATH.read_text() if EMBED_PATH.exists() else ""
        if embed_current != generated:
            print("internal/cli/schema/schema_index.json is out of date.")
            print("Run: make update-schema-index")
            return 1
        print(f"schema_index.json is up to date ({len(index)} endpoints).")
        return 0

    EMBED_PATH.parent.mkdir(parents=True, exist_ok=True)
    EMBED_PATH.write_text(generated)
    print(
        f"Generated {EMBED_PATH.relative_to(REPO_ROOT)} "
        f"({len(index)} endpoints, {len(generated) // 1024} KB)"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
