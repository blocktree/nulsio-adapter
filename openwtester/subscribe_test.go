/*
 * Copyright 2018 The openwallet Authors
 * This file is part of the openwallet library.
 *
 * The openwallet library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The openwallet library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 */

package openwtester

import (
	"github.com/astaxie/beego/config"
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openw"
	"github.com/blocktree/openwallet/openwallet"
	"path/filepath"
	"testing"
	"time"
)

//var tm = testInitWalletManager()


////////////////////////// 测试单个扫描器 //////////////////////////

type subscriberSingle struct {
	manager *openw.WalletManager
}

//BlockScanNotify 新区块扫描完成通知
func (sub *subscriberSingle) BlockScanNotify(header *openwallet.BlockHeader) error {
	log.Notice("header:", header)
	return nil
}

//BlockTxExtractDataNotify 区块提取结果通知
func (sub *subscriberSingle) BlockExtractDataNotify(sourceKey string, data *openwallet.TxExtractData) error {
	log.Notice("account:", sourceKey)

	for i, input := range data.TxInputs {
		log.Std.Notice("data.TxInputs[%d]: %+v", i, input)
	}

	for i, output := range data.TxOutputs {
		log.Std.Notice("data.TxOutputs[%d]: %+v", i, output)
	}

	log.Std.Notice("data.Transaction: %+v", data.Transaction)


	//tm := testInitWalletManager()
	//walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	//accountID := "HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU"

	//testGetAssetsAccountBalance(tm, walletID, accountID)
	//walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	//accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"
	//
	//contract := openwallet.SmartContract{
	//	Address:"NseCpCRzVU3U9RSYyTwSFhdL71wEnpDv",
	//	Decimals:8,
	//	Name:"angel",
	//}
	//balance, err := tm.GetAssetsAccountTokenBalance(testApp,walletID, accountID, contract)
	////balance, err := sub.manager.GetAssetsAccountBalance(testApp, walletID, accountID)
	//if err != nil {
	//	log.Error("GetAssetsAccountBalance failed, unexpected error:", err)
	//	return nil
	//}
	//log.Notice("account balance:", balance.Balance.Balance)

	return nil
}



func TestGetTokenBlance(t *testing.T){
	tm := testInitWalletManager()
	for i:=0;i<100;i++{
		walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
		accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"

		contract := openwallet.SmartContract{
			Address:"NseCpCRzVU3U9RSYyTwSFhdL71wEnpDv",
			Decimals:8,
			Name:"angel",
		}

		balance, err := tm.GetAssetsAccountTokenBalance(testApp,walletID, accountID, contract)
		if err != nil {
			log.Error("GetAssetsAccountBalance failed, unexpected error:", err)
			return
		}
		log.Notice("account balance:", balance.Balance.Balance)
	}

}

func TestSubscribeAddress(t *testing.T) {

	var (
		endRunning = make(chan bool, 1)
		symbol     = "NULS"
		addrs      = map[string]string{
			"Nse6cbCoZenQo7wwEFjs3th9MgNXcs2A":"Nse6cbCoZenQo7wwEFjs3th9MgNXcs2A",
			"Nse2aof9WeCoNiR86JfMfMCUmwc7vVQr":"Nse2aof9WeCoNiR86JfMfMCUmwc7vVQr",
			"Nse5A6LQwwJiioY6VQa9znMsc7eAzdnj":"Nse5A6LQwwJiioY6VQa9znMsc7eAzdnj",
			"Nse7HNez8z8zcQ98yCCL9PYKm5NGqxxg":"Nse7HNez8z8zcQ98yCCL9PYKm5NGqxxg",
			"Nsdy2uqQLRoiTx1x2jRYEqxhtFu6CRkn":"Nsdy2uqQLRoiTx1x2jRYEqxhtFu6CRkn",
			"NsdxePUrgcKstvbwyfi8oDmrmWDTiM1L":"NsdxePUrgcKstvbwyfi8oDmrmWDTiM1L",
			"Nse7fHdcBPcey6gFyWjhMKfZYLEAmAEj":"Nse7fHdcBPcey6gFyWjhMKfZYLEAmAEj",
			"Nsdx573EYsEXRiwXXagTJdEbuFz9o1vg":"Nsdx573EYsEXRiwXXagTJdEbuFz9o1vg",
			"NsdybWu4R8TX46NEYwEfGoVwvf6tn5RM":"NsdybWu4R8TX46NEYwEfGoVwvf6tn5RM",
			"Nse8SXiKiiEXiYdJ1dmAKJaAyS1TMwEV":"Nse8SXiKiiEXiYdJ1dmAKJaAyS1TMwEV",
		}
	)

	tm := testInitWalletManager()

	go func() {
		//time.Sleep(10 * time.Second)
		//TestTransferNrc20(t)
	}()

	//GetSourceKeyByAddress 获取地址对应的数据源标识
	scanAddressFunc := func(address string) (string, bool) {
		key, ok := addrs[address]
		if !ok {
			return "", false
		}
		return key, true
	}

	assetsMgr, err := openw.GetAssetsAdapter(symbol)
	if err != nil {
		log.Error(symbol, "is not support")
		return
	}

	//读取配置
	absFile := filepath.Join(configFilePath, symbol+".ini")

	c, err := config.NewConfig("ini", absFile)
	if err != nil {
		return
	}
	assetsMgr.LoadAssetsConfig(c)

	assetsLogger := assetsMgr.GetAssetsLogger()
	if assetsLogger != nil {
		assetsLogger.SetLogFuncCall(true)
	}

	//log.Debug("already got scanner:", assetsMgr)
	scanner := assetsMgr.GetBlockScanner()
	scanner.SetRescanBlockHeight(3166839)

	if scanner == nil {
		log.Error(symbol, "is not support block scan")
		return
	}

	scanner.SetBlockScanAddressFunc(scanAddressFunc)

	sub := subscriberSingle{manager:tm}
	scanner.AddObserver(&sub)

	scanner.Run()


	<-endRunning
}


func TestSubscribeScanBlock(t *testing.T) {

	var (
		symbol     = "NULS"
		addrs      = map[string]string{
			//"Nsdzhw5UzcewtbpweAwXvU7MngCmdNag": "sender",
			"Nse6cbCoZenQo7wwEFjs3th9MgNXcs2A":"Nse6cbCoZenQo7wwEFjs3th9MgNXcs2A",
			"Nse2aof9WeCoNiR86JfMfMCUmwc7vVQr":"Nse2aof9WeCoNiR86JfMfMCUmwc7vVQr",
			"Nse5A6LQwwJiioY6VQa9znMsc7eAzdnj":"Nse5A6LQwwJiioY6VQa9znMsc7eAzdnj",
			"Nse7HNez8z8zcQ98yCCL9PYKm5NGqxxg":"Nse7HNez8z8zcQ98yCCL9PYKm5NGqxxg",
			"Nsdy2uqQLRoiTx1x2jRYEqxhtFu6CRkn":"Nsdy2uqQLRoiTx1x2jRYEqxhtFu6CRkn",
			"NsdxePUrgcKstvbwyfi8oDmrmWDTiM1L":"NsdxePUrgcKstvbwyfi8oDmrmWDTiM1L",
			"Nse7fHdcBPcey6gFyWjhMKfZYLEAmAEj":"Nse7fHdcBPcey6gFyWjhMKfZYLEAmAEj",
			"Nsdx573EYsEXRiwXXagTJdEbuFz9o1vg":"Nsdx573EYsEXRiwXXagTJdEbuFz9o1vg",
			"NsdybWu4R8TX46NEYwEfGoVwvf6tn5RM":"NsdybWu4R8TX46NEYwEfGoVwvf6tn5RM",
			"Nse8SXiKiiEXiYdJ1dmAKJaAyS1TMwEV":"Nse8SXiKiiEXiYdJ1dmAKJaAyS1TMwEV",
		}
	)

	//GetSourceKeyByAddress 获取地址对应的数据源标识
	scanAddressFunc := func(address string) (string, bool) {
		key, ok := addrs[address]
		if !ok {
			return "", false
		}
		return key, true
	}

	assetsMgr, err := openw.GetAssetsAdapter(symbol)
	if err != nil {
		log.Error(symbol, "is not support")
		return
	}

	//读取配置
	absFile := filepath.Join(configFilePath, symbol+".ini")

	c, err := config.NewConfig("ini", absFile)
	if err != nil {
		return
	}
	assetsMgr.LoadAssetsConfig(c)

	assetsLogger := assetsMgr.GetAssetsLogger()
	if assetsLogger != nil {
		assetsLogger.SetLogFuncCall(true)
	}

	//log.Debug("already got scanner:", assetsMgr)
	scanner := assetsMgr.GetBlockScanner()
	if scanner == nil {
		log.Error(symbol, "is not support block scan")
		return
	}

	scanner.SetBlockScanAddressFunc(scanAddressFunc)

	sub := subscriberSingle{}
	scanner.AddObserver(&sub)

	scanner.ScanBlock(3166839)

	time.Sleep(5*time.Second)
}
