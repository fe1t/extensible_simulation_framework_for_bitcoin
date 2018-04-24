package main

// Bc for Singleton blockchain connection
var Bc *Blockchain

func main() {
	Bc = nil
	cli := CLI{}
	cli.Run()
}
