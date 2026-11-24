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
  Dry run: the answer and its evidence files were not sent to Apple; only the requirements list was read.
  ```

- Read-only mode refuses a save before the evidence files are read, and the
  first form request is refused as well, so no answer or file is sent.

One thing to know: `--dry-run` checks the answer and file types locally;
Apple's own form checks run only with `--confirm`. Because a dry run no longer
fills Apple's form, a first `--confirm` can still be refused by Apple's form,
for a field only the form asks for or a file type the form narrows further.
`--confirm` fills the form before saving and saves nothing when it is refused.
