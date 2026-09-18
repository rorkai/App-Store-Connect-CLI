# Experimental signing sync remote stores

## Placement and invocation

This change extends the existing `asc signing sync push` and
`asc signing sync pull` commands. It adds no new command group and no second
encryption format. Encrypted git remains the default and keeps its current
invocation, flags, and output.

```sh
# Default, unchanged.
asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
  --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

# Experimental GitLab Secure Files.
asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
  --storage gitlab-secure-files --gitlab-project 1234 --prefix asc-signing \
  --gitlab-token-file ~/.config/asc/gitlab-token \
  --password-file ~/.config/asc/signing-sync-password

# Experimental AWS Secrets Manager.
asc signing sync pull --storage aws-secrets-manager --region us-east-1 \
  --prefix asc-signing --password-file ~/.config/asc/signing-sync-password \
  --output-dir ./signing
```

## Storage selection

`--storage` accepts `git` (default), `gitlab-secure-files`, and
`aws-secrets-manager`. Any other value is a usage error; no unsupported value
is silently ignored.

| Storage | Required locators | Rejected locators |
| --- | --- | --- |
| `git` | `--repo` | `--prefix`, `--region`, `--gitlab-*` |
| `gitlab-secure-files` | `--gitlab-project`, `--prefix`, `--gitlab-token-file` | `--repo`, `--branch`, `--region` |
| `aws-secrets-manager` | `--prefix`, `--region` | `--repo`, `--branch`, `--gitlab-*` |

`--gitlab-host` is optional and defaults to `https://gitlab.com`. It must be an
https URL without embedded credentials. The GitLab token is read only through
the existing protected secret-file helper, is sent only in the `PRIVATE-TOKEN`
request header, and never appears in output, diagnostics, or errors. AWS
credentials come from the standard AWS environment that the SDK already
expects; this change adds no credential file format.

`asc signing sync rotate-password` remains git-only. Invoking it with any other
storage returns a usage error rather than a partial rotation.

## Artifact model

Every backend transports already-encrypted bytes. Push still encrypts into an
isolated temporary working tree with the existing envelope, metadata,
repository path validation, artifact count limit, and cumulative size limit;
pull still decrypts and validates from that tree. The storage backend only
replaces the clone and publish steps:

- `git`: clone, commit, push, unchanged.
- `gitlab-secure-files`: download every secure file under the prefix, then
  upload or replace changed artifacts.
- `aws-secrets-manager`: read every prefixed secret, then create or update the
  corresponding secrets.

Each remote request is bounded by the shared CLI request timeout, and GitLab
uploads use the shared upload timeout, so one stalled request cannot hang a
multi-artifact push or pull while the batch itself stays uncapped. AWS
credential discovery shares the same budget because the default provider chain
can reach container or instance metadata over the network.

Artifact-count limits apply to the configured prefix, not to the whole remote
namespace, so a project or account holding unrelated files still syncs. GitLab
pagination is bounded separately by a page budget.

A stored object is named `<prefix>/<encrypted relative path>.enc`. The relative
path passes the existing encrypted-path validator, and remote names are
additionally rejected when they contain a traversal component, so a hostile
remote name cannot escape the working tree.

## GitLab Secure Files

The GitLab API v4 project-level secure files endpoints are used directly with
`net/http`; no GitLab SDK is added.

- `GET /api/v4/projects/:id/secure_files` lists files, paginated through
  `X-Next-Page`.
- `POST /api/v4/projects/:id/secure_files` uploads `name` and `file`. GitLab
  stores the record under the required `name` attribute, so the full scoped
  name including the prefix and relative path is preserved; the multipart part
  filename is not the stored name. The upload response is checked against the
  requested name, so an instance that stored the artifact elsewhere fails the
  push instead of leaving an artifact a later pull cannot find.
- `GET /api/v4/projects/:id/secure_files/:id/download` returns file content.
- `DELETE /api/v4/projects/:id/secure_files/:id` removes a file.

The create endpoint documents `name` and `file` as separate required
attributes, and the upstream implementation builds the record with
`secure_files.new(name: params[:name])` while assigning the uploaded part to
`secure_file.file`. The stored name therefore carries the prefix and relative
path, and the record has no format restriction beyond presence, per-project
uniqueness, and a path-traversal check that the existing encrypted-path
validator already satisfies.

Secure file names are unique per project and the API has no replace operation,
so an artifact whose content changed is deleted and re-uploaded. The previous
ciphertext is downloaded and checksum-verified first and is re-uploaded if the
replacement upload fails, so a transient failure cannot leave the artifact
missing. An artifact whose published sha256 checksum already matches the local
ciphertext is skipped, so unchanged pushes perform no deletion. Files outside
the configured prefix are never listed into scope, replaced, or deleted, and no
cleanup of unknown files is performed.

Failures are closed: a non-JSON or HTML response, a redirect to another host or
scheme, a checksum mismatch, an artifact above the existing encrypted size
limit, or an artifact above GitLab's documented 5 MiB secure file limit stops
the operation. Errors carry the HTTP status and GitLab's public error title
after control characters are removed, the title is truncated, and any token
occurrence is redacted.

## AWS Secrets Manager

One secret holds one encrypted artifact. The secret string is the ciphertext in
standard base64 so the SDK's string secret round-trips losslessly. `CreateSecret`
publishes a new artifact, and `PutSecretValue` updates an existing one, also as
the fallback when a concurrent writer already created the secret. An existing
secret whose value already matches is skipped so repeated pushes do not consume
the account's secret version quota. No secret is deleted.

An artifact whose base64 encoding exceeds 60 KiB fails before any AWS call and
explains the limit rather than truncating. Secret names are validated against
the documented AWS character set, so an artifact path that AWS cannot name
fails closed instead of being silently renamed.

## Output and exit codes

Structured output keeps its existing shape. `repoUrl` carries the redacted git
remote for git storage and a non-secret locator for remote storage
(`gitlab-secure-files://host/projects/<id>/<prefix>` or
`aws-secrets-manager://<region>/<prefix>`). Data goes to stdout and progress to
stderr. Invalid flag combinations use exit code 2 and operational failures use
exit code 1.

## Tests

RED-GREEN coverage includes an httptest GitLab server that round-trips
encrypted bytes through push and pull, asserts the token header, asserts that
errors keep the status and title without the token, and rejects HTML responses,
cross-host redirects, oversize downloads, oversize uploads, out-of-prefix
names, and traversal names. AWS coverage uses a stub client so no test dials
AWS and asserts create/put names, base64 round-trips, that no plaintext is
stored, and that an oversize artifact fails before the first AWS call. Command
coverage asserts the storage selection matrix, the experimental help text, the
protected token-file contract, and that rotation stays git-only. Existing git
signing sync tests are unchanged and still pass.

## Alternatives

Adding a second encryption path per backend was rejected: the backends
transport ciphertext only, so encryption, metadata authentication, and path
validation stay in one place. Deleting remote artifacts that no longer exist
locally was rejected because a shared store may hold artifacts written by other
teams or tools. Implementing rotation for remote stores was rejected for now
because rotation depends on an atomic whole-store replacement that neither
backend provides; a partial implementation would risk a half-rotated store.
