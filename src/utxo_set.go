package main

import (
	"encoding/hex"
	"log"

	"github.com/boltdb/bolt"
)

const utxoBucket = "chainstate"

type UTXOSet struct {
	bc *Blockchain
}

func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	u.bc.RLock()
	defer u.bc.RUnlock()

	var unspentUTXO = make(map[string][]int)
	db := u.bc.db
	acc := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)

			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && acc < amount {
					acc += out.Value
					unspentUTXO[txID] = append(unspentUTXO[txID], outIdx)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return acc, unspentUTXO
}

func (u UTXOSet) FindUTXO(pubKeyHash []byte) []TXOutput {
	// TODO: always freeze here ?
	u.bc.RLock()
	defer u.bc.RUnlock()

	var utxos []TXOutput
	db := u.bc.db

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs := DeserializeOutputs(v)

			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					utxos = append(utxos, out)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return utxos
}

func (u UTXOSet) CountTransactions() int {
	db := u.bc.db
	counter := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			counter++
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return counter
}

func (u UTXOSet) Reindex() {
	bucketName := []byte(utxoBucket)
	u.ClearChainState(bucketName)

	utxos := u.bc.FindUTXO()

	db := u.bc.db
	u.bc.Lock()
	defer u.bc.Unlock()
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		for txID, outs := range utxos {
			txid, err := hex.DecodeString(txID)
			err = b.Put(txid, SerializeOutputs(outs))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

func (u UTXOSet) Update(block Block) [][]byte {
	u.bc.Lock()
	defer u.bc.Unlock()

	var usedTxs = [][]byte{}

	db := u.bc.db

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoBucket))

		for _, tx := range block.Transactions {
			if tx.IsCoinbase() == false {
				for _, inTx := range tx.Vin {
					updatedOuts := TXOutputs{}
					outsBytes := bucket.Get(inTx.Txid)
					outs := DeserializeOutputs(outsBytes)

					for outIdx, outTx := range outs.Outputs {
						if outIdx != inTx.Vout {
							updatedOuts.Outputs = append(updatedOuts.Outputs, outTx)
						}
					}

					if len(updatedOuts.Outputs) == 0 {
						err := bucket.Delete(inTx.Txid)
						if err != nil {
							return err
						}
					} else {
						err := bucket.Put(inTx.Txid, SerializeOutputs(updatedOuts))
						if err != nil {
							return err
						}
					}

				}
				usedTxs = append(usedTxs, tx.ID)
			}

			newOutputs := TXOutputs{}
			for _, outTx := range tx.Vout {
				newOutputs.Outputs = append(newOutputs.Outputs, outTx)
			}

			err := bucket.Put(tx.ID, SerializeOutputs(newOutputs))
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return usedTxs
}

func (u UTXOSet) ClearChainState(bucketName []byte) {
	u.bc.Lock()
	defer u.bc.Unlock()

	db := u.bc.db
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound {
			log.Panic(err)
		}

		_, err = tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}
}
