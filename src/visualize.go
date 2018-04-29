package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
)

const blocksBucket = "blocks"

var blocks = make(map[string][]string)

// var blocks = make(map[string]Block)

type TreeHierarchy struct {
	Name     string           `json:"name"`
	Children []*TreeHierarchy `json:"children"`
}

func (parent *TreeHierarchy) addChild(Name string) *TreeHierarchy {
	child := &TreeHierarchy{Name: Name}
	parent.Children = append(parent.Children, child)
	return child
}

func createTreeHierarchy(parentBlock *TreeHierarchy) *TreeHierarchy {
	if _, ok := blocks[parentBlock.Name]; !ok {
		return &TreeHierarchy{}
	}
	for _, c := range blocks[parentBlock.Name] {
		child := parentBlock.addChild(c)
		child = createTreeHierarchy(child)
	}
	return parentBlock
}

func appendIfMissing(slice []string, s string) []string {
	for _, el := range slice {
		if el == s {
			return slice
		}
	}
	slice = append(slice, s)
	return slice
}

func blocksHanlder(w http.ResponseWriter, r *http.Request) {
	var blocks []*Block

	if Bc == nil {
		Bc = NewBlockchain(nodeId)
	}

	bci := Bc.Iterator()

	for {
		block := bci.Next()
		blocks = append(blocks, block)
		if len(block.PrevHash) == 0 {
			break
		}
	}

	res, err := json.MarshalIndent(blocks, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.Write(res)
}

func dbHandler(w http.ResponseWriter, r *http.Request) {
	var ret *TreeHierarchy

	w.Header().Set("Access-Control-Allow-Origin", "*")
	if Bc == nil {
		Bc = NewBlockchain(nodeId)
	}
	err := Bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		c := b.Cursor()

		// start := b.Get([]byte("l"))
		// blocks["l"] = append(blocks["l"], hex.EncodeToString(start))

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.Compare(k, []byte("l")) == 0 {
				// blocks["l"] = &Block{Hash: v}
				continue
			}
			block, err := Deserialize(v)
			if err != nil {
				log.Panic("ERROR:", err)
			}
			// blocks[hex.EncodeToString(k)] = *block
			pointTo := hex.EncodeToString(k)
			if block.PrevHash == nil {
				// blocks["first"] = append(blocks["first"], pointTo)
				blocks["first"] = appendIfMissing(blocks["first"], pointTo)
				continue
			}
			blocks[hex.EncodeToString(block.PrevHash)] = appendIfMissing(blocks[hex.EncodeToString(block.PrevHash)], pointTo)
		}
		return nil
	})

	parentBlock := &TreeHierarchy{Name: blocks["first"][0]}
	if len(blocks) == 1 {
		ret = parentBlock
	} else {
		ret = createTreeHierarchy(parentBlock)
	}

	res, err := json.MarshalIndent(ret, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.Write(res)
}

func RunHTTP() {
	// http.HandleFunc("/blocks", blocksHanlder)
	http.HandleFunc("/blocks", dbHandler)

	port := fmt.Sprintf(":2%s", nodeId)
	fmt.Println("HTTP listening at port:", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		// TODO: implement global logger
		log.Panic(err)
	}
}
