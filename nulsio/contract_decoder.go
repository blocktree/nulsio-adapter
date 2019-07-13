package nulsio

import (
	"errors"
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
)

type NulsContractDecoder struct {
	*openwallet.SmartContractDecoderBase
	wm *WalletManager
}



func NewContractDecoder(wm *WalletManager) *NulsContractDecoder {
	decoder := NulsContractDecoder{}
	decoder.wm = wm
	return &decoder
}


type AddrBalance struct {
	Address      string
	Balance      *decimal.Decimal
	TokenBalance *decimal.Decimal
	Index        int
}

func (this *AddrBalance) SetTokenBalance(b *decimal.Decimal) {
	this.TokenBalance = b
}

func (this *AddrBalance) GetAddress() string {
	return this.Address
}

func (this *AddrBalance) ValidTokenBalance() bool {
	if this.TokenBalance == nil {
		return false
	}
	return true
}

type AddrBalanceInf interface {
	SetTokenBalance(b decimal.Decimal)
	GetAddress() string
	ValidTokenBalance() bool
}

func (this *WalletManager) GetTokenBalanceByAddress(contractAddr string, addrs ...AddrBalanceInf) error {
	threadControl := make(chan int, 20)
	defer close(threadControl)
	resultChan := make(chan AddrBalanceInf, 1024)
	defer close(resultChan)
	done := make(chan int, 1)
	count := len(addrs)
	var err error

	go func() {
		log.Debugf("in save thread.")
		for i := 0; i < count; i++ {
			addr := <-resultChan
			if !addr.ValidTokenBalance() {
				err = errors.New("query token balance failed")
			}
		}
		done <- 1
	}()


	queryBalance := func(addr AddrBalanceInf) {
		threadControl <- 1
		defer func() {
			resultChan <- addr
			<-threadControl
		}()

		balance, err := this.Api.GetTokenBalances(contractAddr,addr.GetAddress())
		if err != nil {
			log.Errorf("get address[%v] nrc20 token balance failed, err=%v", addr.GetAddress(), err)
			return
		}

		addr.SetTokenBalance(balance)
	}

	for i := range addrs {
		go queryBalance(addrs[i])
	}

	<-done

	return err
}

func (this *NulsContractDecoder) GetTokenBalanceByAddress(contract openwallet.SmartContract, address ...string) ([]*openwallet.TokenBalance, error) {
	threadControl := make(chan int, 20)
	defer close(threadControl)
	resultChan := make(chan *openwallet.TokenBalance, 1024)
	defer close(resultChan)
	done := make(chan int, 1)
	var tokenBalanceList []*openwallet.TokenBalance
	count := len(address)

	go func() {
		//		log.Debugf("in save thread.")
		for i := 0; i < count; i++ {
			balance := <-resultChan
			if balance != nil {
				tokenBalanceList = append(tokenBalanceList, balance)
			}
			//log.Debugf("got one balance.")
		}
		done <- 1
	}()

	queryBalance := func(address string) {
		threadControl <- 1
		var balance *openwallet.TokenBalance
		defer func() {
			resultChan <- balance
			<-threadControl
		}()

		balanceTemp, err := this.wm.Api.GetTokenBalancesReal(contract.Address,address)
		if err != nil {
			log.Errorf("get address[%v] nrc20 token balance failed, err=%v",address, err)
			return
		}
		//log.Error(balanceTemp.String())

		balanceTemp = balanceTemp.Shift(int32(-contract.Decimals))
		//log.Error(balanceTemp.String())

		balance = &openwallet.TokenBalance{
			Contract: &contract,
			Balance: &openwallet.Balance{
				Address:          address,
				Symbol:           contract.Symbol,
				Balance:          balanceTemp.String(),
				ConfirmBalance:   balanceTemp.String(),
				UnconfirmBalance: balanceTemp.String(),
			},
		}
	}

	for i := range address {
		go queryBalance(address[i])
	}

	<-done

	if len(tokenBalanceList) != count {
		log.Error("unknown errors occurred .")
		return nil, errors.New("unknown errors occurred ")
	}
	return tokenBalanceList, nil
}
