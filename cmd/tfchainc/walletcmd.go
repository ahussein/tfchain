package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/threefoldfoundation/tfchain/pkg/types"

	"github.com/rivine/rivine/pkg/cli"
	"github.com/rivine/rivine/pkg/client"
	rivinetypes "github.com/rivine/rivine/types"

	"github.com/spf13/cobra"
)

func createWalletSubCmds(cli *client.CommandLineClient) {
	walletSubCmds := &walletSubCmds{cli: cli}

	// define commands
	var (
		createMinterDefinitionTxCmd = &cobra.Command{
			Use:   "minterdefinitiontransaction <dest>|<rawCondition>",
			Short: "Create a new minter definition transaction",
			Long: `Create a new minter definition transaction using the given mint condition.
The mint condition is used to overwrite the current globally defined mint condition,
and can be given as a raw output condition (or address, which resolves to a singlesignature condition).

The returned (raw) MinterDefinitionTransaction still has to be signed, prior to sending.
	`,
			Run: walletSubCmds.createMinterDefinitionTxCmd,
		}
		createCoinCreationTxCmd = &cobra.Command{
			Use:   "coincreationtransaction <dest>|<rawCondition> <amount> [<dest>|<rawCondition> <amount>]...",
			Short: "Create a new coin creation transaction",
			Long: `Create a new coin creation transaction using the given outputs.
The outputs can be given as a pair of value and a raw output condition (or
address, which resolves to a singlesignature condition).

Amounts have to be given expressed in the OneCoin unit, and without the unit of currency.
Decimals are possible and have to be defined using the decimal point.

The Minimum Miner Fee will be added on top of the total given amount automatically.

The returned (raw) CoinCreationTransaction still has to be signed, prior to sending.
	`,
			Run: walletSubCmds.createCoinCreationTxCmd,
		}
	)

	// add commands as wallet sub commands
	cli.WalletCmd.RootCmdCreate.AddCommand(
		createMinterDefinitionTxCmd,
		createCoinCreationTxCmd,
	)

	// register flags
	createMinterDefinitionTxCmd.Flags().StringVar(
		&walletSubCmds.minterDefinitionTxCfg.Description, "description", "",
		"optionally add a description to describe the reasons of transfer of minting power, added as arbitrary data")
	createCoinCreationTxCmd.Flags().StringVar(
		&walletSubCmds.coinCreationTxCfg.Description, "description", "",
		"optionally add a description to describe the origins of the coin creation, added as arbitrary data")
}

type walletSubCmds struct {
	cli                   *client.CommandLineClient
	minterDefinitionTxCfg struct {
		Description string
	}
	coinCreationTxCfg struct {
		Description string
	}
}

func (walletSubCmds *walletSubCmds) createMinterDefinitionTxCmd(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.UsageFunc()
		cli.Die("Invalid amount of arguments. One argume has to be given: <dest>|<rawCondition>")
	}

	// create a minter definition tx with a random nonce and the minimum required miner fee
	tx := types.MinterDefinitionTransaction{
		Nonce:     types.RandomTransactionNonce(),
		MinerFees: []rivinetypes.Currency{walletSubCmds.cli.Config.MinimumTransactionFee},
	}

	// parse the given mint condition
	var err error
	tx.MintCondition, err = parseConditionString(args[0])
	if err != nil {
		cmd.UsageFunc()(cmd)
		cli.Die(err)
	}

	// if a description is given, use it as arbitrary data
	if n := len(walletSubCmds.minterDefinitionTxCfg.Description); n > 0 {
		tx.ArbitraryData = make([]byte, n)
		copy(tx.ArbitraryData[:], walletSubCmds.minterDefinitionTxCfg.Description[:])
	}

	// encode the transaction as a JSON-encoded string and print it to the STDOUT
	json.NewEncoder(os.Stdout).Encode(tx.Transaction())
}

func (walletSubCmds *walletSubCmds) createCoinCreationTxCmd(cmd *cobra.Command, args []string) {
	currencyConvertor := walletSubCmds.cli.CreateCurrencyConvertor()

	// Check that the remaining args are condition + value pairs
	if len(args)%2 != 0 {
		cmd.UsageFunc()
		cli.Die("Invalid arguments. Arguments must be of the form <dest>|<rawCondition> <amount> [<dest>|<rawCondition> <amount>]...")
	}

	// parse the remainder as output coditions and values
	pairs, err := parsePairedOutputs(args, currencyConvertor.ParseCoinString)
	if err != nil {
		cmd.UsageFunc()(cmd)
		cli.Die(err)
	}

	tx := types.CoinCreationTransaction{
		Nonce:     types.RandomTransactionNonce(),
		MinerFees: []rivinetypes.Currency{walletSubCmds.cli.Config.MinimumTransactionFee},
	}
	if n := len(walletSubCmds.coinCreationTxCfg.Description); n > 0 {
		tx.ArbitraryData = make([]byte, n)
		copy(tx.ArbitraryData[:], walletSubCmds.coinCreationTxCfg.Description[:])
	}
	for _, pair := range pairs {
		tx.CoinOutputs = append(tx.CoinOutputs, rivinetypes.CoinOutput{
			Value:     pair.Value,
			Condition: pair.Condition,
		})
	}
	json.NewEncoder(os.Stdout).Encode(tx.Transaction())
}

type (
	// parseCurrencyString takes the string representation of a currency value
	parseCurrencyString func(string) (rivinetypes.Currency, error)

	outputPair struct {
		Condition rivinetypes.UnlockConditionProxy
		Value     rivinetypes.Currency
	}
)

func parsePairedOutputs(args []string, parseCurrency parseCurrencyString) (pairs []outputPair, err error) {
	argn := len(args)
	if argn < 2 {
		err = errors.New("not enough arguments, at least 2 required")
		return
	}
	if argn%2 != 0 {
		err = errors.New("arguments have to be given in pairs of '<dest>|<rawCondition>'+'<value>'")
		return
	}

	for i := 0; i < argn; i += 2 {
		// parse value first, as it's the one without any possibility of ambiguity
		var pair outputPair
		pair.Value, err = parseCurrency(args[i+1])
		if err != nil {
			err = fmt.Errorf("failed to parse amount/value for output #%d: %v", i/2, err)
			return
		}

		// parse condition second
		pair.Condition, err = parseConditionString(args[i])
		if err != nil {
			err = fmt.Errorf("failed to parse condition for output #%d: %v", i/2, err)
			return
		}

		// append succesfully parsed pair
		pairs = append(pairs, pair)
	}
	return
}

// try to parse the string first as an unlock hash,
// if that fails parse it as a
func parseConditionString(str string) (condition rivinetypes.UnlockConditionProxy, err error) {
	// try to parse it as an unlock hash
	var uh rivinetypes.UnlockHash
	err = uh.LoadString(str)
	if err == nil {
		// parsing as an unlock hash was succesfull, store the pair and continue to the next pair
		condition = rivinetypes.NewCondition(rivinetypes.NewUnlockHashCondition(uh))
		return
	}

	// try to parse it as a JSON-encoded unlock condition
	err = condition.UnmarshalJSON([]byte(str))
	if err != nil {
		return rivinetypes.UnlockConditionProxy{}, fmt.Errorf(
			"condition has to be UnlockHash or JSON-encoded UnlockCondition, output %q is neither", str)
	}
	return
}
