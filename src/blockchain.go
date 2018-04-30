package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/davecgh/go-spew/spew"
)

const (
	dbFile              = "blockchain_%s.db"
	blockBucket         = "blocks"
	genesisCoinbaseData = "KU-Coin Genesis Data"
)

// Blockchain struct
type Blockchain struct {
	sync.RWMutex
	tip []byte
	db  *bolt.DB
}

type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

func (bc *Blockchain) Iterator() *BlockchainIterator {
	bc.RLock()
	defer bc.RUnlock()
	bci := &BlockchainIterator{bc.tip, bc.db}

	return bci
}

func (i *BlockchainIterator) Next() Block {
	var ret Block

	err := i.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		encodedBlock := bucket.Get(i.currentHash)
		block, err := Deserialize(encodedBlock)
		if err != nil {
			return err
		}
		ret = block
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	i.currentHash = ret.PrevHash

	return ret
}

func (bc *Blockchain) MineBlock(transactions []Transaction) Block {

	for _, tx := range transactions {
		// TODO: ignore transaction if it's not valid
		if !bc.VerifyTransaction(&tx) {
			log.Panic("ERROR: Invalid transaction")
		}
	}

	lastHash, lastHeight := bc.GetLatest()

	cpyTxs := make([]Transaction, len(transactions))
	copy(cpyTxs, transactions)

	cpyPHash := make([]byte, len(lastHash))
	copy(cpyPHash, lastHash)

	newBlock := NewBlock(cpyTxs, cpyPHash, lastHeight+1)

	// TODO: verify again
	for _, tx := range newBlock.Transactions {
		// TODO: ignore transaction if it's not valid
		if !bc.VerifyTransaction(&tx) {
			log.Panic("ERROR: Invalid transaction")
		}
	}

	Bc.Lock()
	defer Bc.Unlock()
	fmt.Println("GO")
	err := bc.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))

		// TODO: Serialize race ?
		var err error
		err = bucket.Put(newBlock.Hash, Serialize(newBlock))
		if err != nil {
			return err
		}
		err = bucket.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			return err
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
	bc.RLock()
	defer bc.RUnlock()

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
	bc.RLock()
	defer bc.RUnlock()

	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction is not found")
}

func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	bc.RLock()
	defer bc.RUnlock()

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
	bc.RLock()
	defer bc.RUnlock()

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
	// db.NoSync = true

	if err != nil {
		log.Panic(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		tip = b.Get([]byte("l"))

		return nil
	})

	return &Blockchain{tip: tip, db: db}
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

	bc := Blockchain{tip: tip, db: db}

	return &bc
}

func (bc *Blockchain) AddBlock(block Block) error {
	fmt.Println("Enter AddBlock()")
	pow := NewProofOfWork(block)
	if !pow.Validate() {
		errMsg := fmt.Sprintf("Proof of Work false with nounce: %d\n", pow.block.Nonce)
		return errors.New(errMsg)
	}

	fmt.Println("before Update")
	Bc.Lock()
	defer Bc.Unlock()
	fmt.Println("GOGO")
	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		oldBlock := b.Get(block.Hash)

		if oldBlock != nil {
			errMsg := fmt.Sprintf("Already have block: n%x\n", block.Hash)
			return errors.New(errMsg)
		}

		blockData := Serialize(block)
		err := b.Put(block.Hash, blockData)
		if err != nil {
			return err
		}

		lastHash := b.Get([]byte("l"))
		lastBlockData := b.Get(lastHash)
		lastBlock, err := Deserialize(lastBlockData)
		if err != nil {
			return err
		}

		// TODO: should return err ?
		if lastBlock.Height >= block.Height {
			return nil
		}

		err = b.Put([]byte("l"), block.Hash)
		if err != nil {
			return err
		}
		bc.tip = block.Hash

		return nil
	})
	fmt.Println("Before return")
	spew.Dump(err)
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) GetLastBlockHeight() int {
	bc.RLock()
	defer bc.RUnlock()

	var ret Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		lastHash := b.Get([]byte("l"))
		blockData := b.Get(lastHash)

		// TODO: *Deserialize
		lastBlock, err := Deserialize(blockData)
		if err != nil {
			return err
		}
		ret = lastBlock

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return ret.Height
}

func (bc *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	bc.RLock()
	defer bc.RUnlock()

	var ret Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))

		blockData := b.Get(blockHash)

		if blockData == nil {
			return errors.New("Block not found")
		}

		// TODO: *Deserialize
		block, err := Deserialize(blockData)
		if err != nil {
			return err
		}
		ret = block

		return nil
	})
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func (bc *Blockchain) GetBlockHashes() [][]byte {
	bc.RLock()
	defer bc.RUnlock()

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

func (bc *Blockchain) GetLatest() ([]byte, int) {
	var (
		lastHash   []byte
		lastHeight int
	)

	bc.RLock()
	defer bc.RUnlock()
	err := bc.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockBucket))
		lastHash = bucket.Get([]byte("l"))
		blockData := bucket.Get(lastHash)
		block, err := Deserialize(blockData)
		if err != nil {
			return err
		}
		lastHeight = block.Height
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return lastHash, lastHeight
}

func GetBlockchain() *Blockchain {
	once.Do(func() {
		logger.Logf(LogFatal, "Should not get HERE twice")
		Bc = NewBlockchain(nodeId)
	})
	return Bc
}
