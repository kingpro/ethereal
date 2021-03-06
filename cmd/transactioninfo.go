// Copyright © 2017 Weald Technology Trading
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	etherutils "github.com/orinocopay/go-etherutils"
	"github.com/spf13/cobra"
	"github.com/wealdtech/ethereal/cli"
	"github.com/wealdtech/ethereal/ens"
	"github.com/wealdtech/ethereal/util/txdata"
)

var transactionInfoRaw bool
var transactionInfoJson bool
var transactionInfoSignatures string

// transactionInfoCmd represents the transaction info command
var transactionInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Obtain information about a transaction",
	Long: `Obtain information about a transaction.  For example:

    ethereal transaction info --transaction=0x5FfC014343cd971B7eb70732021E26C35B744cc4

In quiet mode this will return 0 if the transaction exists, otherwise 1.`,
	Run: func(cmd *cobra.Command, args []string) {
		cli.Assert(transactionStr != "", quiet, "--transaction is required")
		var txHash common.Hash
		var pending bool
		var tx *types.Transaction
		if len(transactionStr) > 66 {
			// Assume input is a raw transaction
			data, err := hex.DecodeString(strings.TrimPrefix(transactionStr, "0x"))
			cli.ErrCheck(err, quiet, "Failed to decode data")
			tx = &types.Transaction{}
			stream := rlp.NewStream(bytes.NewReader(data), 0)
			err = tx.DecodeRLP(stream)
			cli.ErrCheck(err, quiet, "Failed to decode raw transaction")
			txHash = tx.Hash()
		} else {
			// Assume input is a transaction ID
			txHash = common.HexToHash(transactionStr)
			ctx, cancel := localContext()
			defer cancel()
			var err error
			tx, pending, err = client.TransactionByHash(ctx, txHash)
			cli.ErrCheck(err, quiet, fmt.Sprintf("Failed to obtain transaction %s", txHash.Hex()))
		}

		if quiet {
			os.Exit(0)
		}

		if transactionInfoRaw {
			buf := new(bytes.Buffer)
			tx.EncodeRLP(buf)
			fmt.Printf("0x%s\n", hex.EncodeToString(buf.Bytes()))
			os.Exit(0)
		}

		if transactionInfoJson {
			json, err := tx.MarshalJSON()
			cli.ErrCheck(err, quiet, fmt.Sprintf("Failed to obtain JSON for transaction %s", txHash.Hex()))
			fmt.Printf("%s\n", string(json))
			os.Exit(0)
		}

		txdata.InitFunctionMap()
		if transactionInfoSignatures != "" {
			for _, signature := range strings.Split(transactionInfoSignatures, ";") {
				txdata.AddFunctionSignature(signature)
			}
		}

		var receipt *types.Receipt
		if pending {
			if tx.To() == nil {
				fmt.Printf("Type:\t\t\tPending contract creation\n")
			} else {
				fmt.Printf("Type:\t\t\tPending transaction\n")
			}
		} else {
			if tx.To() == nil {
				fmt.Printf("Type:\t\t\tMined contract creation\n")
			} else {
				fmt.Printf("Type:\t\t\tMined transaction\n")
			}
			ctx, cancel := localContext()
			defer cancel()
			receipt, err = client.TransactionReceipt(ctx, txHash)
			if receipt != nil {
				if receipt.Status == 0 {
					fmt.Printf("Result:\t\t\tFailed\n")
				} else {
					fmt.Printf("Result:\t\t\tSucceeded\n")
				}
			}
		}

		fromAddress, err := txFrom(tx)
		if err == nil {
			to, err := ens.ReverseResolve(client, &fromAddress)
			if err == nil {
				fmt.Printf("From:\t\t\t%v (%s)\n", to, fromAddress.Hex())
			} else {
				fmt.Printf("From:\t\t\t%v\n", fromAddress.Hex())
			}
		}

		// To
		if tx.To() == nil {
			if receipt != nil {
				contractAddress := receipt.ContractAddress
				to, err := ens.ReverseResolve(client, &contractAddress)
				if err == nil {
					fmt.Printf("Contract address:\t%v (%s)\n", to, contractAddress.Hex())
				} else {
					fmt.Printf("Contract address:\t%v\n", contractAddress.Hex())
				}
			}
		} else {
			to, err := ens.ReverseResolve(client, tx.To())
			if err == nil {
				fmt.Printf("To:\t\t\t%v (%s)\n", to, tx.To().Hex())
			} else {
				fmt.Printf("To:\t\t\t%v\n", tx.To().Hex())
			}
		}

		fmt.Printf("Nonce:\t\t\t%v\n", tx.Nonce())
		fmt.Printf("Gas limit:\t\t%v\n", tx.Gas())
		if receipt != nil {
			fmt.Printf("Gas used:\t\t%v\n", receipt.GasUsed)
		}
		fmt.Printf("Gas price:\t\t%v\n", etherutils.WeiToString(tx.GasPrice(), true))
		fmt.Printf("Value:\t\t\t%v\n", etherutils.WeiToString(tx.Value(), true))

		if len(tx.Data()) > 0 {
			fmt.Printf("Data:\t\t\t%v\n", txdata.DataToString(tx.Data()))
		}

		if verbose && len(receipt.Logs) > 0 {
			fmt.Printf("Logs:\n")
			for i, log := range receipt.Logs {
				fmt.Printf("\t%d:\n", i)
				fmt.Printf("\t\tAddress:\t%v\n", log.Address.Hex())
				if len(log.Topics) > 0 {
					fmt.Printf("\t\tTopics:\n")
					for j, topic := range log.Topics {
						fmt.Printf("\t\t\t%d:\t%v\n", j, topic.Hex())
					}
				}
				if len(log.Data) > 0 {
					fmt.Printf("\t\tData:\n")
					for j := 0; j*32 < len(log.Data); j++ {
						fmt.Printf("\t\t\t%d:\t0x%s\n", j, hex.EncodeToString(log.Data[j*32:(j+1)*32]))
					}
				}
			}
		}
	},
}

func init() {
	transactionCmd.AddCommand(transactionInfoCmd)
	transactionFlags(transactionInfoCmd)
	transactionInfoCmd.Flags().BoolVar(&transactionInfoRaw, "raw", false, "Output the transaction as raw hex")
	transactionInfoCmd.Flags().BoolVar(&transactionInfoJson, "json", false, "Output the transaction as json")
	transactionInfoCmd.Flags().StringVar(&transactionInfoSignatures, "signatures", "", "Semicolon-separated list of custom transaction signatures (e.g. myFunc(address,bytes32);myFunc2(bool)")
}
