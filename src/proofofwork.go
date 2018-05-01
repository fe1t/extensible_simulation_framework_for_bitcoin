package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/davecgh/go-spew/spew"
)

const (
	maxNonce   = math.MaxInt64
	targetBits = 18
)

// ProofOfWork structure
type ProofOfWork struct {
	block  Block
	target *big.Int
}

// PrepareData to get bytes stream
func (pow ProofOfWork) prepareData(nonce int) []byte {
	// fmt.Println("PrevHash")
	// spew.Dump(pow.block.PrevHash)
	// fmt.Println("HashTx")
	// spew.Dump(pow.block.HashTransactions())
	// fmt.Println("TimeStamp")
	// spew.Dump(pow.block.Timestamp)
	// fmt.Println("targetBits")
	// spew.Dump(targetBits)
	// fmt.Println("nonce")
	// spew.Dump(nonce)
	data := bytes.Join(
		[][]byte{
			pow.block.PrevHash,
			pow.block.HashTransactions(),
			IntToHex(pow.block.Timestamp),
			// []byte("1234"),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)
	return data
}

// Run to get nonce, hash for the block
func (pow ProofOfWork) Run() (int, []byte, bool, BlockUpdated) {
	var (
		hashInt big.Int
		hash    [32]byte
	)
	done := true
	nonce := 0
	logger.Logf(LogInfo, "Mining the new block")

FOR_LOOP:
	for {
		select {
		case updated := <-blockUpdate:
			done = false
			time.Sleep(time.Second * 2)
			filteredTxs := []Transaction{}
			for i := 0; i < len(pow.block.Transactions); i++ {
				deleted := false
				for _, usedTx := range updated.usedTxs {
					// spew.Dump("=============")
					// spew.Dump("current txs:", hex.EncodeToString(pow.block.Transactions[i].ID))
					// spew.Dump("usedTx:", hex.EncodeToString(usedTx))
					// spew.Dump("=============")
					if bytes.Compare(pow.block.Transactions[i].ID, usedTx) == 0 {
						deleted = true
					}
				}
				if !deleted {
					filteredTxs = append(filteredTxs, pow.block.Transactions[i])
				}
			}
			nonce = 0
			cpyTxs := make([]Transaction, len(filteredTxs))
			copy(cpyTxs, filteredTxs)
			cpyPHash := make([]byte, len(updated.lastHash))
			copy(cpyPHash, updated.lastHash)
			blockUpdated := BlockUpdated{cpyTxs, cpyPHash, updated.lastHeight + 1}
			spew.Dump(blockUpdated)
			if len(cpyTxs) == 1 {
				return 0, []byte{}, true, BlockUpdated{lastHeight: -1}
			}
			return 0, []byte{}, done, blockUpdated
		default:
			data := pow.prepareData(nonce)
			hash = sha256.Sum256(data)
			// fmt.Printf("\r")
			// logger.Logfn(LogDebug, "%x\n", hash)
			hashInt.SetBytes(hash[:])
			if hashInt.Cmp(pow.target) == -1 {
				break FOR_LOOP
			}
			nonce++
			if nonce >= maxNonce {
				done = false
				break FOR_LOOP
			}
		}

	}
	logger.Logfn(LogDebug, "%x\n", hash)
	fmt.Printf("\n\n")
	return nonce, hash[:], done, BlockUpdated{}
}

// NewProofOfWork creates new PoW for the current block
func NewProofOfWork(b Block) ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))
	pow := ProofOfWork{b, target}
	return pow
}

// Validate checks if nonce is valid for the hash
func (pow ProofOfWork) Validate() bool {
	var hashInt big.Int
	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])
	return hashInt.Cmp(pow.target) == -1
}
