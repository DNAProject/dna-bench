package config

import (
	"encoding/hex"
	"fmt"
	goSdk "github.com/DNAProject/DNA-go-sdk"
	"github.com/DNAProject/DNA/common"
	"github.com/DNAProject/DNA/common/log"
	"github.com/DNAProject/DNA/core/types"
	"github.com/ontio/ontology-crypto/keypair"
	"io/ioutil"
	"strings"
	"time"
)

const (
	DEFAULT_GAS_PRICE       = 0
	DEFAULT_GAS_LIMIT       = 20000
	DEFAULT_DEPLOY_GASLIMIT = 200000000
)

func SetGasPrice(sdk *goSdk.DNASdk, consensusAccounts []*goSdk.Account, gasPrice uint64) {
	params := map[string]string{
		"gasPrice": fmt.Sprint(gasPrice),
	}
	tx, err := sdk.Native.GlobalParams.NewSetGlobalParamsTransaction(DEFAULT_GAS_PRICE, DEFAULT_GAS_LIMIT, params)
	if err != nil {
		log.Errorf("SetGasPrice: build tx failed, err: %s", err)
		return
	}
	err = MultiSign(tx, sdk, consensusAccounts)
	if err != nil {
		log.Errorf("SetGasPrice: multi sign failed, err: %s", err)
		return
	}
	hash, err := sdk.SendTransaction(tx)
	if err != nil {
		log.Errorf("SetGasPrice: send tx failed, err: %s", err)
		return
	}
	log.Infof("SetGasPrice: success, tx hash is %s", hash.ToHexString())
}

func WithdrawAsset(sdk *goSdk.DNASdk, consensusAccounts []*goSdk.Account, destAcc *goSdk.Account) {
	pubKeys := make([]keypair.PublicKey, len(consensusAccounts))
	for index, account := range consensusAccounts {
		pubKeys[index] = account.PublicKey
	}
	m := (5*len(pubKeys) + 6) / 7
	multiSignAddr, err := types.AddressFromMultiPubKeys(pubKeys, m)
	if err != nil {
		log.Errorf("WithdrawAsset: build multi sign addr failed, err: %s", err)
		return
	}
	log.Infof("WithdrawAsset: start withdraw gas...")
	balance, err := sdk.Native.Gas.BalanceOf(multiSignAddr)
	if err != nil {
		log.Errorf("WithdrawAsset: get unbound gas num failed, err: %s", err)
		return
	}
	log.Infof("WithdrawAsset: multi sign addr %s unbound gas is %d", multiSignAddr.ToBase58(), balance)
	withdrawGasTx, err := sdk.Native.Gas.NewTransferTransaction(DEFAULT_GAS_PRICE, DEFAULT_GAS_LIMIT, multiSignAddr,
		destAcc.Address, balance)
	if err != nil {
		log.Errorf("WithdrawAsset: build withdraw gas tx failed, err: %s", err)
		return
	}
	err = MultiSign(withdrawGasTx, sdk, consensusAccounts)
	if err != nil {
		log.Errorf("WithdrawAsset: multi sign withdraw gas tx failed, err: %s", err)
		return
	}
	withdrawGasHash, err := sdk.SendTransaction(withdrawGasTx)
	if err != nil {
		log.Errorf("WithdrawAsset: send withdraw gas tx failed, err: %s", err)
		return
	}
	log.Infof("WithdrawAsset: withdraw gas success, tx hash is %s, wait one block to confirm", withdrawGasHash.ToHexString())
	wait, err := sdk.WaitForGenerateBlock(30*time.Second, 1)
	if !wait || err != nil {
		log.Errorf("WithdrawAsset: wait withdraw gas failed, err: %s", err)
		return
	}
	log.Infof("WithdrawAsset: completed withdraw gas")
}

func InitOep4(sdk *goSdk.DNASdk, acc *goSdk.Account, avmPath string) {
	fileContent, err := ioutil.ReadFile(avmPath)
	if err != nil {
		log.Errorf("InitOep4: read source code failed, err: %s", err)
		return
	}
	contractStr := strings.TrimSpace(string(fileContent))
	deployHash, err := sdk.NeoVM.DeployNeoVMSmartContract(DEFAULT_GAS_PRICE, DEFAULT_DEPLOY_GASLIMIT, acc, true,
		contractStr, "MYT", "1.0", "my", "1@1.com", "test")
	if err != nil {
		log.Errorf("InitOep4: deploy failed, err: %s", err)
		return
	}
	log.Infof("InitOep4: deploy success, tx hash is %s", deployHash.ToHexString())
	avmCode, err := hex.DecodeString(contractStr)
	if err != nil {
		log.Errorf("InitOep4: decode avm code failed, err: %s", err)
	}
	contractAddr := common.AddressFromVmCode(avmCode)
	initHash, err := sdk.NeoVM.InvokeNeoVMContract(DEFAULT_GAS_PRICE, DEFAULT_GAS_LIMIT, acc, contractAddr,
		[]interface{}{"init", []interface{}{}})
	if err != nil {
		log.Errorf("InitOep4: init contract failed, err: %s", err)
		return
	}
	log.Infof("InitOep4: init contract %s success, tx hash is %s", contractAddr.ToHexString(), initHash.ToHexString())
}

func MultiSign(tx *types.MutableTransaction, sdk *goSdk.DNASdk, consensusAccounts []*goSdk.Account) error {
	pubKeys := make([]keypair.PublicKey, len(consensusAccounts))
	for index, account := range consensusAccounts {
		pubKeys[index] = account.PublicKey
	}
	m := uint16((5*len(pubKeys) + 6) / 7)
	for index, account := range consensusAccounts {
		err := sdk.MultiSignToTransaction(tx, m, pubKeys, account)
		if err != nil {
			return fmt.Errorf("MultiSign: index %d, account %s failed, err: %s", index, account.Address.ToBase58(), err)
		}
	}
	return nil
}
