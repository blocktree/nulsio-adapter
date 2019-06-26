package openwtester

import (
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openw"
	"github.com/blocktree/openwallet/openwallet"
	"path/filepath"
	"testing"
)

var (
	testApp        = "NULScoin-adapter"
	configFilePath = filepath.Join("conf")
)

func testInitWalletManager() *openw.WalletManager {
	log.SetLogFuncCall(true)
	tc := openw.NewConfig()

	tc.ConfigDir = configFilePath
	tc.EnableBlockScan = false
	tc.SupportAssets = []string{
		"NULS",
	}
	return openw.NewWalletManager(tc)
	//tm.Init()
}

func TestWalletManager_CreateWallet(t *testing.T) {
	tm := testInitWalletManager()
	w := &openwallet.Wallet{Alias: "HELLO NULS", IsTrust: true, Password: "12345678"}
	nw, key, err := tm.CreateWallet(testApp, w)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("wallet:", nw)
	log.Info("key:", key)

}

func TestWalletManager_GetWalletInfo(t *testing.T) {

	tm := testInitWalletManager()

	wallet, err := tm.GetWalletInfo(testApp, "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N")
	if err != nil {
		log.Error("unexpected error:", err)
		return
	}
	log.Info("wallet:", wallet)
}

func TestWalletManager_GetWalletList(t *testing.T) {

	tm := testInitWalletManager()

	list, err := tm.GetWalletList(testApp, 0, 10000000)
	if err != nil {
		log.Error("unexpected error:", err)
		return
	}
	for i, w := range list {
		log.Info("wallet[", i, "] :", w)
	}
	log.Info("wallet count:", len(list))

	tm.CloseDB(testApp)
}

//HhMp9EJwZpNFhfUuSSXanocxgPGz9eLoSbPbqawcWtWU
//Nse5VJW4vNDyJXkcMmCTHRD9QQC5T1WS
func TestWalletManager_CreateAssetsAccount(t *testing.T) {

	tm := testInitWalletManager()

	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	account := &openwallet.AssetsAccount{Alias: "mainnetNULS2", WalletID: walletID, Required: 1, Symbol: "NULS", IsTrust: true}
	account, address, err := tm.CreateAssetsAccount(testApp, walletID, "12345678", account, nil)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("account:", account)
	log.Info("address:", address)

	tm.CloseDB(testApp)
}

func TestWalletManager_GetAssetsAccountList(t *testing.T) {

	tm := testInitWalletManager()

	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	list, err := tm.GetAssetsAccountList(testApp, walletID, 0, 10000000)
	if err != nil {
		log.Error("unexpected error:", err)
		return
	}
	for i, w := range list {
		log.Infof("account[%d] : %+v", i, w)
	}
	log.Info("account count:", len(list))

	tm.CloseDB(testApp)

}

func TestWalletManager_CreateAddress(t *testing.T) {

	tm := testInitWalletManager()

	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"
	address, err := tm.CreateAddress(testApp, walletID, accountID, 5)
	if err != nil {
		log.Error(err)
		return
	}

	for _, addr := range address {
		log.Info(addr.Address)
	}


	tm.CloseDB(testApp)
}

func TestWalletManager_GetAddressList(t *testing.T) {

	tm := testInitWalletManager()

	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"
	//accountID := "4h4wnCmpzgy3ZTeoMHs3gjDCuWyXQcxDsk9dcwbNGhmR"
	list, err := tm.GetAddressList(testApp, walletID, accountID, 0, -1, false)
	if err != nil {
		log.Error("unexpected error:", err)
		return
	}
	for i, w := range list {
		log.Info("address[", i, "] :", w.Address)
		//log.Info("address[", i, "] :", w.PublicKey)
	}
	log.Info("address count:", len(list))

	tm.CloseDB(testApp)
}


func TestBatchCreateAddressByAccount(t *testing.T) {

	tm := testInitWalletManager()

	symbol := "NULS"
	walletID := "VzLUoGiZioDZDyisPtKFMD7Sfy485Qih2N"
	accountID := "CbhEiN6Pm3ZjJDCkwybanzs192Mo32jhph2RY4ZLMAFN"

	account, err := tm.GetAssetsAccountInfo(testApp, walletID, accountID)
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}


	assetsMgr, err := openw.GetAssetsAdapter(symbol)
	if err != nil {
		log.Error(symbol, "is not support")
		return
	}

	decoder := assetsMgr.GetAddressDecode()

	addrArr, err := openwallet.BatchCreateAddressByAccount(account, decoder, 10, 20)
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}
	addrs := make([]string, 0)
	for _, a := range addrArr {
		log.Infof("address[%d]: %s", a.Index, a.Address)
		addrs = append(addrs, a.Address)
	}
	log.Infof("create address")

}

