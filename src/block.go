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
}

func (block *Block) HashTransactions() []byte {
	var transactions [][]byte
	for _, tx := range block.Transactions {
		transactions = append(transactions, SerializeTransaction(*tx))
	}
	merkleTree := NewMerkleTree(transactions)
	return merkleTree.RootNode.Data
}

// NewBlock creates simple block
func NewBlock(transactions []*Transaction, prevHash []byte) *Block {
	block := &Block{time.Now().Unix(), transactions, prevHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Nonce = nonce
	block.Hash = hash[:]
	return block
}

// NewGenesisBlock creates simple block without defining prevHash
func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{})
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
