# OpenAPI snapshot (offline)

This folder keeps an offline snapshot of the App Store Connect OpenAPI spec for
agents that cannot access the internet.

## Files

- `latest.json`: full OpenAPI spec snapshot (see source below)
- `paths.txt`: generated path+method index for quick existence checks

## Source

Preferred sources for the OpenAPI spec:

- Official Apple download (zip): `https://developer.apple.com/sample-code/app-store-connect/app-store-connect-openapi-specification.zip`
- Community mirror that tracks Apple's published spec: `https://github.com/EvanBacon/App-Store-Connect-OpenAPI-Spec`

Note: The published OpenAPI spec can lag reality and may omit some operations that
still work in the API (parity checks can surface these gaps).

## Update process

1. Replace `latest.json` with a newer spec file.
2. Run `scripts/update-openapi-index.py` to regenerate `paths.txt`.
3. Update the "Last synced" date below.

Last synced: 2026-03-11
