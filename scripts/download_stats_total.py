#!/usr/bin/env python3
"""
Combined install/download signals: GitHub release assets + Homebrew analytics.

GitHub counts all-time downloads of *uploaded release assets* only.
Homebrew publishes trailing 30d / 90d / 365d install-on-request counts (not all-time).

See: https://til.bhupesh.me/shell/get-download-stats-github-brew

Usage:
  python3 scripts/download_stats_total.py
  python3 scripts/download_stats_total.py --write-badge docs/badges/installs-total.json
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.request
from pathlib import Path

REPO = "rorkai/App-Store-Connect-CLI"
FORMULA = "asc"
WRITEUP = "https://til.bhupesh.me/shell/get-download-stats-github-brew"


def _github_api_headers() -> dict[str, str]:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "asc-download-stats",
    }
    token = os.getenv("GITHUB_TOKEN", "").strip()
    if token:
        headers["Authorization"] = f"Bearer {token}"
        headers["X-GitHub-Api-Version"] = "2022-11-28"
    return headers


def _coerce_count(value: object) -> int | None:
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value) if value.is_integer() else None
    if value is None:
        return None

    text = str(value).strip()
    if not text:
        return None
    try:
        return int(text)
    except ValueError:
        try:
            parsed = float(text)
        except ValueError:
            return None
        return int(parsed) if parsed.is_integer() else None


def _sum_analytics_counts(block: dict[str, object]) -> int:
    total = 0
    for value in block.values():
        count = _coerce_count(value)
        if count is not None:
            total += count
    return total


def github_release_asset_downloads() -> int:
    base = f"https://api.github.com/repos/{REPO}/releases"
    total = 0
    page = 1
    headers = _github_api_headers()
    while True:
        url = f"{base}?per_page=100&page={page}"
        req = urllib.request.Request(url, headers=headers)
        with urllib.request.urlopen(req, timeout=60) as r:
            batch = json.load(r)
        if not batch:
            break
        for rel in batch:
            for a in rel.get("assets") or []:
                total += int(a.get("download_count") or 0)
        if len(batch) < 100:
            break
        page += 1
    return total


def homebrew_install_on_request() -> dict[str, int]:
    url = f"https://formulae.brew.sh/api/formula/{FORMULA}.json"
    req = urllib.request.Request(url, headers={"User-Agent": "asc-download-stats"})
    with urllib.request.urlopen(req, timeout=30) as r:
        data = json.load(r)
    ior = (data.get("analytics") or {}).get("install_on_request") or {}
    out = {}
    for window in ("30d", "90d", "365d"):
        block = ior.get(window) or {}
        out[window] = _sum_analytics_counts(block)
    return out


def _format_badge_total(n: int) -> str:
    # k-suffix rounds e.g. 999_950 → "1000.0k"; use M from ~1M onward.
    if n >= 999_500:
        s = f"{n / 1e6:.1f}M"
        return s.replace(".0M", "M")
    if n >= 1_000:
        s = f"{n / 1000:.1f}k"
        return s.replace(".0k", "k")
    return str(n)


def write_shields_endpoint_badge(path: str) -> int:
    """Write Shields endpoint JSON (GitHub assets all-time + Homebrew 365d)."""
    gh = github_release_asset_downloads()
    brew = homebrew_install_on_request()
    brew365 = brew.get("365d", 0)
    total = gh + brew365
    payload = {
        "schemaVersion": 1,
        "label": "total",
        "message": _format_badge_total(total),
        "color": "brightgreen",
    }
    out = Path(path)
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2)
        f.write("\n")
    print(f"Wrote {path}: GitHub {gh:,} + Homebrew 365d {brew365:,} → {payload['message']}", file=sys.stderr)
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description="GitHub + Homebrew install signals")
    ap.add_argument(
        "--write-badge",
        metavar="PATH",
        help="Write Shields endpoint JSON for README badge",
    )
    args = ap.parse_args()
    if args.write_badge:
        try:
            return write_shields_endpoint_badge(args.write_badge)
        except Exception as e:  # noqa: BLE001
            print(f"error: {e}", file=sys.stderr)
            return 1

    try:
        gh = github_release_asset_downloads()
    except Exception as e:  # noqa: BLE001
        print(f"GitHub: error ({e})", file=sys.stderr)
        gh = -1
    try:
        brew = homebrew_install_on_request()
    except Exception as e:  # noqa: BLE001
        print(f"Homebrew: error ({e})", file=sys.stderr)
        brew = {}

    print("asc — combined download / install signals")
    print("=" * 50)
    if gh >= 0:
        print(f"  GitHub release assets (all-time):     {gh:,}")
    if brew:
        print(f"  Homebrew install-on-request (30d):     {brew.get('30d', 0):,}")
        print(f"  Homebrew install-on-request (90d):     {brew.get('90d', 0):,}")
        print(f"  Homebrew install-on-request (365d):    {brew.get('365d', 0):,}")
    print()
    print("Caveats:")
    print("  • Different time windows (GitHub is cumulative; Homebrew is trailing year max).")
    print("  • Same person may use both GitHub + Homebrew — do not sum for “unique users”.")
    print("  • Install script / Linux / CI may bypass both counters.")
    print()
    if gh >= 0 and brew:
        naive = gh + brew.get("365d", 0)
        print(f"  Naïve sum (GitHub all-time + Homebrew 365d): ~{naive:,}")
        print("    (overcounts; use only as a rough visibility ceiling.)")
    print()
    print(f"Method write-up: {WRITEUP}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
