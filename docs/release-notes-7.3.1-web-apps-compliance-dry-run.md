# `asc web apps compliance set --dry-run` no longer sends the answer or evidence to Apple

Fixed in 7.3.1. Before this fix, `asc web apps compliance set --dry-run`, and
the same command under read-only mode (`ASC_READ_ONLY` or `--read-only`), still
sent the compliance form requests to Apple, including evidence files. Each step
of Apple's App Compliance form (`POST .../requirements/{requirementId}/metadata`)
carries the draft answer, and the last step carries the `--proof`, `--report`
and `--third-party-authorization` files, base64-encoded. Those requests were
treated as reads, so neither a dry run nor read-only mode stopped them. Nothing
was saved as a declaration, but the files left the machine.

Now:

- `--dry-run` only reads the requirement (`GET .../requirements`), refuses a
  requirement pending verification, and checks each file's type locally.
  It sends no form request and no file, and stderr says so:

  ```text
  Dry run: nothing was sent to Apple; the answer and its evidence files were checked locally and not uploaded.
  ```

- Read-only mode refuses the command at the first form request, before any
  answer or file is sent.

One thing to know: because a dry run no longer fills Apple's form, it cannot
report a field that only the form asks for, or a file type the form narrows
further. `--confirm` still fills the form first and saves nothing if it cannot
answer every field.
