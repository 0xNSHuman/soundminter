package minter

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xNSHuman/dapp-tools/client"
	"github.com/0xNSHuman/dapp-tools/http"
	"github.com/0xNSHuman/dapp-tools/schedule"
	"github.com/0xNSHuman/dapp-tools/utils"
	"github.com/0xNSHuman/dapp-tools/wallet"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type EventField_MerkleDropMintCreated uint16

const (
	EventField_MerkleDropMintCreated_EventName EventField_MerkleDropMintCreated = iota
	EventField_MerkleDropMintCreated_EditionAddress
	EventField_MerkleDropMintCreated_MintId
	EventField_MerkleDropMintCreated_MerkleRootHash
	EventField_MerkleDropMintCreated_Price
	EventField_MerkleDropMintCreated_StartTime
	EventField_MerkleDropMintCreated_EndTime
	EventField_MerkleDropMintCreated_AffiliateFeeBPS
	EventField_MerkleDropMintCreated_MaxMintable
	EventField_MerkleDropMintCreated_MaxMintablePerAccount
)

type MerkleProofResponse struct {
	UnhashedLeaf string   `json:"unhashedLeaf"`
	Proof        []string `json:"proof"`
}

type Soundminter struct {
	merkleDropMinterAddress common.Address
	editionAddress          common.Address
	abi                     abi.ABI
	client                  *client.Client
	wallet                  *wallet.WalletKeeper
}

func NewSoundminter(
	rpcEndpoint string,
	wallet *wallet.WalletKeeper,
	merkleDropMinterAddress common.Address,
	editionAddress common.Address,
) (*Soundminter, error) {
	client, err := client.NewClient(rpcEndpoint)
	if err != nil {
		return nil, err
	}

	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	abiPath := filepath.Join(
		execPath,
		"abi",
		"SoundMerkleDropMinter.json",
	)
	contractABIFile, err := os.ReadFile(abiPath)
	if err != nil {
		return nil, err
	}

	abi, err := abi.JSON(bytes.NewReader(contractABIFile))
	if err != nil {
		return nil, err
	}

	return &Soundminter{
		merkleDropMinterAddress: merkleDropMinterAddress,
		editionAddress:          editionAddress,
		abi:                     abi,
		client:                  client,
		wallet:                  wallet,
	}, nil
}

func (sm *Soundminter) Automint(minterAddress common.Address) (*string, error) {
	blockNum, err := sm.client.EthClient.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	// Step 1:
	//		Source mint details from the edition contract deployment logs:
	//			- mintId
	//			- mint price
	//			- merkle root hash
	//			- start time (as UNIX timestamp, not block num)

	logs, err := sm.client.ReadLogs(
		big.NewInt(int64(blockNum-216_000)), // Last 30 days lookup
		big.NewInt(int64(blockNum)),
		sm.abi,
		common.HexToAddress(sm.merkleDropMinterAddress.Hex()),
		[][]string{
			// Topic 0 targets
			{
				"MerkleDropMintCreated(address,uint128,bytes32,uint96,uint32,uint32,uint16,uint32,uint32)",
			},
			// Topic 1 targets
			{
				string(sm.editionAddress.Hex()),
			},
		},
	)
	if err != nil {
		return nil, err
	}

	if len(logs) == 0 {
		return nil, errors.New("failed to load the mint info on-chain")
	}

	// Trim leading zeros but leave the '0x' prefix
	mintIdHex := strings.TrimLeft(logs[EventField_MerkleDropMintCreated_MintId].(string), "0x")
	mintIdHex = strings.Join([]string{"0x", mintIdHex}, "")
	mintId, err := hexutil.DecodeBig(mintIdHex)
	if err != nil {
		return nil, err
	}

	merkleRootHashBytes32 := logs[EventField_MerkleDropMintCreated_MerkleRootHash].([32]uint8)
	merkleRootHash := common.Bytes2Hex(merkleRootHashBytes32[:])
	_ = merkleRootHash

	price := logs[EventField_MerkleDropMintCreated_Price].(*big.Int)

	startTimestamp := logs[EventField_MerkleDropMintCreated_StartTime].(uint32)

	// Step 2:
	//		Get the merkle proof from for the currently stored wallet
	// 		using the off-chain service where it's stored

	var proofResponse MerkleProofResponse
	query := strings.Join(
		[]string{
			"https://lanyard.org/api/v1/proof",
			"?root=", merkleRootHash,
			"&unhashedLeaf=", minterAddress.Hex(),
		},
		"",
	)
	err = http.GetObject(query, &proofResponse)
	if err != nil {
		return nil, err
	}

	merkleProofBytes := make([][32]byte, len(proofResponse.Proof))

	for i, hash := range proofResponse.Proof {
		byteRow, _ := hex.DecodeString(strings.TrimPrefix(hash, "0x"))
		bytes32 := new([32]byte)
		copy(bytes32[:], byteRow)
		merkleProofBytes[i] = *bytes32
	}

	fmt.Println("===== MINT DATA LOADED =====")
	fmt.Println("Contract address:        ", sm.merkleDropMinterAddress.Hex())
	fmt.Println("Edition address:         ", sm.editionAddress.Hex())
	fmt.Println("Mint ID:                 ", mintId)
	fmt.Println("Merkle root hash:        ", merkleRootHash)
	fmt.Println("Mint price:              ", price)
	fmt.Println("Mint start UNIX time:    ", startTimestamp)
	fmt.Println("Minter wallet:           ", minterAddress.Hex())
	fmt.Println("Merkle proof:			  ")
	for _, proofByteRow := range merkleProofBytes {
		fmt.Println(common.Bytes2Hex(proofByteRow[:]))
	}
	fmt.Println()

	// Step 3:
	//		Wait for the valid block to mint in

	_, err = sm.waitForBlockAfterTimestamp(uint64(startTimestamp))
	if err != nil {
		return nil, err
	}

	// Step 4:
	//		Fire the mint TX sequence
	//		Make X attempts every Y seconds for every Z wallets

	var attemptsNum = 10

	for {
		txHash, err := sm.makeMintAttempt(
			sm.editionAddress,
			minterAddress,
			mintId,
			uint32(1),
			merkleProofBytes,
			price,
		)
		if err != nil {
			attemptsNum -= 1

			if attemptsNum == 0 {
				return nil, err
			}

			fmt.Println("Mint attempt failed:", err)
			fmt.Println("Making", attemptsNum, "more attempts in 1 second..")
			time.Sleep(time.Second * 1)
		} else {
			return txHash, nil
		}
	}
}

// Wait for a block with the target UNIX timestamp
func (sm *Soundminter) waitForBlockAfterTimestamp(targetTimestamp uint64) (uint64, error) {
	type BlockWaitResult struct {
		blockNum  uint64
		timestamp uint64
		err       error
	}

	var lastBlockNum uint64 = 0
	blockWaitResult := make(chan BlockWaitResult)

	fmt.Println("Waiting for a block coming after timestamp", targetTimestamp, "...")

	job, _ := Scheduler().AddJob(
		schedule.NewPeriodicJobConfig(time.Second*2),
		func() {
			blockNum, err := sm.client.EthClient.BlockNumber(context.Background())
			if err != nil {
				blockWaitResult <- BlockWaitResult{0, 0, err}
			}

			header, err := sm.client.EthClient.HeaderByNumber(context.Background(), big.NewInt(int64(blockNum)))
			if err != nil {
				blockWaitResult <- BlockWaitResult{0, 0, err}
			}

			if header.Time >= targetTimestamp {
				blockWaitResult <- BlockWaitResult{blockNum, header.Time, err}
				close(blockWaitResult)
				return
			}

			if blockNum > lastBlockNum {
				lastBlockNum = blockNum
				fmt.Println(
					"Block", blockNum, "has timestamp", header.Time, "<", targetTimestamp,
				)
			}
		},
	)

	waitResult := <-blockWaitResult
	Scheduler().CancelJob(job.ID)
	if waitResult.err != nil {
		return 0, waitResult.err
	}

	fmt.Println(
		"Block", waitResult.blockNum, "has timestamp", waitResult.timestamp, ">=", targetTimestamp,
	)
	fmt.Println("Found the block!")
	fmt.Println()

	return waitResult.blockNum, nil
}

func (sm *Soundminter) makeMintAttempt(
	editionAddress common.Address,
	minterAddress common.Address,
	mintId *big.Int,
	requestedQuantity uint32,
	merkleProof [][32]byte,
	price *big.Int,
) (*string, error) {
	type MintResult struct {
		txHash *string
		err    error
	}

	mintResultCh := make(chan MintResult)

	Scheduler().AddJob(
		schedule.NewPrimitiveJobConfig(),
		func() {
			txHash, err := sm.mint(
				editionAddress,
				minterAddress,
				mintId,
				requestedQuantity,
				merkleProof,
				price,
			)
			if err != nil {
				mintResultCh <- MintResult{nil, err}
			}
			mintResultCh <- MintResult{txHash, nil}
		},
	)

	mintResult := <-mintResultCh
	return mintResult.txHash, mintResult.err
}

// The contract mint function invoker.
//
// Params:
//
//	editionAddress 		The address of the deployed Sound.xyz edition contract.
//	mintId 				The ID generated during the deployment. See `MintConfigCreated`
//						log event emitted in the deployment transaction.
//	requestedQuantity	How many tokens to mint.
//	merkleProof			Merkle proof generated earlier. The Merkle tree must be sourced
//						from somewhere for that (like an allowlist).
//	price				Mint price.
func (sm *Soundminter) mint(
	editionAddress common.Address,
	minterAddress common.Address,
	mintId *big.Int,
	requestedQuantity uint32,
	merkleProof [][32]byte,
	price *big.Int,
) (*string, error) {
	fmt.Println("Minting...")

	to := common.HexToAddress(sm.merkleDropMinterAddress.Hex())
	value := price

	callData, err := sm.abi.Pack(
		"mint",
		editionAddress,
		mintId,
		requestedQuantity,
		merkleProof,
		common.Address{}, // affiliate address
	)
	if err != nil {
		return nil, err
	}

	tx, err := utils.EncodeTransaction(sm.client, minterAddress, to, value, callData, 2.0)
	if err != nil {
		return nil, err
	}

	chainId, err := sm.client.ChainID()
	if err != nil {
		return nil, err
	}

	signedTx, err := sm.wallet.SignTransaction(chainId, tx, minterAddress, true)
	if err != nil {
		return nil, err
	}

	txHash, err := sm.client.SendTransaction(signedTx)
	if err != nil {
		return nil, err
	}

	fmt.Println("Minted! Transaction hash:", *txHash)
	fmt.Println()

	return txHash, nil
}
