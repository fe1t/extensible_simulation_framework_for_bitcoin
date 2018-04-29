package main

import (
	"github.com/joho/godotenv"
)

// Bc for Singleton blockchain connection
var Bc *Blockchain

func loadConfiguration() error {
	return godotenv.Load("config.env")
}

func main() {
	if err := loadConfiguration(); err != nil {
		logger.Logf(LogFatal, "ERROR: loading configuration file")
		return
	}

	Bc = nil
	cli := CLI{}
	cli.Run()
}
