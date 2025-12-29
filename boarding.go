package arkbuilders

import (
	"errors"
	"sort"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// BuildBoardingTx creates a boarding transaction for depositing into Ark
// Returns a deterministic transaction that locks BTC in a Taproot output with:
// - Cooperative path: MuSig2_Agg(user, operator)
// - Timeout path: user signature + relative timelock
func (tb *TxBuilder) BuildBoardingTx(params *BoardingTxParams) (*wire.MsgTx, error) {
	// Validate parameters
	if params.FundingUTXO == nil {
		return nil, errors.New("funding UTXO is required")
	}
	if params.UserPubKey == nil || params.OperatorPubKey == nil {
		return nil, errors.New("user and operator public keys are required")
	}
	if params.Amount <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if params.FeeRate < MinFeeRate {
		params.FeeRate = MinFeeRate
	}

	// Create new transaction with deterministic fields
	tx := newDeterministicTx(TxVersion, 0)

	// Add input: spending fundingUTXO with sequence 0xFFFFFFFD
	txIn := wire.NewTxIn(
		wire.NewOutPoint(&params.FundingUTXO.TxHash, params.FundingUTXO.OutputIndex),
		nil,
		nil,
	)
	txIn.Sequence = SequenceBoardingTx
	tx.AddTxIn(txIn)

	// Build Taproot output script
	// Path 1: MuSig2 aggregated key (cooperative)
	aggKey, err := MuSig2AggregateKeys(params.UserPubKey, params.OperatorPubKey)
	if err != nil {
		return nil, err
	}
	path1Script, err := BuildCheckSigScript(aggKey)
	if err != nil {
		return nil, err
	}

	// Path 2: User key + relative timelock (timeout)
	path2Script, err := BuildCheckSigWithTimelockScript(params.UserPubKey, params.TimeoutBlocks)
	if err != nil {
		return nil, err
	}

	// Create Taproot script with both paths
	// Sort scripts deterministically
	scripts := sortScripts([][]byte{path1Script, path2Script})
	taprootScript, err := CreateTaprootScript(nil, scripts) // nil = unspendable keypath
	if err != nil {
		return nil, err
	}

	// Add main output
	tx.AddTxOut(wire.NewTxOut(params.Amount, taprootScript))

	// Calculate fee
	estimatedSize := estimateTxSize(tx, 1, 0) // 1 input, no witness data for estimation
	fee := estimatedSize * params.FeeRate

	// Check if we need a change output
	change := params.FundingUTXO.Amount - params.Amount - fee
	if change > DustLimit && params.ChangeAddress != "" {
		// Parse change address
		changeAddr, err := btcutil.DecodeAddress(params.ChangeAddress, nil)
		if err != nil {
			return nil, err
		}
		changeScript, err := txscript.PayToAddrScript(changeAddr)
		if err != nil {
			return nil, err
		}

		// Add change output
		tx.AddTxOut(wire.NewTxOut(change, changeScript))

		// Re-estimate fee with change output
		estimatedSize = estimateTxSize(tx, 1, 0)
		fee = estimatedSize * params.FeeRate
		change = params.FundingUTXO.Amount - params.Amount - fee

		// Update change amount
		if change > DustLimit {
			tx.TxOut[1].Value = change
		} else {
			// Remove change output if it would be dust
			tx.TxOut = tx.TxOut[:1]
		}
	}

	// Sort outputs deterministically (BIP-69 style)
	sortTxOutputs(tx)

	return tx, nil
}

// estimateTxSize estimates the size of a transaction in vbytes
func estimateTxSize(tx *wire.MsgTx, numInputs, witnessSize int) int64 {
	// Base size (non-witness data)
	baseSize := tx.SerializeSize()

	// Estimate witness size if not provided
	if witnessSize == 0 {
		// P2TR witness: ~66 bytes (control block + signature)
		witnessSize = numInputs * 66
	}

	// Weight = base_size * 4 + witness_size
	// vsize = weight / 4
	weight := baseSize*4 + witnessSize
	vsize := (weight + 3) / 4 // Round up

	return int64(vsize)
}

// sortScripts sorts scripts deterministically by their byte representation
func sortScripts(scripts [][]byte) [][]byte {
	sorted := make([][]byte, len(scripts))
	copy(sorted, scripts)

	sort.Slice(sorted, func(i, j int) bool {
		// First compare by length
		if len(sorted[i]) != len(sorted[j]) {
			return len(sorted[i]) < len(sorted[j])
		}
		// Then compare lexicographically
		for k := 0; k < len(sorted[i]); k++ {
			if sorted[i][k] != sorted[j][k] {
				return sorted[i][k] < sorted[j][k]
			}
		}
		return false
	})

	return sorted
}

// sortTxOutputs sorts transaction outputs deterministically
// Based on BIP-69: amount ascending, then script ascending
func sortTxOutputs(tx *wire.MsgTx) {
	sort.Slice(tx.TxOut, func(i, j int) bool {
		// First compare by amount
		if tx.TxOut[i].Value != tx.TxOut[j].Value {
			return tx.TxOut[i].Value < tx.TxOut[j].Value
		}

		// Then compare by script
		iScript := tx.TxOut[i].PkScript
		jScript := tx.TxOut[j].PkScript

		// Compare length
		if len(iScript) != len(jScript) {
			return len(iScript) < len(jScript)
		}

		// Compare lexicographically
		for k := 0; k < len(iScript); k++ {
			if iScript[k] != jScript[k] {
				return iScript[k] < jScript[k]
			}
		}

		return false
	})
}
