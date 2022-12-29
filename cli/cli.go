package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/0xNSHuman/dapp-tools/client"
	"github.com/0xNSHuman/dapp-tools/ui"
	"github.com/0xNSHuman/dapp-tools/wallet"
	"github.com/0xNSHuman/soundminter/minter"
	"github.com/ethereum/go-ethereum/common"
)

/// This is basically a load of garbage code
/// copy-pasted over from the dapp-tools lib
/// debug setup. To be cleaned up.

var (
	walletCLI   *ui.WalletCLI
	rpcEndpoint string
)

// +++++++++++++++++++++++++++++++++++++++
// 		  		    ENTRY
// +++++++++++++++++++++++++++++++++++++++

func main() {
	walletCLI = ui.NewWalletCLI()

	flag.StringVar(&rpcEndpoint, "rpc", "", "rpc endpoint")

	flag.Parse()
	args := flag.Args()
	handleArgs(args)
}

func handleArgs(args []string) {
	if len(args) == 0 {
		fmt.Println("Command not provided")
		return
	}

	switch args[0] {
	case "wallet":
		handleWalletCommand(args[1:])
	case "app":
		handleAppCommand(args[1:])
	default:
		fmt.Println("Command not found")
		return
	}
}

// +++++++++++++++++++++++++++++++++++++++
// 		  	   WALLET COMMANDS
// +++++++++++++++++++++++++++++++++++++++

func handleWalletCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	switch args[0] {
	case "create":
		executeWalletCreateCommand(args[1:])
	case "pubkey":
		executeWalletPubkeyCommand(args[1:])
	case "export":
		executeWalletExportCommand(args[1:])
	case "import":
		executeWalletImportCommand(args[1:])
	case "balance":
		executeWalletBalanceCommand(args[1:])
	default:
		fmt.Println("Invalid command usage")
		return
	}
}

func executeWalletCreateCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	passphrase := findArg(args, "--pwd=")

	if passphrase == nil {
		fmt.Println("--pwd parameter is required")
		return
	}

	walletKeeper, err := wallet.NewWalletKeeper(walletCLI, false)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = walletKeeper.CreateWallet(*passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}

	pubkey, err := walletKeeper.PublicKey(walletKeeper.NumberOfAccounts() - 1)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Wallet created! Public address:", pubkey)
}

func executeWalletPubkeyCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	passphrase := findArg(args, "--pwd=")
	at := findArg(args, "--at=")

	if passphrase == nil {
		fmt.Println("--pwd parameter is required")
		return
	}
	if at == nil {
		fmt.Println("--at parameter is required")
		return
	}
	index, err := strconv.Atoi(*at)
	if err != nil {
		fmt.Println("--at parameter must be a valid integer")
		return
	}

	walletKeeper, err := wallet.NewWalletKeeper(walletCLI, false)
	if err != nil {
		fmt.Println(err)
		return
	}

	pubkey, err := walletKeeper.PublicKey(index)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Public address:", pubkey)
}

func executeWalletExportCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	passphrase := findArg(args, "--pwd=")
	at := findArg(args, "--at=")

	if passphrase == nil {
		fmt.Println("--pwd parameter is required")
		return
	}
	if at == nil {
		fmt.Println("--at parameter is required")
		return
	}
	index, err := strconv.Atoi(*at)
	if err != nil {
		fmt.Println("--at parameter must be a valid integer")
		return
	}

	walletKeeper, err := wallet.NewWalletKeeper(walletCLI, false)
	if err != nil {
		fmt.Println(err)
		return
	}

	privateKey, err := walletKeeper.ExportWallet(index, wallet.ExportModePrivateKey, *passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Private key:", common.Bytes2Hex(privateKey))
}

func executeWalletImportCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	passphrase := findArg(args, "--pwd=")
	privateKey := findArg(args, "--privateKey=")

	if passphrase == nil {
		fmt.Println("--pwd parameter is required")
		return
	}
	if privateKey == nil {
		fmt.Println("--privateKey parameter is required")
		return
	}

	walletKeeper, err := wallet.NewWalletKeeper(walletCLI, false)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = walletKeeper.ImportWallet(wallet.ImportModePrivateKey, common.Hex2Bytes(*privateKey), *passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}

	pubkey, err := walletKeeper.PublicKey(walletKeeper.NumberOfAccounts() - 1)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Wallet imported! Public address:", pubkey)
}

func executeWalletBalanceCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	rpcEndpoint := findArg(args, "--rpc=")
	passphrase := findArg(args, "--pwd=")
	at := findArg(args, "--at=")

	if passphrase == nil {
		fmt.Println("--pwd parameter is required")
		return
	}
	if rpcEndpoint == nil {
		fmt.Println("--rpc parameter is required")
		return
	}
	if at == nil {
		fmt.Println("--at parameter is required")
		return
	}
	index, err := strconv.Atoi(*at)
	if err != nil {
		fmt.Println("--at parameter must be a valid integer")
		return
	}

	walletKeeper, err := wallet.NewWalletKeeper(walletCLI, false)
	if err != nil {
		fmt.Println(err)
		return
	}

	pubkey, err := walletKeeper.PublicKey(index)
	if err != nil {
		fmt.Println(err)
		return
	}

	client, err := client.NewClient(*rpcEndpoint)
	if err != nil {
		fmt.Println(err)
		return
	}

	balance, err := client.EthClient.BalanceAt(context.Background(), common.HexToAddress(pubkey), nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Address balance:", balance)
}

// +++++++++++++++++++++++++++++++++++++++
// 		  	    APP COMMANDS
// +++++++++++++++++++++++++++++++++++++++

func handleAppCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	switch args[0] {
	case "soundminter":
		handlAppSoundminterCommand(args[1:])
	default:
		fmt.Println("Invalid command usage")
		return
	}
}

func handlAppSoundminterCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	switch args[0] {
	case "automint":
		executeAppSoundminterAutomint(args[1:])
	default:
		fmt.Println("Invalid command usage")
		return
	}
}

func executeAppSoundminterAutomint(args []string) {
	if len(args) == 0 {
		fmt.Println("Invalid command usage")
		return
	}

	rpcEndpoint := findArg(args, "--rpc=")
	walletArg := findArg(args, "--wallet=")
	passphrase := findArg(args, "--pwd=")

	if rpcEndpoint == nil {
		fmt.Println("--rpc parameter is required")
		return
	}
	if passphrase == nil {
		fmt.Println("--pwd parameter is required")
		return
	}
	if walletArg == nil {
		fmt.Println("--wallet parameter is required")
		return
	}
	walletIndex, err := strconv.Atoi(*walletArg)
	if err != nil {
		fmt.Println("--wallet parameter must be a valid integer")
		return
	}

	wallet, err := wallet.NewWalletKeeper(walletCLI, true)
	if err != nil {
		fmt.Println(err)
		return
	}

	soundminter, err := minter.NewSoundminter(
		*rpcEndpoint,
		wallet,
		common.HexToAddress("Master contract address"),
		common.HexToAddress("Edition address"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	pubkey, err := wallet.PublicKey(walletIndex)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = wallet.Unlock(walletIndex, *passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println()
	_, err = soundminter.Automint(common.HexToAddress(pubkey))
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Execution completed")
}

// +++++++++++++++++++++++++++++++++++++++
// 		  		   HELPERS
// +++++++++++++++++++++++++++++++++++++++

func findArg(args []string, target string) *string {
	for _, arg := range args {
		if strings.HasPrefix(arg, target) {
			result := strings.TrimPrefix(arg, target)
			return &result
		}
	}

	return nil
}
