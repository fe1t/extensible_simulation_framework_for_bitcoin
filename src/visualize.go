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

func blocksHanlder(w http.ResponseWriter, r *http.Request) {
	var blocks []*Block

	if Bc == nil {
		Bc = NewBlockchain(NODE_ID)
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
	blocks := make(map[string]*Block)

	if Bc == nil {
		Bc = NewBlockchain(NODE_ID)
	}
	err := Bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.Compare(k, []byte("l")) == 0 {
				continue
			}
			block := Deserialize(v)
			blocks[hex.EncodeToString(k)] = block
		}
		return nil
	})

	res, err := json.MarshalIndent(blocks, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.Write(res)
}

func RunHTTP() {
	http.HandleFunc("/blocks", dbHandler)

	port := fmt.Sprintf(":2%s", NODE_ID)
	fmt.Println("HTTP listening at port:", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		// TODO: implement global logger
		log.Panic(err)
	}
}
