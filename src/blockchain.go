package main

const (
	dbFile      = "blockchain.db"
	blockBucket = "blocks"
)

// Blockchain struct
type Blockchain struct {
	blocks []*Block
}

// type Blockchain struct {
// 	tip [32]byte
// 	db  *bolt.DB
// }

// AddBlock adds new block to the chain
func (bc *Blockchain) AddBlock(data string) {
	prevBlock := bc.blocks[len(bc.blocks)-1]
	newBlock := NewBlock(data, prevBlock.Hash)
	bc.blocks = append(bc.blocks, newBlock)
}

// NewBlockchain creates new chain
func NewBlockchain() *Blockchain {
	return &Blockchain{[]*Block{NewGenesisBlock()}}
}
