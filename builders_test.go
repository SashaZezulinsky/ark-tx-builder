package arkbuilders

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test private key
func createTestPrivKey(t *testing.T, seed byte) *btcec.PrivateKey {
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = seed
	}
	privKey, _ := btcec.PrivKeyFromBytes(keyBytes)
	return privKey
}

// Helper function to create a test UTXO
func createTestUTXO(amount int64, index uint32) *UTXO {
	hash, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000001")
	return &UTXO{
		TxHash:      *hash,
		OutputIndex: index,
		Amount:      amount,
	}
}

// TestBoardingDeterminism verifies that the same parameters produce the same txid
func TestBoardingDeterminism(t *testing.T) {
	builder := NewTxBuilder()

	// Create test keys
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	params := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	}

	// Build the transaction 100 times
	var txids []string
	for i := 0; i < 100; i++ {
		tx, err := builder.BuildBoardingTx(params)
		require.NoError(t, err)
		require.NotNil(t, tx)

		txid := tx.TxHash().String()
		txids = append(txids, txid)
	}

	// Verify all txids are identical
	firstTxid := txids[0]
	for i, txid := range txids {
		assert.Equal(t, firstTxid, txid, "Transaction %d has different txid", i)
	}

	t.Logf("Determinism verified: all 100 transactions have txid: %s", firstTxid)
}

// TestCommitmentSighashStability verifies that the same parameters produce the same sighash
func TestCommitmentSighashStability(t *testing.T) {
	builder := NewTxBuilder()

	// Create test keys
	operatorPrivKey := createTestPrivKey(t, 0x02)
	user1PrivKey := createTestPrivKey(t, 0x03)
	user2PrivKey := createTestPrivKey(t, 0x04)

	params := &CommitmentTxParams{
		OperatorUTXOs: []*UTXO{
			createTestUTXO(500000, 0),
		},
		BoardingOutputs: []*UTXO{
			createTestUTXO(100000, 1),
		},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		UserPubKeys: []*btcec.PublicKey{
			user1PrivKey.PubKey(),
			user2PrivKey.PubKey(),
		},
		BatchExpiry: 800000,
		FeeRate:     1,
	}

	// Build the transaction 100 times and compute sighashes
	var sighashes []string
	for i := 0; i < 100; i++ {
		tx, err := builder.BuildCommitmentTx(params)
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Compute sighash for first input
		// Create a proper prev output fetcher
		prevOut := wire.NewTxOut(params.OperatorUTXOs[0].Amount, []byte{0x51, 0x20}) // Simple P2TR script
		prevFetcher := txscript.NewCannedPrevOutputFetcher(
			prevOut.PkScript,
			prevOut.Value,
		)
		sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)

		sigHash, err := txscript.CalcTaprootSignatureHash(
			sigHashes,
			txscript.SigHashAll,
			tx,
			0,
			prevFetcher,
		)
		require.NoError(t, err)

		sighashes = append(sighashes, hex.EncodeToString(sigHash))
	}

	// Verify all sighashes are identical
	firstSighash := sighashes[0]
	for i, sighash := range sighashes {
		assert.Equal(t, firstSighash, sighash, "Transaction %d has different sighash", i)
	}

	t.Logf("Sighash stability verified: all 100 transactions have sighash: %s", firstSighash)
}

// TestForfeitAtomicity verifies that forfeit transaction correctly references the commitment
func TestForfeitAtomicity(t *testing.T) {
	builder := NewTxBuilder()

	// Create test keys
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// First create a commitment transaction
	commitParams := &CommitmentTxParams{
		OperatorUTXOs: []*UTXO{
			createTestUTXO(500000, 0),
		},
		BoardingOutputs: []*UTXO{
			createTestUTXO(100000, 1),
		},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		UserPubKeys:     []*btcec.PublicKey{},
		BatchExpiry:     800000,
		FeeRate:         1,
	}

	commitTx, err := builder.BuildCommitmentTx(commitParams)
	require.NoError(t, err)
	require.NotNil(t, commitTx)

	commitTxHash := commitTx.TxHash()

	// Now create a forfeit transaction that spends from the commitment
	vtxo := &UTXO{
		TxHash:      commitTxHash,
		OutputIndex: 0, // Batch output
		Amount:      400000,
	}

	connectorAnchor := &UTXO{
		TxHash:      commitTxHash,
		OutputIndex: 1, // Connector output
		Amount:      1000,
	}

	forfeitParams := &ForfeitTxParams{
		VTXO:            vtxo,
		ConnectorAnchor: connectorAnchor,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	}

	forfeitTx, err := builder.BuildForfeitTx(forfeitParams)
	require.NoError(t, err)
	require.NotNil(t, forfeitTx)

	// Verify forfeit transaction references the commitment transaction
	assert.Equal(t, commitTxHash, forfeitTx.TxIn[0].PreviousOutPoint.Hash,
		"Forfeit tx input 0 should reference commitment batch output")
	assert.Equal(t, commitTxHash, forfeitTx.TxIn[1].PreviousOutPoint.Hash,
		"Forfeit tx input 1 should reference commitment connector output")

	// Verify SIGHASH_ALL is used (this ensures atomicity)
	sighashType := GetSighashType()
	assert.Equal(t, txscript.SigHashAll, sighashType,
		"Forfeit transaction must use SIGHASH_ALL for atomicity")

	t.Logf("Atomicity verified: forfeit tx %s references commitment tx %s",
		forfeitTx.TxHash().String(), commitTxHash.String())
}

// TestMuSig2KeyAggregation verifies that MuSig2 key aggregation works correctly
func TestMuSig2KeyAggregation(t *testing.T) {
	// Create test keys
	key1 := createTestPrivKey(t, 0x01).PubKey()
	key2 := createTestPrivKey(t, 0x02).PubKey()
	key3 := createTestPrivKey(t, 0x03).PubKey()

	// Test aggregation with 2 keys
	agg2, err := MuSig2AggregateKeys(key1, key2)
	require.NoError(t, err)
	require.NotNil(t, agg2)

	// Test determinism: aggregating same keys should give same result
	agg2_v2, err := MuSig2AggregateKeys(key1, key2)
	require.NoError(t, err)
	assert.True(t, agg2.IsEqual(agg2_v2), "Aggregation should be deterministic")

	// Test order independence: different order should give same result
	agg2_reversed, err := MuSig2AggregateKeys(key2, key1)
	require.NoError(t, err)
	assert.True(t, agg2.IsEqual(agg2_reversed), "Aggregation should be order-independent")

	// Test aggregation with 3 keys
	agg3, err := MuSig2AggregateKeys(key1, key2, key3)
	require.NoError(t, err)
	require.NotNil(t, agg3)

	// Aggregated key should be different from individual keys
	assert.False(t, agg2.IsEqual(key1), "Aggregated key should differ from individual key")
	assert.False(t, agg3.IsEqual(agg2), "Different key sets should produce different aggregates")

	t.Logf("MuSig2 aggregation verified: 2-key agg = %x, 3-key agg = %x",
		agg2.SerializeCompressed(), agg3.SerializeCompressed())
}

// TestBoardingWithChange verifies that change outputs are handled correctly
func TestBoardingWithChange(t *testing.T) {
	builder := NewTxBuilder()

	// Create test keys
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test without change address (no change output expected)
	paramsNoChange := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         99000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	}

	txNoChange, err := builder.BuildBoardingTx(paramsNoChange)
	require.NoError(t, err)
	assert.Len(t, txNoChange.TxOut, 1, "Should have 1 output (no change)")

	// Test with change address
	paramsWithChange := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(200000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		ChangeAddress:  "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq", // Example P2WPKH
		FeeRate:        1,
	}

	txWithChange, err := builder.BuildBoardingTx(paramsWithChange)
	require.NoError(t, err)
	assert.Len(t, txWithChange.TxOut, 2, "Should have 2 outputs (main + change)")

	// Verify change output is above dust limit
	var changeOutput *UTXO
	for _, out := range txWithChange.TxOut {
		if out.Value != paramsWithChange.Amount {
			assert.Greater(t, out.Value, int64(DustLimit),
				"Change output should be above dust limit")
			changeOutput = &UTXO{Amount: out.Value}
		}
	}
	require.NotNil(t, changeOutput, "Change output should exist")

	// Verify outputs are sorted deterministically
	if len(txWithChange.TxOut) == 2 {
		// Outputs should be sorted by value
		if txWithChange.TxOut[0].Value > txWithChange.TxOut[1].Value {
			t.Error("Outputs should be sorted by value (ascending)")
		}
	}

	t.Logf("Change handling verified: no-change tx has %d outputs, with-change tx has %d outputs",
		len(txNoChange.TxOut), len(txWithChange.TxOut))
}

// TestCommitmentInputOrdering verifies that commitment tx inputs are sorted deterministically
func TestCommitmentInputOrdering(t *testing.T) {
	builder := NewTxBuilder()

	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Create UTXOs in different orders
	utxo1 := createTestUTXO(100000, 0)
	utxo2 := createTestUTXO(200000, 1)
	utxo3 := createTestUTXO(150000, 0)

	// Build with UTXOs in order 1, 2, 3
	params1 := &CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{utxo1, utxo2, utxo3},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	}

	tx1, err := builder.BuildCommitmentTx(params1)
	require.NoError(t, err)

	// Build with UTXOs in order 3, 1, 2 (different order)
	params2 := &CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{utxo3, utxo1, utxo2},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	}

	tx2, err := builder.BuildCommitmentTx(params2)
	require.NoError(t, err)

	// Both transactions should have identical txids despite input order
	assert.Equal(t, tx1.TxHash().String(), tx2.TxHash().String(),
		"Commitment tx should be deterministic regardless of input order")

	// Verify inputs are sorted
	for i := 0; i < len(tx1.TxIn)-1; i++ {
		hash1 := tx1.TxIn[i].PreviousOutPoint.Hash[:]
		hash2 := tx1.TxIn[i+1].PreviousOutPoint.Hash[:]
		cmp := bytes.Compare(hash1, hash2)
		assert.True(t, cmp <= 0, "Inputs should be sorted by hash")
		if cmp == 0 {
			assert.True(t, tx1.TxIn[i].PreviousOutPoint.Index <= tx1.TxIn[i+1].PreviousOutPoint.Index,
				"Inputs with same hash should be sorted by index")
		}
	}

	t.Logf("Input ordering verified: txid = %s", tx1.TxHash().String())
}

// TestTransactionBasicProperties verifies basic transaction properties
func TestTransactionBasicProperties(t *testing.T) {
	builder := NewTxBuilder()

	// Create test keys
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test Boarding Transaction
	boardingParams := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	}

	boardingTx, err := builder.BuildBoardingTx(boardingParams)
	require.NoError(t, err)

	assert.Equal(t, int32(TxVersion), boardingTx.Version, "Version should be 2")
	assert.Equal(t, uint32(0), boardingTx.LockTime, "Locktime should be 0")
	assert.Equal(t, uint32(SequenceBoardingTx), boardingTx.TxIn[0].Sequence,
		"Sequence should be 0xFFFFFFFD")

	// Test Commitment Transaction
	commitParams := &CommitmentTxParams{
		OperatorUTXOs: []*UTXO{
			createTestUTXO(500000, 0),
		},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	}

	commitTx, err := builder.BuildCommitmentTx(commitParams)
	require.NoError(t, err)

	assert.Equal(t, int32(TxVersion), commitTx.Version, "Version should be 2")
	assert.Equal(t, uint32(0), commitTx.LockTime, "Locktime should be 0")
	assert.Equal(t, uint32(SequenceCommitmentTx), commitTx.TxIn[0].Sequence,
		"Sequence should be 0xFFFFFFFF")
	assert.Len(t, commitTx.TxOut, 2, "Should have 2 outputs (batch + connector)")

	// Verify output ordering (batch first, connector second)
	assert.Equal(t, commitParams.BatchAmount, commitTx.TxOut[0].Value,
		"First output should be batch")
	assert.Equal(t, commitParams.ConnectorAmount, commitTx.TxOut[1].Value,
		"Second output should be connector")

	// Test Forfeit Transaction
	forfeitParams := &ForfeitTxParams{
		VTXO:            createTestUTXO(50000, 0),
		ConnectorAnchor: createTestUTXO(1000, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	}

	forfeitTx, err := builder.BuildForfeitTx(forfeitParams)
	require.NoError(t, err)

	assert.Equal(t, int32(TxVersion), forfeitTx.Version, "Version should be 2")
	assert.Equal(t, uint32(0), forfeitTx.LockTime, "Locktime should be 0")
	assert.Len(t, forfeitTx.TxIn, 2, "Should have 2 inputs (VTXO + connector)")
	assert.Len(t, forfeitTx.TxOut, 1, "Should have 1 output (to operator)")
	assert.Equal(t, uint32(SequenceForfeitTx), forfeitTx.TxIn[0].Sequence,
		"Sequence should be 0xFFFFFFFF")

	t.Log("All basic transaction properties verified")
}

// TestBoardingTxValidation tests input validation for boarding transactions
func TestBoardingTxValidation(t *testing.T) {
	builder := NewTxBuilder()
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test nil funding UTXO
	_, err := builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    nil,
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "funding UTXO is required")

	// Test nil user key
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         90000,
		UserPubKey:     nil,
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user and operator public keys are required")

	// Test nil operator key
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: nil,
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user and operator public keys are required")

	// Test negative amount
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         -1000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount must be positive")

	// Test zero amount
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         0,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount must be positive")

	// Test amount below dust limit
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         DustLimit - 1,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount below dust limit")

	// Test negative funding UTXO amount
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(-10000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "funding UTXO amount must be positive")

	// Test zero funding UTXO amount
	_, err = builder.BuildBoardingTx(&BoardingTxParams{
		FundingUTXO:    createTestUTXO(0, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "funding UTXO amount must be positive")

	t.Log("All boarding tx validation tests passed")
}

// TestCommitmentTxValidation tests input validation for commitment transactions
func TestCommitmentTxValidation(t *testing.T) {
	builder := NewTxBuilder()
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test empty operator UTXOs
	_, err := builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one operator UTXO is required")

	// Test nil operator key
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(500000, 0)},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  nil,
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator public key is required")

	// Test negative batch amount
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(500000, 0)},
		BatchAmount:     -100,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch amount must be positive")

	// Test zero batch amount
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(500000, 0)},
		BatchAmount:     0,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch amount must be positive")

	// Test batch amount below dust limit
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(500000, 0)},
		BatchAmount:     DustLimit - 1,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch amount below dust limit")

	// Test negative operator UTXO amount
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(-100, 0)},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator UTXO amount must be positive")

	// Test negative boarding output amount
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(500000, 0)},
		BoardingOutputs: []*UTXO{createTestUTXO(-100, 1)},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boarding output amount must be positive")

	// Test insufficient funds
	_, err = builder.BuildCommitmentTx(&CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(1000, 0)}, // Very small amount
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient input amount to cover outputs and fees")

	t.Log("All commitment tx validation tests passed")
}

// TestForfeitTxValidation tests input validation for forfeit transactions
func TestForfeitTxValidation(t *testing.T) {
	builder := NewTxBuilder()
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test nil VTXO
	_, err := builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            nil,
		ConnectorAnchor: createTestUTXO(1000, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VTXO is required")

	// Test nil connector anchor
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(50000, 0),
		ConnectorAnchor: nil,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector anchor is required")

	// Test nil operator key
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(50000, 0),
		ConnectorAnchor: createTestUTXO(1000, 1),
		OperatorPubKey:  nil,
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator public key is required")

	// Test negative VTXO amount
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(-1000, 0),
		ConnectorAnchor: createTestUTXO(1000, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VTXO amount must be positive")

	// Test zero VTXO amount
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(0, 0),
		ConnectorAnchor: createTestUTXO(1000, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VTXO amount must be positive")

	// Test negative connector anchor amount
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(50000, 0),
		ConnectorAnchor: createTestUTXO(-100, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector anchor amount must be positive")

	// Test zero connector anchor amount
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(50000, 0),
		ConnectorAnchor: createTestUTXO(0, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector anchor amount must be positive")

	// Test insufficient funds to cover fees
	_, err = builder.BuildForfeitTx(&ForfeitTxParams{
		VTXO:            createTestUTXO(100, 0), // Very small
		ConnectorAnchor: createTestUTXO(100, 1), // Very small
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         10, // High fee rate
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient input amount to cover fees")

	t.Log("All forfeit tx validation tests passed")
}

// TestMuSig2EdgeCases tests edge cases for MuSig2 aggregation
func TestMuSig2EdgeCases(t *testing.T) {
	// Test empty key list
	_, err := MuSig2AggregateKeys()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one public key is required")

	// Test single key
	key1 := createTestPrivKey(t, 0x01).PubKey()
	agg1, err := MuSig2AggregateKeys(key1)
	require.NoError(t, err)
	require.NotNil(t, agg1)

	t.Log("All MuSig2 edge case tests passed")
}

// TestSortingEdgeCases tests edge cases for sorting functions
func TestSortingEdgeCases(t *testing.T) {
	// Test sortScripts with empty slice
	emptyScripts := [][]byte{}
	sortedEmpty := sortScripts(emptyScripts)
	assert.Len(t, sortedEmpty, 0)

	// Test sortScripts with single element
	singleScript := [][]byte{{0x01, 0x02, 0x03}}
	sortedSingle := sortScripts(singleScript)
	assert.Len(t, sortedSingle, 1)
	assert.Equal(t, singleScript[0], sortedSingle[0])

	// Test sortScripts with identical elements
	identicalScripts := [][]byte{{0x01, 0x02}, {0x01, 0x02}, {0x01, 0x02}}
	sortedIdentical := sortScripts(identicalScripts)
	assert.Len(t, sortedIdentical, 3)
	for i := 0; i < len(sortedIdentical)-1; i++ {
		assert.Equal(t, sortedIdentical[i], sortedIdentical[i+1])
	}

	// Test sortTxOutputs with empty transaction
	emptyTx := &wire.MsgTx{
		TxOut: []*wire.TxOut{},
	}
	sortTxOutputs(emptyTx)
	assert.Len(t, emptyTx.TxOut, 0)

	// Test sortTxOutputs with single output
	singleOutTx := &wire.MsgTx{
		TxOut: []*wire.TxOut{
			wire.NewTxOut(1000, []byte{0x01}),
		},
	}
	sortTxOutputs(singleOutTx)
	assert.Len(t, singleOutTx.TxOut, 1)

	// Test sortTxOutputs with equal values (tests script comparison)
	equalValueTx := &wire.MsgTx{
		TxOut: []*wire.TxOut{
			wire.NewTxOut(1000, []byte{0x03, 0x02}),
			wire.NewTxOut(1000, []byte{0x01, 0x04}),
			wire.NewTxOut(1000, []byte{0x02, 0x03}),
		},
	}
	sortTxOutputs(equalValueTx)
	// Verify sorted by script (lexicographically)
	assert.Equal(t, []byte{0x01, 0x04}, equalValueTx.TxOut[0].PkScript)
	assert.Equal(t, []byte{0x02, 0x03}, equalValueTx.TxOut[1].PkScript)
	assert.Equal(t, []byte{0x03, 0x02}, equalValueTx.TxOut[2].PkScript)

	// Test sortTxInputs with empty transaction
	emptyInTx := &wire.MsgTx{
		TxIn: []*wire.TxIn{},
	}
	sortTxInputs(emptyInTx)
	assert.Len(t, emptyInTx.TxIn, 0)

	// Test sortTxInputs with single input
	hash1, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000001")
	singleInTx := &wire.MsgTx{
		TxIn: []*wire.TxIn{
			wire.NewTxIn(wire.NewOutPoint(hash1, 0), nil, nil),
		},
	}
	sortTxInputs(singleInTx)
	assert.Len(t, singleInTx.TxIn, 1)

	// Test sortTxInputs with same hash but different indices
	hash2, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000002")
	sameHashTx := &wire.MsgTx{
		TxIn: []*wire.TxIn{
			wire.NewTxIn(wire.NewOutPoint(hash2, 5), nil, nil),
			wire.NewTxIn(wire.NewOutPoint(hash2, 2), nil, nil),
			wire.NewTxIn(wire.NewOutPoint(hash2, 8), nil, nil),
		},
	}
	sortTxInputs(sameHashTx)
	// Verify sorted by index
	assert.Equal(t, uint32(2), sameHashTx.TxIn[0].PreviousOutPoint.Index)
	assert.Equal(t, uint32(5), sameHashTx.TxIn[1].PreviousOutPoint.Index)
	assert.Equal(t, uint32(8), sameHashTx.TxIn[2].PreviousOutPoint.Index)

	t.Log("All sorting edge case tests passed")
}

// TestBoardingDustChangeRemoval tests that dust change outputs are properly removed
func TestBoardingDustChangeRemoval(t *testing.T) {
	builder := NewTxBuilder()
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Create params where change would be below dust limit after fees
	// With funding 90300 and amount 90000, after fees (~200 sats), change would be ~100 sats (below dust)
	params := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(90300, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		ChangeAddress:  "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
		FeeRate:        1,
	}

	tx, err := builder.BuildBoardingTx(params)
	require.NoError(t, err)

	// Change should be removed because it would be dust after fees
	// Transaction should have only 1 output
	assert.Len(t, tx.TxOut, 1, "Dust change output should be removed")

	// Test case with sufficient change (above dust limit)
	paramsWithChange := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(200000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		ChangeAddress:  "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
		FeeRate:        1,
	}

	txWithChange, err := builder.BuildBoardingTx(paramsWithChange)
	require.NoError(t, err)
	assert.Len(t, txWithChange.TxOut, 2, "Should have 2 outputs when change is above dust")

	t.Log("Dust change removal test passed")
}

// TestTaprootScriptCreation tests Taproot script creation edge cases
func TestTaprootScriptCreation(t *testing.T) {
	userPrivKey := createTestPrivKey(t, 0x01)
	userPubKey := userPrivKey.PubKey()

	// Test with internal key and scripts
	script1, err := BuildCheckSigScript(userPubKey)
	require.NoError(t, err)

	taprootScript, err := CreateTaprootScript(userPubKey, [][]byte{script1})
	require.NoError(t, err)
	require.NotNil(t, taprootScript)

	// Test with nil internal key (uses NUMS point)
	taprootScriptNil, err := CreateTaprootScript(nil, [][]byte{script1})
	require.NoError(t, err)
	require.NotNil(t, taprootScriptNil)

	// Test with no scripts (key-path only)
	taprootKeyOnly, err := CreateTaprootScript(userPubKey, [][]byte{})
	require.NoError(t, err)
	require.NotNil(t, taprootKeyOnly)

	// Test with nil key and no scripts
	taprootNumsOnly, err := CreateTaprootScript(nil, [][]byte{})
	require.NoError(t, err)
	require.NotNil(t, taprootNumsOnly)

	// Test with multiple scripts
	script2, err := BuildCheckSigWithTimelockScript(userPubKey, 144)
	require.NoError(t, err)

	taprootMulti, err := CreateTaprootScript(userPubKey, [][]byte{script1, script2})
	require.NoError(t, err)
	require.NotNil(t, taprootMulti)

	t.Log("All Taproot script creation tests passed")
}

// TestInvalidAddressHandling tests error handling for invalid addresses
func TestInvalidAddressHandling(t *testing.T) {
	builder := NewTxBuilder()
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test with invalid change address
	params := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(200000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		ChangeAddress:  "invalid_address_format",
		FeeRate:        1,
	}

	_, err := builder.BuildBoardingTx(params)
	assert.Error(t, err)

	t.Log("Invalid address handling test passed")
}

// TestCommitmentWithBoardingOutputs tests commitment tx with boarding outputs
func TestCommitmentWithBoardingOutputs(t *testing.T) {
	builder := NewTxBuilder()
	operatorPrivKey := createTestPrivKey(t, 0x02)
	user1PrivKey := createTestPrivKey(t, 0x03)

	// Test with multiple operator UTXOs and boarding outputs
	params := &CommitmentTxParams{
		OperatorUTXOs: []*UTXO{
			createTestUTXO(300000, 0),
			createTestUTXO(200000, 1),
		},
		BoardingOutputs: []*UTXO{
			createTestUTXO(100000, 2),
			createTestUTXO(50000, 3),
		},
		BatchAmount:     400000,
		ConnectorAmount: DustLimit, // Use exact dust limit
		OperatorPubKey:  operatorPrivKey.PubKey(),
		UserPubKeys: []*btcec.PublicKey{
			user1PrivKey.PubKey(),
		},
		BatchExpiry: 800000,
		FeeRate:     1,
	}

	tx, err := builder.BuildCommitmentTx(params)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Verify inputs are sorted
	assert.Len(t, tx.TxIn, 4, "Should have 4 inputs (2 operator + 2 boarding)")

	// Verify outputs
	assert.Len(t, tx.TxOut, 2, "Should have 2 outputs (batch + connector)")
	assert.Equal(t, params.BatchAmount, tx.TxOut[0].Value, "First output should be batch")
	assert.Equal(t, int64(DustLimit), tx.TxOut[1].Value, "Second output should be connector at dust limit")

	t.Log("Commitment with boarding outputs test passed")
}

// TestFeeRateValidation tests fee rate validation and defaults
func TestFeeRateValidation(t *testing.T) {
	builder := NewTxBuilder()
	userPrivKey := createTestPrivKey(t, 0x01)
	operatorPrivKey := createTestPrivKey(t, 0x02)

	// Test boarding tx with zero fee rate (should use MinFeeRate)
	boardingParams := &BoardingTxParams{
		FundingUTXO:    createTestUTXO(100000, 0),
		Amount:         90000,
		UserPubKey:     userPrivKey.PubKey(),
		OperatorPubKey: operatorPrivKey.PubKey(),
		TimeoutBlocks:  144,
		FeeRate:        0, // Zero fee rate
	}
	tx1, err := builder.BuildBoardingTx(boardingParams)
	require.NoError(t, err)
	require.NotNil(t, tx1)

	// Test commitment tx with zero fee rate
	commitParams := &CommitmentTxParams{
		OperatorUTXOs:   []*UTXO{createTestUTXO(500000, 0)},
		BatchAmount:     400000,
		ConnectorAmount: 1000,
		OperatorPubKey:  operatorPrivKey.PubKey(),
		BatchExpiry:     800000,
		FeeRate:         0, // Zero fee rate
	}
	tx2, err := builder.BuildCommitmentTx(commitParams)
	require.NoError(t, err)
	require.NotNil(t, tx2)

	// Test forfeit tx with zero fee rate
	forfeitParams := &ForfeitTxParams{
		VTXO:            createTestUTXO(50000, 0),
		ConnectorAnchor: createTestUTXO(1000, 1),
		OperatorPubKey:  operatorPrivKey.PubKey(),
		FeeRate:         0, // Zero fee rate
	}
	tx3, err := builder.BuildForfeitTx(forfeitParams)
	require.NoError(t, err)
	require.NotNil(t, tx3)

	t.Log("Fee rate validation tests passed")
}
