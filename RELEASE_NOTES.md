# Release Notes

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
