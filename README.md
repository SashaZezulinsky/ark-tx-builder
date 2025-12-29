# Ark Transaction Builders

[![CI](https://github.com/SashaZezulinsky/ark-tx-builder/workflows/CI/badge.svg)](https://github.com/SashaZezulinsky/ark-tx-builder/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/SashaZezulinsky/ark-tx-builder)](https://goreportcard.com/report/github.com/SashaZezulinsky/ark-tx-builder)
[![Coverage](https://img.shields.io/badge/coverage-76.8%25-brightgreen)](https://codecov.io/gh/SashaZezulinsky/ark-tx-builder)
[![Go Version](https://img.shields.io/badge/go-1.22-blue.svg)](https://golang.org/dl/)

Deterministic Bitcoin transaction builders for the Ark protocol implementing boarding, commitment, and forfeit transactions.

## Build & Test

```bash
# Run tests
make test

# Run all checks (fmt, vet, lint, test)
make check

# Build binary
make build
```

## Design Choices

### MuSig2 Implementation
**Library**: `github.com/btcsuite/btcd/btcec/v2/schnorr/musig2`

**Why**: BIP-327 compliant, battle-tested implementation from btcd. Provides secure key aggregation with protection against rogue key attacks. Using a proven library instead of custom crypto reduces attack surface.

### Taproot Approach
**Strategy**: Script-path-only spending with NUMS internal key

**Implementation**:
- Uses btcd's Taproot helpers (`txscript.AssembleTaprootScriptTree`, `txscript.PayToTaprootScript`)
- Scripts sorted lexicographically before Merkle tree construction for determinism
- NUMS point (0x5092...) ensures keypath is provably unspendable

### Fee Calculation
**Strategy**: Weight-based estimation with configurable rate

**Formula**: `fee = vsize * feeRate` where `vsize = (baseSize * 4 + witnessSize) / 4`

**Witness estimation**: ~66 bytes per P2TR input (control block + signature)

**Minimum**: 1 sat/vbyte

### Determinism Strategy
Six layers ensure byte-identical transactions:
1. Fixed version (2) and locktime (0)
2. Fixed sequence numbers per transaction type
3. Deterministic input sorting (txid hash → index)
4. Deterministic output sorting (BIP-69: amount → script)
5. Deterministic script ordering (lexicographic)
6. Deterministic key ordering (musig2 with sort=true)

## Transaction Types

### Boarding Transaction
User deposits into Ark with two exit paths:
- **Cooperative**: MuSig2(user, operator) for instant exit
- **Timeout**: user + CSV(timeoutBlocks) for unilateral exit
- Sequence: 0xFFFFFFFD
- Optional change output (BIP-69 sorted)

### Commitment Transaction
Operator batches VTXOs into on-chain commitment:
- **Inputs**: Operator UTXOs + boarding outputs (sorted deterministically)
- **Output 1**: Batch with sweep (operator + timelock) and unroll (covenant) paths
- **Output 2**: Connector (dust, operator-controlled) for forfeit atomicity
- Sequence: 0xFFFFFFFF

### Forfeit Transaction
Ensures atomic VTXO swaps via connector mechanism:
- **Inputs**: [VTXO, connector anchor] - deterministic order
- **Output**: Single P2TR to operator
- **SIGHASH_ALL**: Binds to specific commitment transaction
- Sequence: 0xFFFFFFFF

## Test Coverage (76.8%)

All required tests pass with 100 iterations:
- ✅ `TestBoardingDeterminism` - Same params → same txid (100 runs)
- ✅ `TestCommitmentSighashStability` - Same params → same sighash (100 runs)
- ✅ `TestForfeitAtomicity` - Forfeit references correct commitment
- ✅ `TestMuSig2KeyAggregation` - Key aggregation determinism
- ✅ `TestBoardingWithChange` - Change output handling
- ✅ `TestCommitmentInputOrdering` - Input sorting verification
- ✅ `TestTransactionBasicProperties` - Version/locktime/sequences

## Usage

```go
import arkbuilders "github.com/SashaZezulinsky/ark-tx-builder"

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

## Project Structure

```
.
├── types.go          # Core types and constants
├── taproot.go        # Taproot & MuSig2 utilities
├── boarding.go       # Boarding transaction builder
├── commitment.go     # Commitment transaction builder
├── forfeit.go        # Forfeit transaction builder
├── builders_test.go  # Comprehensive test suite
└── cmd/              # CLI binary
```

## References

- [Ark Litepaper](https://www.arkpill.me/deep-dive) - Sections 4.3, 4.5, Definition 4.9
- [BIP-327 (MuSig2)](https://github.com/bitcoin/bips/blob/master/bip-0327.mediawiki)
- [BIP-341 (Taproot)](https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki)
- [BIP-340 (Schnorr)](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki)
- [Ark Reference Implementation](https://github.com/ark-network/ark)

## License

MIT
