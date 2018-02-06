package main

import (
	"bytes"
	"encoding/gob"
	"log"
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
	return &block
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
