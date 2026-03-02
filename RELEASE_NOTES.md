# Release Notes

## v0.0.26

- Date: 2026-03-02
- Version: v0.0.26

Summary: This release covers the changes listed below.

```json
{
  "New Features": [
    "Added automated static analysis to catch nil-related issues earlier in the development process. The release introduced a GitHub Actions workflow and a `Makefile` target that run `nilaway` as part of CI and local checks.",
    "Added a new JSON encoding format so serialized Cardano data is easier to integrate with other tools and services. The release implemented custom `cardano-detailed-schema` JSON serialization/deserialization for `PlutusData` types and added tests to validate round-trip behavior.",
    "Added stricter and more predictable CBOR encoding behavior so encoded data matches expected on-chain and tooling conventions. The release refactored CBOR array encoding and updated `Constr`, `List`, `Map`, and `ByteString` `MarshalCBOR` implementations to follow specific definite/indefinite encoding rules, with tests that verify exact bytes and boundary conditions.",
    "Added automation to keep project tracking up to date when work items are completed. The release added GitHub workflows that react to closed issues and update the project `Closed Date` field."
  ]
}

```
