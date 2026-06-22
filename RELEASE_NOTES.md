# Release Notes

## v0.1.16 - security hardening and CEK/data performance improvements

- Date: 2026-06-22
- Version: v0.1.16

Summary: This release strengthens input handling and dependency protection, improves evaluation and data processing speed, tightens decoding and parsing validation, and refreshes maintenance updates across the library.

### Security

* Strengthened malformed input handling and runtime safeguards to improve protection during evaluation and decoding.
* Updated `golang.org/x/crypto` to `v0.53.0` to incorporate the latest upstream security hardening.

### Performance

* Accelerated signature verification cache access to reduce contention during concurrent verification workloads.
* Streamlined `PlutusData` JSON decoding into a single pass to reduce decoding overhead and improve throughput.
* Optimized common list drop evaluation paths so repeated data heavy workloads complete with less processing overhead.

### Bug Fixes

* Stabilized cache reuse by resetting full verification and integer caches before stale entries can degrade repeated evaluation behavior.
* Limited parser application width to the configured depth guard so hostile inputs cannot expand parsing work beyond expected bounds.
* Hardened term decoding by rejecting trailing bytes and validating the version before term decoding begins, which improves failure handling for malformed inputs.
* Standardized builtin argument validation failures so error handling stays more consistent across invalid inputs.
* Corrected string escape decoding so Unicode escape sequences now resolve to the intended characters.
* Resolved an unhandled term type path that could otherwise panic during syntax decoding.

### Documentation

* Clarified the published release history by adding the previous release entry so the release sequence remains complete.

### Additional Changes

* Updated the checkout workflow dependency to `actions/checkout` `v7.0.0` to keep release automation current.
* Refreshed the coverage reporting workflow by updating `codecov/codecov-action` to `v7.0.0`.
* Advanced the checkout workflow dependency to `actions/checkout` `v6.0.3` as part of ongoing CI maintenance.

## v0.1.15 - 32-bit target support

- Date: 2026-06-08
- Version: v0.1.15

Summary: This release adds 32-bit target support by changing `unConstrData` to compare the constructor tag via `uint64`, preventing build-time overflow on 32-bit targets while preserving the existing overflow guard behavior on 64-bit systems.

### Bug Fixes

* Improved `unConstrData` by comparing the constructor tag via `uint64`, preventing build-time overflow on 32-bit targets while preserving the existing overflow guard behavior on 64-bit systems.

## v0.1.14 - CEK slippage handling, nil-safety hardening, and dependency updates

- Date: 2026-06-01
- Version: v0.1.14

Summary: This release adds optional CEK final slippage flush skipping for restricting validation mode, hardens evaluator nil handling, removes self referential release notes maintenance text, and updates runtime and CI dependencies.

### Bug Fixes

* Fixed restricting validation mode so successful evaluations can leave the final deferred budget batch uncharged when exact charging should remain disabled.
* Improved evaluator safety so missing values now fail consistently and zero length allocations now return empty results instead of nil slices.

### Security

* Strengthened runtime dependency protection by updating `golang.org/x/crypto` to `v0.52.0` and `golang.org/x/sys` to `v0.45.0` for upstream security fixes.

### Documentation

* Clarified `RELEASE_NOTES.md` by removing self referential maintenance bullets so each entry stays focused on substantive library changes.

### Additional Changes

* Added the `v0.1.13` release entry to keep the published release sequence complete.
* Refreshed `github.com/btcsuite/btcd/chaincfg/chainhash` from `v1.1.0` to `v1.2.0`.
* Updated `github.com/btcsuite/btcd/btcec/v2` to `v2.5.0` and refreshed related `secp256k1` dependencies.
* Advanced `github.com/ethereum/go-ethereum` from `v1.17.2` to `v1.17.3`.
* Modernized the module publishing workflow by updating `andrewslotin/go-proxy-pull-action` from `v1.4.0` to `v1.5.0`.
* Upgraded coverage reporting by updating `codecov/codecov-action` from `v6.0.0` to `v6.0.1`.
* Streamlined lint automation by updating `golangci/golangci-lint-action` from `v9.2.0` to `v9.2.1`.

## v0.1.13 - expmod and plutus data decode limit fixes

- Date: 2026-05-15
- Version: v0.1.13

Summary: This release fixes `expModInteger` budget cost overflow handling and hardens `PlutusData` decoding with nesting depth, node count, and trailing byte validation.

### Bug Fixes

* Corrected `expModInteger` budget cost overflow handling so overflow-safe cost checks now return a budget error instead of corrupting runtime budget tracking.
* Hardened `PlutusData` decoding with a hard nesting depth limit, a hard total node limit, and rejection of unexpected trailing bytes during decoding.

## v0.1.12 - CEK performance, CBOR encoding, and syntax output fixes

- Date: 2026-05-13
- Version: v0.1.12

Summary: This release improves CEK performance, fixes CBOR encoding and syntax pretty-print output, updates key dependencies, and expands fuzz coverage.

### Performance

* Improved builtin evaluation speed by caching builtin force and arity lookups in the CEK stack machine so repeated metadata work no longer slows builtin evaluation.
* Refined allocator performance by replacing modulo based chunk offset calculations with bitmask based indexing in CEK arena and environment allocators to reduce allocator overhead.
* Accelerated the DeBruijn evaluator by using unsafe interface tab dispatch in hot paths and a direct lambda fast path that skips extra stack work.

### Bug Fixes

* Corrected syntax output so additional control characters now escape cleanly and other non-printable ASCII bytes use octal escapes instead of appearing raw in generated output.
* Fixed empty `CBOR` byte string encoding so empty decoded values now round-trip back to canonical `0x40` output.

### Security

* Updated `golang.org/x/crypto` to `v0.51.0` and `golang.org/x/sys` to `v0.44.0` to incorporate upstream security and maintenance fixes.

### Additional Changes

* Enhanced `github.com/fxamacker/cbor/v2` from `v2.9.1` to `v2.9.2`, bringing in upstream CBOR encoding hardening and validation improvements.
* Expanded fuzz coverage across parser, pretty-printer, lexer, FLAT decoding, builtin lookup validation, and CEK execution with new fuzz targets and stronger round-trip checks.

## v0.1.11 - reverse stack machine performance rewrite

- Date: 2026-05-09
- Version: v0.1.11

Summary: This release improves reverse stack machine performance.

### Performance

* Improved stack machine execution performance by rewriting the DeBruijn CEK evaluator to use shared machine helpers and ordinary type switches instead of previous unsafe style implementation details, which also simplifies apply, force, and case handling.

## v0.1.10 - environment, decoder, and stack-machine caching optimizations

- Date: 2026-05-04
- Version: v0.1.10

Summary: This release improves environment extension, syntax decoding, stack machine throughput, and decoder allocation reuse, and updates the conventional commits workflow dependency.

### Performance

* Improved environment extension speed by caching the active environment chunk and its boundary limit during repeated value binding.
* Refined syntax decoding throughput by switching `bits4()` to a direct branch selection path for used bit checks.
* Enhanced stack machine throughput with specialized term checks, more local frame stack and environment handling, and faster lambda application paths.
* Streamlined decoder allocation reuse by caching the active chunk in both data and syntax decoders.

### Additional Changes

* Updated `.github/workflows/conventional-commits.yml` to bump `webiny/action-conventional-commits` from `v1.3.1` to `v1.4.2`.

## v0.1.9 - CEK and syntax decoding performance optimizations

- Date: 2026-04-20
- Version: v0.1.9

Summary: This release improves CEK execution and syntax decoding performance by adding a DeBruijn-specialized no-slippage evaluator, caching builtin cost calculations, adding a direct forced-builtin fast path, and optimizing low-nibble bits4 decoding.

### Performance

* Added a DeBruijn-specialized no-slippage CEK stack evaluator with optimized environment lookup and immediate-term handling, and routed `Machine.Run` through it for DeBruijn terms to reduce evaluation overhead.
* Cached one-argument and three-argument builtin cost models and avoided unnecessary `ExMem` evaluation when cost parameters are constant to reduce builtin budget-accounting overhead.
* Added a direct `force (builtin ...)` fast path in the DeBruijn stack machine so forced builtins can be evaluated without the slower general immediate-term path.
* Optimized syntax decoding by teaching `bits4()` to return the low nibble directly when four bits remain in the current byte, reducing work in a hot decoding path.

## v0.1.8 - cost model handling and CEK hot-path tweaks

- Date: 2026-04-18
- Version: v0.1.8

Summary: This release fixes CEK cost model handling and improves CEK hot-path performance to improve budget correctness and reduce interpreter overhead.

### Bug Fixes

* Corrected cost model handling so budget calculations stay accurate across a broader range of builtin cost parameters, including configurable `dropList` CPU settings.

### Performance

* Improved CEK hot path evaluation to reduce temporary allocation overhead during apply and case handling.

## v0.1.7 - decoder retention and reset updates

- Date: 2026-04-15
- Version: v0.1.7

Summary: This release covers decoder retention and reset updates to improve repeated-decoding correctness and reduce allocation overhead.

### New Features

- Added specialized arena reset handling for retained `big.Int` values to prevent stale large-number state across repeated decoding.
- Added arena-backed byte storage and `UTF-8` validation to expand decode retention capacity and reduce allocations during repeated decoding.

## v0.1.6 - workflow and decoding maintenance

- Date: 2026-04-13
- Version: v0.1.6

Summary: This release covers constant list construction fixes, flat decoding correctness updates, dependency security updates, and CI and release workflow maintenance.

### Bug Fixes

- Fixed `syn.Constant` list construction to unwrap typed constants consistently.
- Fixed chunk handling and state clearing in `syn/flat_decode.go` to prevent subtle decoding issues.

### Security

- Updated `golang.org/x/crypto` and `golang.org/x/sys` to incorporate upstream security and stability fixes.

### Additional Changes

- Updated `.github/workflows/publish.yml` to use `actions/github-script@v9.0.0` instead of `v8.0.0`.
- Updated `nilaway` invocation flags and excluded `cek/stack_machine.go` from analysis to improve static analysis reliability.

## v0.1.5 - arena-backed allocation and evaluation fast paths

- Date: 2026-04-12
- Version: v0.1.5

Summary: This release includes arena-backed allocation for decoded terms and constants, an `ed25519` verification cache, evaluator fast paths for common lambda application patterns, and decoding and cost-accounting updates that reduce allocation overhead.

### New Features

- Added arena-backed allocation for decoded terms and constants to reduce allocation overhead during evaluation.
- Added an `ed25519` verification cache to reduce repeated signature verification overhead.
- Added environment skip pointers to reduce environment traversal overhead in common evaluation paths.
- Added lambda-application fast paths to reduce overhead in common call patterns.
- Added support for configuring the benchmark root directory via `PLUTIGO_BENCH_DIR`.

### Bug Fixes

- Fixed constant handling to behave more consistently across decoding and runtime evaluation.
- Fixed arena lifecycle management so temporary allocations were released after `Machine.Run` completes.

### Performance

- Updated `CBOR` decoding to allocate decoded values from arenas to reduce per-decode allocations.
- Updated value and builtin allocation to reduce peak memory usage by reusing instances, adapting arena sizes, and reusing no-arg builtin instances.
- Updated decoding to use smaller input chunks by reducing the data decoding chunk size from `256` to `64`.
- Updated memory-cost accounting to more accurately reflect allocation behavior under arena-backed reuse.

### Additional Changes

- Updated test coverage to validate caching, arena reuse, and evaluator fast-path behaviors.

## v0.1.4 - arena-backed CBOR decoding and runtime cleanup

- Date: 2026-04-10
- Version: v0.1.4

Summary: This release includes arena-backed `CBOR` decoding for additional `PlutusData` primitives, retained arena cleanup after `Machine.Run`, and improved constant handling to reduce memory pressure.

### New Features

- Added arena-backed `CBOR` decoding for `PlutusData` maps, lists, integers, and byte strings to reduce per-decode allocations and improve reuse across operations.

### Bug Fixes

- Fixed retained arena state leaking between executions by ensuring arenas are cleared after `Machine.Run` completes.

### Performance

- Improved constant handling to reduce memory pressure during repeated decoding and evaluation workloads.

### Additional Changes

- Expanded test coverage to validate arena-backed `CBOR` and DeBruijn decoding behavior across supported `PlutusData` shapes.

## v0.1.3 - CBOR decoding and DeBruijn updates

- Date: 2026-04-08
- Version: v0.1.3

Summary: This release includes streaming CBOR decoding utilities, improved CBOR parsing validation, and refined DeBruijn decoding performance and type checking correctness.

### New Features

- Added streaming `CBOR` decoding utilities to support incremental parsing and container-aware `PlutusData` decoding.

### Bug Fixes

- Fixed CBOR constructor, `map`, and `array` parsing to reject invalid inputs earlier and prevent out-of-bounds behavior.
- Fixed type equality and DeBruijn type checking behavior to avoid incorrect comparisons and excessive environment clearing.

### Performance

- Improved DeBruijn decoding throughput and reduced allocation pressure by reusing an arena-backed decoder.

## v0.1.2 - maintenance updates

- Date: 2026-04-06
- Version: v0.1.2

Summary: This release includes interpreter reliability improvements, performance refinements, and dependency updates.

### New Features

- Updated the interpreter to return from calls more reliably and behave consistently across common execution paths.

### Performance

- Improved build and interpretation performance while keeping behavior consistent in frequently used cases.

### Additional Changes

- Updated project dependencies to incorporate upstream fixes and keep the toolchain current.
- Updated the `nilaway` GitHub Actions workflow to run only when explicitly triggered.

## v0.1.1 - go toolchain and cek interpreter updates

- Date: 2026-04-03
- Version: v0.1.1

Summary: This release includes Go toolchain baseline updates, a CEK interpreter refactor, and runtime performance improvements.

### New Features

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

- Updated the Codecov GitHub Action from `v5.5.3` to `v6.0.0`.

## v0.0.29 - maintenance updates

- Date: 2026-03-23
- Version: v0.0.29

Summary: This release includes ECDSA verification behavior changes, flat encoding fixes, and dependency updates.

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

## v0.0.26 - data serialization and ci automation

- Date: 2026-03-02
- Version: v0.0.26

Summary: This release includes `PlutusData` serialization updates and additional CI and project automation.

### New Features

- Added automated static analysis to catch nil-related issues earlier in the development process by introducing a GitHub Actions workflow and a `Makefile` target that run `nilaway` as part of CI and local checks.
- Added `cardano-detailed-schema` JSON serialization and deserialization for `PlutusData` types to simplify integration with external tools and services, with tests validating round-trip behavior.
- Added stricter and more predictable CBOR encoding behavior by refactoring CBOR array encoding and updating `Constr`, `List`, `Map`, and `ByteString` `MarshalCBOR` implementations to follow specific definite and indefinite-length encoding rules, with tests verifying exact bytes and boundary conditions.
- Added GitHub workflows that react to closed issues and update the GitHub Projects `Closed Date` field to keep project tracking current.
