# Ark Transaction Builders

[![CI](https://github.com/SashaZezulinsky/ark-tx-builder/workflows/CI/badge.svg)](https://github.com/SashaZezulinsky/ark-tx-builder/actions)
[![Coverage](https://img.shields.io/badge/coverage-90.9%25-brightgreen)](https://codecov.io/gh/SashaZezulinsky/ark-tx-builder)
[![Go Version](https://img.shields.io/badge/go-1.22-blue.svg)](https://golang.org/dl/)

Deterministic Bitcoin transaction builders for the Ark protocol implementing boarding, commitment, and forfeit transactions per the [Ark specification](docs/ark.pdf).

## Build & Test

```bash
# Run all tests with coverage
make test

# Run all checks (fmt, vet, lint, test)
make check

# Build binary
make build
```

## Design Choices

### MuSig2 Library Selection
**Library**: `github.com/btcsuite/btcd/btcec/v2/schnorr/musig2` (BIP-327)

**Rationale**: Battle-tested implementation from btcd with proven security. Provides key aggregation with protection against rogue key attacks and ensures deterministic ordering. Using a proven cryptographic library instead of custom implementation significantly reduces attack surface and eliminates potential vulnerabilities in critical crypto operations.

### Taproot Approach
**Strategy**: Script-path spending with NUMS (Nothing Up My Sleeve) internal key

Uses btcd's built-in Taproot helpers (`txscript.AssembleTaprootScriptTree`, `txscript.PayToTaprootScript`) for constructing Taproot outputs. Scripts are sorted lexicographically before Merkle tree construction to ensure deterministic transaction IDs. NUMS point (0x5092...) ensures the keypath is provably unspendable, requiring script-path spending.

### Fee Calculation
**Formula**: `fee = vsize × feeRate` where `vsize = (baseSize × 4 + witnessSize) / 4`

Weight-based estimation using SegWit v1 sizing rules. P2TR witness estimated at ~66 bytes per input (control block + signature). Minimum fee rate: 1 sat/vbyte. All builders validate sufficient funds before returning transactions.

### Determinism
Six layers ensure byte-identical transactions from identical inputs:
1. Fixed version (2) and locktime (0)
2. Transaction-specific sequence numbers (boarding: 0xFFFFFFFD, commitment/forfeit: 0xFFFFFFFF)
3. BIP-69 style input sorting (txid → index)
4. BIP-69 style output sorting (amount → script)
5. Lexicographic script ordering
6. Deterministic MuSig2 key aggregation (sort=true)

Verified with 100-iteration determinism tests for each transaction type.

## Transaction Types

### Boarding Transaction
User deposits with cooperative (MuSig2) and timeout (CSV) exit paths. Optional change output for excess funds.

### Commitment Transaction
Batches VTXOs on-chain with batch output (sweep + unroll paths) and connector output (operator-controlled dust for forfeit atomicity).

### Forfeit Transaction
Ensures atomic VTXO swaps using SIGHASH_ALL to bind forfeit to specific commitment transaction via connector mechanism.

## Test Coverage

**90.9%** statement coverage across 17 test cases including:
- 100-iteration determinism tests (boarding, commitment)
- Input validation (negative amounts, dust limits, insufficient funds)
- Edge cases (empty inputs, sorting, change output removal)
- MuSig2 aggregation and Taproot script creation
- Atomic forfeit verification

## Project Structure

```
├── types.go          # Core types and constants
├── taproot.go        # MuSig2 & Taproot utilities
├── boarding.go       # Boarding transaction builder
├── commitment.go     # Commitment transaction builder
├── forfeit.go        # Forfeit transaction builder
└── builders_test.go  # Test suite (17 tests, 90.9% coverage)
```

## References

- [Ark Protocol Specification](docs/ark.pdf) - Implementation based on Sections 4.3, 4.5, Definition 4.9
- [BIP-327 (MuSig2)](https://github.com/bitcoin/bips/blob/master/bip-0327.mediawiki)
- [BIP-341 (Taproot)](https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki)

## License

MIT
