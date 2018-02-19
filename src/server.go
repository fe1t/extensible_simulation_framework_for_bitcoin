package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"net"
)

type version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

const (
	protocol      = "tcp"
	nodeVersion   = 1
	commandLength = "12"
)

var (
	nodeAddress     string
	rewardToAddress string
	knownNodes      = []string{"localhost:3000"}
)

func StartServer(nodeID, minerAddress string) {
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	rewardToAddress = minerAddress
	ln, err := net.Listen(protocol, nodeAddress)
	defer ln.Close()

	bc := NewBlockchain(nodeID)

	if nodeAddress != knownNodes[0] {
		sendVersion(knownNodes[0], bc)
	}

	for {
		conn, err := ln.Accept()
		go handleConnection(conn, bc)
	}
}

func sendVersion(addr string, bc *Blockchain) {
	bestHeight := bc.GetLastBlockHeight()
	payload := gobEncode(version{nodeVersion, bestHeight, nodeAddress})

	request := append(commandToBytes("version"), payload...)

	sendData(addr, request)
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
		handleInv(request, bc)
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

func gobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	gobEncoder := gob.NewEncoder(&buff)
	err := gobEncoder.Encode(data)

	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}
