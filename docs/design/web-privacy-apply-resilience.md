# Resilient `asc web privacy apply`

Status: implementation contract for `asc web privacy plan` and
`asc web privacy apply`. It adds no App Store Connect endpoints and no new
flags; it changes mutation ordering, adds a pre-mutation preflight, and makes
the apply receipt honest about partial outcomes.

## Problem

`apply` executed `delete` -> `update` -> `create` against Apple's iris
`appDataUsages` collection. A failure in the update or create phase left the
remote declaration missing tuples that the local file still declares, with no
receipt naming what had already committed. Separately, category, purpose, and
data-protection ids are sent back to Apple as relationship ids; a token Apple
has since deleted was indistinguishable from a live one until the mutation
itself was rejected, after earlier mutations had already committed.

## Preflight

`plan` and `apply` both read the live catalog (`appDataUsageCategories`,
`appDataUsagePurposes`, `appDataUsageDataProtections`) alongside the app's
current `dataUsages`. Every token the desired declaration would send is
classified against that catalog:

- present and `deleted: true` -> `deleted`
- absent from a non-empty catalog dimension -> `unknown`
- any token in a dimension Apple returned empty -> not classified

An empty dimension proves nothing, so it never produces a finding. This keeps
the preflight from failing every apply when a catalog endpoint degrades to an
empty page.

`plan` stays a read-only diagnostic: findings appear as `staleTokens` in JSON,
markdown, and table output, and the exit status stays `0` so drift inspection
keeps working. `apply` fails closed before the first mutation, naming each
stale token and its reason.

## Mutation order

Steps are ordered so that an interruption leaves a superset of the desired
declaration rather than a hole:

1. Prerequisite deletes.
2. Updates - an in-place linkage flip never leaves a tuple absent.
3. Creates.
4. Remaining deletes.

A delete is a prerequisite when the create that replaces it cannot coexist with
it. `DATA_NOT_COLLECTED` and collected tuples are mutually exclusive, so a plan
that adds `DATA_NOT_COLLECTED` runs every delete first, and a delete of an
existing `DATA_NOT_COLLECTED` tuple runs before collected creates. Every other
delete runs last, after the creates that make it safe.

The superset property therefore holds for every plan except one with
prerequisite deletes. Apple rejects a declaration that holds
`DATA_NOT_COLLECTED` alongside collected tuples, so no ordering can make that
transition atomic: an interruption between the prerequisite delete and its
replacement create leaves a tuple missing until a rerun. The help text and the
receipt say so rather than promising a guarantee the API cannot support.

## Receipt

`apply` reports each planned step in exactly one bucket:

- `actions`: committed, confirmed by a successful response or by the
  post-failure re-read.
- `unknownActions`: attempted with an unresolved outcome.
- `notAppliedActions`: proven not committed, or never attempted.

After a failure, `apply` re-reads `dataUsages` once and reclassifies the failed
step from remote evidence, because a 5xx can still have committed the write:

- `create`: the desired tuple present remotely -> committed, adopting the
  remote usage id; absent -> not applied.
- `delete`: the usage id still present -> not applied; the usage id gone
  and the tuple gone, or still present only under a different usage id
  with no ID-less sibling -> committed; the tuple still present with no
  usage id, or with an ID-less member beside another identified usage ->
  unknown. The planner records those leftovers as skipped deletes, so
  calling the mutation committed would set `changed: true` while the extra
  declaration remains.
- `update`: the usage id present under the target tuple key -> committed;
  present under another key -> not applied; absent entirely -> unknown.

The same re-read produces `recheck.remainingChanges`, the number of plan steps
still outstanding, so an operator knows how much a rerun has left to do.
`remainingChanges` still counts only executable leftover updates, adds, and
deletes: a skipped delete is not something a rerun can attempt. When the
re-read leaves `skippedDeletes` populated, the stderr diagnostic does not
promise that a rerun converges; those tuples have no usage id and need
manual cleanup. When the re-read itself fails, `recheck.succeeded` is
`false`, `remainingChanges` is omitted entirely rather than reported as `0`,
and attempted steps stay `unknown` rather than being reported as either
outcome.

Apple can also commit the final mutation and still fail the response. When the
re-read resolves every action and leaves no remaining change, `applied` is
`true` and the diagnostic says a rerun is a no-op. The exit stays non-zero
because the transport failure is real.

The receipt prints on stdout in the requested format even on the failure path,
and the command exits non-zero with a stderr diagnostic that carries the
sanitized mutation cause, and the re-read cause when that failed too. The
returned error is already reported, so without this the operator would never
see the session hint, HTTP status, request id, or Apple service codes. No raw
Apple response body reaches stdout or stderr.

## Idempotency

`apply` re-plans from live remote state on every invocation, so rerunning the
same file after a partial failure converges: already-committed steps are absent
from the new plan and are never re-created. A fully converged rerun performs no
mutation and reports `applied: true`, `changed: false`. A leftover
`skippedDeletes` entry is not convergence: `applied` stays `false` and the
command exits non-zero even when every executable mutation succeeded or there
were none to run. Those tuples have no usage id, so a rerun cannot delete
them.

## Compatibility

All output changes are additive: `changed`, `unknownActions`,
`notAppliedActions`, and `recheck` on the apply receipt, and `staleTokens` on
the plan payload. `applied` keeps its meaning - the whole plan committed - and
is now reachable as `false` on the partial path, including a no-mutation
apply that still has skipped deletes. `changed` is omitted when an
attempted action is still unresolved, so automation cannot read a failed
recheck as a confirmed no-op. `--allow-deletes` and `--confirm` gating is
unchanged, and no prompt is introduced.

The privacy receipts stay CLI-local structs rendered through
`d.PrintOutputWithRenderers`, matching every other command in
`internal/cli/web`. Migrating that whole family to `internal/asc/output_*.go`
is a separate, package-wide change; doing it for one subcommand inside a bug
fix would leave the group inconsistent.

## Alternatives considered

- **Rollback of committed steps.** Rejected: compensating writes are
  themselves mutations that can fail, and a half-undone rollback is less
  legible than a receipt plus a converging rerun.
- **A durable on-disk resume journal.** Rejected: remote state is the
  authority, and re-planning from it makes a journal redundant while avoiding a
  new artifact the operator must manage.
- **Failing `plan` non-zero on stale tokens.** Rejected: `plan` exists to
  inspect drift, including drift caused by a retired token, and a non-zero exit
  would break that use.
