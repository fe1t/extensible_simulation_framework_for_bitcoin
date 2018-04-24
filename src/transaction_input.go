package main

import "bytes"

type TXInput struct {
	Txid      []byte
	Vout      int
	Signature []byte
	PublicKey []byte
}

func (inTx *TXInput) UsesKey(pubKeyHash []byte) bool {
	return bytes.Compare(pubKeyHash, HashPubKey(inTx.PublicKey)) == 0
}
