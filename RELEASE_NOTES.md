# Release Notes

## v0.1.2 - maintenance updates

- Date: 2026-04-06
- Version: v0.1.2

Summary: This release includes the changes listed below.

### Changes

```json
{
  "Additional Changes": [
    "The project dependencies were updated to incorporate upstream fixes and keep the toolchain current. This includes upgrading go-ethereum from v1.17.1 to v1.17.2, bumping fxamacker/cbor from v2.9.0 to v2.9.1 (with checksum updates), and updating GitHub Actions to use actions/setup-go v6.4.0.",
    "Automated analysis for nilaway now runs only when explicitly triggered rather than on every change. Concretely, the nilaway GitHub Actions workflow was changed to run only on workflow_dispatch instead of on pushes to main or pull requests.",
    "The release documentation was updated to reflect the current published build. Specifically, a release notes entry was added for version v0.1.1."
  ],
  "New Features": [
    "The interpreter now returns from calls more reliably and behaves consistently across common execution paths. Specifically, the stack interpreter refines return-frame handling for normal and no-slippage paths, adds immediate-term shortcuts and builtin fast paths, improves frame cleanup, and completes delay/lambda closure representation using an AST-based model."
  ],
  "Performance": [
    "Builds and interpretation run faster while keeping behavior consistent in frequently used cases. In particular, the interpreter introduces builtin fast paths and immediate-term shortcuts to reduce overhead in hot execution paths."
  ]
}

```

## v0.1.1 - go toolchain and cek interpreter updates

- Date: 2026-04-03
- Version: v0.1.1

Summary: This release includes Go toolchain baseline updates, a CEK interpreter refactor, and runtime performance improvements.

### New Features

- Added release notes for `v0.1.0` consolidating pre-`v0.1.0` changes and CI pipeline updates.
- Updated the CEK evaluator to use an explicit stack-based interpreter and added tests for frame stack reuse, builtin partial-application discharge, and case-on-pair semantics.

### Breaking Changes

- Updated the minimum supported Go toolchain to `Go 1.25+` (recommended `Go 1.26+`) and refreshed benchmarks against `Go 1.26`.

### Performance

- Improved CEK runtime efficiency by optimizing environment lookup, refining chunked value allocation and cost handling, and restructuring stack frame management with specialized await-arg handling for lambdas and builtins.

## v0.1.0 - CEK performance and tooling updates

- Date: 2026-03-31
- Version: v0.1.0

Summary: This release includes CEK evaluation hot-path improvements, cached integer metadata, and CI and documentation updates.

### New Features

- Added cached `ex_mem` and `int64` fields to `syn.Integer` and updated CEK runtime paths to use them.

### Performance

- Added CEK hot-path helpers for environment lookup, immediate value computation, and value return, and refactored case evaluation to use them.

### Additional Changes

- Updated `RELEASE_NOTES.md` with a consolidated entry for `v0.0.29` covering ECDSA handling, encoding behavior, and dependency upgrades.
- Updated the Codecov GitHub Action from `v5.5.3` to `v6.0.0`.

## v0.0.29 - maintenance updates

- Date: 2026-03-23
- Version: v0.0.29

Summary: This release includes ECDSA verification behavior changes, flat encoding fixes, and dependency updates.

### New Features

- Updated `RELEASE_NOTES.md` to document `v0.0.28` with a consolidated changelog entry covering toolchain updates, performance changes, security-related changes, and dependency updates.

### Breaking Changes

- Removed `BIP-146` low-S enforcement from `secp256k1` ECDSA verification, wrapping public key parse failures as `BuiltinError` and updating conformance tests to accept high-S signatures.

### Bug Fixes

- Fixed flat encoding roundtrip issues by expanding test coverage and correcting encoder bit packing and constant type list markers.

### Additional Changes

- Updated `github.com/consensys/gnark-crypto` from `v0.20.0` to `v0.20.1` and updated the Codecov GitHub Action from `v5.5.2` to `v5.5.3`.

## v0.0.28 - go toolchain and dependency updates

- Date: 2026-03-16
- Version: v0.0.28

Summary: This release includes a Go toolchain requirement update, performance improvements, and dependency updates.

### Breaking Changes

- Updated the minimum supported Go version to `1.25.7` and the module toolchain to `go1.25.8`.

### Performance

- Improved evaluation performance by reducing allocations and adding fast `int64` paths for common integer operations.

### Security

- Updated `golang.org/x/crypto` to `v0.49.0` and `golang.org/x/sys` to `v0.42.0`.

### Additional Changes

- Added release notes for `v0.0.27` and referenced `v0.0.26` for continuity.
- Updated `github.com/consensys/gnark-crypto` to `v0.20.0` and `github.com/bits-and-blooms/bitset` to `v1.24.4`.

## v0.0.27 - performance and tooling updates

- Date: 2026-03-14
- Version: v0.0.27

Summary: This release includes performance improvements and tooling updates across the library.

### Performance

- Improved CEK evaluation performance by reducing allocations, caching constants, reusing machines in benchmarks, and caching two-argument builtin costs.

### Additional Changes

- Updated GitHub workflows to use `actions/setup-go@v6.3.0` and updated `github.com/ethereum/go-ethereum` from `v1.17.0` to `v1.17.1`.
- Updated documentation and tests to match runtime string-escape handling, an `eval` context helper, and builtin availability filtering.
- Added `v0.0.26` release notes to improve change tracking.

## v0.0.26 - data serialization and ci automation

- Date: 2026-03-02
- Version: v0.0.26

Summary: This release includes `PlutusData` serialization updates and additional CI and project automation.

### New Features

- Added automated static analysis to catch nil-related issues earlier in the development process by introducing a GitHub Actions workflow and a `Makefile` target that run `nilaway` as part of CI and local checks.
- Added `cardano-detailed-schema` JSON serialization and deserialization for `PlutusData` types to simplify integration with external tools and services, with tests validating round-trip behavior.
- Added stricter and more predictable CBOR encoding behavior by refactoring CBOR array encoding and updating `Constr`, `List`, `Map`, and `ByteString` `MarshalCBOR` implementations to follow specific definite and indefinite-length encoding rules, with tests verifying exact bytes and boundary conditions.
- Added GitHub workflows that react to closed issues and update the GitHub Projects `Closed Date` field to keep project tracking current.
