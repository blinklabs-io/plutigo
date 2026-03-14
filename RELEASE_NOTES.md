# Release Notes

## v0.0.27 - maintenance updates

- Date: 2026-03-14
- Version: v0.0.27

Summary: This release includes incremental improvements and fixes across the library.

{
  "Additional Changes": [
    "Project dependencies and CI tooling were updated to newer versions to keep builds current and reliable. GitHub workflows now use actions/setup-go v6.3.0 and the ethereum go-ethereum module was updated from v1.17.0 to v1.17.1.",
    "Documentation and tests were updated to match runtime behavior and improve developer ergonomics. The implementation preserves raw string escapes, introduces an eval-context helper, adds builtin-availability filtering, and adjusts related docs and test fixtures accordingly.",
    "Release documentation was added for this version to make changes easier to track. A release notes document for v0.0.26 was introduced and pre-populated from the knowledge base."
  ],
  "Performance": [
    "Program evaluation now runs faster and uses less memory during execution and benchmarking. The CEK machine reduces allocations via arena-backed storage and cached constants, reuses machines in benchmarks, and caches two-argument builtin costs to lower runtime overhead."
  ]
}


## v0.0.26 - data serialization and ci automation

- Date: 2026-03-02
- Version: v0.0.26

Summary: This release includes `PlutusData` serialization updates and additional CI and project automation.

### New Features

- Added automated static analysis to catch nil-related issues earlier in the development process by introducing a GitHub Actions workflow and a `Makefile` target that run `nilaway` as part of CI and local checks.
- Added `cardano-detailed-schema` JSON serialization and deserialization for `PlutusData` types to simplify integration with external tools and services, with tests validating round-trip behavior.
- Added stricter and more predictable CBOR encoding behavior by refactoring CBOR array encoding and updating `Constr`, `List`, `Map`, and `ByteString` `MarshalCBOR` implementations to follow specific definite and indefinite-length encoding rules, with tests verifying exact bytes and boundary conditions.
- Added GitHub workflows that react to closed issues and update the GitHub Projects `Closed Date` field to keep project tracking current.
