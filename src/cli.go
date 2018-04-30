package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/clockworksoul/smudge"
	"github.com/olekukonko/tablewriter"
	clii "github.com/urfave/cli"
)

// Terminal lines...
const (
	instructionLine = "> Enter command to execute (CTRL-X to quit, CTRL-B to force back to main menu):"
	goingBack       = "> Going back..."
)

var (
	nodeId   string
	logger   smudge.DefaultLogger
	logLevel string
)

const (
	// LogAll allows all log output of all levels to be emitted.
	LogAll smudge.LogLevel = iota

	// LogTrace restricts log output to trace level and above.
	LogTrace

	// LogDebug restricts log output to debug level and above.
	LogDebug

	// LogInfo restricts log output to info level and above.
	LogInfo

	// LogWarn restricts log output to warn level and above.
	LogWarn

	// LogError restricts log output to error level and above.
	LogError

	// LogFatal restricts log output to fatal level.
	LogFatal

	// LogOff prevents all log output entirely.
	LogOff
)

type CLI struct{}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  createblockchain -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
	fmt.Println("  createwallet - Generates a new key-pair and saves it into the wallet file")
	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
	fmt.Println("  startnode - Run KU-Coin client")

	/*
		fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
		fmt.Println("  printchain - Print all the blocks of the blockchain")
		fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
		fmt.Println("  reindexutxo - Rebuilds the UTXO set")
	*/
}

func (cli *CLI) Run() {
	cli.validateArgs()

	if err := getEnvs(); err != nil {
		log.Panic(err)
	}

	if nodeId == "" {
		fmt.Printf("NODE_ID env. var is not set!")
		os.Exit(1)
	}

	smudge.SetLogThreshold(getLogLevel(logLevel))

	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)

	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)

	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	startNodeInteractive := startNodeCmd.Bool("interactive", false, "Enable interactive mode for easier usage")
	startNodeMiner := startNodeCmd.String("miner", "", "Enable mining mode and send reward to ADDRESS")

	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	sendMine := sendCmd.Bool("mine", false, "Mine immediately on the same node")
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")

	switch os.Args[1] {
	case "createblockchain":
		err := createBlockchainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "startnode":
		err := startNodeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "listaddresses":
		err := listAddressesCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "reindexutxo":
		err := reindexUTXOCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		cli.printUsage()
		os.Exit(1)
	}

	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			os.Exit(1)
		}
		createBlockchain(*createBlockchainAddress, nodeId)
	}

	if createWalletCmd.Parsed() {
		createWallet(nodeId)
	}

	if listAddressesCmd.Parsed() {
		listAddresses(nodeId)
	}

	if startNodeCmd.Parsed() {
		if nodeId == "" {
			startNodeCmd.Usage()
			os.Exit(1)
		}
		cli.startNode(nodeId, *startNodeMiner, *startNodeInteractive)
		if *startNodeInteractive == true {
			app := clii.NewApp()
			app.Action = func(c *clii.Context) error {
				var i impl
				i = impl{fmt: &tableFormatter{}}
				i.readInput()
				return nil
			}
			err := app.Run(os.Args)
			if err != nil {
				panic(err)
			}
		}
	}

	if printChainCmd.Parsed() {
		printChain(nodeId)
	}

	if reindexUTXOCmd.Parsed() {
		reindexUTXO(nodeId)
	}
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}
		getBalance(*getBalanceAddress, nodeId)
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			os.Exit(1)
		}

		send(*sendFrom, *sendTo, *sendAmount, nodeId, *sendMine)
	}

}

func (cli *CLI) startNode(nodeID, minerAddress string, interactive bool) {
	fmt.Printf("Starting node %s\n", nodeID)
	if len(minerAddress) > 0 {
		if ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!")
		}
	}
	go RunHTTP()
	if interactive {
		go StartServer(nodeID, minerAddress)
	} else {
		StartServer(nodeID, minerAddress)
	}
}

type impl struct {
	fmt formatter
}

type commandUsage struct {
	command string
	usage   string
}

type peerNode struct {
	address string
	status  smudge.NodeStatus
}

type wallet struct {
	address string
	balance int
}

type formatter interface {
	DumpUsage([]commandUsage)
	DumpPeers([]peerNode)
	DumpWallet([]wallet)
}

func (i *impl) readInput() {
	i.printCommandUsage()
	fmt.Print("Command: ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		command := scanner.Text()
		switch command {
		case "createwallet":
			i.createwalletInput()
		case "getbalance":
			i.getbalanceInput()
		case "listaddresses":
			i.listaddressesInput()
		case "showwallets":
			i.showwalletsInput()
		case "printchain":
			i.printchainInput()
		case "printpeer":
			i.printpeerInput()
		case "checkupdate":
			i.checkupdateInput()
		case "reindexutxo":
			i.reindexutxoInput()
		case "send":
			i.sendInput()
		case "\x18":
			return
		case "\x1a":
			i.printCommandUsage()
		case "":
			fmt.Println("Use following commands.")
		default:
			fmt.Println("Unknown command")
		}
		i.printCommandUsage()
		fmt.Print("Command: ")
	}
	fmt.Println(scanner.Err())
}

func (i *impl) createwalletInput() {
	defer recoverer()
	createWallet(nodeId)
}

func (i *impl) getbalanceInput() {
	defer recoverer()
	var address string
	fmt.Print("Address: ")
	fmt.Scanf("%s", &address)
	if address == "\x02" {
		return
	}
	getBalance(address, nodeId)
}

func (i *impl) listaddressesInput() {
	defer recoverer()
	fmt.Println(" > Here are your available addresses:")
	listAddresses(nodeId)
}

func (i *impl) showwalletsInput() {
	defer recoverer()
	fmt.Println(" > Here are all your wallets")
	m := getAllBalances(nodeId)
	i.fmt.DumpWallet(m)
}

func (i *impl) printchainInput() {
	defer recoverer()
	fmt.Println(" > Your blockchain looks like this:")
	printChain(nodeId)
}

func (i *impl) printpeerInput() {
	defer recoverer()
	fmt.Println(" > Print known nodes")
	pu := []peerNode{}
	for _, n := range smudge.AllNodes() {
		pu = append(pu, peerNode{n.Address(), n.Status()})
	}
	i.fmt.DumpPeers(pu)
}

func (i *impl) checkupdateInput() {
	defer recoverer()
	fmt.Printf(" > Updating version:")
	sendVersion("all")
	time.Sleep(time.Millisecond * 500)
	fmt.Printf(" ...DONE!\n")
}

func (i *impl) reindexutxoInput() {
	defer recoverer()
	fmt.Println(" > Reindexing the UTXO set:")
	reindexUTXO(nodeId)
}

func (i *impl) sendInput() {
	defer recoverer()
	var (
		fromAddr     string
		toAddr       string
		amountS      string
		amount       int
		confirmation string
	)
	fmt.Print("From address: ")
	fmt.Scanf("%s", &fromAddr)
	if fromAddr == "\x02" {
		return
	}
	fmt.Print("To address: ")
	fmt.Scanf("%s", &toAddr)
	if toAddr == "\x02" {
		return
	}
	fmt.Print("Amount coins (int): ")
	fmt.Scanf("%s", &amountS)
	if amountS == "\x02" {
		return
	}
	amount, err := strconv.Atoi(amountS)
	if err != nil {
		fmt.Printf("Cannot parse %s to integer\n", amountS)
		return
	}
	fmt.Printf(" > You're trying to send %d coins from %s to %s\n", amount, fromAddr, toAddr)
	fmt.Print("Confirm (y/n): ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		confirmation = scanner.Text()
		switch confirmation {
		case "y":
			send(fromAddr, toAddr, amount, nodeId, false)
			return
		case "n":
			return
		case "\x02":
			return
		default:
			fmt.Print("Confirm (y/n): ")
		}
	}
	fmt.Println(scanner.Err())
}

func (i *impl) printCommandUsage() {
	time.Sleep(time.Second * 1)
	cu := []commandUsage{}
	cu = append(cu, commandUsage{"createwallet", "Generates a new key-pair and saves it into the wallet file"})
	// cu = append(cu, commandUsage{"getbalance", "Get balance of 'Address'"})
	// cu = append(cu, commandUsage{"listaddresses", "Lists all addresses from the wallet file"})
	cu = append(cu, commandUsage{"showwallets", "Lists all wallets and balances"})
	cu = append(cu, commandUsage{"printchain", "Print all the blocks of the blockchain"})
	cu = append(cu, commandUsage{"printpeer", "Print all peers connected"})
	cu = append(cu, commandUsage{"reindexutxo", "Rebuild the UTXO set"})
	cu = append(cu, commandUsage{"send", "Send 'Amount' of coins 'From' address to 'To' address"})
	cu = append(cu, commandUsage{"checkupdate", "Check for version update"})

	fmt.Fprint(os.Stdout, "\nCommand Usage Layout:\n\n")
	i.fmt.DumpUsage(cu)
	outputInstructionline()
}

func createBlockchain(address, nodeID string) {
	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}
	if Bc == nil {
		Bc = CreateBlockchain(address, nodeID)
	}
	// defer bc.db.Close()

	utxoSet := UTXOSet{Bc}
	utxoSet.Reindex()

	fmt.Println("Done!")
}

func createWallet(nodeID string) {
	wallets, _ := NewWallets(nodeID)
	address := wallets.CreateWallet()
	wallets.SaveToFile(nodeID)

	fmt.Printf("Your new address: %s\n", address)
}

func getBalance(address, nodeID string) {
	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}
	Bc = GetBlockchain()
	utxoSet := UTXOSet{Bc}
	// defer bc.db.Close()

	balance := 0
	pubKeyHash := Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	utxos := utxoSet.FindUTXO(pubKeyHash)

	for _, out := range utxos {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
}

func getAllBalances(nodeID string) []wallet {
	var ret []wallet
	wallets, err := NewWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	addresses := wallets.GetAddresses()
	for _, address := range addresses {
		if !ValidateAddress(address) {
			continue
		}

		Bc = GetBlockchain()
		utxoSet := UTXOSet{Bc}

		balance := 0
		pubKeyHash := Base58Decode([]byte(address))
		pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
		utxos := utxoSet.FindUTXO(pubKeyHash)

		for _, out := range utxos {
			balance += out.Value
		}
		ret = append(ret, wallet{address, balance})
	}
	return ret
}

func listAddresses(nodeID string) {
	wallets, err := NewWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	addresses := wallets.GetAddresses()

	for _, address := range addresses {
		if ValidateAddress(address) {
			fmt.Println(address)
		}
	}
}

func printChain(nodeID string) {
	Bc = GetBlockchain()
	bci := Bc.Iterator()

	for {
		block := bci.Next()

		fmt.Printf("============ Block %x ============\n", block.Hash)
		headersFormat := blockHeader(block)
		for _, header := range headersFormat {
			fmt.Println(header)
		}
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		fmt.Printf("\n\n")

		if len(block.PrevHash) == 0 {
			break
		}
	}
}

func blockHeader(block Block) []string {
	var formatHeader []string
	headers := []string{"Timestamp", "Height", "Prev block", "Merkle root", "Nonce", "PoW"}
	max := 0
	for _, header := range headers {
		if len(header) > max {
			max = len(header)
		}
	}
	pow := NewProofOfWork(block)
	formatHeader = append(formatHeader, fmt.Sprintf("%*s : %d (%s)", -max, headers[0], block.Timestamp, time.Unix(block.Timestamp, 0)))
	formatHeader = append(formatHeader, fmt.Sprintf("%*s : %d", -max, headers[1], block.Height))
	formatHeader = append(formatHeader, fmt.Sprintf("%*s : %x", -max, headers[2], block.PrevHash))
	formatHeader = append(formatHeader, fmt.Sprintf("%*s : %x", -max, headers[3], block.HashTransactions()))
	formatHeader = append(formatHeader, fmt.Sprintf("%*s : %d", -max, headers[4], block.Nonce))
	formatHeader = append(formatHeader, fmt.Sprintf("%*s : %s", -max, headers[5], strconv.FormatBool(pow.Validate())))
	return formatHeader
}

func reindexUTXO(nodeID string) {
	Bc = GetBlockchain()
	UTXOSet := UTXOSet{Bc}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
}

func send(from, to string, amount int, nodeID string, mineNow bool) {
	if !ValidateAddress(from) {
		log.Panic("ERROR: Sender address is not valid")
	}
	if !ValidateAddress(to) {
		log.Panic("ERROR: Recipient address is not valid")
	}

	Bc = GetBlockchain()
	UTXOSet := UTXOSet{Bc}
	// defer bc.db.Close()

	wallets, err := NewWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)

	tx := NewUTXOTransaction(&wallet, to, amount, &UTXOSet)

	if mineNow {
		cbTx := NewCoinbaseTX(from, "")
		txs := []Transaction{cbTx, tx}

		newBlock := Bc.MineBlock(txs)
		UTXOSet.Update(newBlock)
	} else {
		sendTx("all", tx)
		// sendTx(knownNodes[0], tx)
	}

	fmt.Println("Success!")
}

type tableFormatter struct{}

func (tf tableFormatter) DumpUsage(commandUsages []commandUsage) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(100)
	table.SetHeader([]string{"Command", "Usage"})
	for _, u := range commandUsages {
		row := []string{u.command, u.usage}
		table.Append(row)
	}
	table.Render()
}

func (tf tableFormatter) DumpPeers(peers []peerNode) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(100)
	table.SetHeader([]string{"Address", "Status"})
	for _, n := range peers {
		row := []string{n.address, n.status.String()}
		table.Append(row)
	}
	table.Render()
}

func (tf tableFormatter) DumpWallet(wallets []wallet) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(100)
	table.SetHeader([]string{"Wallet Address", "Balance"})
	for _, w := range wallets {
		row := []string{w.address, strconv.Itoa(w.balance)}
		table.Append(row)
	}
	table.Render()
}

func outputInstructionline() {
	fmt.Fprintf(os.Stdout, "\n%s\n\n", instructionLine)
}

func getEnvs() error {
	var err error
	nodeId = os.Getenv("NODE_ID")
	nodeVersion, err = strconv.Atoi(os.Getenv("KU_COIN_VERSION"))
	if err != nil {
		fmt.Println("Incorrect KU_COIN_VERSION")
		os.Exit(1)
	}
	etherIface = os.Getenv("GET_IP_ON_INTERFACE")
	logLevel = os.Getenv("LOG_LEVEL")
	return err
}

func getLogLevel(level string) smudge.LogLevel {
	switch level {
	case "ALL":
		return LogAll
	case "TRACE":
		return LogTrace
	case "DEBUG":
		return LogDebug
	case "INFO":
		return LogInfo
	case "WARN":
		return LogWarn
	case "ERROR":
		return LogError
	case "FATAL":
		return LogFatal
	case "OFF":
		return LogOff
	default:
		return LogOff
	}
}

func recoverer() {
	if r := recover(); r != nil {
		logger.Logf(LogFatal, "Recovered from: %s", r)
	}
}
