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
	"github.com/blocktree/openwallet/openw"
	"testing"
	"time"

	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
)

func testGetAssetsAccountBalance(tm *openw.WalletManager, walletID, accountID string) {
	balance, err := tm.GetAssetsAccountBalance(testApp, walletID, accountID)
	if err != nil {
		log.Error("GetAssetsAccountBalance failed, unexpected error:", err)
		return
	}
	log.Error("balance:", balance)
}

func testGetAssetsAccountTokenBalance(tm *openw.WalletManager, walletID, accountID string, contract openwallet.SmartContract) {
	balance, err := tm.GetAssetsAccountTokenBalance(testApp, walletID, accountID, contract)
	if err != nil {
		log.Error("GetAssetsAccountTokenBalance failed, unexpected error:", err)
		return
	}
	log.Info("token balance:", balance.Balance)
}

func testCreateTransactionStep(tm *openw.WalletManager, walletID, accountID, to, amount, feeRate string, contract *openwallet.SmartContract) (*openwallet.RawTransaction, error) {

	//err := tm.RefreshAssetsAccountBalance(testApp, accountID)
	//if err != nil {
	//	log.Error("RefreshAssetsAccountBalance failed, unexpected error:", err)
	//	return nil, err
	//}

	rawTx, err := tm.CreateTransaction(testApp, walletID, accountID, amount, to, feeRate, "", contract)

	if err != nil {
		log.Error("CreateTransaction failed, unexpected error:", err)
		return nil, err
	}

	return rawTx, nil
}

func testCreateSummaryTransactionStep(
	tm *openw.WalletManager,
	walletID, accountID, summaryAddress, minTransfer, retainedBalance, feeRate string,
	start, limit int,
	contract *openwallet.SmartContract,
	feeSupportAccount *openwallet.FeesSupportAccount) ([]*openwallet.RawTransactionWithError, error) {

	rawTxArray, err := tm.CreateSummaryRawTransactionWithError(testApp, walletID, accountID, summaryAddress, minTransfer,
		retainedBalance, feeRate, start, limit, contract, feeSupportAccount)

	if err != nil {
		log.Error("CreateSummaryTransaction failed, unexpected error:", err)
		return nil, err
	}

	return rawTxArray, nil
}

func testSignTransactionStep(tm *openw.WalletManager, rawTx *openwallet.RawTransaction) (*openwallet.RawTransaction, error) {

	_, err := tm.SignTransaction(testApp, rawTx.Account.WalletID, rawTx.Account.AccountID, "12345678", rawTx)
	if err != nil {
		log.Error("SignTransaction failed, unexpected error:", err)
		return nil, err
	}

	log.Infof("rawTx: %+v", rawTx)
	return rawTx, nil
}

func testVerifyTransactionStep(tm *openw.WalletManager, rawTx *openwallet.RawTransaction) (*openwallet.RawTransaction, error) {

	//log.Info("rawTx.Signatures:", rawTx.Signatures)

	_, err := tm.VerifyTransaction(testApp, rawTx.Account.WalletID, rawTx.Account.AccountID, rawTx)
	if err != nil {
		log.Error("VerifyTransaction failed, unexpected error:", err)
		return nil, err
	}

	log.Infof("rawTx: %+v", rawTx)
	return rawTx, nil
}

func testSubmitTransactionStep(tm *openw.WalletManager, rawTx *openwallet.RawTransaction) (*openwallet.RawTransaction, error) {

	tx, err := tm.SubmitTransaction(testApp, rawTx.Account.WalletID, rawTx.Account.AccountID, rawTx)
	if err != nil {
		log.Error("SubmitTransaction failed, unexpected error:", err)
		return nil, err
	}

	log.Std.Info("tx: %+v", tx)
	log.Info("wxID:", tx.WxID)
	log.Info("txID:", rawTx.TxID)

	return rawTx, nil
}

func TestTransfer(t *testing.T) {

	//fiiimScGn5TGQFjxYXWrCaHsqUeGJW9MTcfZ5L
	//fiiimScVPvfegV4Wzf1hx1UvnQNLv1sVatwUyj
	//fiiimJVwi1mPr4ebAD9x18Wzbt2NQxbsCvGLxE
	//fiiimUkH3QEr2yYT3NmbrWKWSKX739EoMQ4e7Y
	//fiiimS8BWk1oKnDr1LJn5EpXbSVijKrHVHRybE

	tm := testInitWalletManager()
	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	accountID := "HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU"
	to := "Nse2PgVTn7K3CsLrDxjHTjxpxyvp3zyP"

	//accountID := "4h4wnCmpzgy3ZTeoMHs3gjDCuWyXQcxDsk9dcwbNGhmR"
	//to := "fiiimYt7qZekpQKZauBGxv8kGFJGdMyYtzSgdP"

	testGetAssetsAccountBalance(tm, walletID, accountID)




	rawTx, err := testCreateTransactionStep(tm, walletID, accountID, to, "0.01", "", nil)
	if err != nil {
		return
	}

	_, err = testSignTransactionStep(tm, rawTx)
	if err != nil {
		return
	}

	_, err = testVerifyTransactionStep(tm, rawTx)
	if err != nil {
		return
	}

	_, err = testSubmitTransactionStep(tm, rawTx)
	if err != nil {
		return
	}

}



func TestTransferNrc20(t *testing.T) {

	//fiiimScGn5TGQFjxYXWrCaHsqUeGJW9MTcfZ5L
	//fiiimScVPvfegV4Wzf1hx1UvnQNLv1sVatwUyj
	//fiiimJVwi1mPr4ebAD9x18Wzbt2NQxbsCvGLxE
	//fiiimUkH3QEr2yYT3NmbrWKWSKX739EoMQ4e7Y
	//fiiimS8BWk1oKnDr1LJn5EpXbSVijKrHVHRybE

	address := []string{
		//"Nse1BC7HwSNf69BrcqGpw69ZvQTRJ9e6",
		//"Nse6tSvVHwBPTm3XfY9x37HRLjb9oq8h",
		//"NsdvAokV31ZxHBJnwbxQeug91BJdScMs",
		//"Nse5WHdQnRC3dd87BnAjGEWyjj3yWzx2",
		//"NsdwWYFeyCiWi8gaK6igqJurjoY9TCqo",


		"Nse4CJKNJboQijoaXBTVr94ZtSkAvb4N",
		"Nse4e4z9w7sXcTmYWezZGSrBNdC5Fp3q",
		"Nse2cP94tauqzR6LmvXFsn8Bx5B4zitQ",
		"NsdvafMC4mvpYk6Srq9J2Z5AsGEJ7iF6",

		//"Nse6cbCoZenQo7wwEFjs3th9MgNXcs2A",
		//"Nse2aof9WeCoNiR86JfMfMCUmwc7vVQr",
		//"Nse5A6LQwwJiioY6VQa9znMsc7eAzdnj",
		//"Nse7HNez8z8zcQ98yCCL9PYKm5NGqxxg",
		//"Nsdy2uqQLRoiTx1x2jRYEqxhtFu6CRkn",
		//"NsdxePUrgcKstvbwyfi8oDmrmWDTiM1L",
		//"Nse7fHdcBPcey6gFyWjhMKfZYLEAmAEj",
		//"Nsdx573EYsEXRiwXXagTJdEbuFz9o1vg",
		//"NsdybWu4R8TX46NEYwEfGoVwvf6tn5RM",
		//"Nse8SXiKiiEXiYdJ1dmAKJaAyS1TMwEV",



	}


	tm := testInitWalletManager()
	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	//accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"
	accountID := "HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU"


	//accountID := "4h4wnCmpzgy3ZTeoMHs3gjDCuWyXQcxDsk9dcwbNGhmR"
	//to := "fiiimYt7qZekpQKZauBGxv8kGFJGdMyYtzSgdP"

	//testGetAssetsAccountBalance(tm, walletID, accountID)

	contract := &openwallet.SmartContract{
		Address:"NseCpCRzVU3U9RSYyTwSFhdL71wEnpDv",
		Decimals:8,
		Name:"angel",
	}

	for _,a := range address{
		time.Sleep(15*time.Second)
		rawTx, err := testCreateTransactionStep(tm, walletID, accountID, a, "0.2", "", contract)
		if err != nil {
			return
		}

		_, err = testSignTransactionStep(tm, rawTx)
		if err != nil {
			return
		}

		_, err = testVerifyTransactionStep(tm, rawTx)
		if err != nil {
			return
		}

		_, err = testSubmitTransactionStep(tm, rawTx)
		if err != nil {
			return
		}
	}


}

func TestSummary(t *testing.T) {
	tm := testInitWalletManager()
	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	//accountID := "HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU"
	accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"
	summaryAddress := "Nse5VJW4vNDyJXkcMmCTHRD9QQC5T1WS"

	testGetAssetsAccountBalance(tm, walletID, accountID)

	rawTxArray, err := testCreateSummaryTransactionStep(tm, walletID, accountID,
		summaryAddress, "", "", "",
		0, 100, nil, nil)
	if err != nil {
		log.Errorf("CreateSummaryTransaction failed, unexpected error: %v", err)
		return
	}

	//执行汇总交易
	for _, rawTxWithErr := range rawTxArray {

		if rawTxWithErr.Error != nil {
			log.Error(rawTxWithErr.Error.Error())
			continue
		}

		_, err = testSignTransactionStep(tm, rawTxWithErr.RawTx)
		if err != nil {
			return
		}

		_, err = testVerifyTransactionStep(tm, rawTxWithErr.RawTx)
		if err != nil {
			return
		}

		_, err = testSubmitTransactionStep(tm, rawTxWithErr.RawTx)
		if err != nil {
			return
		}
	}

}


func TestTokenSummary(t *testing.T) {
	tm := testInitWalletManager()
	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"
	summaryAddress := "Nse5VJW4vNDyJXkcMmCTHRD9QQC5T1WS"

	testGetAssetsAccountBalance(tm, walletID, accountID)


	for {
		contract := &openwallet.SmartContract{
			Address:"NseCpCRzVU3U9RSYyTwSFhdL71wEnpDv",
			Decimals:8,
			Name:"angel",
		}

		fee := &openwallet.FeesSupportAccount{
			AccountID : "HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU",
			FixSupportAmount:"0.2",
		}

		//feeSupport, _ := tm.GetAssetsAccountInfo(testApp,walletID,"HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU")

		rawTxArray, err := testCreateSummaryTransactionStep(tm, walletID, accountID,
			summaryAddress, "", "", "",
			0, 100, contract, fee)
		if err != nil {
			log.Errorf("CreateSummaryTransaction failed, unexpected error: %v", err)
			return
		}

		//执行汇总交易
		for _, rawTxWithErr := range rawTxArray {

			if rawTxWithErr.Error != nil {
				log.Error(rawTxWithErr.Error.Error())
				continue
			}



			_, err = testSignTransactionStep(tm, rawTxWithErr.RawTx)
			if err != nil {
				return
			}

			_, err = testVerifyTransactionStep(tm, rawTxWithErr.RawTx)
			if err != nil {
				return
			}

			_, err = testSubmitTransactionStep(tm, rawTxWithErr.RawTx)
			if err != nil {
				return
			}
		}
		time.Sleep(50*time.Second)
	}



}

