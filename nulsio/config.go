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
	"github.com/blocktree/go-owcrypt"
	"github.com/blocktree/openwallet/common/file"
	"path/filepath"
	"strings"
)

const (
	//币种
	Symbol    = "NULS"
	CurveType = owcrypt.ECC_CURVE_SECP256K1

	//默认配置内容
	defaultConfig = `

# RPC api url
serverAPI = ""

`
)

type WalletConfig struct {
	//币种
	Symbol string
	//配置文件路径
	configFilePath string
	//配置文件名
	configFileName string
	//区块链数据文件
	BlockchainFile string
	//本地数据库文件路径
	dbPath string
	//钱包服务API
	ServerAPI string
	//默认配置内容
	DefaultConfig string
	//曲线类型
	CurveType uint32
	//链ID
	//ChainID uint64

	ChainId     string //链ID
	MaxTxInputs int

	DataDir string
}

func NewConfig(symbol string) *WalletConfig {

	c := WalletConfig{}

	//币种
	c.Symbol = symbol
	c.CurveType = CurveType
	c.MaxTxInputs = 50
	//区块链数据
	//blockchainDir = filepath.Join("data", strings.ToLower(Symbol), "blockchain")
	//配置文件路径
	c.configFilePath = filepath.Join("conf")
	//配置文件名
	c.configFileName = c.Symbol + ".ini"
	//区块链数据文件
	c.BlockchainFile = "blockchain.db"
	//本地数据库文件路径
	c.dbPath = filepath.Join("data", strings.ToLower(c.Symbol), "db")
	//钱包服务API
	c.ServerAPI = ""

	//创建目录
	file.MkdirAll(c.dbPath)

	return &c
}

func (wc *WalletConfig) makeDataDir() {

	if len(wc.DataDir) == 0 {
		//默认路径当前文件夹./data
		wc.DataDir = "data"
	}

	//本地数据库文件路径
	wc.dbPath = filepath.Join(wc.DataDir, strings.ToLower(wc.Symbol), "db")

	//创建目录
	file.MkdirAll(wc.dbPath)
}
