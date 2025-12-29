package arkbuilders

import (
	"errors"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// BuildForfeitTx creates a forfeit transaction for atomic VTXO swaps
// Returns a deterministic transaction with:
// - Inputs: [vtxo, connector anchor] with sequence 0xFFFFFFFF
// - Output: single P2TR to operator
// - CRITICAL: Uses SIGHASH_ALL to bind to specific commitment tx
func (tb *TxBuilder) BuildForfeitTx(params *ForfeitTxParams) (*wire.MsgTx, error) {
	// Validate parameters
	if params.VTXO == nil {
		return nil, errors.New("VTXO is required")
	}
	if params.ConnectorAnchor == nil {
		return nil, errors.New("connector anchor is required")
	}
	if params.OperatorPubKey == nil {
		return nil, errors.New("operator public key is required")
	}
	if params.VTXO.Amount <= 0 {
		return nil, errors.New("VTXO amount must be positive")
	}
	if params.ConnectorAnchor.Amount <= 0 {
		return nil, errors.New("connector anchor amount must be positive")
	}
	if params.FeeRate < MinFeeRate {
		params.FeeRate = MinFeeRate
	}

	// Create new transaction with deterministic fields
	tx := newDeterministicTx(TxVersion, 0)

	// Add VTXO input (first)
	vtxoIn := wire.NewTxIn(
		wire.NewOutPoint(&params.VTXO.TxHash, params.VTXO.OutputIndex),
		nil,
		nil,
	)
	vtxoIn.Sequence = SequenceForfeitTx
	tx.AddTxIn(vtxoIn)

	// Add connector anchor input (second)
	anchorIn := wire.NewTxIn(
		wire.NewOutPoint(&params.ConnectorAnchor.TxHash, params.ConnectorAnchor.OutputIndex),
		nil,
		nil,
	)
	anchorIn.Sequence = SequenceForfeitTx
	tx.AddTxIn(anchorIn)

	// Create operator output (P2TR)
	operatorScript, err := BuildCheckSigScript(params.OperatorPubKey)
	if err != nil {
		return nil, err
	}

	// Wrap in Taproot
	taprootScript, err := CreateTaprootScript(nil, [][]byte{operatorScript})
	if err != nil {
		return nil, err
	}

	// Calculate output amount (inputs - fee)
	totalInput := params.VTXO.Amount + params.ConnectorAnchor.Amount
	estimatedSize := estimateTxSize(tx, 2, 0) // 2 inputs
	fee := estimatedSize * params.FeeRate

	outputAmount := totalInput - fee
	if outputAmount <= 0 {
		return nil, errors.New("insufficient input amount to cover fees")
	}

	// Add single operator output
	tx.AddTxOut(wire.NewTxOut(outputAmount, taprootScript))

	return tx, nil
}

// GetSighashType returns the sighash type for forfeit transactions
// CRITICAL: Always returns SIGHASH_ALL to bind to specific commitment tx
func GetSighashType() txscript.SigHashType {
	return txscript.SigHashAll
}
