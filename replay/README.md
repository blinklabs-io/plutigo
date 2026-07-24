# Mainnet replay corpus

`plutigo-replay` executes normalized Plutus validation cases and compares
plutigo's outcome and execution units with values recorded from cardano-node.

The normalization boundary is deliberate: plutigo is a UPLC evaluator and does
not reconstruct ledger `ScriptContext` values from full Cardano transactions.
A transaction-aware collector must resolve inputs and reference scripts, build
the era-appropriate script arguments, and record cardano-node's reference
result.

## Corpus format

```json
{
  "schema_version": 1,
  "network": "mainnet",
  "reference": {
    "implementation": "cardano-node",
    "version": "10.14.0.0"
  },
  "cases": [
    {
      "id": "<transaction-id>#spend:0",
      "transaction": {
        "id": "<transaction-id>",
        "slot": 123456789,
        "block": "<block-hash>",
        "redeemer": "spend:0"
      },
      "language": "PlutusV2",
      "protocol_version": {"major": 10, "minor": 0},
      "flat_program_hex": "<raw-flat-encoded-uplc>",
      "arguments_cbor_hex": [
        "<datum-cbor>",
        "<redeemer-cbor>",
        "<script-context-cbor>"
      ],
      "cost_model": {"use_default": true},
      "budget_limit": {"steps": 10000000000, "memory": 14000000},
      "expected": {
        "success": true,
        "ex_units": {"steps": 123456, "memory": 789}
      },
      "metadata": {
        "collector": "name and version",
        "cardano_node": "version used for the reference result"
      }
    }
  ]
}
```

`language` is the ledger Plutus language and is intentionally separate from the
UPLC version encoded in `flat_program_hex`. The program is raw FLAT, without a
CBOR bytestring envelope. `arguments_cbor_hex` is ordered exactly as the ledger
applies arguments to the script. `steps` corresponds to plutigo's CPU budget.

For smoke tests only, `"cost_model": {"use_default": true}` selects plutigo's
built-in model. Mainnet parity corpora should always include the historical
cost-model parameter array active at the transaction's slot.

## Run

```sh
go run ./cmd/plutigo-replay -corpus ./mainnet-corpus.json -pretty
```

The command exits with status 0 when every case matches, 1 for parity
mismatches, and 2 for an invalid corpus or runner error. Its JSON report
contains each case's measured latency and aggregate median, p95, and throughput.

## Collection checklist

For each representative V1, V2, and V3 transaction:

1. Record the network, slot, block hash, transaction ID, and redeemer pointer.
2. Resolve transaction inputs, datums, and reference scripts at that ledger
   state.
3. Export the raw FLAT program and the ledger-ordered CBOR `Data` arguments.
4. Record the active protocol version and complete cost-model parameter array.
5. Evaluate with cardano-node and record success/failure and exact ExUnits.
6. Keep collector and cardano-node versions in `metadata` for reproducibility.

With an offline transaction bundle, the reference ExUnits can be produced by:

```sh
cardano-cli latest transaction calculate-plutus-script-cost offline \
  --start-time-posix <network-start-time> \
  --era-history-file <era-history.json> \
  --utxo-file <resolved-inputs.json> \
  --protocol-params-file <protocol-parameters.json> \
  --tx-file <transaction.json> \
  --out-file <cardano-node-costs.json>
```

The transaction-aware collector remains responsible for exporting each raw
script and the exact ledger-ordered `Data` arguments alongside those costs.
