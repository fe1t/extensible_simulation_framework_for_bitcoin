package main

import (
	"log"

	"github.com/boltdb/bolt"
)

const (
	dbFile      = "blockchain.db"
	blockBucket = "blocks"
)

// Blockchain struct
type Blockchain struct {
	tip []byte
	db  *bolt.DB
}

type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tip, bc.db}

	return bci
}

func (i *BlockchainIterator) Next() *Block {
	var block *Block

	err := i.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		encodedBlock := bucket.Get(i.currentHash)
		block = Deserialize(encodedBlock)
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	i.currentHash = block.PrevHash

	return block
}

// AddBlock adds new block to the chain
func (bc *Blockchain) AddBlock(data string) {
	var lastHash []byte
	err := bc.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		lastHash = bucket.Get([]byte("l"))
		return nil
	})

	newBlock := NewBlock(data, lastHash)

	err = bc.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		err = bucket.Put(newBlock.Hash, Serialize(newBlock))
		err = bucket.Put([]byte("l"), newBlock.Hash)
		return nil
	})

	bc.tip = newBlock.Hash
}

// NewBlockchain creates new chain
func NewBlockchain() *Blockchain {
	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		if bucket == nil {
			newBucket, err := tx.CreateBucket([]byte(blockBucket))
			if err != nil {
				log.Panic(err)
			}
			block := NewGenesisBlock()
			err = newBucket.Put(block.Hash, Serialize(block))
			err = newBucket.Put([]byte("l"), block.Hash)
		} else {
			tip = bucket.Get([]byte("l"))
		}
		return nil
	})
	return &Blockchain{tip, db}
}
