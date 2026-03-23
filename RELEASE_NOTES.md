# Release Notes

## v0.0.29 - maintenance updates

- Date: 2026-03-23
- Version: v0.0.29

Summary: This release includes the changes listed below.

```json
{
  "Additional Changes": [
    "Project dependencies were refreshed to incorporate upstream fixes and keep CI tooling current, updating the `gnark-crypto` Go module from v0.20.0 to v0.20.1 and bumping the Codecov GitHub Action used in the Go test workflow from v5.5.2 to v5.5.3."
  ],
  "Breaking Changes": [
    "ECDSA signature verification no longer enforces the low-S requirement, so some previously rejected signatures may now validate, removing BIP-146 low-s enforcement from secp256k1 verification, wrapping public key parse failures as `BuiltinError`, and updating conformance tests to accept high-s signatures."
  ],
  "Bug Fixes": [
    "Flat encoding roundtrip coverage was expanded and several correctness issues in the encoder were fixed to improve reliability of serialized programs and constants, strengthening tests for constants/programs and fixing flat-encoder bit packing along with constant type list markers to ensure correct roundtrip behavior."
  ],
  "New Features": [
    "The release notes were updated to document the v0.0.28 release, adding a consolidated v0.0.28 changelog entry that summarizes toolchain updates, performance work, security-related changes, and dependency bumps for downstream consumers."
  ]
}

```

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
