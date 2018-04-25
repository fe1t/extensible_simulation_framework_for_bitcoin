package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

// Block struct
type Block struct {
	Timestamp    int64
	Transactions []*Transaction
	PrevHash     []byte
	Hash         []byte
	Nonce        int
	Height       int
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() []byte {
	var transactions []Content

	for _, tx := range b.Transactions {
		transactions = append(transactions, NodeContent{tx.ID})
	}

	t, _ := NewTree(transactions)
	mr := t.MerkleRoot()

	return mr
}

// NewBlock creates simple block
func NewBlock(transactions []*Transaction, prevHash []byte, height int) *Block {
	block := &Block{time.Now().Unix(), transactions, prevHash, []byte{}, 0, height}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Nonce = nonce
	block.Hash = hash[:]
	return block
}

// NewGenesisBlock creates simple block without defining prevHash
func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{}, 0)
}

// Serialize block header to byte array
func Serialize(block *Block) []byte {
	var result bytes.Buffer
	gobEncoder := gob.NewEncoder(&result)
	err := gobEncoder.Encode(block)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// Deserialize byte array to block header
func Deserialize(data []byte) *Block {
	var block Block
	gobDecoder := gob.NewDecoder(bytes.NewReader(data))
	err := gobDecoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}
