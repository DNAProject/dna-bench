package main

import (
	"fmt"
	goSdk "github.com/DNAProject/DNA-go-sdk"
	"github.com/DNAProject/DNA-go-sdk/client"
	"github.com/DNAProject/DNA-go-sdk/utils"
	"github.com/DNAProject/DNA/common"
	"github.com/DNAProject/DNA/common/log"
	"github.com/DNAProject/DNA/core/types"
	"github.com/DNAProject/dna-bench/config"
	"math"
	"os"
	"time"
)

const (
	INIT_PRIVATE_NETWORK = "init"
	TEST_OEP4            = "test-oep4"
	TEST_GAS             = "test-gas"
	BALANCE_OF           = "balanceOf"
)

func main() {
	log.InitLog(log.InfoLog, log.PATH, log.Stdout)
	if len(os.Args) < 2 {
		log.Errorf("not enough args")
	}
	cmd := os.Args[1]
	configPath := "config.json"
	if len(os.Args) >= 3 {
		configPath = os.Args[2]
	}
	cfg, err := config.ParseConfig(configPath)
	if err != nil {
		log.Error(err)
		return
	}
	sdk := goSdk.NewDNASdk()
	wallet, err := sdk.OpenWallet(cfg.Wallet)
	if err != nil {
		log.Errorf("parse wallet err: %s", err)
		return
	}
	account, err := wallet.GetDefaultAccount([]byte(cfg.Password))
	if err != nil {
		log.Errorf("get account err: %s", err)
		return
	}
	if cmd == INIT_PRIVATE_NETWORK {
		consensusAccounts := make([]*goSdk.Account, 0)
		for _, consensusWallet := range cfg.ConsensusPeerPath {
			wallet, err := sdk.OpenWallet(consensusWallet[0])
			if err != nil {
				log.Errorf("parse consensus wallet err: %s", err)
				return
			}
			account, err := wallet.GetDefaultAccount([]byte(consensusWallet[1]))
			if err != nil {
				log.Errorf("get consensus account err: %s", err)
				return
			}
			consensusAccounts = append(consensusAccounts, account)
		}
		rpcClient := client.NewRpcClient()
		rpcClient.SetAddress(cfg.Rpc[0])
		sdk.SetDefaultClient(rpcClient)
		config.SetGasPrice(sdk, consensusAccounts, 500)
		config.WithdrawAsset(sdk, consensusAccounts, account)
		config.InitOep4(sdk, account, cfg.ContractCodePath)
	} else if cmd == TEST_OEP4 {
		testTransfer(cfg, account, config.OEP4)
	} else if cmd == TEST_GAS {
		testTransfer(cfg, account, config.GAS)
	} else if cmd == BALANCE_OF {
		rpcClient := client.NewRpcClient()
		rpcClient.SetAddress(cfg.Rpc[0])
		sdk.SetDefaultClient(rpcClient)
		addr := account.Address
		if len(os.Args) >= 4 {
			argAddr, err := utils.AddressFromBase58(os.Args[3])
			if err != nil {
				log.Errorf("decode arg %s to address failed, err: %s", os.Args[3], err)
			}
			addr = argAddr
		}
		balanceOf(cfg, sdk, addr)
	} else {
		log.Errorf("un support cmd")
		return
	}

}

func testTransfer(cfg *config.Config, account *goSdk.Account, token config.Token) {
	txNum := cfg.TxNum * cfg.TxFactor
	if txNum > math.MaxUint32 {
		txNum = math.MaxUint32
	}
	toAddr, err := utils.AddressFromBase58(cfg.To)
	if err != nil {
		log.Errorf("parse to addr failed, err: %s", err)
		return
	}
	genTxSdk := goSdk.NewDNASdk()
	exitChan := make(chan int)
	txNumPerRoutine := txNum / cfg.RoutineNum
	tpsPerRoutine := int64(cfg.TPS / cfg.RoutineNum)
	startTestTime := time.Now().UnixNano() / 1e6
	for i := uint(0); i < cfg.RoutineNum; i++ {
		var mutTx *types.MutableTransaction
		if token == config.GAS {
			mutTx, err = genTxSdk.Native.Gas.NewTransferTransaction(cfg.GasPrice, cfg.GasLimit, account.Address, toAddr,
				cfg.Amount*100000000)
		} else {
			params := []interface{}{"transfer", []interface{}{account.Address, toAddr, cfg.Amount}}
			contractAddress, err := utils.AddressFromHexString(cfg.Contract)
			if err != nil {
				log.Errorf("parse contract addr failed, err: %s", err)
				return
			}
			mutTx, err = genTxSdk.NeoVM.NewNeoVMInvokeTransaction(cfg.GasPrice, cfg.GasLimit, contractAddress, params)
		}
		if err != nil {
			log.Errorf("construct tx failed, err: %s", err)
			return
		}
		go func(nonce uint32, routineIndex uint, mutTx *types.MutableTransaction) {
			sendTxSdk := goSdk.NewDNASdk()
			rpcClient := client.NewRpcClient()
			rpcClient.SetAddress(cfg.Rpc[int(routineIndex)%len(cfg.Rpc)])
			sendTxSdk.SetDefaultClient(rpcClient)
			startTime := time.Now().UnixNano() / 1e6 // ms
			sentNum := int64(0)
			var fileObj *os.File
			if cfg.SaveTx {
				fileObj, err = os.OpenFile(fmt.Sprintf("invoke_%d.txt", routineIndex),
					os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					fmt.Println("Failed to open the file", err.Error())
					os.Exit(2)
				}
			}
			for j := uint(0); j < txNumPerRoutine; j++ {
				if err := signTx(sendTxSdk, mutTx, nonce, account); err != nil {
					log.Error(err)
					continue
				}
				if cfg.SendTx {
					hash, err := sendTxSdk.SendTransaction(mutTx)
					if err != nil {
						log.Errorf("send tx failed, err: %s", err)
					} else {
						log.Infof("send tx %s", hash.ToHexString())
					}
					sentNum++
					now := time.Now().UnixNano() / 1e6 // ms
					diff := sentNum - (now-startTime)/1e3*tpsPerRoutine
					if now > startTime && diff > 0 {
						sleepTime := time.Duration(diff*1000/tpsPerRoutine) * time.Millisecond
						time.Sleep(sleepTime)
						log.Infof("sleep %d ms", sleepTime.Nanoseconds()/1e6)
					}
				}
				nonce++
				if cfg.SaveTx {
					tx, _ := mutTx.IntoImmutable()
					txHash := tx.Hash()
					sink := common.NewZeroCopySink(nil)
					tx.Serialization(sink)
					txContent := common.ToHexString(sink.Bytes())
					fileObj.WriteString(txHash.ToHexString() + "," + txContent + "\n")
				}
			}
			exitChan <- 1
		}(uint32(txNumPerRoutine*i)+cfg.StartNonce, i, mutTx)
	}
	for i := uint(0); i < cfg.RoutineNum; i++ {
		<-exitChan
	}
	endTestTime := time.Now().UnixNano() / 1e6
	log.Infof("send tps is %f", float64(txNum*1000)/float64(endTestTime-startTestTime))
}

func balanceOf(cfg *config.Config, sdk *goSdk.DNASdk, address common.Address) {
	contractAddr, err := utils.AddressFromHexString(cfg.Contract)
	if err != nil {
		log.Errorf("balanceOf: decode contract addr failed, err: %s", err)
		return
	}
	preResult, err := sdk.NeoVM.PreExecInvokeNeoVMContract(contractAddr, []interface{}{"balanceOf", []interface{}{address}})
	if err != nil {
		log.Errorf("balanceOf: pre-execute failed, err: %s", err)
		return
	}
	balance, err := preResult.Result.ToInteger()
	if err != nil {
		log.Errorf("balanceOf: parse result %v failed, err: %s", preResult, err)
		return
	}
	log.Infof("balanceOf: addr %s is %d", address.ToBase58(), balance)
}

func signTx(sdk *goSdk.DNASdk, tx *types.MutableTransaction, nonce uint32, signer goSdk.Signer) error {
	tx.Nonce = nonce
	tx.Sigs = make([]types.Sig, 0)
	err := sdk.SignToTransaction(tx, signer)
	if err != nil {
		return fmt.Errorf("sign tx failed, err: %s", err)
	}
	return nil
}
