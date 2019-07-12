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
package nulsio

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blocktree/openwallet/log"
	"github.com/imroc/req"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
	"strconv"
)

type Client struct {
	BaseURL string
	Debug   bool
}

type Response struct {
	Id      int         `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

func (this *Client) GetUnSpent(address string) ([]*UtxoDto, error) {
	//result, err := this.CallReq("/api/utxo/limit/" + address + "/0")
	//if err != nil {
	//	log.Errorf("get block number faield, err = %v \n", err)
	//	return nil, err
	//}
	//
	//if result.Type != gjson.JSON {
	//	log.Errorf("result of block number type error")
	//	return nil, errors.New("result of block number type error")
	//}
	//
	//log.Warn("result.Str:", result.Raw)
	//
	//dataStr := gjson.Get(result.Raw, "utxoDtoList").Raw
	//
	//if dataStr == "" {
	//	return nil, errors.New("utxoDtoList is nil")
	//}
	//var utxoDtoList []*UtxoDto
	//err = json.Unmarshal([]byte(dataStr), &utxoDtoList)
	//if err != nil {
	//	log.Errorf("GetBalance decode json [%v] failed, err=%v", []byte(result.Raw), err)
	//	return nil, err
	//}
	//if utxoDtoList != nil && len(utxoDtoList) > 0 {
	//	for _, v := range utxoDtoList {
	//		v.Address = address
	//	}
	//}

	params := []interface{}{
		address,
		10000000000000000, //最大值
	}
	result, err := this.Call("getUTXO", 1, params)
	if err != nil {
		log.Errorf("get block number faield, err = %v \n", err)
		return nil, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of block number type error")
		return nil, errors.New("result of block number type error")
	}

	var utxoDtoList []*UtxoDto
	err = json.Unmarshal([]byte(result.Raw), &utxoDtoList)
	if err != nil {
		log.Errorf("GetBalance decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}
	height, err := this.GetNewHeight()
	if err != nil {
		log.Errorf("GetBalance  GetNewHeight failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}
	utxoDtoListResult := make([]*UtxoDto, 0)
	if utxoDtoList != nil && len(utxoDtoList) > 0 {
		for _, v := range utxoDtoList {
			v.Address = address
			if v.LockTime < height {
				utxoDtoListResult = append(utxoDtoListResult, v)
			}
		}
	}

	return utxoDtoListResult, nil
}
func (this *Client) GetAddressBalance(address string) (*NulsBalance, error) {
	params := []interface{}{
		address,
	}
	nulsBalance := &NulsBalance{
		Balance:       decimal.Zero,
		UnLockBalance: decimal.Zero,
	}
	result, err := this.Call("getAccount", 1, params)
	if err != nil {
		//log.Errorf("get block number faield, err = %v \n", err)
		return nulsBalance, nil
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of block number type error")
		return nulsBalance, errors.New("result of block number type error")
	}

	if result.Get("balance").Exists() {
		nulsBalance.UnLockBalance = decimal.New(result.Get("balance").Int(), 0)
	}
	if result.Get("totalBalance").Exists() {
		nulsBalance.Balance = decimal.New(result.Get("totalBalance").Int(), 0)
	}

	return nulsBalance, nil
}

//获取最新高度
func (this *Client) GetNewHeight() (int64, error) {
	result, err := this.CallReq("/api/block/newest/height")
	if err != nil {
		log.Errorf("get GetNewHeight faield, err = %v \n", err)
		return 0, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetNewHeight type error")
		return 0, errors.New("result of GetNewBlock type error")
	}

	return result.Get("value").Int(), nil
}

//获取最新高度
func (this *Client) GetTokenBalances(contractAddress, address string) (decimal.Decimal, error) {
	balance := decimal.Zero
	target := "/api/contract/balance/token/" + contractAddress + "/" + address
	result, err := this.CallReq(target)
	if err != nil {
		log.Errorf("get GetNewHeight faield, err = %v \n", err)
		return balance, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetNewHeight type error")
		return balance, errors.New("result of GetNewBlock type error")
	}

	var tokenBalance *TokenBalance
	err = json.Unmarshal([]byte(result.Raw), &tokenBalance)
	if err != nil {
		log.Errorf("GetBalance decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return balance, err
	}

	if tokenBalance == nil {
		return balance, errors.New("token balances is zero.")
	}

	balanceStr, err := decimal.NewFromString(tokenBalance.Amount)
	if err != nil {
		return balance, errors.New("token balanceStr is zero:" + err.Error())
	}

	balance = balanceStr

	return balance, nil
}

//获取最新高度区块信息
func (this *Client) GetNewBlock() (*NusBlock, error) {
	result, err := this.CallReq("/api/block/newest")
	if err != nil {
		log.Errorf("get GetNewBlock faield, err = %v \n", err)
		return nil, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetNewBlock type error")
		return nil, errors.New("result of GetNewBlock type error")
	}
	var nusBlock *NusBlock
	err = json.Unmarshal([]byte(result.Raw), &nusBlock)
	if err != nil {
		log.Errorf("GetNewBlock decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}

	return nusBlock, nil
}

//通过高度获取区块
func (this *Client) GetBlockByHeight(height int64) (*NusBlock, error) {
	result, err := this.CallReq("/api/block/height/" + strconv.FormatInt(height, 10))
	if err != nil {
		log.Errorf("GetBlockByHeight  faield, err = %v \n", err)
		return nil, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetBlockByHeight type error")
		return nil, errors.New("result of GetBlockByHeight type error")
	}

	var nusBlock *NusBlock
	err = json.Unmarshal([]byte(result.Raw), &nusBlock)
	if err != nil {
		log.Errorf("GetBlockByHeight decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}

	return nusBlock, nil
}

//通过hash获取区块
func (this *Client) GetBlockByHash(hash string) (*NusBlock, error) {
	result, err := this.CallReq("/api/block/hash/" + hash)
	if err != nil {
		log.Errorf("GetBlockByHash  faield, err = %v \n", err)
		return nil, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetBlockByHash type error")
		return nil, errors.New("result of GetBlockByHeight type error")
	}

	var nusBlock *NusBlock
	err = json.Unmarshal([]byte(result.Raw), &nusBlock)
	if err != nil {
		log.Errorf("GetBlockByHash decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}

	return nusBlock, nil
}

//通过tx获取交易
func (this *Client) GetTxByTxId(txId string) (*Tx, error) {
	result, err := this.CallReq("/api/tx/hash/" + txId)
	if err != nil {
		log.Errorf("GetBlockByHash  faield, err = %v \n", err)
		return nil, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetBlockByHash type error")
		return nil, errors.New("result of GetBlockByHeight type error")
	}

	var tx *Tx
	err = json.Unmarshal([]byte(result.Raw), &tx)
	if err != nil {
		log.Errorf("GetBlockByHash decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}

	return tx, nil
}

//通过tx获取合约
func (this *Client) GetTokenByHash(hash string) ([]*NulsToken, error) {
	result, err := this.CallReq("/api/contract/result/" + hash)
	if err != nil {
		log.Errorf("GetTokenByHash  faield, err = %v \n", err)
		return nil, err
	}

	if result.Type != gjson.JSON {
		log.Errorf("result of GetTokenByHash type error")
		return nil, errors.New("result of GetTokenByHash type error")
	}

	if !result.Get("data").Exists() {
		return nil, errors.New("can't find the Token Trans")
	}

	data := result.Get("data")

	if !data.Get("success").Bool() {
		return nil, errors.New("the token does't success")
	}

	tokenTransfers := data.Get("tokenTransfers").Raw

	var nulsTokens []*NulsToken
	err = json.Unmarshal([]byte(tokenTransfers), &nulsTokens)
	if err != nil {
		log.Errorf("GetTokenByHash decode json [%v] failed, err=%v", []byte(result.Raw), err)
		return nil, err
	}

	if nulsTokens != nil && len(nulsTokens) > 0 {
		for _, v := range nulsTokens {
			v.Hash = hash
		}
	}

	return nulsTokens, nil
}

//广播交易
func (this *Client) VaildTransaction(hex string) (bool, error) {

	params := make(map[string]interface{})
	params["txHex"] = hex
	_, err := this.CallPost("/api/accountledger/transaction/valiTransaction", params)
	if err != nil {
		log.Errorf("VaildTransaction  faield, err = %v \n", err)
		return false, err
	}

	return true, nil
}

//广播交易
func (this *Client) SendRawTransaction(hex string) (string, error) {

	params := make(map[string]interface{})
	params["txHex"] = hex
	result, err := this.CallPost("/api/accountledger/transaction/broadcast", params)
	if err != nil {
		log.Errorf("SendRawTransaction  faield, err = %v \n", err)
		return "", err
	}
	log.Warn("result:", result)
	if result.Type != gjson.JSON {
		log.Errorf("result of SendRawTransaction type error")
		return "", errors.New("result of SendRawTransaction type error")
	}

	if result.Get("value").Exists() {
		return result.Get("value").String(), nil
	}

	return "", errors.New("unknow error")
}

func (c *Client) Call(method string, id int64, params []interface{}) (*gjson.Result, error) {
	authHeader := req.Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}
	body := make(map[string]interface{}, 0)
	body["jsonrpc"] = "2.0"
	body["id"] = id
	body["method"] = method
	body["params"] = params

	if c.Debug {
		log.Debug("Start Request API...")
	}

	r, err := req.Post("https://api.nuls.io", req.BodyJSON(&body), authHeader)

	if c.Debug {
		log.Debug("Request API Completed")
	}

	if c.Debug {
		log.Debugf("%+v\n", r)
	}

	if err != nil {
		return nil, err
	}

	resp := gjson.ParseBytes(r.Bytes())
	err = isApiError(&resp)
	if err != nil {
		return nil, err
	}

	result := resp.Get("result")

	return &result, nil
}

func (c *Client) CallPost(url string, params map[string]interface{}) (*gjson.Result, error) {
	authHeader := req.Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	if c.Debug {
		log.Debug("Start Request API...")
	}

	r, err := req.Post(c.BaseURL+url, req.BodyJSON(&params), authHeader)

	if err != nil {
		return nil, err
	}
	if c.Debug {
		log.Debug("Request API Completed")
	}

	if c.Debug {
		log.Debugf("%+v\n", r)
	}

	resp := gjson.ParseBytes(r.Bytes())
	err = isError(&resp)
	if err != nil {
		return nil, err
	}

	result := resp.Get("data")

	return &result, nil
}

func (c *Client) CallReq(method string) (*gjson.Result, error) {

	if c.Debug {
		log.Debug("Start Request API...")
	}

	r, err := req.Get(c.BaseURL + method)
	if err != nil {
		return nil, err
	}
	if c.Debug {
		log.Debug("Request API Completed")
	}

	if c.Debug {
		log.Debugf("%+v\n", r)
	}

	if err != nil {
		return nil, err
	}

	resp := gjson.ParseBytes(r.Bytes())
	err = isError(&resp)
	if err != nil {
		return nil, err
	}

	result := resp.Get("data")

	return &result, nil
}

//isError 是否报错
func isError(result *gjson.Result) error {
	var (
		err error
	)

	if !result.Get("success").Bool() {
		errMsg := ""
		if result != nil && result.Type == gjson.JSON {
			errMsg = result.Get("msg").Raw
		} else {
			errMsg = "验签未知错误"
			return errors.New("success is false! " + errMsg)
		}
	}

	if !result.Get("data").Exists() {
		return errors.New("data is empty! ")
	}

	return err
}

//isError 是否报错
func isApiError(result *gjson.Result) error {
	var (
		err error
	)

	if !result.Get("error").IsObject() {

		if !result.Get("result").Exists() {
			return errors.New("Response is empty! ")
		}

		return nil
	}

	errInfo := fmt.Sprintf("[%d]%s",
		result.Get("error.code").Int(),
		result.Get("error.message").String())
	err = errors.New(errInfo)

	return err
}
