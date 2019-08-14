package nulsio

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/blocktree/go-owcrypt"
	"github.com/blocktree/nulsio-adapter/nulsio_trans"
	"github.com/blocktree/openwallet/common"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
	"math/big"
	"sort"
	"strings"
	"time"
)

var (
	DEFAULT_GAS_LIMIT = "250000"
	DEFAULT_GAS_PRICE = decimal.New(4, -7)
)

type TransactionDecoder struct {
	openwallet.TransactionDecoderBase
	wm *WalletManager //钱包管理者
}

//NewTransactionDecoder 交易单解析器
func NewTransactionDecoder(wm *WalletManager) *TransactionDecoder {
	decoder := TransactionDecoder{}
	decoder.wm = wm
	return &decoder
}

//CreateRawTransaction 创建交易单
func (decoder *TransactionDecoder) CreateRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {
	if rawTx.Coin.IsContract {
		return decoder.CreateNrc20RawTransaction(wrapper, rawTx, "")
	} else {
		return decoder.CreateSimpleRawTransaction(wrapper, rawTx)
	}
}

//CreateSummaryRawTransaction 创建汇总交易，返回原始交易单数组
func (decoder *TransactionDecoder) CreateSummaryRawTransactionWithError(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {
	if sumRawTx.Coin.IsContract {
		return decoder.CreateTokenSummaryRawTransaction(wrapper, sumRawTx)
	} else {
		return decoder.CreateSimpleSummaryRawTransaction(wrapper, sumRawTx)
	}
}

//CreateRawTransaction 创建交易单
func (decoder *TransactionDecoder) CreateSimpleRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {

	var (
		usedUTXO         []*UtxoDto
		outputAddrs      = make(map[string]decimal.Decimal)
		balance          = decimal.New(0, 0)
		totalSend        = decimal.New(0, 0)
		actualFees       = decimal.New(0, 0)
		feesRate         = decimal.New(0, 0)
		accountID        = rawTx.Account.AccountID
		destinations     = make([]string, 0)
		accountTotalSent = decimal.Zero
	)

	address, err := wrapper.GetAddressList(0, -1, "AccountID", rawTx.Account.AccountID)
	if err != nil {
		return err
	}

	if len(address) == 0 {
		return fmt.Errorf("[%s] have not addresses", accountID)
	}

	searchAddrs := make([]string, 0)
	for _, address := range address {
		searchAddrs = append(searchAddrs, address.Address)
	}

	//decoder.wm.Log.Debug(searchAddrs)
	//查找账户的utxo

	unspent := make([]*UtxoDto, 0)
	for _, v := range searchAddrs {
		unspentTemps, err := decoder.wm.Api.GetUnSpent(v)
		if err != nil {
			decoder.wm.Log.Warn("cant find the unspent...")
			continue
		}
		unspent = append(unspent, unspentTemps...)
	}

	if len(unspent) == 0 {
		return openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAddress, "[%s] balance is not enough", accountID)
	}

	if len(rawTx.To) == 0 {
		return errors.New("Receiver addresses is empty!")
	}

	//计算总发送金额
	for addr, amount := range rawTx.To {
		deamount, _ := decimal.NewFromString(amount)
		totalSend = totalSend.Add(deamount)
		destinations = append(destinations, addr)
		//计算账户的实际转账amount
		addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", rawTx.Account.AccountID, "Address", addr)
		if findErr != nil || len(addresses) == 0 {
			amountDec, _ := decimal.NewFromString(amount)
			accountTotalSent = accountTotalSent.Add(amountDec)
		}
	}

	//获取utxo，按小到大排序
	sort.Sort(UnspentSort{usedUTXO, func(a, b *UtxoDto) int {
		a_amount := decimal.New(a.Value, 0)
		b_amount := decimal.New(b.Value, 0)
		if a_amount.GreaterThan(b_amount) {
			return 1
		} else {
			return -1
		}
	}})

	if len(rawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateFeeRate()
		if err != nil {
			return err
		}
	} else {
		feesRate, _ = decimal.NewFromString(rawTx.FeeRate)
	}

	decoder.wm.Log.Info("Calculating wallet unspent record to build transaction...")
	computeTotalSend := totalSend
	//循环的计算余额是否足够支付发送数额+手续费
	for {

		usedUTXO = make([]*UtxoDto, 0)
		balance = decimal.New(0, 0)

		//计算一个可用于支付的余额
		for _, u := range unspent {

			ua := decimal.New(u.Value, 0)
			ua = ua.Shift(-decoder.wm.Decimal())
			balance = balance.Add(ua)
			usedUTXO = append(usedUTXO, u)
			if balance.GreaterThanOrEqual(computeTotalSend) {
				break
			}
		}

		if balance.LessThan(computeTotalSend) {
			return openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAddress, "[%s] balance is not enough", balance.StringFixed(decoder.wm.Decimal()))
		}

		//计算手续费，找零地址有2个，一个是发送，一个是新创建的
		fees, err := decoder.wm.EstimateFee(int64(len(usedUTXO)), int64(len(destinations)+1), "", feesRate)
		if err != nil {
			return err
		}

		//如果要手续费有发送支付，得计算加入手续费后，计算余额是否足够
		//总共要发送的
		computeTotalSend = totalSend.Add(fees)
		if computeTotalSend.GreaterThan(balance) {
			continue
		}
		computeTotalSend = totalSend

		actualFees = fees

		break

	}

	////UTXO如果大于设定限制，则分拆成多笔交易单发送
	//if len(usedUTXO) > decoder.wm.config.maxTxInputs {
	//	errStr := fmt.Sprintf("The transaction is use max inputs over: %d", decoder.wm.config.maxTxInputs)
	//	return errors.New(errStr)
	//}

	//取账户最后一个地址
	changeAddress := usedUTXO[0].Address

	changeAmount := balance.Sub(computeTotalSend).Sub(actualFees)
	rawTx.FeeRate = feesRate.StringFixed(decoder.wm.Decimal())
	rawTx.Fees = actualFees.StringFixed(decoder.wm.Decimal())

	decoder.wm.Log.Std.Notice("-----------------------------------------------")
	decoder.wm.Log.Std.Notice("From Account: %s", accountID)
	decoder.wm.Log.Std.Notice("To Address: %s", strings.Join(destinations, ", "))
	decoder.wm.Log.Std.Notice("Use: %v", balance.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Fees: %v", actualFees.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Receive: %v", computeTotalSend.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change: %v", changeAmount.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change Address: %v", changeAddress)
	decoder.wm.Log.Std.Notice("-----------------------------------------------")

	//装配输出
	for to, amount := range rawTx.To {
		decamount, _ := decimal.NewFromString(amount)
		outputAddrs = appendOutput(outputAddrs, to, decamount)
	}

	//changeAmount := balance.Sub(totalSend).Sub(actualFees)
	if changeAmount.GreaterThan(decimal.New(0, 0)) {
		//outputAddrs[changeAddress] = changeAmount.StringFixed(decoder.wm.Decimal())
		outputAddrs = appendOutput(outputAddrs, changeAddress, changeAmount)
	}

	err = decoder.createSimpleRawTransaction(wrapper, rawTx, usedUTXO, outputAddrs)
	if err != nil {
		return err
	}

	return nil
}

//CreateNrc20RawTransaction 创建合约交易
func (decoder *TransactionDecoder) CreateNrc20RawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction, sendTragetAddress string) (error) {

	var (
		outputAddrs        = make(map[string]decimal.Decimal)
		balance            = decimal.New(0, 0)
		totalSend          = decimal.New(0, 0)
		actualFees         = decimal.New(0, 0)
		feesRate           = decimal.New(0, 0)
		to                 string
		tokenAddress       string
		tokenDecimal       uint64
		sendAddressBalance decimal.Decimal
		changeAddress      string
		changeAmount       decimal.Decimal
		sendAddress        string
		accountID          = rawTx.Account.AccountID
	)

	address, err := wrapper.GetAddressList(0, -1, "AccountID", rawTx.Account.AccountID)
	if err != nil {
		return err
	}
	if rawTx.Coin.IsContract {
		tokenAddress = rawTx.Coin.Contract.Address
		tokenDecimal = rawTx.Coin.Contract.Decimals
	} else {
		return errors.New("This is a token transaction!")
	}

	if len(address) == 0 {
		return fmt.Errorf("[%s] have not addresses", accountID)
	}

	if len(rawTx.To) == 0 {
		return errors.New("Receiver addresses is empty!")
	}

	if len(rawTx.To) > 1 {
		return errors.New("Don't support !")
	}

	if len(rawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateTokenFeeRate()
		if err != nil {
			return err
		}
	} else {
		feesRate, _ = decimal.NewFromString(rawTx.FeeRate)
	}

	actualFees = feesRate //临定费率

	//计算总发送金额
	for addr, amount := range rawTx.To {
		deamount, _ := decimal.NewFromString(amount)
		totalSend = totalSend.Add(deamount)
		to = addr
	}

	unspent := make([]*UtxoDto, 0)

	sendAddressBalance = decimal.Zero

	for _, address := range address {
		if sendTragetAddress != "" {
			if address.Address != sendTragetAddress {
				continue
			}
		}
		tokenBalance, err := decoder.wm.Api.GetTokenBalances(tokenAddress, address.Address)
		if err == nil {

			//token是否足够
			if tokenBalance.GreaterThanOrEqual(totalSend) {
				unspentTemps, err := decoder.wm.Api.GetUnSpent(address.Address)
				if err != nil {
					decoder.wm.Log.Warn("cant find the unspent...")
					continue
				}
				if unspentTemps != nil {
					//查找utxo成功结束标记
					unspentTempsFinish := false
					//判断utxo是否足够
					comSend := decimal.Zero
					for _, u := range unspentTemps {
						utxoBalance := decimal.New(u.Value, 0)
						comSend = comSend.Add(utxoBalance)
					}
					if comSend.GreaterThanOrEqual(actualFees) {
						//再选择最优的utxo
						//获取utxo，按小到大排序
						sort.Sort(UnspentSort{unspentTemps, func(a, b *UtxoDto) int {
							a_amount := decimal.New(a.Value, 0)
							b_amount := decimal.New(b.Value, 0)
							if a_amount.GreaterThan(b_amount) {
								return 1
							} else {
								return -1
							}
						}})
						comSend = decimal.Zero
						for _, u := range unspentTemps {
							utxoBalance := decimal.New(u.Value, 0).Shift(-decoder.wm.Decimal())
							comSend = comSend.Add(utxoBalance)
							unspent = append(unspent, u)
							if comSend.GreaterThanOrEqual(actualFees) {
								balance = comSend
								changeAddress = u.Address
								changeAmount = comSend.Sub(actualFees)
								unspentTempsFinish = true
								break
							}
						}

					}
					if unspentTempsFinish {
						sendAddress = address.Address
						sendAddressBalance = tokenBalance
						break
					}

				}

			}
		}

	}

	if sendAddressBalance.LessThanOrEqual(decimal.Zero) {

		return openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAddress, "the [%s] balance: %s is not enough to call smart contract", rawTx.Coin.Symbol, sendAddressBalance)

	}

	if len(unspent) == 0 {
		return openwallet.Errorf(openwallet.ErrInsufficientFees, "the [%s] balance: %s is not enough to call smart contract", rawTx.Coin.Symbol)

	}

	if changeAmount.LessThan(decimal.Zero) {
		return fmt.Errorf("[%s] balance is not enough", rawTx.Coin.Contract.Name)
	}
	decoder.wm.Log.Info("Calculating wallet unspent record to build transaction...")
	computeTotalSend := totalSend

	rawTx.FeeRate = feesRate.StringFixed(decoder.wm.Decimal())
	rawTx.Fees = actualFees.StringFixed(decoder.wm.Decimal())

	decoder.wm.Log.Std.Notice("-----------------------------------------------")
	decoder.wm.Log.Std.Notice("From Account: %s", accountID)
	decoder.wm.Log.Std.Notice("From Token Address: %s", sendAddress)
	decoder.wm.Log.Std.Notice("From ContractAddress Address: %s", tokenAddress)
	decoder.wm.Log.Std.Notice("From Token Address Balance: %s", sendAddressBalance.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("To Token Address: %s", to)
	decoder.wm.Log.Std.Notice("To Token : %s", totalSend.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Use: %v", balance.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Fees: %v", actualFees.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Receive: %v", computeTotalSend.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change: %v", changeAmount.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change Address: %v", changeAddress)
	decoder.wm.Log.Std.Notice("-----------------------------------------------")

	if changeAmount.GreaterThan(decimal.New(0, 0)) {
		outputAddrs = appendOutput(outputAddrs, changeAddress, changeAmount)
	}

	token := &nulsio_trans.TxToken{
		Sender:          sendAddress,
		ContractAddress: tokenAddress,
		Value:           0,
		GasLimit:        20000,
		Price:           25,
		MethodName:      "transfer",
		ArgsCount:       2,
		Args:            []string{to, totalSend.Shift(int32(tokenDecimal)).String()},
	}

	err = decoder.createSimpleNrc20RawTransaction(wrapper, rawTx, unspent, outputAddrs, token)
	if err != nil {
		return err
	}

	return nil
}

//CreateSimpleSummaryRawTransaction 创建主币汇总交易
func (decoder *TransactionDecoder) CreateSimpleSummaryRawTransaction(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {

	var (
		feesRate           = decimal.New(0, 0)
		accountID          = sumRawTx.Account.AccountID
		minTransfer, _     = decimal.NewFromString(sumRawTx.MinTransfer)
		retainedBalance, _ = decimal.NewFromString(sumRawTx.RetainedBalance)
		sumAddresses       = make([]string, 0)
		rawTxArray         = make([]*openwallet.RawTransactionWithError, 0)
		sumUnspents        []*UtxoDto
		outputAddrs        map[string]decimal.Decimal
		totalInputAmount   decimal.Decimal
	)

	if minTransfer.LessThan(retainedBalance) {
		return nil, fmt.Errorf("mini transfer amount must be greater than address retained balance")
	}

	address, err := wrapper.GetAddressList(sumRawTx.AddressStartIndex, sumRawTx.AddressLimit, "AccountID", sumRawTx.Account.AccountID)
	if err != nil {
		return nil, err
	}

	if len(address) == 0 {
		return nil, fmt.Errorf("[%s] have not addresses", accountID)
	}

	searchAddrs := make([]string, 0)
	for _, address := range address {
		searchAddrs = append(searchAddrs, address.Address)
	}

	addrBalanceArray, err := decoder.wm.GetBlockScanner().GetBalanceByAddress(searchAddrs...)
	if err != nil {
		return nil, err
	}

	for _, addrBalance := range addrBalanceArray {
		//decoder.wm.Log.Debugf("addrBalance: %+v", addrBalance)
		//检查余额是否超过最低转账
		addrBalance_dec, _ := decimal.NewFromString(addrBalance.Balance)
		if addrBalance_dec.GreaterThanOrEqual(minTransfer) {
			//添加到转账地址数组
			sumAddresses = append(sumAddresses, addrBalance.Address)
		}
	}

	if len(sumAddresses) == 0 {
		return nil, nil
	}

	//取得费率
	if len(sumRawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateFeeRate()
		if err != nil {
			return nil, err
		}
	} else {
		feesRate, _ = decimal.NewFromString(sumRawTx.FeeRate)
	}

	sumUnspents = make([]*UtxoDto, 0)
	outputAddrs = make(map[string]decimal.Decimal, 0)
	totalInputAmount = decimal.Zero

	for i, addr := range sumAddresses {

		unspents, err := decoder.wm.Api.GetUnSpent(addr)
		if err != nil {
			continue
		}

		//尽可能筹够最大input数
		if len(unspents)+len(sumUnspents) < decoder.wm.Config.MaxTxInputs {

			for _, u := range unspents {
				sumUnspents = append(sumUnspents, u)
				if retainedBalance.GreaterThan(decimal.Zero) {
					//outputAddrs[addr] = retainedBalance.StringFixed(decoder.wm.Decimal())
					outputAddrs = appendOutput(outputAddrs, addr, retainedBalance)
				}
			}

			//decoder.wm.Log.Debugf("sumUnspents: %+v", sumUnspents)
		}

		//如果utxo已经超过最大输入，或遍历地址完结，就可以进行构建交易单
		if i == len(sumAddresses)-1 || len(sumUnspents) >= decoder.wm.Config.MaxTxInputs {
			//执行构建交易单工作
			//decoder.wm.Log.Debugf("sumUnspents: %+v", sumUnspents)
			//计算手续费，构建交易单inputs，地址保留余额>0，地址需要加入输出，最后+1是汇总地址
			fees, createErr := decoder.wm.EstimateFee(int64(len(sumUnspents)), int64(len(outputAddrs)+1), "", feesRate)
			if createErr != nil {
				return nil, createErr
			}

			//计算这笔交易单的汇总数量
			for _, u := range sumUnspents {

				ua := decimal.New(u.Value, 0)
				ua = ua.Shift(-decoder.wm.Decimal())
				totalInputAmount = totalInputAmount.Add(ua)

			}

			/*
					汇总数量计算：
					1. 输入总数量 = 合计账户地址的所有utxo
					2. 账户地址输出总数量 = 账户地址保留余额 * 地址数
				    3. 汇总数量 = 输入总数量 - 账户地址输出总数量 - 手续费
			*/
			retainedBalanceTotal := retainedBalance.Mul(decimal.New(int64(len(outputAddrs)), 0))
			sumAmount := totalInputAmount.Sub(retainedBalanceTotal).Sub(fees)

			decoder.wm.Log.Debugf("totalInputAmount: %v", totalInputAmount)
			decoder.wm.Log.Debugf("retainedBalanceTotal: %v", retainedBalanceTotal)
			decoder.wm.Log.Debugf("fees: %v", fees)
			decoder.wm.Log.Debugf("sumAmount: %v", sumAmount)

			//最后填充汇总地址及汇总数量
			outputAddrs = appendOutput(outputAddrs, sumRawTx.SummaryAddress, sumAmount)
			//outputAddrs[sumRawTx.SummaryAddress] = sumAmount.StringFixed(decoder.wm.Decimal())

			raxTxTo := make(map[string]string, 0)
			for a, m := range outputAddrs {
				raxTxTo[a] = m.StringFixed(decoder.wm.Decimal())
			}

			//创建一笔交易单
			rawTx := &openwallet.RawTransaction{
				Coin:     sumRawTx.Coin,
				Account:  sumRawTx.Account,
				FeeRate:  sumRawTx.FeeRate,
				To:       raxTxTo,
				Fees:     fees.StringFixed(decoder.wm.Decimal()),
				Required: 1,
			}

			createErr = decoder.createSimpleRawTransaction(wrapper, rawTx, sumUnspents, outputAddrs)

			rawTxWithErr := &openwallet.RawTransactionWithError{
				RawTx: rawTx,
				Error: openwallet.ConvertError(createErr),
			}

			//创建成功，添加到队列
			rawTxArray = append(rawTxArray, rawTxWithErr)

			//创建成功，添加到队列
			//rawTxArray = append(rawTxArray, rawTx)

			//清空临时变量
			sumUnspents = make([]*UtxoDto, 0)
			outputAddrs = make(map[string]decimal.Decimal, 0)
			totalInputAmount = decimal.Zero

		}
	}

	return rawTxArray, nil
}

//CreateTokenSummaryRawTransaction 创建代币汇总交易
func (decoder *TransactionDecoder) CreateTokenSummaryRawTransaction(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {

	var (
		rawTxArray         = make([]*openwallet.RawTransactionWithError, 0)
		accountID          = sumRawTx.Account.AccountID
		feesSupportAccount *openwallet.AssetsAccount
		supportFessAccount decimal.Decimal
		fixSupportAmount   decimal.Decimal
	)

	tokenDecimals := int32(sumRawTx.Coin.Contract.Decimals)
	minTransfer := common.StringNumToBigIntWithExp(sumRawTx.MinTransfer, tokenDecimals)
	retainedBalance := common.StringNumToBigIntWithExp(sumRawTx.RetainedBalance, tokenDecimals)

	// 如果有提供手续费账户，检查账户是否存在
	if feesAcount := sumRawTx.FeesSupportAccount; feesAcount != nil {
		var err error
		fixSupportAmount, err = decimal.NewFromString(feesAcount.FixSupportAmount)
		if err != nil {
			return nil, openwallet.Errorf(openwallet.ErrAccountNotFound, "fixSupportAmount can't be nil")
		}
		account, supportErr := wrapper.GetAssetsAccountInfo(feesAcount.AccountID)
		if supportErr != nil {
			return nil, openwallet.Errorf(openwallet.ErrAccountNotFound, "can not find fees support account")
		}

		feesSupportAccount = account

		//查询主币余额是否足够
		feesAddress, err := wrapper.GetAddressList(0, -1, "AccountID", feesSupportAccount.AccountID)
		if err != nil {
			return nil, openwallet.Errorf(openwallet.ErrAccountNotFound, "can not find fees support address")
		}

		searchAddrs := make([]string, 0)
		for _, address := range feesAddress {
			searchAddrs = append(searchAddrs, address.Address)
		}

		//查询主币余额是否足够
		feeBalance, createErr := decoder.wm.Blockscanner.GetBalanceByAddress(searchAddrs...)
		if createErr != nil {
			return nil, createErr
		}

		if feeBalance != nil && len(feeBalance) > 0 {
			for _, f := range feeBalance {
				b, _ := decimal.NewFromString(f.Balance)
				supportFessAccount = supportFessAccount.Add(b)
			}
		}
	}

	if minTransfer.Cmp(retainedBalance) < 0 {
		return nil, fmt.Errorf("mini transfer amount must be greater than address retained balance")
	}

	//获取wallet
	addresses, err := wrapper.GetAddressList(sumRawTx.AddressStartIndex, sumRawTx.AddressLimit,
		"AccountID", sumRawTx.Account.AccountID)
	if err != nil {
		return nil, err
	}

	if len(addresses) == 0 {
		return nil, fmt.Errorf("[%s] have not addresses", accountID)
	}

	searchAddrs := make([]string, 0)
	for _, address := range addresses {
		searchAddrs = append(searchAddrs, address.Address)
	}

	addrBalanceArray, err := decoder.wm.ContractDecoder.GetTokenBalanceByAddress(sumRawTx.Coin.Contract, searchAddrs...)
	if err != nil {
		return nil, err
	}

	toMap := make(map[string]string)
	for _, addrBalance := range addrBalanceArray {

		//trcBalance := big.NewInt(0)

		//检查余额是否超过最低转账
		addrBalance_BI := common.StringNumToBigIntWithExp(addrBalance.Balance.Balance, tokenDecimals)

		if addrBalance_BI.Cmp(minTransfer) < 0 || addrBalance_BI.Cmp(big.NewInt(0)) <= 0 {
			continue
		}
		//计算汇总数量 = 余额 - 保留余额
		sumAmount_BI := new(big.Int)
		sumAmount_BI.Sub(addrBalance_BI, retainedBalance)
		sumAmount := common.BigIntToDecimals(sumAmount_BI, tokenDecimals)

		//计算token手续费
		fee, createErr := decoder.wm.EstimateTokenFeeRate()
		if createErr != nil {
			decoder.wm.Log.Std.Error("GetTransactionFeeEstimated from[%v] -> to[%v] failed, err=%v", addrBalance.Balance.Address, sumRawTx.SummaryAddress, createErr)
			return nil, createErr
		}

		//计算手续费
		feeMain, createErr := decoder.wm.EstimateFeeRate()
		if createErr != nil {
			decoder.wm.Log.Std.Error("GetTransactionFeeEstimated from[%v] -> to[%v] failed, err=%v", addrBalance.Balance.Address, sumRawTx.SummaryAddress, createErr)
			return nil, createErr
		}

		//查询主币余额是否足够
		addrNulsBalanceArray, createErr := decoder.wm.Blockscanner.GetBalanceByAddress(addrBalance.Balance.Address)
		if createErr != nil {
			return nil, createErr
		}
		if len(addrNulsBalanceArray) > 0 {
			nulsBalance, _ := decimal.NewFromString(addrNulsBalanceArray[0].Balance)
			//不够手续费，要充
			if nulsBalance.LessThan(fee) {
				totalMainFee := fixSupportAmount.Add(feeMain)
				if supportFessAccount.LessThan(totalMainFee) {
					decoder.wm.Log.Std.Error(" fees support address have not enough money,from[%v] -> to[%v] ,supportFessAccount[%v],fixSupportAmount[%v] failed", addrBalance.Balance.Address, sumRawTx.SummaryAddress, supportFessAccount, fixSupportAmount)
					continue
				} else {
					decoder.wm.Log.Info("send fee Amount: %v", fixSupportAmount)
					decoder.wm.Log.Info("fees: %v", feeMain)

					sumRawTx.Coin.IsContract = false
					toMap[addrBalance.Balance.Address] = fixSupportAmount.Truncate(decoder.wm.Decimal()).String()
					supportFessAccount = supportFessAccount.Sub(totalMainFee)
					//最后一个,或者差1个到50
					if len(toMap) == decoder.wm.Config.MaxTxInputs {
						//创建一笔交易单
						coinTemp := sumRawTx.Coin
						coinTemp.IsContract = false
						rawTx := &openwallet.RawTransaction{
							Coin:     coinTemp,
							Account:  feesSupportAccount,
							To:       toMap,
							Required: 1,
						}

						createTxErr := decoder.CreateSimpleRawTransaction(
							wrapper,
							rawTx)
						rawTxWithErr := &openwallet.RawTransactionWithError{
							RawTx: rawTx,
							Error: openwallet.ConvertError(createTxErr),
						}

						//创建成功，添加到队列
						rawTxArray = append(rawTxArray, rawTxWithErr)

						//清理toMap
						toMap = make(map[string]string)
						return rawTxArray, nil
					}

					continue

				}
			}
		}

		decoder.wm.Log.Debugf("balance: %v", addrBalance.Balance.Balance)
		decoder.wm.Log.Debugf("fees: %v", fee)
		decoder.wm.Log.Debugf("sumAmount: %v", sumAmount)
		coinTemp := sumRawTx.Coin
		coinTemp.IsContract = true
		//创建一笔交易单
		rawTx := &openwallet.RawTransaction{
			Coin:    coinTemp,
			Account: sumRawTx.Account,
			To: map[string]string{
				sumRawTx.SummaryAddress: sumAmount.StringFixed(int32(tokenDecimals)),
			},
			Required: 1,
		}

		createTxErr := decoder.CreateNrc20RawTransaction(
			wrapper,
			rawTx,
			addrBalance.Balance.Address, )
		rawTxWithErr := &openwallet.RawTransactionWithError{
			RawTx: rawTx,
			Error: openwallet.ConvertError(createTxErr),
		}

		//创建成功，添加到队列
		rawTxArray = append(rawTxArray, rawTxWithErr)

	}

	//如果已经有toMap先组装
	if len(toMap) > 0 {

		decoder.wm.Log.Debugf("have %v trans be send the fee", len(toMap))
		//创建一笔交易单
		coinTemp := sumRawTx.Coin
		coinTemp.IsContract = false
		rawTx := &openwallet.RawTransaction{
			Coin:     coinTemp,
			Account:  feesSupportAccount,
			To:       toMap,
			Required: 1,
		}

		createTxErr := decoder.CreateSimpleRawTransaction(
			wrapper,
			rawTx)
		rawTxWithErr := &openwallet.RawTransactionWithError{
			RawTx: rawTx,
			Error: openwallet.ConvertError(createTxErr),
		}

		//创建成功，添加到队列
		rawTxArray = append(rawTxArray, rawTxWithErr)

		//清理toMap
		toMap = make(map[string]string)
	}

	return rawTxArray, nil
}

//SignRawTransaction 签名交易单
func (decoder *TransactionDecoder) SignRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {

	if rawTx.Signatures == nil || len(rawTx.Signatures) == 0 {
		//this.wm.Log.Std.Error("len of signatures error. ")
		return fmt.Errorf("transaction signature is empty")
	}

	key, err := wrapper.HDKey()
	if err != nil {
		return err
	}

	keySignatures := rawTx.Signatures[rawTx.Account.AccountID]
	if keySignatures != nil {
		for _, keySignature := range keySignatures {

			childKey, err := key.DerivedKeyWithPath(keySignature.Address.HDPath, keySignature.EccType)
			keyBytes, err := childKey.GetPrivateKeyBytes()
			txHash,err := hex.DecodeString(keySignature.Message)
			if err != nil {
				return err
			}

			//签名交易
			/////////交易单哈希签名
			signature, err := nulsio_trans.SignTransactionMessage(txHash, keyBytes)
			if err != nil {
				return fmt.Errorf("transaction hash sign failed, unexpected error: %v", err)
			} else {

			}

			keySignature.Signature = hex.EncodeToString(signature)
		}
	}


	rawTx.Signatures[rawTx.Account.AccountID] = keySignatures


	return nil
}

//VerifyRawTransaction 验证交易单，验证交易单并返回加入签名后的交易单
func (decoder *TransactionDecoder) VerifyRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {


	rawHex, err := hex.DecodeString(rawTx.RawHex)
	if err != nil {
		return err
	}

	if rawTx.Signatures == nil || len(rawTx.Signatures) == 0 {
		//this.wm.Log.Std.Error("len of signatures error. ")
		return fmt.Errorf("transaction signature is empty")
	}

	sigPubByte := make([]byte, 0)

	for accountID, keySignatures := range rawTx.Signatures {
		decoder.wm.Log.Debug("accountID Signatures:", accountID)
		for _, keySignature := range keySignatures {

			signature, _ := hex.DecodeString(keySignature.Signature)
			pub,_ := hex.DecodeString(keySignature.Address.PublicKey)
			pub = owcrypt.PointCompress(pub, owcrypt.ECC_CURVE_SECP256K1)

			sigPub := &nulsio_trans.SigPub{
				pub,
				signature,
			}


			result := make([]byte, 0)
			result = append(result, byte(len(pub)))
			result = append(result, pub...)

			result = append(result, 0)
			resultSig := make([]byte, 0)
			resultSig = append(resultSig, sigPub.ToBytes()...)

			result = append(result, resultSig...)


			sigPubByte = append(sigPubByte, result...)
		}
	}

	sigPubByte, _ = nulsio_trans.GetBytesWithLength(sigPubByte)

	rawBytes := make([]byte, 0)
	rawBytes = append(rawBytes, rawHex...)
	rawBytes = append(rawBytes, sigPubByte...)
	rawTx.IsCompleted = true
	rawTx.RawHex = hex.EncodeToString(rawBytes)

	_, err = decoder.wm.Api.VaildTransaction(rawTx.RawHex)
	if err != nil {
		return err
	}

	return nil
}

//SendRawTransaction 广播交易单
func (decoder *TransactionDecoder) SubmitRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) (*openwallet.Transaction, error) {

	//先加载是否有配置文件
	//err := decoder.wm.loadConfig()
	//if err != nil {
	//	return err
	//}

	if len(rawTx.RawHex) == 0 {
		return nil, fmt.Errorf("transaction hex is empty")
	}

	if !rawTx.IsCompleted {
		return nil, fmt.Errorf("transaction is not completed validation")
	}

	txId, err := decoder.wm.Api.SendRawTransaction(rawTx.RawHex)
	if err != nil {
		return nil, err
	}
	rawTx.TxID = txId

	decimals := int32(0)
	//fees := "0"
	//if rawTx.Coin.IsContract {
	//	decimals = int32(rawTx.Coin.Contract.Decimals)
	//	fees = "0"
	//} else {
	//	decimals = int32(decoder.wm.Decimal())
	fees := rawTx.Fees
	//}

	//rawTx.TxID = txid
	rawTx.IsSubmit = true

	//记录一个交易单
	tx := &openwallet.Transaction{
		From:       rawTx.TxFrom,
		To:         rawTx.TxTo,
		Amount:     rawTx.TxAmount,
		Coin:       rawTx.Coin,
		TxID:       rawTx.TxID,
		Decimal:    decimals,
		AccountID:  rawTx.Account.AccountID,
		Fees:       fees,
		SubmitTime: time.Now().Unix(),
	}

	tx.WxID = openwallet.GenTransactionWxID(tx)

	return tx, nil
}

//GetRawTransactionFeeRate 获取交易单的费率
func (decoder *TransactionDecoder) GetRawTransactionFeeRate() (feeRate string, unit string, err error) {
	rate, err := decoder.wm.EstimateFeeRate()
	if err != nil {
		return "", "", err
	}

	return rate.StringFixed(decoder.wm.Decimal()), "K", nil
}

//createSimpleRawTransaction 创建原始交易单
func (decoder *TransactionDecoder) createSimpleRawTransaction(
	wrapper openwallet.WalletDAI,
	rawTx *openwallet.RawTransaction,
	usedUTXO []*UtxoDto,
	to map[string]decimal.Decimal,
) error {

	var (
		err   error
		vins  = make([]nulsio_trans.Vin, 0)
		vouts = make([]nulsio_trans.Vout, 0)
		//txUnlocks        = make([]nulsio_trans.TxUnlock, 0)
		totalSend        = decimal.New(0, 0)
		destinations     = make([]string, 0)
		accountTotalSent = decimal.Zero
		txFrom           = make([]string, 0)
		txTo             = make([]string, 0)
		accountID        = rawTx.Account.AccountID
	)

	if len(usedUTXO) == 0 {
		return fmt.Errorf("utxo is empty")
	}

	if len(to) == 0 {
		return fmt.Errorf("Receiver addresses is empty! ")
	}

	//计算总发送金额
	for addr, deamount := range to {
		//deamount, _ := decimal.NewFromString(amount)
		totalSend = totalSend.Add(deamount)
		destinations = append(destinations, addr)
		//计算账户的实际转账amount
		addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", accountID, "Address", addr)
		if findErr != nil || len(addresses) == 0 {
			//amountDec, _ := decimal.NewFromString(amount)
			accountTotalSent = accountTotalSent.Add(deamount)
		}
	}

	////UTXO如果大于设定限制，则分拆成多笔交易单发送
	//if len(usedUTXO) > decoder.wm.Config.maxTxInputs {
	//	errStr := fmt.Sprintf("The transaction is use max inputs over: %d", decoder.wm.config.maxTxInputs)
	//	return errors.New(errStr)
	//}

	addressMap := make(map[string]string)

	//装配输入
	for _, utxo := range usedUTXO {
		in := nulsio_trans.Vin{utxo.TxHash, uint32(utxo.TxIndex), uint64(utxo.Value), uint64(utxo.LockTime)}
		vins = append(vins, in)
		addressMap[utxo.Address] = utxo.Address
		//scriptPubkey, err := nulsio_addrdec.GetInputOwnerKey(utxo.TxHash, int64(utxo.TxIndex))
		//if err != nil {
		//	return fmt.Errorf("utxo.TxHash can't decode, unexpected error: %v", err)
		//}
		//txUnlock := nulsio_trans.TxUnlock{LockScript: hex.EncodeToString(scriptPubkey), Address: utxo.Address}
		//txUnlocks = append(txUnlocks, txUnlock)

		txFrom = append(txFrom, fmt.Sprintf("%s:%x", utxo.Address, utxo.Value))
	}

	//装配输入
	for to, amount := range to {
		//deamount, _ := decimal.NewFromString(amount)
		amount = amount.Shift(decoder.wm.Decimal())
		out := nulsio_trans.Vout{to, uint64(amount.IntPart()), 0}
		vouts = append(vouts, out)

		txTo = append(txTo, fmt.Sprintf("%s:%s", to, amount))
	}

	//锁定时间
	lockTime := uint32(0)

	//追加手续费支持
	replaceable := false

	/////////构建空交易单
	signTrans, _, err := nulsio_trans.CreateEmptyRawTransaction(vins, vouts, "", lockTime, replaceable, nil)

	if err != nil {
		return fmt.Errorf("create transaction failed, unexpected error: %v", err)
		//decoder.wm.Log.Error("构建空交易单失败")
	}

	////////构建用于签名的交易单哈希
	//transHash, err := nulsio_trans.CreateRawTransactionHashForSig(emptyTrans, txUnlocks)
	//if err != nil {
	//	return fmt.Errorf("create transaction hash for sig failed, unexpected error: %v", err)
	//	//decoder.wm.Log.Error("获取待签名交易单哈希失败")
	//}

	rawTx.RawHex = signTrans

	if rawTx.Signatures == nil {
		rawTx.Signatures = make(map[string][]*openwallet.KeySignature)
	}

	//装配签名
	keySigs := make([]*openwallet.KeySignature, 0)

	//按照地址来
	for i, _ := range addressMap {

		beSignHex := signTrans

		addr, err := wrapper.GetAddress(i)
		if err != nil {
			return err
		}

		signature := openwallet.KeySignature{
			EccType: decoder.wm.Config.CurveType,
			Nonce:   "",
			Address: addr,
			Message: beSignHex,
		}

		keySigs = append(keySigs, &signature)

	}

	feesDec, _ := decimal.NewFromString(rawTx.Fees)
	accountTotalSent = accountTotalSent.Add(feesDec)
	accountTotalSent = decimal.Zero.Sub(accountTotalSent)

	rawTx.Signatures[rawTx.Account.AccountID] = keySigs
	rawTx.IsBuilt = true
	rawTx.TxAmount = accountTotalSent.StringFixed(decoder.wm.Decimal())
	rawTx.TxFrom = txFrom
	rawTx.TxTo = txTo

	return nil
}

//createSimpleRawTransaction 创建原始交易单
func (decoder *TransactionDecoder) createSimpleNrc20RawTransaction(
	wrapper openwallet.WalletDAI,
	rawTx *openwallet.RawTransaction,
	usedUTXO []*UtxoDto,
	to map[string]decimal.Decimal,
	token *nulsio_trans.TxToken,
) error {

	var (
		err   error
		vins  = make([]nulsio_trans.Vin, 0)
		vouts = make([]nulsio_trans.Vout, 0)
		//txUnlocks        = make([]nulsio_trans.TxUnlock, 0)
		totalSend        = decimal.New(0, 0)
		destinations     = make([]string, 0)
		accountTotalSent = decimal.Zero
		txFrom           = make([]string, 0)
		txTo             = make([]string, 0)
		accountID        = rawTx.Account.AccountID
	)

	if len(usedUTXO) == 0 {
		return fmt.Errorf("utxo is empty")
	}

	if len(to) == 0 {
		return fmt.Errorf("Receiver addresses is empty! ")
	}

	//计算总发送金额
	for addr, deamount := range to {
		totalSend = totalSend.Add(deamount)
		destinations = append(destinations, addr)
		//计算账户的实际转账amount
		addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", accountID, "Address", addr)
		if findErr != nil || len(addresses) == 0 {
			accountTotalSent = accountTotalSent.Add(deamount)
		}
	}

	addressMap := make(map[string]string)
	addressMap[token.Sender] = token.Sender
	//装配输入
	for _, utxo := range usedUTXO {
		in := nulsio_trans.Vin{utxo.TxHash, uint32(utxo.TxIndex), uint64(utxo.Value), uint64(utxo.LockTime)}
		vins = append(vins, in)
		addressMap[utxo.Address] = utxo.Address

		txFrom = append(txFrom, fmt.Sprintf("%s:%x", utxo.Address, utxo.Value))
	}

	//装配输入
	for to, amount := range to {
		amount = amount.Shift(decoder.wm.Decimal())
		out := nulsio_trans.Vout{to, uint64(amount.IntPart()), 0}
		vouts = append(vouts, out)

		txTo = append(txTo, fmt.Sprintf("%s:%s", to, amount))
	}

	//锁定时间
	lockTime := uint32(0)

	//追加手续费支持
	replaceable := false

	/////////构建空交易单
	signTrans, _, err := nulsio_trans.CreateEmptyRawTransaction(vins, vouts, "", lockTime, replaceable, token)

	if err != nil {
		return fmt.Errorf("create transaction failed, unexpected error: %v", err)
		//decoder.wm.Log.Error("构建空交易单失败")
	}

	rawTx.RawHex = signTrans

	if rawTx.Signatures == nil {
		rawTx.Signatures = make(map[string][]*openwallet.KeySignature)
	}

	//装配签名
	keySigs := make([]*openwallet.KeySignature, 0)

	//按照地址来
	for i, _ := range addressMap {

		beSignHex := signTrans
		beSignHexHex, err := hex.DecodeString(beSignHex)
		if err != nil {
			return err
		}

		beSignHexHex = nulsio_trans.Sha256Twice(beSignHexHex) //sha256


		addr, err := wrapper.GetAddress(i)
		if err != nil {
			return err
		}

		signature := openwallet.KeySignature{
			EccType: decoder.wm.Config.CurveType,
			Nonce:   "",
			Address: addr,
			Message: hex.EncodeToString(beSignHexHex),
		}

		keySigs = append(keySigs, &signature)

	}

	feesDec, _ := decimal.NewFromString(rawTx.Fees)
	accountTotalSent = accountTotalSent.Add(feesDec)
	accountTotalSent = decimal.Zero.Sub(accountTotalSent)

	rawTx.Signatures[rawTx.Account.AccountID] = keySigs
	rawTx.IsBuilt = true
	rawTx.TxAmount = accountTotalSent.StringFixed(decoder.wm.Decimal())
	rawTx.TxFrom = txFrom
	rawTx.TxTo = txTo

	return nil
}

//CreateSummaryRawTransactionWithError 创建汇总交易，返回能原始交易单数组（包含带错误的原始交易单）
//func (decoder *TransactionDecoder) CreateSummaryRawTransactionWithError(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {
//	raTxWithErr := make([]*openwallet.RawTransactionWithError, 0)
//	rawTxs, err := decoder.CreateSummaryRawTransaction(wrapper, sumRawTx)
//	if err != nil {
//		return nil, err
//	}
//	for _, tx := range rawTxs {
//		raTxWithErr = append(raTxWithErr, &openwallet.RawTransactionWithError{
//			RawTx: tx,
//			Error: nil,
//		})
//	}
//	return raTxWithErr, nil
//}

func appendOutput(output map[string]decimal.Decimal, address string, amount decimal.Decimal) map[string]decimal.Decimal {
	if origin, ok := output[address]; ok {
		origin = origin.Add(amount)
		output[address] = origin
	} else {
		output[address] = amount
	}
	return output
}
