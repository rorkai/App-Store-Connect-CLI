#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import posixpath
import re
import sys
from pathlib import Path

from doc_links import extract_targets, is_within_root, normalize_target


def reject_duplicate_keys(pairs: list[tuple[str, object]]) -> dict:
    keys = [key for key, _ in pairs]
    duplicates = sorted({key for key in keys if keys.count(key) > 1})
    if duplicates:
        raise ValueError(f"duplicate keys {duplicates}")
    return dict(pairs)


def load_navigation(navigation_path: Path) -> dict:
    return json.loads(navigation_path.read_text(), object_pairs_hook=reject_duplicate_keys)


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


def iter_navigation_pages(items: list) -> list[str]:
    pages: list[str] = []
    for item in items:
        if isinstance(item, str):
            pages.append(item)
        elif isinstance(item, dict):
            if isinstance(item.get("slug"), str):
                pages.append(item["slug"])
            pages.extend(iter_navigation_pages(item.get("items", [])))
    return pages


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


def check_navigation(navigation: dict, page_ids: set[str]) -> list[str]:
    errors: list[str] = []
    for page in iter_navigation_pages(navigation.get("topics", [])):
        if page not in page_ids:
            errors.append(f"navigation.json references missing page {page!r}")
    return errors


def check_redirects(navigation: dict, routes: set[str]) -> list[str]:
    errors: list[str] = []
    for source, destination in navigation.get("redirects", {}).items():
        if destination not in routes:
            errors.append(
                f"navigation.json redirect destination {destination!r} does not resolve to a page"
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
    parser = argparse.ArgumentParser(description="Validate website docs.")
    parser.add_argument(
        "--website-root",
        default="website/src/content/docs",
        help="Path to the website docs content root.",
    )
    parser.add_argument(
        "--navigation",
        default="website/navigation.json",
        help="Path to the website sidebar and redirect config.",
    )
    args = parser.parse_args(argv)

    repo_root = Path(__file__).resolve().parents[1]
    website_root = (repo_root / args.website_root).resolve()
    navigation = load_navigation(repo_root / args.navigation)
    page_ids, routes = collect_site_state(website_root)

    errors = []
    errors.extend(check_navigation(navigation, page_ids))
    errors.extend(check_redirects(navigation, routes))
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
