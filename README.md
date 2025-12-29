# Ark UTXO Transaction Builders

Deterministic Bitcoin transaction builders for the Ark protocol, implementing boarding, commitment, and forfeit transactions with Taproot support and MuSig2 key aggregation.

## Build & Test

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Build binary
make build

# Run all checks (format, vet, lint, test)
make check

# Clean build artifacts
make clean
```

## Quick Start

```bash
# Install dependencies
go mod download

# Run tests
go test -v ./...

# Run specific test
go test -v -run TestBoardingDeterminism
```

## Design Choices

### MuSig2 Library

**Choice:** Custom deterministic MuSig2 key aggregation implementation

**Rationale:**
- **Determinism:** The implementation ensures that key aggregation is completely deterministic and produces byte-identical results every time
- **Simplicity:** Focused on key aggregation only (no interactive signing), which is sufficient for constructing deterministic transaction templates
- **Order Independence:** Keys are sorted before aggregation to ensure the same aggregate key regardless of input order
- **BIP-340 Compliance:** Uses Schnorr signatures and tagged hashing as per BIP-340 specification

**Implementation Details:**
```go
func MuSig2AggregateKeys(pubKeys ...*btcec.PublicKey) (*btcec.PublicKey, error)
```
- Sorts public keys lexicographically for deterministic ordering
- Computes aggregation coefficients using tagged hashes
- Returns a single aggregated public key suitable for Taproot keypath or scriptpath spending

**Alternative Considered:** External libraries like `github.com/decred/dcrd/dcrec/secp256k1/v4` were considered, but a custom implementation provides better control over determinism and reduces dependencies.

### Taproot Approach

**Choice:** Script-path-only Taproot outputs with unspendable internal key

**Rationale:**
- **Flexibility:** All spending paths are explicitly defined in the script tree
- **Transparency:** No hidden keypath spend; all conditions are visible in the script paths
- **NUMS Point:** Uses "Nothing Up My Sleeve" point as the internal key to make keypath provably unspendable
- **Merkle Tree:** Organizes multiple spending conditions in a Taproot script tree for efficient verification

**Implementation:**
- Boarding Transaction:
  - Path 1: MuSig2(user, operator) - cooperative spend
  - Path 2: user + CSV(timeout) - unilateral exit after timeout

- Commitment Transaction:
  - Batch Output:
    - Path 1: operator + CLTV(expiry) - sweep after batch expiry
    - Path 2: covenant/multisig - unroll path for users
  - Connector Output:
    - Simple operator-controlled output for forfeit atomicity

- Forfeit Transaction:
  - Single output to operator
  - Uses SIGHASH_ALL to bind to specific commitment transaction

**Script Determinism:**
- All scripts are sorted lexicographically before building the Taproot tree
- Ensures consistent Merkle root and output addresses

### Fee Calculation Strategy

**Choice:** Size-based estimation with configurable fee rate

**Rationale:**
- **Predictability:** Fee is calculated based on estimated transaction virtual size (vsize)
- **Flexibility:** Configurable fee rate (sat/vbyte) with 1 sat/vbyte minimum
- **Accuracy:** Accounts for witness data in weight calculations (weight = base_size * 4 + witness_size)

**Implementation Details:**
```go
func estimateTxSize(tx *wire.MsgTx, numInputs, witnessSize int) int64
```
- Base size: Non-witness transaction data
- Witness size: Estimated at ~66 bytes per P2TR input (signature + control block)
- vsize = (weight + 3) / 4 (rounded up)
- Final fee = vsize * fee_rate

**Fee Handling:**
- Boarding: Supports optional change output if funding exceeds amount + fees
- Commitment: Validates that inputs cover outputs + fees
- Forfeit: Deducts fee from total input amount

**Dust Limit:**
- Enforces 546 satoshi minimum for P2TR outputs
- Change outputs below dust limit are excluded from final transaction

### Transaction Determinism

**Guarantees:**
1. **Version & Locktime:** Always set to deterministic values (version=2, locktime=0)
2. **Sequence Numbers:** Fixed per transaction type (boarding=0xFFFFFFFD, commitment/forfeit=0xFFFFFFFF)
3. **Output Ordering:** BIP-69 style sorting (by amount, then by script)
4. **Script Ordering:** Lexicographic sorting of Taproot script paths
5. **Key Ordering:** Sorted before MuSig2 aggregation

**Testing:**
- All transaction builders tested with 100 iterations to verify identical txids
- Sighash stability verified across multiple builds
- Atomicity verified by checking input references

## Architecture

### Project Structure

```
.
├── README.md              # This file
├── go.mod                 # Go module dependencies
├── Makefile              # Build and test automation
├── types.go              # Core types and constants
├── taproot.go            # Taproot script and MuSig2 utilities
├── boarding.go           # Boarding transaction builder
├── commitment.go         # Commitment transaction builder
├── forfeit.go            # Forfeit transaction builder
├── builders_test.go      # Comprehensive test suite
└── .github/
    └── workflows/
        └── ci.yml        # GitHub Actions CI pipeline
```

### Key Components

**TxBuilder:** Main transaction builder interface
```go
type TxBuilder struct{}

func (tb *TxBuilder) BuildBoardingTx(params *BoardingTxParams) (*wire.MsgTx, error)
func (tb *TxBuilder) BuildCommitmentTx(params *CommitmentTxParams) (*wire.MsgTx, error)
func (tb *TxBuilder) BuildForfeitTx(params *ForfeitTxParams) (*wire.MsgTx, error)
```

**Core Functions:**
- `MuSig2AggregateKeys`: Deterministic key aggregation
- `CreateTaprootScript`: Build Taproot outputs with script paths
- `BuildCheckSigScript`: Create checksig scripts
- `BuildCheckSigWithTimelockScript`: Create checksig + timelock scripts

## Test Coverage

The test suite includes:

1. **TestBoardingDeterminism** - Verifies same params → same txid (100 runs)
2. **TestCommitmentSighashStability** - Verifies same params → same sighash (100 runs)
3. **TestForfeitAtomicity** - Verifies forfeit references correct commitment tx
4. **TestMuSig2KeyAggregation** - Verifies MuSig2 aggregation determinism and order independence
5. **TestBoardingWithChange** - Verifies change output handling and dust limit enforcement
6. **TestTransactionBasicProperties** - Verifies version, locktime, sequence numbers, output ordering

Run tests:
```bash
make test                # Run all tests
make test-verbose        # Run with verbose output
make test-coverage       # Generate coverage report
```

## CI/CD Pipeline

GitHub Actions workflow (`.github/workflows/ci.yml`) includes:

1. **Lint Job:**
   - Runs golangci-lint for code quality checks
   - Ensures consistent code style

2. **Test Job:**
   - Tests on Go 1.20, 1.21, 1.22
   - Runs with race detector
   - Generates coverage reports
   - Uploads to Codecov

3. **Build Job:**
   - Builds binary artifact
   - Verifies binary integrity
   - Uploads artifact for download

4. **Validate Job:**
   - Checks code formatting
   - Runs go vet
   - Verifies go.mod is tidy

## Dependencies

**Core:**
- `github.com/btcsuite/btcd` - Bitcoin protocol implementation
- `github.com/btcsuite/btcd/btcec/v2` - Elliptic curve cryptography
- `github.com/btcsuite/btcd/btcutil` - Bitcoin utilities

**Testing:**
- `github.com/stretchr/testify` - Testing assertions and utilities

**Development:**
- `golangci-lint` - Comprehensive Go linter

## References

**Bitcoin Improvement Proposals (BIPs):**
- [BIP-341 (Taproot)](https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki) - Taproot specification
- [BIP-327 (MuSig2)](https://github.com/bitcoin/bips/blob/master/bip-0327.mediawiki) - MuSig2 multi-signatures
- [BIP-340 (Schnorr)](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki) - Schnorr signatures
- [BIP-69](https://github.com/bitcoin/bips/blob/master/bip-0069.mediawiki) - Deterministic transaction ordering

**Ark Protocol:**
- Ark Litepaper - Sections 4.3 (Batch Swaps), 4.5 (Boarding and Leaving), Definition 4.9 (Commitment Transactions)
- [Ark Go Implementation](https://github.com/arkade-os/arkd) - Reference implementation

## Design Decisions Summary

| Aspect | Choice | Reason |
|--------|--------|--------|
| **MuSig2** | Custom deterministic implementation | Full control over determinism, minimal dependencies |
| **Taproot** | Script-path-only with NUMS internal key | Explicit spending conditions, provably unspendable keypath |
| **Fee Calc** | vsize-based with configurable rate | Predictable, accurate weight accounting |
| **Output Order** | BIP-69 style (amount, then script) | Deterministic, privacy-preserving |
| **Script Order** | Lexicographic sorting | Deterministic Merkle root |
| **Key Order** | Lexicographic sorting before aggregation | Order-independent aggregation |
| **Dust Handling** | 546 sat minimum, exclude below dust | Standard Bitcoin dust limit |
| **Testing** | 100 iterations for determinism tests | High confidence in determinism |

## Time Spent

**Total: ~8 hours**

Breakdown:
- Research and design (Ark litepaper, BIPs): 2 hours
- Core implementation (transaction builders): 3 hours
- Taproot and MuSig2 utilities: 1.5 hours
- Comprehensive test suite: 1 hour
- CI/CD pipeline and documentation: 0.5 hours

## License

MIT
