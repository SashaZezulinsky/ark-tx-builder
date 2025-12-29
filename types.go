package arkbuilders

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// UTXO represents an unspent transaction output
type UTXO struct {
	TxHash       chainhash.Hash
	OutputIndex  uint32
	Amount       int64
	ScriptPubKey []byte
}

// BoardingTxParams contains parameters for building a boarding transaction
type BoardingTxParams struct {
	FundingUTXO    *UTXO
	Amount         int64
	UserPubKey     *btcec.PublicKey
	OperatorPubKey *btcec.PublicKey
	TimeoutBlocks  uint16
	ChangeAddress  string // Optional, for change output
	FeeRate        int64  // satoshis per vbyte
}

// CommitmentTxParams contains parameters for building a commitment transaction
type CommitmentTxParams struct {
	OperatorUTXOs   []*UTXO
	BoardingOutputs []*UTXO
	BatchAmount     int64
	ConnectorAmount int64 // Dust amount
	OperatorPubKey  *btcec.PublicKey
	UserPubKeys     []*btcec.PublicKey
	BatchExpiry     uint32 // Absolute lock time
	FeeRate         int64
}

// ForfeitTxParams contains parameters for building a forfeit transaction
type ForfeitTxParams struct {
	VTXO            *UTXO
	ConnectorAnchor *UTXO
	OperatorPubKey  *btcec.PublicKey
	FeeRate         int64
}

// TxBuilder provides methods to build deterministic transactions
type TxBuilder struct{}

// NewTxBuilder creates a new TxBuilder instance
func NewTxBuilder() *TxBuilder {
	return &TxBuilder{}
}

const (
	// Transaction version
	TxVersion = 2

	// Sequence numbers
	SequenceBoardingTx   = 0xFFFFFFFD
	SequenceCommitmentTx = 0xFFFFFFFF
	SequenceForfeitTx    = 0xFFFFFFFF

	// Dust limit (546 satoshis for P2TR outputs)
	DustLimit = 546

	// Default fee rate (1 sat/vbyte minimum)
	MinFeeRate = 1
)

// Helper function to create a new transaction with deterministic fields
func newDeterministicTx(version int32, lockTime uint32) *wire.MsgTx {
	tx := wire.NewMsgTx(version)
	tx.LockTime = lockTime
	return tx
}
