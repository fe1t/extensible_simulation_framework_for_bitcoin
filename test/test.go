package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"math/big"
	"time"
)

// IntToHex convert int to hex stream
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func gob_encode(data string) []byte {
	var result bytes.Buffer
	gobEncoder := gob.NewEncoder(&result)
	err := gobEncoder.Encode(data)
	fmt.Println(err)
	return result.Bytes()
}

func gob_decode(data []byte) string {
	var result string
	fmt.Println(data)
	fmt.Println(bytes.NewReader(data))
	gobDecoder := gob.NewDecoder(bytes.NewReader(data))
	err := gobDecoder.Decode(&result)
	fmt.Println(err)
	return result
}

func main() {
	var hashInt big.Int
	msg := "Hello"
	byteMsg := []byte(msg)
	hash := sha256.Sum256(byteMsg)
	fmt.Println(byteMsg)
	fmt.Println(hash)

	hashInt.SetBytes(hash[:])
	fmt.Println(hashInt.String())

	target := 1000
	fmt.Println([]byte(string(target)))

	fmt.Println(time.Hour)
	data := "Hello"
	serializedData := gob_encode(data)
	if data == gob_decode(serializedData) {
		fmt.Println("EQUAL")
	} else {
		fmt.Println("NOT EQUAL")
	}
}
