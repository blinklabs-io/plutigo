# Release Notes

## v0.0.28 - incremental library updates

- Date: 2026-03-16
- Version: v0.0.28

Summary: This release includes incremental updates across the library.

```json
{
  "Additional Changes": [
    "Release documentation was updated so the current version has explicit release notes and the prior version is referenced for continuity. Specifically, documented release notes were added for v0.0.27 and they also point readers to the existing v0.0.26 notes for context.",
    "The Go toolchain and supporting dependencies were updated to keep the build aligned with the currently supported ecosystem. This includes a Go minor version update as well as dependency bumps for gnark-crypto and bitset in go.mod/go.sum."
  ],
  "Breaking Changes": [
    "The project now requires a newer Go toolchain so builds and CI runs use a more recent compiler by default. Specifically, the minimum Go version was raised and CI workflows were updated to run against Go 1.25/1.26, so downstream users may need to update their local toolchain and pipelines accordingly."
  ],
  "Performance": [
    "Execution performance was improved by reducing repeated allocations and speeding up common integer operations during evaluation. Concretely, machine-local arenas and caches were introduced for values/constants, fast int64 paths were added for integer builtins, and environment lookup plus step budgeting were refactored with tests updated to match the new behavior."
  ],
  "Security": [
    "Project cryptography and system dependency baselines were refreshed to pick up upstream fixes and hardening changes. In particular, module dependencies were updated to golang.org/x/crypto v0.49.0 and golang.org/x/sys v0.42.0, alongside bumps to gnark-crypto and bitset."
  ]
}

```

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
