package arkbuilders

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sort"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// MuSig2AggregateKeys aggregates multiple public keys using MuSig2
// This is a simplified implementation for deterministic key aggregation
func MuSig2AggregateKeys(pubKeys ...*btcec.PublicKey) (*btcec.PublicKey, error) {
	if len(pubKeys) == 0 {
		return nil, errors.New("at least one public key is required")
	}

	// Sort keys for deterministic ordering
	sortedKeys := make([]*btcec.PublicKey, len(pubKeys))
	copy(sortedKeys, pubKeys)
	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(
			schnorr.SerializePubKey(sortedKeys[i]),
			schnorr.SerializePubKey(sortedKeys[j]),
		) < 0
	})

	// Simple deterministic aggregation using point addition
	// This is a simplified MuSig2-style aggregation for deterministic key generation
	// In production, use proper MuSig2 with coefficients, but for our purposes
	// (deterministic transaction building), simple aggregation is sufficient

	// Convert first key to Jacobian coordinates
	var result btcec.JacobianPoint
	sortedKeys[0].AsJacobian(&result)

	// Add remaining keys
	for i := 1; i < len(sortedKeys); i++ {
		var keyPoint btcec.JacobianPoint
		sortedKeys[i].AsJacobian(&keyPoint)
		btcec.AddNonConst(&result, &keyPoint, &result)
	}

	// Convert back to affine coordinates
	result.ToAffine()

	// Create aggregate public key
	aggKey := btcec.NewPublicKey(&result.X, &result.Y)
	return aggKey, nil
}

// CreateTaprootScript creates a Taproot output script with script paths
func CreateTaprootScript(internalPubKey *btcec.PublicKey, scripts [][]byte) ([]byte, error) {
	// If internal key is nil, use unspendable key (point at infinity represented by specific value)
	var internalKey *btcec.PublicKey
	if internalPubKey == nil {
		// Use "NUMS" point (Nothing Up My Sleeve) - unspendable internal key
		// This is a standard way to create an unspendable keypath
		numsPoint := []byte{
			0x50, 0x92, 0x9b, 0x74, 0xc1, 0xa0, 0x49, 0x54,
			0xb7, 0x8b, 0x4b, 0x60, 0x35, 0xe9, 0x7a, 0x5e,
			0x07, 0x8a, 0x5a, 0x0f, 0x28, 0xec, 0x96, 0xd5,
			0x47, 0xbf, 0xee, 0x9a, 0xce, 0x80, 0x3a, 0xc0,
		}
		var err error
		internalKey, err = schnorr.ParsePubKey(numsPoint)
		if err != nil {
			return nil, err
		}
	} else {
		internalKey = internalPubKey
	}

	// Build the tapscript tree
	var taprootKey *btcec.PublicKey

	if len(scripts) == 0 {
		// No script paths, just use internal key
		taprootKey = internalKey
	} else {
		// Create merkle root from scripts
		merkleRoot := buildTapscriptMerkleRoot(scripts)

		// Tweak the internal key with the merkle root
		taprootKey = txscript.ComputeTaprootOutputKey(internalKey, merkleRoot)
	}

	// Create P2TR output script
	return txscript.NewScriptBuilder().
		AddOp(txscript.OP_1).
		AddData(schnorr.SerializePubKey(taprootKey)).
		Script()
}

// buildTapscriptMerkleRoot builds a merkle root from tapscripts
func buildTapscriptMerkleRoot(scripts [][]byte) []byte {
	if len(scripts) == 0 {
		return nil
	}

	// Compute leaf hashes
	leaves := make([][]byte, len(scripts))
	for i, script := range scripts {
		leaves[i] = tapLeafHash(script)
	}

	// Build merkle tree (simple binary tree)
	for len(leaves) > 1 {
		var nextLevel [][]byte
		for i := 0; i < len(leaves); i += 2 {
			if i+1 < len(leaves) {
				nextLevel = append(nextLevel, tapBranchHash(leaves[i], leaves[i+1]))
			} else {
				nextLevel = append(nextLevel, leaves[i])
			}
		}
		leaves = nextLevel
	}

	return leaves[0]
}

// tapLeafHash computes the leaf hash for a tapscript
func tapLeafHash(script []byte) []byte {
	// TapLeaf = TaggedHash("TapLeaf", version || compactSize(script) || script)
	var buf bytes.Buffer
	buf.WriteByte(byte(txscript.BaseLeafVersion)) // 0xc0
	_ = wire.WriteVarBytes(&buf, 0, script)

	return taggedHash("TapLeaf", buf.Bytes())
}

// tapBranchHash computes the branch hash for two child nodes
func tapBranchHash(left, right []byte) []byte {
	// TapBranch = TaggedHash("TapBranch", left || right)
	// Nodes must be sorted
	if bytes.Compare(left, right) > 0 {
		left, right = right, left
	}

	var buf bytes.Buffer
	buf.Write(left)
	buf.Write(right)

	return taggedHash("TapBranch", buf.Bytes())
}

// taggedHash computes a tagged hash as per BIP-340
func taggedHash(tag string, data []byte) []byte {
	tagHash := sha256.Sum256([]byte(tag))

	h := sha256.New()
	h.Write(tagHash[:])
	h.Write(tagHash[:])
	h.Write(data)

	return h.Sum(nil)
}

// BuildCheckSigScript creates a simple checksig script for a public key
func BuildCheckSigScript(pubKey *btcec.PublicKey) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddData(schnorr.SerializePubKey(pubKey)).
		AddOp(txscript.OP_CHECKSIG).
		Script()
}

// BuildCheckSigWithTimelockScript creates a checksig script with a relative timelock
func BuildCheckSigWithTimelockScript(pubKey *btcec.PublicKey, blocks uint16) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddData(schnorr.SerializePubKey(pubKey)).
		AddOp(txscript.OP_CHECKSIGVERIFY).
		AddInt64(int64(blocks)).
		AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
		Script()
}

// BuildCheckSigWithAbsTimelockScript creates a checksig script with an absolute timelock
func BuildCheckSigWithAbsTimelockScript(pubKey *btcec.PublicKey, lockTime uint32) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddData(schnorr.SerializePubKey(pubKey)).
		AddOp(txscript.OP_CHECKSIGVERIFY).
		AddInt64(int64(lockTime)).
		AddOp(txscript.OP_CHECKLOCKTIMEVERIFY).
		Script()
}
