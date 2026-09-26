# Rooted identity-checked file mutations

## Scope

This follow-up hardens the rooted file operations used by the Xcode signing
transaction. It is an internal filesystem contract, not a new CLI command or
App Store Connect API. The existing `asc xcode signing plan` and `apply`
invocations and their output contracts remain unchanged.

The transaction must carry a descriptor-backed identity from preparation to
publication and rollback. A caller-supplied `os.FileInfo` is only a snapshot;
it does not keep an inode or Windows file ID alive and cannot be the strict
rollback token.

## Contract

`internal/rootfs` exposes an opaque identity value whose fields remain private
to the package. The value owns a rooted no-follow descriptor or handle, the
identity observed through that descriptor, expected bytes, and the metadata
needed for a replacement. It is valid until the owning `Root` is closed; all
copies of a `Root` share the lifetime, and close is idempotent.

```go
type FileIdentity struct { /* unexported descriptor/handle and snapshot */ }

func (r Root) CaptureFile(name string) (*FileIdentity, error)

func (r Root) CaptureFileLimited(name string, limit int64) (*FileIdentity, error)

func (r Root) ReleaseFileIdentity(identity *FileIdentity) error

func (r Root) ReplaceFileIfSame(
	name string,
	expected *FileIdentity,
	data []byte,
	perm os.FileMode,
	preserveMetadata bool,
) (installed *FileIdentity, err error)

func (r Root) RemoveFileIfSameIdentity(name string, expected *FileIdentity) error

func (r Root) CreateNewFileAtomicWithIdentity(
	name string,
	data []byte,
	perm os.FileMode,
) (*FileIdentity, error)
```

On supported platforms, publication retains the staging descriptor before any
later destination observation. The operation returns success only after descriptor,
content, metadata, and final rooted-entry checks prove that the staged inode is
still installed. Once publication has succeeded, its identity also accompanies
any later observation, cleanup, or durability error so the caller can target
only that inode during recovery. If retaining the identity fails, the operation
returns a nil identity with `ErrFilePublicationUncertain`;
callers must then preserve transaction evidence and must not perform a
path-based rollback. Default capture, identity-backed mutation, and retained
publication data are bounded at 8 MiB, matching the existing Xcode signing-plan
input limit. `CaptureFileLimited` can explicitly capture up to 16 MiB for a
caller with a larger bounded input contract without widening mutation or
publication limits, and refuses oversize files with
`ErrFileIdentityDataTooLarge`. Oversize identity-backed replacements fail
before mutation. Capture and verification use repeated bounded reads plus
descriptor and rooted-entry observations so an overlapping in-place write,
path replacement, permission change, or late transition to or from a multiply
linked file fails closed. Verification also compares the complete file mode,
including the setuid, setgid and sticky bits, and the owning user and group,
because a `chmod` of a special bit or a `chown` changes neither the ordinary
permission bits nor the modification time. `preserveMetadata` would otherwise
carry drifted ownership onto the replacement and silently drop the special
bits. `preserveMetadata` deliberately preserves those special bits: after ACL/xattr copying and content writes, the complete source mode is reapplied immediately before publication. Access-control lists and extended attributes are copied from the
identity-retained descriptor and are included in the bounded strict identity
snapshot. Darwin and Linux strict verification re-reads descriptor-backed ACL
and xattr digests before any entry move; legacy `FileInfo` adapters retain
their prior compatibility boundary, and platforms without the descriptor
metadata facilities do not claim this stronger check.

The historical `os.FileInfo` methods remain compatibility adapters and do not
inherit the strict 8 MiB input-snapshot limit. The existing `WithInfo` forms
retain a verified publication descriptor until `Root.Close` when the platform
can keep that descriptor open or recover it after publication. Windows may
require the staging descriptor to close before publication. If the published
destination cannot then be reopened and verified, a `WithInfo` method returns
nil metadata with the observation error instead of claiming an identity it no
longer holds. The basic forms do not add retained descriptors. The legacy
hard-link fallback marks the destination as published as soon as its link
succeeds. If removal of the private staging link then fails, that entry is
preserved as evidence and deferred cleanup does not retry an unchecked pathname
removal. New transaction code must use the descriptor-backed methods above.

A caller that supersedes many inventory identities can release each old token
after successful replacement with `ReleaseFileIdentity`. Release checks root
ownership, is idempotent, and makes the old token unusable. The replacement
token remains retained until explicitly released or `Root.Close`.

## Bounded media removal

`RemoveFileIfSHA256Same(name, expectedSize, expectedDigest)` supports recorded
exported media larger than the byte capture limit. It streams SHA-256 with an
exact size bound and one sentinel byte, retains the opened descriptor, and uses
the same metadata, hard-link, quarantine, recovery, and final content checks.
The private removal-only identity never leaves the operation; byte publication
APIs reject it. Its retained descriptor is released on every exit. Existing
8 MiB mutation and 16 MiB explicit capture limits remain unchanged.

The operation is synchronous, like the existing rooted primitives, and performs
repeated streaming verification rather than retaining the media in memory.
Callers check cancellation between files. Windows returns
`ErrFileIdentityMutationUnsupported` without moving the file. The portable
close-to-unlink boundary described below still applies.

## Platform boundary and recovery

Linux `renameat2(RENAME_NOREPLACE)` and Darwin `renameatx_np(RENAME_EXCL)`
guard only the destination name. POSIX has no portable operation that renames
a pathname only when it still names a particular inode, nor a portable
unlink-by-descriptor. Strict operations therefore use a rooted quarantine and
descriptor checks to protect the ordinary concurrent editor-save cases at the
caller-visible destination: a replacement observed before quarantine is
rejected, and a replacement that appears after quarantine is never removed via
the original destination name. If the entry moved into quarantine is found to
be a different inode, it is restored to the destination with rooted
no-replace rename when that name is still absent; if the name was recreated,
the quarantine is left as evidence. A deliberate same-user actor that
enumerates or manipulates the library-owned random staging/quarantine names is
outside this portable capability boundary.

The final quarantine unlink still has no portable identity-coupled primitive.
The implementation holds the verified descriptor through the recheck,
compares the complete snapshot and hard-link state, re-reads the live bytes,
and removes only after a matching observation; if the quarantine disappears, changes identity, or
cannot be removed, it performs no further cleanup mutation and returns
`ErrQuarantineCleanupUncertain` with recoverable evidence. The remaining
name-based unlink interval is an explicit Unix/Darwin limitation, not an
atomic compare-and-act guarantee.

Receipt cleanup distinguishes an expected identity that is known absent from a
replacement that occupies the receipt pathname. A known-absent result carries
`ErrFileIdentityRemoved`, allowing earlier project writes to roll back while
the durability error is still surfaced. If a replacement appears before or
after quarantine cleanup, the operation preserves it and returns
`ErrFileIdentityChanged` without the removed sentinel. The caller keeps earlier
project writes in place because that pathname may contain a concurrently
published completed receipt.

Unpublished strict transactions apply the same rule to their private random
staging entry. While its descriptor is live, cleanup verifies the staged
identity before removal; if that observation is unavailable or mismatches,
the entry is left in place and `ErrStagingCleanupUncertain` reports the name.
The private-name race has the same explicit non-interference assumption as the
quarantine path and is not presented as an identity-coupled unlink guarantee.

`ReplaceFileIfSame` requires native no-replace publication and returns
`secureopen.ErrRenameNoReplaceUnsupported` without using the legacy hard-link
fallback. The compatibility `WriteFileIfSame*` adapters retain their historic
fallback behavior, but Xcode transaction callers do not use those adapters.

The current secureopen surface has no handle-backed publication or
compare-and-remove primitive that can retain and prove the installed Windows
file identity without a reuse window. Accordingly, strict creation,
replacement, and removal return `ErrFileIdentityMutationUnsupported` before
moving any entry on Windows. This boundary can be narrowed only when
handle-backed publication, rename, and delete implementations are available
and tested for the target filesystem.
Directory durability remains separately reported where a directory handle
cannot be flushed.

Before publication, complete staged bytes and file metadata are synced. On a
failure before publication, the original identity is restored only when the
destination is still empty or the expected replacement is still present. If a
concurrent writer occupies the destination, both entries remain recoverable
and the operation reports uncertainty. Strict publication repeats its rooted
pathname observation after final descriptor and content validation and again
after cleanup or directory durability work. After publication, rollback is
another identity-checked replacement; it never reopens a path and trusts a
snapshot.

## Callers and tests

`internal/xcode/version_project.go` stores `FileIdentity` values for ordinary
project/xcconfig writes and create-only receipts. It removes the duplicate
open/Lstat checks in receipt rollback, consumes the identity returned directly
by publication, and keeps the existing portable version-command fallback
separate. Receipt verification checks the committed identity as well as its
bytes. Signing transactions do not silently fall back to check-then-act.

RED coverage must force same-content and different-content inode/file-ID
swaps between capture and mutation, replacement during quarantine cleanup,
post-publication cleanup errors with a retained identity, Root-close lifetime
failures, unsupported-platform no-mutation behavior, and symlink, hard-link,
FIFO, metadata, and directory-sync cases. Xcode regressions must prove that a
concurrent editor save is preserved and that a receipt cannot certify a failed
rollback.

An alternative is to retain the current `os.FileInfo` API and add more
post-operation checks. That remains vulnerable to identity reuse and the
final check-to-act interval, so it is not the selected contract.
