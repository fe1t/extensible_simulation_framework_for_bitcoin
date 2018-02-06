package main

import (
	"time"
)

// Block struct
type Block struct {
	Timestamp int64
	PrevHash  []byte
	Data      []byte
	Hash      []byte
	Nonce     int
}

// NewBlock creates simple block
func NewBlock(data string, prevHash []byte) *Block {
	block := &Block{time.Now().Unix(), prevHash, []byte(data), []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Nonce = nonce
	block.Hash = hash[:]
	return block
}

// NewGenesisBlock creates simple block without defining prevHash
func NewGenesisBlock() *Block {
	return NewBlock("New Genesis Block", []byte{})
}
