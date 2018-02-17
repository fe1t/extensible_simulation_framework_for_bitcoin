package main

import "bytes"

type TXOutput struct {
	Value         int
	PublicKeyHash []byte
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
