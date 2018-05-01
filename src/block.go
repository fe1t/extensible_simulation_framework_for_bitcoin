package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
)

var blockUpdate = make(chan UpdateInfo)
var paused sync.RWMutex

// Block struct
type Block struct {
	Timestamp    int64
	Transactions []Transaction
	PrevHash     []byte
	Hash         []byte
	Nonce        int
	Height       int
}

type UpdateInfo struct {
	usedTxs    [][]byte
	lastHash   []byte
	lastHeight int
}

type BlockUpdated struct {
	txs        []Transaction
	lastHash   []byte
	lastHeight int
}

func (b *Block) MarshalJSON() ([]byte, error) {
	type Alias Block
	return json.Marshal(&struct {
		Hash     string
		PrevHash string
		*Alias
	}{
		Hash:     hex.EncodeToString(b.Hash),
		PrevHash: hex.EncodeToString(b.PrevHash),
		Alias:    (*Alias)(b),
	})
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var err error
	type Alias Block
	aux := &struct {
		Hash     string
		PrevHash string
		*Alias
	}{
		Alias: (*Alias)(b),
	}
	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	b.PrevHash, err = hex.DecodeString(aux.PrevHash)
	if err != nil {
		return err
	}
	b.Hash, err = hex.DecodeString(aux.Hash)
	if err != nil {
		return err
	}
	return nil
}

// HashTransactions returns a hash of the transactions in the block
func (b Block) HashTransactions() []byte {
	var transactions []Content

	for _, tx := range b.Transactions {
		transactions = append(transactions, NodeContent{tx.ID})
	}

	t, _ := NewTree(transactions)
	mr := t.MerkleRoot()

	return mr
}

// NewBlock creates simple block
// TODO: Undo if not work NewPoW -> PoW -> HashTx return pointer
func NewBlock(transactions []Transaction, prevHash []byte, height int) Block {
	var (
		block        Block
		nonce        int
		hash         []byte
		done         bool
		blockUpdated BlockUpdated
	)

	blockUpdated = BlockUpdated{transactions, prevHash, height}

	for {
		block = Block{time.Now().Unix(), blockUpdated.txs, blockUpdated.lastHash, []byte{}, 0, blockUpdated.lastHeight}
		// fmt.Println("show blocks")
		// spew.Dump(block)
		pow := NewProofOfWork(block)
		nonce, hash, done, blockUpdated = pow.Run()
		spew.Dump(done)
		if done {
			break
		}
	}
	block.Nonce = nonce
	block.Hash = hash[:]
	// spew.Dump(block)
	return block
}

// NewGenesisBlock creates simple block without defining prevHash
func NewGenesisBlock(coinbase Transaction) Block {
	return NewBlock([]Transaction{coinbase}, []byte{}, 0)
}

// Serialize block header to byte array
func Serialize(block Block) []byte {
	var result bytes.Buffer

	gobEncoder := gob.NewEncoder(&result)
	err := gobEncoder.Encode(block)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// Deserialize byte array to block header
func Deserialize(data []byte) (Block, error) {
	var block Block
	var buf bytes.Buffer

	// spew.Dump(data)
	buf.Write(data)
	gobDecoder := gob.NewDecoder(&buf)
	err := gobDecoder.Decode(&block)
	if err != nil {
		return Block{}, err
	}

	return block, nil
}
