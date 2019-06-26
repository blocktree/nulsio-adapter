/*
 * Copyright 2018 The OpenWallet Authors
 * This file is part of the OpenWallet library.
 *
 * The OpenWallet library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The OpenWallet library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 */

package nulsio

import (
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
)

type WalletManager struct {
	openwallet.AssetsAdapterBase

	Api             *Client                         // 节点客户端
	Config          *WalletConfig                   // 节点配置
	Decoder         openwallet.AddressDecoder       //地址编码器
	TxDecoder       openwallet.TransactionDecoder   //交易单编码器
	Log             *log.OWLogger                   //日志工具
	ContractDecoder openwallet.SmartContractDecoder //智能合约解析器
	Blockscanner    *NULSBlockScanner               //区块扫描器
	CacheManager    openwallet.ICacheManager        //缓存管理器
}

func NewWalletManager() *WalletManager {
	wm := WalletManager{}
	wm.Config = NewConfig(Symbol)
	wm.Api = new(Client)
	wm.Blockscanner = NewNULSBlockScanner(&wm)
	wm.Decoder = NewAddressDecoder(&wm)
	wm.TxDecoder = NewTransactionDecoder(&wm)
	wm.Log = log.NewOWLogger(wm.Symbol())
	wm.ContractDecoder = NewContractDecoder(&wm)
	return &wm
}

//EstimateFee 预估手续费
func (wm *WalletManager) EstimateFee(inputs, outputs int64, remark string, feeRate decimal.Decimal) (decimal.Decimal, error) {

	var base int64 = 124

	// int size = 124 + 50 * inputs.size() + 38 * outputs.size() + remark.getBytes().length;
	trx_bytes := decimal.New(inputs*50+outputs*38+base+int64(len([]byte(remark))), 0)
	trx_fee := trx_bytes.Div(decimal.New(1000, 0)).Mul(feeRate)
	trx_fee = trx_fee.Round(wm.Decimal())
	trx_fee, _ = decimal.NewFromString("0.005") //暂时默认啦
	return trx_fee, nil
}

//EstimateFeeRate 预估的没KB手续费率
func (wm *WalletManager) EstimateFeeRate() (decimal.Decimal, error) {

	rate, _ := decimal.NewFromString("0.005")
	return rate, nil
}

//EstimateFeeRate 预估的没KB手续费率
func (wm *WalletManager) EstimateTokenFeeRate() (decimal.Decimal, error) {

	rate, _ := decimal.NewFromString("0.007")
	return rate, nil
}
