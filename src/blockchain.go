package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
)

const (
	dbFile              = "blockchain.db"
	blockBucket         = "blocks"
	genesisCoinbaseData = "KU-Coin incoming"
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

func (bc *Blockchain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	var spendableOutputs = make(map[string][]int)
	unspentTXs := bc.FindUnspentTransactions(address)
	acc := 0

Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID)

		for outIdx, out := range tx.Vout {
			if out.CanBeUnlockedWith(address) && acc < amount {
				acc += out.Value
				spendableOutputs[txID] = append(spendableOutputs[txID], outIdx)
				if acc >= amount {
					break Work
				}
			}
		}
	}

	return acc, spendableOutputs
}

func (bc *Blockchain) MineBlock(transactions []*Transaction) {
	var lastHash []byte
	err := bc.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		lastHash = bucket.Get([]byte("l"))
		return nil
	})

	newBlock := NewBlock(transactions, lastHash)

	err = bc.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		err = bucket.Put(newBlock.Hash, Serialize(newBlock))
		err = bucket.Put([]byte("l"), newBlock.Hash)
		return nil
	})

	bc.tip = newBlock.Hash
}

func NewUTXOTransaction(from, to string, amount int, bc *Blockchain) *Transaction {
	var (
		inTxs  []TXInput
		outTxs []TXOutput
	)
	acc, spendableOutputs := bc.FindSpendableOutputs(from, amount)
	if acc < amount {
		log.Panic("ERROR: Not enough funds")
	}

	// Build inputs
	for txID, outs := range spendableOutputs {
		txid, err := hex.DecodeString(txID)
		if err != nil {
			log.Panic(err)
		}

		for _, out := range outs {
			inTx := TXInput{txid, out, from}
			inTxs = append(inTxs, inTx)
		}
	}

	// Build outputs
	outTxs = append(outTxs, TXOutput{amount, to})
	if acc > amount {
		outTxs = append(outTxs, TXOutput{acc - amount, from})
	}
	tx := Transaction{nil, inTxs, outTxs}
	tx.SetID()
	return &tx
}

func dbExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

// NewBlockchain creates new chain
func NewBlockchain(address string) *Blockchain {
	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		tip = b.Get([]byte("l"))

		return nil
	})

	return &Blockchain{tip, db}
}

// CreateBlockchain creates a new blockchain DB
func CreateBlockchain(address string) *Blockchain {
	if dbExists() {
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}

	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
		genesis := NewGenesisBlock(cbtx)

		b, err := tx.CreateBucket([]byte(blockBucket))
		if err != nil {
			log.Panic(err)
		}

		err = b.Put(genesis.Hash, Serialize(genesis))
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("l"), genesis.Hash)
		if err != nil {
			log.Panic(err)
		}
		tip = genesis.Hash

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	bc := Blockchain{tip, db}

	return &bc
}
