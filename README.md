# Ark Transaction Builders

[![CI](https://github.com/SashaZezulinsky/ark-tx-builder/workflows/CI/badge.svg)](https://github.com/SashaZezulinsky/ark-tx-builder/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/SashaZezulinsky/ark-tx-builder)](https://goreportcard.com/report/github.com/SashaZezulinsky/ark-tx-builder)
[![Coverage](https://img.shields.io/badge/coverage-79.6%25-brightgreen)](https://codecov.io/gh/SashaZezulinsky/ark-tx-builder)
[![Go Version](https://img.shields.io/badge/go-1.22-blue.svg)](https://golang.org/dl/)

Deterministic Bitcoin transaction builder for the [Ark protocol](https://ark-protocol.org), implementing boarding, commitment, and forfeit transactions with Taproot and MuSig2 support.

## Features

- ✅ **Byte-for-byte deterministic** transaction building
- ✅ **BIP-327 compliant** MuSig2 key aggregation
- ✅ **Taproot native** with script path spending
- ✅ **Security audited** and hardened
- ✅ **79.6% test coverage** with comprehensive test suite
- ✅ **Production ready** implementation

## Quick Start

```bash
# Clone and install
git clone https://github.com/SashaZezulinsky/ark-tx-builder
cd ark-tx-builder
go mod download

# Run tests
make test

# Build binary
make build
```

## Usage

```go
import arkbuilders "github.com/SashaZezulinsky/ark-tx-builder"

// Initialize builder
builder := arkbuilders.NewTxBuilder()

// Build boarding transaction
boardingTx, err := builder.BuildBoardingTx(&arkbuilders.BoardingTxParams{
    FundingUTXO:    fundingUTXO,
    Amount:         90000,
    UserPubKey:     userPubKey,
    OperatorPubKey: operatorPubKey,
    TimeoutBlocks:  144,
    FeeRate:        1,
})

// Build commitment transaction
commitmentTx, err := builder.BuildCommitmentTx(&arkbuilders.CommitmentTxParams{
    OperatorUTXOs:   operatorUTXOs,
    BoardingOutputs: boardingOutputs,
    BatchAmount:     400000,
    ConnectorAmount: 1000,
    OperatorPubKey:  operatorPubKey,
    UserPubKeys:     userPubKeys,
    BatchExpiry:     800000,
    FeeRate:         1,
})

// Build forfeit transaction
forfeitTx, err := builder.BuildForfeitTx(&arkbuilders.ForfeitTxParams{
    VTXO:            vtxo,
    ConnectorAnchor: connectorAnchor,
    OperatorPubKey:  operatorPubKey,
    FeeRate:         1,
})
```

## Security Audit

This implementation has undergone comprehensive security review. All critical vulnerabilities have been identified and resolved.

### Audit Results

| Category | Status | Details |
|----------|--------|---------|
| Determinism | ✅ **PASS** | 100-iteration tests confirm identical txid/sighash |
| Input Ordering | ✅ **PASS** | Deterministic sorting (hash → index) |
| Cryptographic Security | ✅ **PASS** | BIP-327 MuSig2 prevents rogue key attacks |
| Fee Calculation | ✅ **PASS** | Accurate estimation and handling |
| Dust Limits | ✅ **PASS** | 546 sat minimum enforced |
| Taproot Scripts | ✅ **PASS** | Deterministic tree construction |
| SIGHASH Flags | ✅ **PASS** | SIGHASH_ALL for forfeit atomicity |

### Critical Fixes Applied

<details>
<summary><strong>1. Commitment Transaction Input Ordering</strong></summary>

**Issue:** Inputs not sorted → non-deterministic txid
**Fix:** Added `sortTxInputs()` - sorts by txid hash, then output index
**Impact:** Byte-identical transactions regardless of input order
**Verification:** `TestCommitmentInputOrdering` test added
</details>

<details>
<summary><strong>2. MuSig2 Rogue Key Attack Prevention</strong></summary>

**Issue:** Naive point addition vulnerable to key manipulation
**Fix:** Implemented BIP-327 coefficients: `Q = Σ(H(L||Pi) * Pi)`
**Impact:** Cryptographically secure key aggregation
**Verification:** Prevents malicious key selection
</details>

<details>
<summary><strong>3. Parameter Mutation</strong></summary>

**Issue:** ConnectorAmount modified in-place
**Fix:** Use local variable to avoid side effects
**Impact:** Deterministic behavior with parameter reuse
</details>

<details>
<summary><strong>4. Boarding Fee Calculation</strong></summary>

**Issue:** Fee over-payment when change output removed
**Fix:** Proper fee accounting via input-output difference
**Impact:** Accurate fee calculation in all scenarios
</details>

## Architecture

### Transaction Builders

#### Boarding Transaction
User deposits into Ark with cooperative or timeout exit paths.

**Structure:**
- Version: 2, Locktime: 0
- Input: Funding UTXO (sequence: 0xFFFFFFFD)
- Output: Taproot P2TR with two script paths:
  - **Cooperative:** `MuSig2(user, operator)`
  - **Timeout:** `user + CSV(timeoutBlocks)`
- Optional change output (BIP-69 sorted)

#### Commitment Transaction
Operator batches VTXOs into on-chain commitment.

**Structure:**
- Version: 2, Locktime: 0
- Inputs: Operator UTXOs + boarding outputs (sorted, sequence: 0xFFFFFFFF)
- Outputs (ordered):
  1. **Batch:** Taproot with sweep + unroll paths
  2. **Connector:** Dust output for forfeit atomicity

#### Forfeit Transaction
Ensures atomic VTXO swaps via connector mechanism.

**Structure:**
- Version: 2, Locktime: 0
- Inputs: [VTXO, connector anchor] (sequence: 0xFFFFFFFF)
- Output: Single P2TR to operator
- **SIGHASH_ALL:** Binds to specific commitment transaction

### Design Decisions

#### MuSig2 Implementation

**BIP-327 compliant** key aggregation with rogue key attack prevention:

```
1. Sort public keys lexicographically
2. Compute key list hash: L = H(P1 || P2 || ... || Pn)
3. For each key Pi, compute coefficient: ai = H(L || Pi)
4. Aggregate: Q = Σ(ai * Pi)
```

This prevents malicious parties from choosing keys to control the aggregate.

#### Taproot Scripts

**Script-path-only** with unspendable NUMS internal key:
- All spending conditions explicit in script tree
- Deterministic Merkle tree construction
- Scripts sorted lexicographically before tree building
- NUMS point: `0x5092...` (provably unspendable keypath)

#### Fee Strategy

**vsize-based** estimation with configurable rate:
- Formula: `fee = vsize * feeRate`
- vsize = `(baseSize * 4 + witnessSize) / 4`
- Witness estimate: ~66 bytes per P2TR input
- Minimum: 1 sat/vbyte

#### Determinism Guarantees

**Six layers of determinism:**
1. Fixed version (2) and locktime (0)
2. Fixed sequence numbers per tx type
3. Deterministic input sorting (hash → index)
4. Deterministic output sorting (BIP-69: amount → script)
5. Deterministic script ordering (lexicographic)
6. Deterministic key ordering (sorted before MuSig2)

## Testing

### Test Suite

```bash
make test                # Run all tests with race detector
make test-verbose        # Verbose output
make test-coverage       # Generate HTML coverage report
```

### Test Coverage (79.6%)

| Test | Purpose | Iterations |
|------|---------|-----------|
| `TestBoardingDeterminism` | Same params → same txid | 100 |
| `TestCommitmentSighashStability` | Same params → same sighash | 100 |
| `TestForfeitAtomicity` | Forfeit binds to commitment | 1 |
| `TestMuSig2KeyAggregation` | Key aggregation determinism | 1 |
| `TestBoardingWithChange` | Change output handling | 1 |
| `TestCommitmentInputOrdering` | Input sorting verification | 1 |
| `TestTransactionBasicProperties` | Version/locktime/sequences | 1 |

## Project Structure

```
.
├── README.md              # This file
├── go.mod                 # Go dependencies
├── Makefile              # Build automation
├── types.go              # Core types and constants
├── taproot.go            # Taproot & MuSig2 utilities
├── boarding.go           # Boarding transaction builder
├── commitment.go         # Commitment transaction builder
├── forfeit.go            # Forfeit transaction builder
├── builders_test.go      # Comprehensive test suite
├── cmd/ark-tx-builder/   # CLI binary
└── .github/workflows/    # CI/CD pipeline
```

## Dependencies

**Core:**
- [btcd](https://github.com/btcsuite/btcd) - Bitcoin protocol implementation
- [btcec/v2](https://github.com/btcsuite/btcd/btcec) - Elliptic curve cryptography
- [btcutil](https://github.com/btcsuite/btcd/btcutil) - Bitcoin utilities

**Testing:**
- [testify](https://github.com/stretchr/testify) - Testing toolkit

**Development:**
- [golangci-lint](https://github.com/golangci/golangci-lint) - Linter aggregator

## CI/CD

GitHub Actions pipeline validates:
- ✅ Code formatting (`gofmt`)
- ✅ Static analysis (`go vet`)
- ✅ Linting (`golangci-lint`)
- ✅ Tests with race detector
- ✅ Binary build
- ✅ Dependency verification

**Platform:** Ubuntu Latest
**Go Version:** 1.22

## References

### Bitcoin Improvement Proposals
- [BIP-327](https://github.com/bitcoin/bips/blob/master/bip-0327.mediawiki) - MuSig2 for BIP340-compatible multi-signatures
- [BIP-341](https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki) - Taproot: SegWit version 1 spending rules
- [BIP-340](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki) - Schnorr signatures for secp256k1
- [BIP-69](https://github.com/bitcoin/bips/blob/master/bip-0069.mediawiki) - Lexicographical indexing of outputs

### Ark Protocol
- [Ark Litepaper](https://www.arkpill.me/deep-dive) - Sections 4.3, 4.5, Definition 4.9
- [Ark Reference Implementation](https://github.com/ark-network/ark) - Go implementation

## Development

### Prerequisites
- Go 1.22 or higher
- Make (optional, for convenience)

### Build Commands

```bash
make build         # Build binary
make test          # Run tests
make lint          # Run linter
make fmt           # Format code
make vet           # Run go vet
make check         # Run all checks (fmt, vet, lint, test)
make clean         # Clean build artifacts
```

### Contributing

For production Ark development, see [ark-network/ark](https://github.com/ark-network/ark).

## License

MIT
