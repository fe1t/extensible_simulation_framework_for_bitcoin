package main

// Bc for Singleton blockchain connection
var Bc *Blockchain

// TODO: only recoverer here
func main() {
	Bc = nil
	cli := CLI{}
	cli.Run()
}
