package main

import (
	"bytes"
	"crypto/sha256"

	"github.com/cbergoon/merkletree"
)

//NodeContent for building Merkle root
type NodeContent struct {
	Data []byte
}

//CalculateHash hashes the values of a NodeContent
func (t NodeContent) CalculateHash() []byte {
	h := sha256.New()
	h.Write([]byte(t.Data))
	return h.Sum(nil)
}

//Equals tests for equality of two Contents
func (t NodeContent) Equals(other merkletree.Content) bool {
	return bytes.Equal(t.Data, other.(NodeContent).Data)
}
