package main

import (
	"crypto/sha256"
)

type MerkleTree struct {
	RootNode *MerkleNode
}

type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	var hash [32]byte
	thisNode := MerkleNode{}

	if left == nil && right == nil {
		hash = sha256.Sum256(data)
	} else {
		newData := append(left.Data, right.Data...)
		hash = sha256.Sum256(newData)
	}

	thisNode.Data = hash[:]
	thisNode.Left = left
	thisNode.Right = right

	return &thisNode
}

func NewMerkleTree(data [][]byte) *MerkleTree {
	var nodes []MerkleNode

	if len(data)%2 != 0 {
		data = append(data, data[len(data)-1])
	}

	for _, d := range data {
		node := NewMerkleNode(nil, nil, d)
		nodes = append(nodes, *node)
	}

	for i := 0; i < len(data)/2; i++ {
		var nodeLevel []MerkleNode
		for j := 0; j < len(nodes); j += 2 {
			node := NewMerkleNode(&nodes[j], &nodes[j+1], nil)
			nodeLevel = append(nodeLevel, *node)
		}
		nodes = nodeLevel
	}

	return &MerkleTree{&nodes[0]}
}
