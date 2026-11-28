# Verify release provenance

New releases attest the five final binaries and their checksum manifest before
making the draft public. macOS subjects contain the signed and notarized bytes.
The publication job downloads the actual draft assets and verifies every subject,
the expected release source commit, and the signing workflow commit before
publishing. A failed verification leaves the release as a draft.

Use a trusted checkout containing `scripts/release_provenance.py` and a current
GitHub CLI with `gh attestation verify`. Set the two full commit SHAs from the
reviewed release source and workflow run; they can differ when a newer workflow
qualifies a candidate built from an older tag. Do not derive expected identities
only from the attestation being checked.

```bash
version=7.8.0
source_sha=REVIEWED_FULL_RELEASE_COMMIT_SHA
workflow_sha=REVIEWED_FULL_WORKFLOW_COMMIT_SHA
gh release download "$version" --repo rorkai/App-Store-Connect-CLI --dir "release-$version"
python3 scripts/release_provenance.py verify \
  --release-dir "release-$version" --version "$version" \
  --source-sha "$source_sha" --workflow-sha "$workflow_sha"
```

The verifier checks the exact asset set and checksums, then uses GitHub CLI to
verify each asset's digest, signature, repository, release workflow, and workflow
commit. It also checks the signed SLSA predicate's release version and source
dependency. `--bundle PATH` optionally uses a previously downloaded Sigstore
bundle instead of looking up attestations by artifact digest. The release
workflow retains its bundle in a separate workflow artifact; the public
attestation service retains the attestations by subject digest.

For an individual binary, the signature and workflow checks can be run directly:

```bash
gh attestation verify "release-$version/asc_${version}_linux_amd64" \
  --repo rorkai/App-Store-Connect-CLI \
  --signer-workflow rorkai/App-Store-Connect-CLI/.github/workflows/release.yml \
  --signer-digest "$workflow_sha" \
  --predicate-type https://slsa.dev/provenance/v1 --deny-self-hosted-runners
```

This individual command does not enforce the release source dependency; use the
Python verifier for that policy. GitHub CLI's `--source-digest` refers to the
workflow's OIDC source identity, which can differ from the checked-out release
source. It is not a replacement for checking `resolvedDependencies` here.

## Release candidate v1

The build type is
`https://github.com/rorkai/App-Store-Connect-CLI/blob/main/docs/RELEASE_PROVENANCE.md#release-candidate-v1`.
It describes qualification of a release candidate by `.github/workflows/release.yml`:
resolve the version to a source commit, build or recover the qualified candidate,
validate its source marker and canonical assets, then attest the final bytes.
The workflow runs on a version-tag push or a manual dispatch with `version`.
Recovered candidates retain their original source identity; the attesting
workflow does not claim to have rebuilt them in the current attempt.

The predicate uses the [SLSA provenance v1 schema](https://slsa.dev/spec/v1.2/build-provenance).
`externalParameters` contains only the numeric `x.y.z` version;
`internalParameters` is absent. The source dependency URI is the repository's
Git URL plus `@refs/tags/` and that version, with its resolved Git commit digest.
The builder identity identifies the release workflow at the executing workflow
commit. Consumers accept this builder only with a matching GitHub Actions
certificate, repository, workflow path and workflow digest.

```json
{
  "buildDefinition": {
    "buildType": "https://github.com/rorkai/App-Store-Connect-CLI/blob/main/docs/RELEASE_PROVENANCE.md#release-candidate-v1",
    "externalParameters": {"version": "7.8.0"},
    "resolvedDependencies": [{
      "uri": "git+https://github.com/rorkai/App-Store-Connect-CLI@refs/tags/7.8.0",
      "digest": {"gitCommit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
    }]
  },
  "runDetails": {
    "builder": {
      "id": "https://github.com/rorkai/App-Store-Connect-CLI/.github/workflows/release.yml@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    }
  }
}
```

The predicate is a signed claim made by the release workflow. GitHub's
certificate authenticates the workflow identity; it does not independently
validate the candidate's source marker. No SLSA Build level or reproducible-build
guarantee is claimed, and dependencies are not a complete software bill of
materials. Historical releases are not retroactively attested; repairing their
package-manager publication keeps its existing behavior.

`python3 scripts/test_release_provenance.py` exercises the publication shell with
local fixtures, including changed bytes and a wrong source commit. It runs under
the read-only release rehearsal's `release-guardrails` target. These fixtures do
not sign or publish attestations and do not validate hosted OIDC issuance.

The workflow uses GitHub's [artifact attestation action](https://github.com/actions/attest)
and [verification policy](https://cli.github.com/manual/gh_attestation_verify).
