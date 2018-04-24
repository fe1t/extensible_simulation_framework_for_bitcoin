package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

type addr struct {
	AddrList []string
}

type block struct {
	AddrFrom string
	Block    []byte
}

type getblocks struct {
	AddrFrom string
}

type getdata struct {
	AddrFrom string
	Type     string
	ID       []byte
}

type inventory struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type tx struct {
	AddFrom     string
	Transaction []byte
}

type version struct {
	Version     int
	BlockHeight int
	AddrFrom    string
}

const (
	protocol      = "tcp"
	nodeVersion   = 1
	commandLength = 12
)

var (
	nodeAddress     string
	rewardToAddress string
	knownNodes      = []string{"localhost:3000"}
	blocksInTransit = [][]byte{}
	mempool         = make(map[string]Transaction)
)

func StartServer(nodeID, minerAddress string) {
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	rewardToAddress = minerAddress
	ln, err := net.Listen(protocol, nodeAddress)

	if err != nil {
		log.Panic(err)
	}

	defer ln.Close()

	if Bc == nil {
		Bc = NewBlockchain(nodeID)
	}

	if nodeAddress != knownNodes[0] {
		sendVersion(knownNodes[0], Bc)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleConnection(conn, Bc)
	}
}

func handleConnection(conn net.Conn, bc *Blockchain) {
	request, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Panic(err)
	}
	command := bytesToCommand(request[:commandLength])

	fmt.Printf("Received %s command\n", command)

	switch command {
	case "addr":
		handleAddr(request)
	case "block":
		handleBlock(request, bc)
	case "inv":
		handleInventory(request, bc)
	case "getblocks":
		handleGetBlocks(request, bc)
	case "getdata":
		handleGetData(request, bc)
	case "tx":
		handleTx(request, bc)
	case "version":
		handleVersion(request, bc)
	default:
		fmt.Println("Unknown command!")
	}

	conn.Close()
}

func handleAddr(request []byte) {
	var (
		buff    bytes.Buffer
		payload addr
	)

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	knownNodes = append(knownNodes, payload.AddrList...)
	fmt.Printf("There are %d known nodes now!\n", len(knownNodes))
	requestBlocks()
}

func handleBlock(request []byte, bc *Blockchain) {
	var (
		buff    bytes.Buffer
		payload block
	)

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blockData := payload.Block
	block := Deserialize(blockData)

	fmt.Println("Recevied a new block!")
	bc.AddBlock(block)

	fmt.Printf("Added block %x\n", block.Hash)
	blockHashes := bc.GetBlockHashes()

	// better check logic
	if nodeAddress == knownNodes[0] {
		for _, node := range knownNodes {
			if node != nodeAddress {
				sendInventory(node, "block", blockHashes)
			}
		}
	}
	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		sendGetData(payload.AddrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := UTXOSet{bc}
		UTXOSet.Reindex()
	}
}

func handleInventory(request []byte, bc *Blockchain) {
	var (
		buff    bytes.Buffer
		payload inventory
	)

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Recevied inventory with %d %s\n", len(payload.Items), payload.Type)

	if payload.Type == "block" {
		blocksInTransit = payload.Items

		blockHash := payload.Items[0]
		sendGetData(payload.AddrFrom, "block", blockHash)

		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if bytes.Compare(b, blockHash) != 0 {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]

		if mempool[hex.EncodeToString(txID)].ID == nil {
			sendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

func handleGetBlocks(request []byte, bc *Blockchain) {
	var (
		buff    bytes.Buffer
		payload getblocks
	)

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blockHashes := bc.GetBlockHashes()
	sendInventory(payload.AddrFrom, "block", blockHashes)
}

func handleGetData(request []byte, bc *Blockchain) {
	var (
		buff    bytes.Buffer
		payload getdata
	)

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	if payload.Type == "block" {
		block, err := bc.GetBlock([]byte(payload.ID))
		if err != nil {
			return
		}

		sendBlock(payload.AddrFrom, &block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := mempool[txID]

		sendTx(payload.AddrFrom, &tx)
		// delete(mempool, txID)
	}

}

func handleTx(request []byte, bc *Blockchain) {
	var (
		buff    bytes.Buffer
		payload tx
	)

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	txData := payload.Transaction
	tx := DeserializeTransaction(txData)
	mempool[hex.EncodeToString(tx.ID)] = tx

	if nodeAddress == knownNodes[0] {
		for _, node := range knownNodes {
			if node != nodeAddress && node != payload.AddFrom {
				sendInventory(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		if len(mempool) >= 2 && len(rewardToAddress) > 0 {
		MineTransactions:
			var txs []*Transaction
			var usedTXInput [][]byte

			for id := range mempool {
				tx := mempool[id]
				if hasSameTXInput(usedTXInput, tx.Vin) {
					delete(mempool, id)
				} else {
					verified := bc.VerifyTransaction(&tx)
					if verified {
						txs = append(txs, &tx)
						for i := range tx.Vin {
							usedTXInput = append(usedTXInput, tx.Vin[i].Txid)
						}
					}
				}
			}

			// for id := range mempool {
			// 	tx := mempool[id]
			// 	if bc.VerifyTransaction(&tx) {
			// 		txs = append(txs, &tx)
			// 	}
			// }

			if len(txs) == 0 {
				fmt.Println("All transactions are invalid! Waiting for new ones...")
				return
			}

			cbTx := NewCoinbaseTX(rewardToAddress, "")
			txs = append(txs, cbTx)

			newBlock := bc.MineBlock(txs)
			UTXOSet := UTXOSet{bc}
			UTXOSet.Reindex()

			fmt.Println("New block is mined!")

			for _, tx := range txs {
				txID := hex.EncodeToString(tx.ID)
				delete(mempool, txID)
			}

			for _, node := range knownNodes {
				if node != nodeAddress {
					sendInventory(node, "block", [][]byte{newBlock.Hash})
				}
			}

			if len(mempool) > 0 {
				goto MineTransactions
			}
		}
	}

}

func handleVersion(request []byte, bc *Blockchain) {
	var (
		buff    bytes.Buffer
		payload version
	)

	buff.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buff)
	err := decoder.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	myHeight := bc.GetLastBlockHeight()
	requestHeight := payload.BlockHeight

	if myHeight < requestHeight {
		sendGetBlocks(payload.AddrFrom)
	} else if myHeight > requestHeight {
		sendVersion(payload.AddrFrom, bc)
	}

	if !nodeIsKnown(payload.AddrFrom) {
		knownNodes = append(knownNodes, payload.AddrFrom)
	}
}

func sendBlock(addr string, b *Block) {
	payload := gobEncode(block{nodeAddress, Serialize(b)})
	request := append(commandToBytes("block"), payload...)
	sendData(addr, request)
}

func sendData(address string, request []byte) {
	conn, err := net.Dial(protocol, address)
	if err != nil {
		fmt.Printf("%s is not available\n", address)
		var updatedNodes []string

		for _, node := range knownNodes {
			if node != address {
				updatedNodes = append(updatedNodes, node)
			}
		}

		knownNodes = updatedNodes
		return
	}

	defer conn.Close()

	_, err = io.Copy(conn, bytes.NewReader(request))
	if err != nil {
		log.Panic(err)
	}
}

func sendGetBlocks(address string) {
	encodedGetBlocks := gobEncode(getblocks{nodeAddress})
	request := append(commandToBytes("getblocks"), encodedGetBlocks...)
	sendData(address, request)
}

func sendGetData(address, kind string, id []byte) {
	encodedGetData := gobEncode(getdata{nodeAddress, kind, id})
	request := append(commandToBytes("getdata"), encodedGetData...)
	sendData(address, request)
}

func sendTx(addr string, tnx *Transaction) {
	encodedTx := gobEncode(tx{nodeAddress, SerializeTransaction(*tnx)})
	request := append(commandToBytes("tx"), encodedTx...)
	sendData(addr, request)
}

func sendInventory(address, kind string, blockHashes [][]byte) {
	encodedInventory := gobEncode(inventory{nodeAddress, kind, blockHashes})
	request := append(commandToBytes("inv"), encodedInventory...)
	sendData(address, request)
}

func sendVersion(address string, bc *Blockchain) {
	lastHeight := bc.GetLastBlockHeight()
	encodedLastHeight := gobEncode(version{nodeVersion, lastHeight, nodeAddress})
	request := append(commandToBytes("version"), encodedLastHeight...)
	sendData(address, request)
}

func nodeIsKnown(address string) bool {
	for _, node := range knownNodes {
		if node == address {
			return true
		}
	}
	return false
}

func requestBlocks() {
	for _, node := range knownNodes {
		sendGetBlocks(node)
	}
}

func commandToBytes(commandString string) []byte {
	var commandBytes [12]byte
	copy(commandBytes[:], commandString)
	return commandBytes[:]
}

func bytesToCommand(commandBytes []byte) string {
	counter := 0
	for _, char := range commandBytes {
		if char == 0x0 {
			break
		}
		counter++
	}
	return string(commandBytes[:counter])
}

func gobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	gobEncoder := gob.NewEncoder(&buff)
	err := gobEncoder.Encode(data)

	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func testSerialization(s1, s2 []byte) bool {
	return bytes.Equal(s1, s2)
}

func hasSameTXInput(listByte [][]byte, inputs []TXInput) bool {
	for i := range inputs {
		for _, val := range listByte {
			if bytes.Compare(val, inputs[i].Txid) == 0 {
				return true
			}
		}
	}
	return false
}
