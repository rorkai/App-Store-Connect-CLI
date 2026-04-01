# ASC Studio

ASC Studio is the first-party desktop workspace for `asc`.

This bootstrap lives in the same repository as the CLI but remains a separate
product with its own build and release path.

## Goals

- Keep `asc` as the canonical CLI and automation surface.
- Build a rich macOS-first desktop workspace around shared Go packages.
- Make ACP the primary agent/chat integration surface.
- Keep command parity and approval-friendly mutation flows visible.

## Local development

The Studio workspace is intentionally isolated from the CLI entrypoints:

```bash
cd apps/studio/frontend
npm install
npm run test -- --run
npm run build

cd ../..
go test ./apps/studio/...
go build ./apps/studio
```

When the Wails CLI is available, run it from `apps/studio`:

```bash
cd apps/studio
wails dev
```

## Architecture notes

- `apps/studio` owns the desktop shell.
- `internal/cli/*` remains CLI-only and must not be imported here.
- Shared packages such as `internal/config`, `internal/auth`, and
  `internal/workflow` remain fair game for structured Studio features.
