# Verify release provenance

New releases attest the five final binaries and their checksum manifest before
making the draft public. macOS subjects contain the signed and notarized bytes.
The publication job downloads the actual draft assets and verifies every subject,
the expected release source commit, and the signing workflow commit before
publishing. A failed verification leaves the release as a draft. An internal
release identity record is a seventh subject in the same signed statement. It binds the six public
files to the expected repository, version, release source and workflow commits.

Use a trusted checkout containing `scripts/release_provenance.py` and a current
GitHub CLI with `gh attestation verify`. Set the two full commit SHAs from the
reviewed release source and workflow run; they can differ when a newer workflow
qualifies a candidate built from an older tag. Do not derive expected identities
only from the attestation being checked. Select the verifier from that workflow
commit, including when a recovery workflow reuses a candidate from an older tag.
The helper at the release tag may implement the earlier provenance format.

```bash
version=7.8.0
source_sha=REVIEWED_FULL_RELEASE_COMMIT_SHA
workflow_sha=REVIEWED_FULL_WORKFLOW_COMMIT_SHA
verifier_dir=../asc-release-verifier
git fetch origin "$workflow_sha"
git worktree add --detach "$verifier_dir" "$workflow_sha"
gh release download "$version" --repo rorkai/App-Store-Connect-CLI --dir "release-$version"
python3 "$verifier_dir/scripts/release_provenance.py" verify \
  --release-dir "release-$version" --version "$version" \
  --source-sha "$source_sha" --workflow-sha "$workflow_sha"
```

The verifier checks the exact asset set and checksums, then uses GitHub CLI to
verify each asset's digest, signature, repository, release workflow, and workflow
commit. It recomputes the release identity record and requires its digest plus
all six public-file digests in a matching verified SLSA statement for each file.
`--bundle PATH` optionally uses a previously downloaded Sigstore bundle instead
of looking up attestations by artifact digest. The release workflow retains its bundle in a separate workflow artifact; the public
attestation service retains the attestations by subject digest.

For an individual binary, the signature and workflow checks can be run directly:

```bash
gh attestation verify "release-$version/asc_${version}_linux_amd64" \
  --repo rorkai/App-Store-Connect-CLI \
  --signer-workflow rorkai/App-Store-Connect-CLI/.github/workflows/release.yml \
  --signer-digest "$workflow_sha" \
  --predicate-type https://slsa.dev/provenance/v1 --deny-self-hosted-runners
```

Use the Python verifier to enforce the release identity and complete subject set
in addition to these individual signature checks. GitHub CLI's `--source-digest`
refers to the workflow's OIDC source identity, which can differ from the checked-out release
source. The Python verifier binds the candidate source through the co-signed
identity record.

## Release identity record

The pinned attestation action generates standard GitHub Actions SLSA provenance
using `https://actions.github.io/buildtypes/workflow/v1`. It signs the six public
release files and `release-identity.json` together in one statement.

`release_provenance.py identity` writes this record with keys sorted, compact JSON
separators and one trailing newline. The verifier independently derives those
exact bytes from the operator's expected identities. The record stays in the
attestation job; it is recomputed for verification and adds no public release
asset. A representative record contains these fields:

```json
{
  "repository": "rorkai/App-Store-Connect-CLI",
  "sourceCommit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "version": "7.8.0",
  "workflowCommit": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
}
```

The standard provenance identifies the executing workflow. `sourceCommit`
identifies the reviewed candidate bytes. Recovered signed candidates retain
that original source identity while `workflowCommit` identifies the recovery
workflow that qualifies and attests them. Verification checks the signer workflow
and its exact commit, then checks all seven subject names and digests within each
verified statement. Separate statements for the identity and binaries are rejected.

The predicate is a signed claim made by the release workflow. GitHub's
certificate authenticates the workflow identity; it does not independently
validate the candidate's source marker. No SLSA Build level or reproducible-build
guarantee is claimed, and dependencies are not a complete software bill of
materials. Historical releases are not retroactively attested; repairing their
package-manager publication keeps its existing behavior.

Retained candidate ZIPs and release TARs are validated before their files are
materialized in a fresh temporary directory, separate from workflow tools. Only
the expected regular files are accepted. Processing is bounded to 256 MiB per
binary, 1 GiB total asset bytes, 16 KiB for the checksum manifest, and 1 GiB plus
1 MiB for each archive. Candidate source markers and all asset checksums must
match before the validated release directory is handed to subsequent steps.
Native macOS TAR producers omit AppleDouble metadata; published downloads keep
their ordinary non-executable file modes.

`python3 scripts/test_release_provenance.py` exercises the publication shell with
local fixtures, including changed bytes, a wrong source commit and identity
records signed separately from the binaries. It runs under
the read-only release rehearsal's `release-guardrails` target. These fixtures do
not sign or publish attestations and do not validate hosted OIDC issuance.

The workflow uses GitHub's [artifact attestation action](https://github.com/actions/attest)
and [verification policy](https://cli.github.com/manual/gh_attestation_verify).
