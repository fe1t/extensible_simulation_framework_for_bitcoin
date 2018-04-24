package main

import (
	"bytes"
	"encoding/gob"
	"log"
)

type TXOutput struct {
	Value         int
	PublicKeyHash []byte
}

type TXOutputs struct {
	Outputs []TXOutput
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

	gobDecoder := gob.NewDecoder(bytes.NewReader(data))
	err := gobDecoder.Decode(&outputs)
	if err != nil {
		log.Panic(err)
	}
	return outputs
}
