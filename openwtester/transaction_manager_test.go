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
	"github.com/blocktree/openwallet/openw"
	"path/filepath"
	"testing"

	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
)

func TestWalletManager_GetTransactions(t *testing.T) {
	tm := testInitWalletManager()
	list, err := tm.GetTransactions(testApp, 0, -1, "Received", false)
	if err != nil {
		log.Error("GetTransactions failed, unexpected error:", err)
		return
	}
	for i, tx := range list {
		log.Info("trx[", i, "] :", tx)
	}
	log.Info("trx count:", len(list))
}

func TestWalletManager_GetTxUnspent(t *testing.T) {
	tm := testInitWalletManager()
	list, err := tm.GetTxUnspent(testApp, 0, -1, "Received", false)
	if err != nil {
		log.Error("GetTxUnspent failed, unexpected error:", err)
		return
	}
	for i, tx := range list {
		log.Info("Unspent[", i, "] :", tx)
	}
	log.Info("Unspent count:", len(list))
}

func TestWalletManager_GetTxSpent(t *testing.T) {
	tm := testInitWalletManager()
	list, err := tm.GetTxSpent(testApp, 0, -1, "Received", false)
	if err != nil {
		log.Error("GetTxSpent failed, unexpected error:", err)
		return
	}
	for i, tx := range list {
		log.Info("Spent[", i, "] :", tx)
	}
	log.Info("Spent count:", len(list))
}

func TestWalletManager_ExtractUTXO(t *testing.T) {
	tm := testInitWalletManager()
	unspent, err := tm.GetTxUnspent(testApp, 0, -1, "Received", false)
	if err != nil {
		log.Error("GetTxUnspent failed, unexpected error:", err)
		return
	}
	for i, tx := range unspent {

		_, err := tm.GetTxSpent(testApp, 0, -1, "SourceTxID", tx.TxID, "SourceIndex", tx.Index)
		if err == nil {
			continue
		}

		log.Info("ExtractUTXO[", i, "] :", tx)
	}

}

func TestWalletManager_GetTransactionByWxID(t *testing.T) {
	tm := testInitWalletManager()
	wxID := openwallet.GenTransactionWxID(&openwallet.Transaction{
		TxID: "bfa6febb33c8ddde9f7f7b4d93043956cce7e0f4e95da259a78dc9068d178fee",
		Coin: openwallet.Coin{
			Symbol:     "LTC",
			IsContract: false,
			ContractID: "",
		},
	})
	log.Info("wxID:", wxID)
	//"D0+rxcKSqEsFMfGesVzBdf6RloM="
	tx, err := tm.GetTransactionByWxID(testApp, wxID)
	if err != nil {
		log.Error("GetTransactionByTxID failed, unexpected error:", err)
		return
	}
	log.Info("tx:", tx)
}

func TestWalletManager_GetAssetsAccountBalance(t *testing.T) {
	tm := testInitWalletManager()
	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	//accountID := "HX4tUVg5eETb6SvZeGeAFwk4PQ1CWS6dQeyjj3CqfYyK"
	accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"

	balance, err := tm.GetAssetsAccountBalance(testApp, walletID, accountID)
	if err != nil {
		log.Error("GetAssetsAccountBalance failed, unexpected error:", err)
		return
	}
	log.Info("balance:", balance)
}

func TestWalletManager_GetAssetsAccountTokenBalance(t *testing.T) {
	tm := testInitWalletManager()
	walletID := "W7tue6SDce38fPwerdKqyebUh6yo2nTQLC"
	accountID := "EPxkNBu6iMospC6aHQppv36UGY4mb1WqUE7oNZ7Xp9Df"

	contract := openwallet.SmartContract{
		Address:  "2",
		Symbol:   "FIII",
		Name:     "TetherUSD",
		Token:    "USDT",
		Decimals: 8,
	}

	balance, err := tm.GetAssetsAccountTokenBalance(testApp, walletID, accountID, contract)
	if err != nil {
		log.Error("GetAssetsAccountTokenBalance failed, unexpected error:", err)
		return
	}
	log.Info("balance:", balance.Balance)
}

func TestWalletManager_GetEstimateFeeRate(t *testing.T) {
	tm := testInitWalletManager()
	coin := openwallet.Coin{
		Symbol: "VSYS",
	}
	feeRate, unit, err := tm.GetEstimateFeeRate(coin)
	if err != nil {
		log.Error("GetEstimateFeeRate failed, unexpected error:", err)
		return
	}
	log.Std.Info("feeRate: %s %s/%s", feeRate, coin.Symbol, unit)
}

func TestGetAddressBalance(t *testing.T) {
	symbol := "FIII"
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
	bs := assetsMgr.GetBlockScanner()

	addrs := []string{
		"fiiimFW25nb5mwXnQxX5V8b29EgyjAQNS9gmD6",
	}

	balances, err := bs.GetBalanceByAddress(addrs...)
	if err != nil {
		log.Errorf(err.Error())
		return
	}
	for _, b := range balances {
		log.Infof("balance[%s] = %s", b.Address, b.Balance)
		log.Infof("UnconfirmBalance[%s] = %s", b.Address, b.UnconfirmBalance)
		log.Infof("ConfirmBalance[%s] = %s", b.Address, b.ConfirmBalance)
	}
}