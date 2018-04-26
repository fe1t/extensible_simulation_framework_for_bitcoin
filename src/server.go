package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/clockworksoul/smudge"
)

type addr struct {
	AddrList []string
	AddrTo   string
}

type block struct {
	AddrFrom string
	AddrTo   string
	Block    []byte
}

type getblocks struct {
	AddrFrom string
	AddrTo   string
}

type getdata struct {
	AddrFrom string
	AddrTo   string
	Type     string
	ID       []byte
}

type inventory struct {
	AddrFrom string
	AddrTo   string
	Type     string
	Items    [][]byte
}

type tx struct {
	AddFrom     string
	AddrTo      string
	Transaction []byte
}

type version struct {
	AddrFrom    string
	AddrTo      string
	Version     int
	BlockHeight int
}

// MyStatusListener extends from smudge.StatusListener
type MyStatusListener struct {
	smudge.StatusListener
}

// MyBroadcastListener extends from smudge.MyBroadcastListener
type MyBroadcastListener struct {
	smudge.BroadcastListener
}

const (
	protocol      = "tcp"
	nodeVersion   = 1
	commandLength = 12
	baseAddress   = "127.0.0.1"
)

var (
	nodeAddress     string
	rewardToAddress string
	knownNodes      = []string{fmt.Sprintf("%s:3000", baseAddress)}
	blocksInTransit = [][]byte{}
	mempool         = make(map[string]Transaction)
)

func (m MyStatusListener) OnChange(node *smudge.Node, status smudge.NodeStatus) {
	fmt.Printf("Node %s is now status %s\n", node.Address(), status)
}

func (m MyBroadcastListener) OnBroadcast(b *smudge.Broadcast) {
	// broadcastMsg from b.Origin().Address()
	go handleConnection(b.Bytes(), Bc)
}

// ConfigServer configuration for Smudge Library
func ConfigServer() error {
	port, err := strconv.Atoi(NODE_ID)
	if err != nil {
		return err
	}

	// Set configuration options
	smudge.SetListenIP(net.ParseIP(baseAddress))
	smudge.SetListenPort(port)
	smudge.SetHeartbeatMillis(500)
	smudge.SetMaxBroadcastBytes(2000)
	smudge.SetLogThreshold(smudge.LogOff)

	smudge.AddStatusListener(MyStatusListener{})
	smudge.AddBroadcastListener(MyBroadcastListener{})

	// Add a new remote node. Currently, to join an existing cluster you must
	// add at least one of its healthy member nodes.

	if nodeAddress != knownNodes[0] {
		node, err := smudge.CreateNodeByAddress(knownNodes[0])
		if err == nil {
			smudge.AddNode(node)
		} else {
			return err
		}
	}

	// start the server
	go func() {
		smudge.Begin()
	}()

	// Handle SIGINT and SIGTERM.
	// quit := make(chan os.Signal, 2)
	// signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// <-quit
	return nil
}

func StartServer(nodeID, minerAddress string) {
	var err error
	var wg sync.WaitGroup

	nodeAddress = fmt.Sprintf("%s:%s", baseAddress, nodeID)
	rewardToAddress = minerAddress

	wg.Add(1)
	go func() {
		err = ConfigServer()
		wg.Done()
	}()
	wg.Wait()

	if err != nil {
		log.Panic(err)
	}

	if Bc == nil {
		Bc = NewBlockchain(nodeID)
	}

	sendVersion("all", Bc)

	// if nodeAddress != knownNodes[0] {
	// 	sendVersion(knownNodes[0], Bc)
	// }
}

func handleConnection(request []byte, bc *Blockchain) {
	command := bytesToCommand(request[:commandLength])

	fmt.Printf("Received %s command\n", command)

	switch command {
	// case "addr":
	// 	handleAddr(request)
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
	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
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
	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
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

	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
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
	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
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
	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
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
	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
	}

	txData := payload.Transaction
	tx := DeserializeTransaction(txData)
	mempool[hex.EncodeToString(tx.ID)] = tx

	/*
		check if guaranteed broadcastTx to every node in single machine

		if nodeAddress == knownNodes[0] {
			for _, node := range knownNodes {
				if node != nodeAddress && node != payload.AddFrom {
					sendInventory(node, "tx", [][]byte{tx.ID})
				}
			}
		} else {
		}
	*/

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
	if payload.AddrTo != "all" && payload.AddrTo != nodeAddress {
		return
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
	payload := gobEncode(block{nodeAddress, addr, Serialize(b)})
	request := append(commandToBytes("block"), payload...)
	smudge.BroadcastBytes(request)
	// sendData(addr, request)
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

func sendGetBlocks(addr string) {
	encodedGetBlocks := gobEncode(getblocks{nodeAddress, addr})
	request := append(commandToBytes("getblocks"), encodedGetBlocks...)
	smudge.BroadcastBytes(request)
	// sendData(addr, request)
}

func sendGetData(addr, kind string, id []byte) {
	encodedGetData := gobEncode(getdata{nodeAddress, addr, kind, id})
	request := append(commandToBytes("getdata"), encodedGetData...)
	smudge.BroadcastBytes(request)
	// sendData(addr, request)
}

func sendTx(addr string, tnx *Transaction) {
	encodedTx := gobEncode(tx{nodeAddress, addr, SerializeTransaction(*tnx)})
	request := append(commandToBytes("tx"), encodedTx...)
	smudge.BroadcastBytes(request)
	// sendData(addr, request)
}

func sendInventory(addr, kind string, blockHashes [][]byte) {
	encodedInventory := gobEncode(inventory{nodeAddress, addr, kind, blockHashes})
	request := append(commandToBytes("inv"), encodedInventory...)
	smudge.BroadcastBytes(request)
	// sendData(addr, request)
}

func sendVersion(addr string, bc *Blockchain) {
	fmt.Println("Send Version")
	lastHeight := bc.GetLastBlockHeight()
	encodedLastHeight := gobEncode(version{nodeAddress, addr, nodeVersion, lastHeight})
	request := append(commandToBytes("version"), encodedLastHeight...)
	smudge.BroadcastBytes(request)
	// sendData(addr, request)
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
