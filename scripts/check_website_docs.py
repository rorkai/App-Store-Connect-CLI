#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import posixpath
import re
import sys
from pathlib import Path

from doc_links import extract_targets, is_within_root, normalize_target


def load_docs_json(website_root: Path) -> dict:
    return json.loads((website_root / "docs.json").read_text())


def route_for_path(path_str: str) -> str:
    path_str = re.sub(r"\.mdx?$", "", path_str.strip("/"))
    if path_str == "index":
        return "/"
    if path_str.endswith("/index"):
        path_str = path_str[: -len("/index")]
    return "/" if not path_str else f"/{path_str}"


def collect_site_state(website_root: Path) -> tuple[set[str], set[str]]:
    page_ids: set[str] = set()
    routes: set[str] = set()
    for file in website_root.rglob("*.mdx"):
        rel = file.relative_to(website_root)
        page_id = rel.with_suffix("").as_posix()
        page_ids.add(page_id)
        routes.add(route_for_path(page_id))
    return page_ids, routes


def iter_navigation_pages(node: object) -> list[str]:
    pages: list[str] = []
    if isinstance(node, dict):
        for key, value in node.items():
            if key == "pages" and isinstance(value, list):
                for page in value:
                    if isinstance(page, str):
                        pages.append(page)
                    else:
                        pages.extend(iter_navigation_pages(page))
            else:
                pages.extend(iter_navigation_pages(value))
    elif isinstance(node, list):
        for item in node:
            pages.extend(iter_navigation_pages(item))
    return pages


def iter_redirects(config: dict) -> list[tuple[str, str]]:
    redirects = []
    for item in config.get("redirects", []):
        if isinstance(item, dict):
            source = item.get("source")
            destination = item.get("destination")
            if isinstance(source, str) and isinstance(destination, str):
                redirects.append((source, destination))
    return redirects


def resolve_route(source_page_id: str, target: str) -> tuple[str, str]:
    if target.startswith("/"):
        normalized = target
    else:
        base_dir = posixpath.dirname(source_page_id)
        normalized = posixpath.normpath(posixpath.join(base_dir, target))
    normalized = normalized.lstrip("/")
    suffix = Path(normalized).suffix.lower()
    if suffix and suffix not in {".md", ".mdx"}:
        return "asset", normalized
    return "route", route_for_path(normalized)


def check_navigation(website_root: Path, page_ids: set[str]) -> list[str]:
    errors: list[str] = []
    config = load_docs_json(website_root)
    for page in iter_navigation_pages(config):
        if page not in page_ids:
            errors.append(f"docs.json references missing page {page!r}")
    return errors


def check_redirects(website_root: Path, routes: set[str]) -> list[str]:
    errors: list[str] = []
    config = load_docs_json(website_root)
    seen_sources: set[str] = set()
    for source, destination in iter_redirects(config):
        if source in seen_sources:
            errors.append(f"docs.json contains duplicate redirect source {source!r}")
            continue
        seen_sources.add(source)
        if destination not in routes:
            errors.append(
                f"docs.json redirect destination {destination!r} does not resolve to a page"
            )
    return errors


def check_internal_links(website_root: Path, routes: set[str]) -> list[str]:
    errors: list[str] = []
    website_root = website_root.resolve()
    for file in website_root.rglob("*.mdx"):
        rel = file.relative_to(website_root)
        source_page_id = rel.with_suffix("").as_posix()
        for target in extract_targets(file.read_text()):
            normalized = normalize_target(target, allow_root_relative=True)
            if normalized is None:
                continue
            kind, resolved = resolve_route(source_page_id, normalized)
            if kind == "asset":
                asset_path = (website_root / resolved).resolve()
                if not is_within_root(website_root, asset_path):
                    errors.append(f"{rel}: website asset {normalized!r} escapes website root")
                    continue
                if not asset_path.exists():
                    errors.append(f"{rel}: missing website asset {normalized!r}")
                continue
            if resolved not in routes:
                errors.append(f"{rel}: missing website route {normalized!r}")
    return errors


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate Mintlify website docs.")
    parser.add_argument(
        "--website-root",
        default=".",
        help="Path to the Mintlify docs root.",
    )
    args = parser.parse_args(argv)

    repo_root = Path(__file__).resolve().parents[1]
    website_root = (repo_root / args.website_root).resolve()
    page_ids, routes = collect_site_state(website_root)

    errors = []
    errors.extend(check_navigation(website_root, page_ids))
    errors.extend(check_redirects(website_root, routes))
    errors.extend(check_internal_links(website_root, routes))
    if errors:
        print("Website docs validation failed:", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1

    print(f"Website docs validation passed for {len(page_ids)} page(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
