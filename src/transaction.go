package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
)

const subsidy = 10

type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

type TXInput struct {
	Txid      []byte
	Vout      int
	Signature []byte
	PublicKey []byte
}

type TXOutput struct {
	Value         int
	PublicKeyHash []byte
}

type TXOutputs struct {
	Outputs []TXOutput
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

func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

// ToByteStream to prevent encoding/gob non-deterministic
func (tx Transaction) ToByteStream() []byte {
	return []byte([]byte(fmt.Sprintf("%v", tx)))
}

func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}

	// hash = sha256.Sum256(txCopy.Serialize())
	hash = sha256.Sum256(txCopy.ToByteStream())

	return hash[:]
}

func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

func (tx Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))

	for i, input := range tx.Vin {

		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PublicKey))
	}

	for i, output := range tx.Vout {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PublicKeyHash))
	}

	return strings.Join(lines, "\n")
}

// TrimmedTx create a copy transaction with only used header
func (tx *Transaction) TrimmedTx() Transaction {
	var (
		inTxs  []TXInput
		outTxs []TXOutput
	)

	for _, inTx := range tx.Vin {
		inTxs = append(inTxs, TXInput{inTx.Txid, inTx.Vout, nil, nil})
	}

	for _, outTx := range tx.Vout {
		outTxs = append(outTxs, TXOutput{outTx.Value, outTx.PublicKeyHash})
	}

	return Transaction{tx.ID, inTxs, outTxs}
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTxs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}

	for _, inTx := range tx.Vin {
		if prevTxs[hex.EncodeToString(inTx.Txid)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}

	trimmedTx := tx.TrimmedTx()
	for txid, inTx := range trimmedTx.Vin {
		prevTx := prevTxs[hex.EncodeToString(inTx.Txid)]
		trimmedTx.Vin[txid].Signature = nil
		trimmedTx.Vin[txid].PublicKey = prevTx.Vout[inTx.Vout].PublicKeyHash

		dataToSign := fmt.Sprintf("%x\n", trimmedTx)
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(dataToSign))
		if err != nil {
			log.Panic(err)
		}
		signature := append(r.Bytes(), s.Bytes()...)

		tx.Vin[txid].Signature = signature
		trimmedTx.Vin[txid].PublicKey = nil
	}
}

// TODO: recheck verify method with the original
func (tx *Transaction) Verify(prevTxs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, inTx := range tx.Vin {
		if prevTxs[hex.EncodeToString(inTx.Txid)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}

	trimmedTx := tx.TrimmedTx()
	curve := elliptic.P256()

	for txid, inTx := range tx.Vin {
		prevTx := prevTxs[hex.EncodeToString(inTx.Txid)]
		trimmedTx.Vin[txid].Signature = nil
		trimmedTx.Vin[txid].PublicKey = prevTx.Vout[inTx.Vout].PublicKeyHash

		r := big.Int{}
		s := big.Int{}
		sigLen := len(inTx.Signature)
		r.SetBytes(inTx.Signature[:(sigLen / 2)])
		s.SetBytes(inTx.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(inTx.PublicKey)
		x.SetBytes(inTx.PublicKey[:(keyLen / 2)])
		y.SetBytes(inTx.PublicKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{curve, &x, &y}
		dataToVerify := fmt.Sprintf("%x\n", trimmedTx)

		if ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) == false {
			return false
		}
		trimmedTx.Vin[txid].PublicKey = nil
	}
	return true
}

func NewUTXOTransaction(wallet *Wallet, to string, amount int, utxoSet *UTXOSet) Transaction {
	var (
		inTxs  []TXInput
		outTxs []TXOutput
	)
	pubKeyHash := HashPubKey(wallet.PublicKey)
	acc, spendableOutputs := utxoSet.FindSpendableOutputs(pubKeyHash, amount)
	if acc < amount {
		log.Panic("ERROR: Not enough funds")
	}

	// Build inputs
	// TODO: check for case send 0 -> 0
	for txID, outs := range spendableOutputs {
		txid, err := hex.DecodeString(txID)
		if err != nil {
			log.Panic(err)
		}

		for _, out := range outs {
			inTx := TXInput{txid, out, nil, wallet.PublicKey}
			inTxs = append(inTxs, inTx)
		}
	}

	// Build outputs
	from := fmt.Sprintf("%s", wallet.GetAddress())
	outTxs = append(outTxs, *NewTXOutput(amount, to))
	if acc > amount {
		outTxs = append(outTxs, *NewTXOutput(acc-amount, from))
	}
	tx := Transaction{nil, inTxs, outTxs}
	tx.ID = tx.Hash()
	utxoSet.bc.SignTransaction(&tx, wallet.PrivateKey)
	return tx
}

// func NewCoinbaseTX(to, data string) *Transaction {

// }

func NewCoinbaseTX(to, data string) Transaction {
	if data == "" { // prevent no reward
		randData := make([]byte, 20)
		_, err := rand.Read(randData)
		if err != nil {
			log.Panic(err)
		}

		data = fmt.Sprintf("%x", randData)
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data)}
	txout := NewTXOutput(subsidy, to)
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()
	return tx

}

func SerializeTransaction(tx Transaction) []byte {
	var buffer bytes.Buffer

	gobEncoder := gob.NewEncoder(&buffer)
	err := gobEncoder.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return buffer.Bytes()
}

func DeserializeTransaction(data []byte) Transaction {
	var tx Transaction
	var buf bytes.Buffer

	buf.Write(data)
	gobDecoder := gob.NewDecoder(&buf)
	err := gobDecoder.Decode(&tx)
	if err != nil {
		log.Panic(err)
	}

	return tx
}

func (inTx *TXInput) UsesKey(pubKeyHash []byte) bool {
	return bytes.Compare(pubKeyHash, HashPubKey(inTx.PublicKey)) == 0
}

func (outTx *TXOutput) Lock(address []byte) {
	pubKeyHash := Base58Decode(address)
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-checksumLength]
	outTx.PublicKeyHash = pubKeyHash
}

func (outTx *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Compare(outTx.PublicKeyHash, pubKeyHash) == 0
}

func NewTXOutput(value int, address string) *TXOutput {
	txo := &TXOutput{value, nil}
	txo.Lock([]byte(address))

	return txo
}

func SerializeOutputs(data TXOutputs) []byte {
	var buff bytes.Buffer

	gobEncoder := gob.NewEncoder(&buff)
	err := gobEncoder.Encode(data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func DeserializeOutputs(data []byte) TXOutputs {
	var outputs TXOutputs
	var buf bytes.Buffer

	buf.Write(data)
	gobDecoder := gob.NewDecoder(&buf)
	err := gobDecoder.Decode(&outputs)
	if err != nil {
		log.Panic(err)
	}
	return outputs
}
