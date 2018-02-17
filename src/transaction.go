package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

const subsidy = 10

type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

func (tx Transaction) SetID() {
	var (
		hash   [32]byte
		buffer bytes.Buffer
	)
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(buffer.Bytes())
	tx.ID = hash[:]
}

type TXInput struct {
	Txid      []byte
	Vout      int
	ScriptSig string
}

func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

func (in *TXInput) CanUnlockOutputWith(unlockingData string) bool {
	return unlockingData == in.ScriptSig
}

type TXOutput struct {
	Value        int
	ScriptPubKey string
}

func (out *TXOutput) CanBeUnlockedWith(unlockingData string) bool {
	return unlockingData == out.ScriptPubKey
}

func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Reward to %s", to)
	}
	txin := TXInput{[]byte{}, -1, data}
	txout := TXOutput{subsidy, to}
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{txout}}
	tx.SetID()
	return &tx
}

func (bc *Blockchain) FindUnspentTransactions(address string) []Transaction {
	var unspentTransactions []Transaction
	spentUTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, outTx := range tx.Vout {
				if spentUTXOs[txID] != nil {
					for _, spentOut := range spentUTXOs[txID] {
						if outIdx == spentOut {
							continue Outputs
						}
					}
				}
				if outTx.CanBeUnlockedWith(address) {
					unspentTransactions = append(unspentTransactions, *tx)
				}
			}
			if !tx.IsCoinbase() {
				for _, inTx := range tx.Vin {
					if inTx.CanUnlockOutputWith(address) {
						inTxID := hex.EncodeToString(inTx.Txid)
						spentUTXOs[inTxID] = append(spentUTXOs[inTxID], inTx.Vout)
					}
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return unspentTransactions
}

func (bc *Blockchain) FindUTXO(address string) []TXOutput {
	var UTXOs []TXOutput
	unspentTransactions := bc.FindUnspentTransactions(address)

	for _, tx := range unspentTransactions {
		for _, utxo := range tx.Vout {
			if utxo.CanBeUnlockedWith(address) { // maybe no need to check
				UTXOs = append(UTXOs, utxo)
			}
		}
	}

	return UTXOs
}
