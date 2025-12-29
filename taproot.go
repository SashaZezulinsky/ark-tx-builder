package arkbuilders

import (
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/txscript"
)

// MuSig2AggregateKeys aggregates multiple public keys using MuSig2
// Uses btcd's BIP-327 compliant implementation to prevent rogue key attacks
// Based on BIP-327: https://github.com/bitcoin/bips/blob/master/bip-0327.mediawiki
func MuSig2AggregateKeys(pubKeys ...*btcec.PublicKey) (*btcec.PublicKey, error) {
	if len(pubKeys) == 0 {
		return nil, errors.New("at least one public key is required")
	}

	// Use btcd's battle-tested MuSig2 implementation
	// sort=true ensures deterministic key aggregation regardless of input order
	aggKey, _, _, err := musig2.AggregateKeys(pubKeys, true)
	if err != nil {
		return nil, err
	}

	// Return the final aggregated key
	return aggKey.FinalKey, nil
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
		// Use btcd's built-in Taproot script tree assembly
		leaves := make([]txscript.TapLeaf, len(scripts))
		for i, script := range scripts {
			leaves[i] = txscript.NewBaseTapLeaf(script)
		}

		scriptTree := txscript.AssembleTaprootScriptTree(leaves...)
		merkleRoot := scriptTree.RootNode.TapHash()

		// Tweak the internal key with the merkle root
		taprootKey = txscript.ComputeTaprootOutputKey(internalKey, merkleRoot[:])
	}

	// Create P2TR output script using btcd's helper
	return txscript.PayToTaprootScript(taprootKey)
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
