package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
)

const (
	dbFile              = "blockchain_%s.db"
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

func (bc *Blockchain) MineBlock(transactions []*Transaction) *Block {
	var (
		lastHash   []byte
		lastHeight int
	)

	for _, tx := range transactions {
		// TODO: ignore transaction if it's not valid
		if !bc.VerifyTransaction(tx) {
			log.Panic("ERROR: Invalid transaction")
		}
	}

	err := bc.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		lastHash = bucket.Get([]byte("l"))
		blockData := bucket.Get(lastHash)
		block := Deserialize(blockData)
		lastHeight = block.Height
		return nil
	})

	newBlock := NewBlock(transactions, lastHash, lastHeight+1)

	err = bc.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		err = bucket.Put(newBlock.Hash, Serialize(newBlock))
		if err != nil {
			log.Panic(err)
		}
		err = bucket.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			log.Panic(err)
		}

		bc.tip = newBlock.Hash
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return newBlock
}

func (bc *Blockchain) FindUTXO() map[string]TXOutputs {
	utxos := make(map[string]TXOutputs)
	spentUTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, outTx := range tx.Vout {
				if spentUTXOs[txID] != nil {
					for _, spentOutIdx := range spentUTXOs[txID] {
						if outIdx == spentOutIdx {
							continue Outputs
						}
					}
				}
				outs := utxos[txID]
				outs.Outputs = append(outs.Outputs, outTx)
				utxos[txID] = outs
			}
			if !tx.IsCoinbase() {
				for _, inTx := range tx.Vin {
					inTxID := hex.EncodeToString(inTx.Txid)
					spentUTXOs[inTxID] = append(spentUTXOs[inTxID], inTx.Vout)
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return utxos
}

func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction is not found")
}

func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, inTx := range tx.Vin {
		prevTX, err := bc.FindTransaction(inTx.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

func (bc *Blockchain) VerifyTransaction(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	prevTXs := make(map[string]Transaction)

	for _, inTx := range tx.Vin {
		prevTX, err := bc.FindTransaction(inTx.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}

func dbExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}

	return true
}

func NewBlockchain(nodeID string) *Blockchain {
	dbFile := fmt.Sprintf(dbFile, nodeID)
	if dbExists(dbFile) == false {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}

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
func CreateBlockchain(address, nodeID string) *Blockchain {
	dbFile := fmt.Sprintf(dbFile, nodeID)
	if dbExists(dbFile) {
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

func (bc *Blockchain) AddBlock(block *Block) {
	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		oldBlock := b.Get(block.Hash)

		if oldBlock != nil {
			return nil
		}

		blockData := Serialize(block)
		err := b.Put(block.Hash, blockData)
		if err != nil {
			log.Panic(err)
		}

		lastHash := b.Get([]byte("l"))
		lastBlockData := b.Get(lastHash)
		lastBlock := Deserialize(lastBlockData)

		if block.Height > lastBlock.Height {
			err = b.Put([]byte("l"), block.Hash)
			if err != nil {
				log.Panic(err)
			}
			bc.tip = block.Hash
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

func (bc *Blockchain) GetLastBlockHeight() int {
	var lastBlock Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		lastHash := b.Get([]byte("l"))
		blockData := b.Get(lastHash)
		lastBlock = *Deserialize(blockData)

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return lastBlock.Height
}

func (bc *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))

		blockData := b.Get(blockHash)

		if blockData == nil {
			return errors.New("Block not found.")
		}

		block = *Deserialize(blockData)

		return nil
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

func (bc *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	bci := bc.Iterator()

	for {
		block := bci.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return blocks
}
