package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"

	"golang.org/x/crypto/ripemd160"
)

const (
	versionAddress = byte(0x00)
	checksumLength = 4
)

type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

func NewWallet() *Wallet {
	privateKey, publicKey := NewKeyPair()
	return &Wallet{privateKey, publicKey}
}

func (w Wallet) GetAddress() []byte {
	versionPublicKey := append([]byte{versionAddress}, HashPubKey(w.PublicKey)...)
	versionPublicKeyChecksum := append(versionPublicKey, GetChecksum(versionPublicKey)...)
	return Base58Encode(versionPublicKeyChecksum)
}

// TODO: check this
func ValidateAddress(address string) bool {
	pubKeyHash := Base58Decode([]byte(address))
	actualChecksum := pubKeyHash[len(pubKeyHash)-checksumLength:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-checksumLength]
	targetChecksum := GetChecksum(append([]byte{version}, pubKeyHash...))

	return bytes.Compare(actualChecksum, targetChecksum) == 0
}

func HashPubKey(pubKey []byte) []byte {
	pubFirstHash := sha256.Sum256(pubKey)
	ripmd160Hasher := ripemd160.New()
	_, err := ripmd160Hasher.Write(pubFirstHash[:])
	if err != nil {
		log.Panic(err)
	}
	pubSecondHash := ripmd160Hasher.Sum(nil)
	return pubSecondHash
}

func GetChecksum(pubKeyHash []byte) []byte {
	firstSHA := sha256.Sum256(pubKeyHash)
	secondSHA := sha256.Sum256(firstSHA[:])
	return secondSHA[:checksumLength]
}

func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Panic(err)
	}
	publicKey := append(privateKey.PublicKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...)
	return *privateKey, publicKey
}
