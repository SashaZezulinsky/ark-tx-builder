package arkbuilders

import (
	"bytes"
	"errors"
	"sort"

	"github.com/btcsuite/btcd/wire"
)

// BuildCommitmentTx creates a commitment transaction that batches VTXOs
// Returns a deterministic transaction with:
// - Inputs: operator UTXOs + boarding outputs (sequence 0xFFFFFFFF)
// - Output 1: Batch (Taproot with sweep and unroll paths)
// - Output 2: Connector (dust, operator controlled)
func (tb *TxBuilder) BuildCommitmentTx(params *CommitmentTxParams) (*wire.MsgTx, error) {
	// Validate parameters
	if len(params.OperatorUTXOs) == 0 {
		return nil, errors.New("at least one operator UTXO is required")
	}
	if params.OperatorPubKey == nil {
		return nil, errors.New("operator public key is required")
	}
	if params.BatchAmount <= 0 {
		return nil, errors.New("batch amount must be positive")
	}
	// Use local variable to avoid mutating params
	connectorAmount := params.ConnectorAmount
	if connectorAmount < DustLimit {
		connectorAmount = DustLimit
	}
	if params.FeeRate < MinFeeRate {
		params.FeeRate = MinFeeRate
	}

	// Create new transaction with deterministic fields
	tx := newDeterministicTx(TxVersion, 0)

	// Add operator UTXO inputs first (deterministic ordering)
	for _, utxo := range params.OperatorUTXOs {
		txIn := wire.NewTxIn(
			wire.NewOutPoint(&utxo.TxHash, utxo.OutputIndex),
			nil,
			nil,
		)
		txIn.Sequence = SequenceCommitmentTx
		tx.AddTxIn(txIn)
	}

	// Add boarding outputs as inputs
	for _, utxo := range params.BoardingOutputs {
		txIn := wire.NewTxIn(
			wire.NewOutPoint(&utxo.TxHash, utxo.OutputIndex),
			nil,
			nil,
		)
		txIn.Sequence = SequenceCommitmentTx
		tx.AddTxIn(txIn)
	}

	// Sort inputs for deterministic ordering
	sortTxInputs(tx)

	// Build Batch output (Output 1)
	// Path 1: Sweep - operator can claim after batch expiry
	sweepScript, err := BuildCheckSigWithAbsTimelockScript(params.OperatorPubKey, params.BatchExpiry)
	if err != nil {
		return nil, err
	}

	// Path 2: Unroll - covenant path for users to exit
	// This is a simplified representation - in practice would use covenant opcodes
	// For now, we use a multisig with all user keys to represent the covenant
	var unrollScript []byte
	if len(params.UserPubKeys) > 0 {
		// Aggregate all user keys for the unroll path
		// This represents the covenant that users can collaboratively unroll the batch
		aggUserKey, err := MuSig2AggregateKeys(params.UserPubKeys...)
		if err != nil {
			return nil, err
		}
		unrollScript, err = BuildCheckSigScript(aggUserKey)
		if err != nil {
			return nil, err
		}
	} else {
		// If no user keys, only sweep path available
		unrollScript = sweepScript
	}

	// Create batch Taproot script
	batchScripts := sortScripts([][]byte{sweepScript, unrollScript})
	batchScript, err := CreateTaprootScript(nil, batchScripts)
	if err != nil {
		return nil, err
	}

	// Add batch output (must be first)
	tx.AddTxOut(wire.NewTxOut(params.BatchAmount, batchScript))

	// Build Connector output (Output 2)
	// Simple operator-controlled output
	connectorScript, err := BuildCheckSigScript(params.OperatorPubKey)
	if err != nil {
		return nil, err
	}

	// Wrap in Taproot
	connectorTaprootScript, err := CreateTaprootScript(nil, [][]byte{connectorScript})
	if err != nil {
		return nil, err
	}

	// Add connector output (must be second)
	tx.AddTxOut(wire.NewTxOut(connectorAmount, connectorTaprootScript))

	// Verify we have enough inputs to cover outputs + fees
	totalInput := int64(0)
	for _, utxo := range params.OperatorUTXOs {
		totalInput += utxo.Amount
	}
	for _, utxo := range params.BoardingOutputs {
		totalInput += utxo.Amount
	}

	totalOutput := params.BatchAmount + connectorAmount
	estimatedSize := estimateTxSize(tx, len(tx.TxIn), 0)
	fee := estimatedSize * params.FeeRate

	if totalInput < totalOutput+fee {
		return nil, errors.New("insufficient input amount to cover outputs and fees")
	}

	// Note: Outputs are already in correct order (batch first, connector second)
	// No sorting needed to maintain deterministic order

	return tx, nil
}

// sortTxInputs sorts transaction inputs deterministically
// Sorts by txid (hash) first, then by output index
func sortTxInputs(tx *wire.MsgTx) {
	sort.Slice(tx.TxIn, func(i, j int) bool {
		// Compare transaction hashes
		cmp := bytes.Compare(
			tx.TxIn[i].PreviousOutPoint.Hash[:],
			tx.TxIn[j].PreviousOutPoint.Hash[:],
		)
		if cmp != 0 {
			return cmp < 0
		}
		// If hashes are equal, compare output indices
		return tx.TxIn[i].PreviousOutPoint.Index < tx.TxIn[j].PreviousOutPoint.Index
	})
}
