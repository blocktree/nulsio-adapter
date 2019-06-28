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
	"errors"
	"fmt"
	"github.com/asdine/storm"
	"github.com/blocktree/openwallet/common"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
	"path/filepath"
	"strings"
	"time"
)

const (
	blockchainBucket = "blockchain" //区块链数据集合
	//periodOfTask      = 5 * time.Second //定时任务执行隔间
	maxExtractingSize = 10 //并发的扫描线程数
)

//NULSBlockScanner nulscoin的区块链扫描器
type NULSBlockScanner struct {
	*openwallet.BlockScannerBase

	CurrentBlockHeight   uint64         //当前区块高度
	extractingCH         chan struct{}  //扫描工作令牌
	wm                   *WalletManager //钱包管理者
	IsScanMemPool        bool           //是否扫描交易池
	RescanLastBlockCount uint64         //重扫上N个区块数量
}

//ExtractResult 扫描完成的提取结果
type ExtractResult struct {
	extractData map[string]*openwallet.TxExtractData
	TxID        string
	BlockHeight uint64
	Success     bool
}

//SaveResult 保存结果
type SaveResult struct {
	TxID        string
	BlockHeight uint64
	Success     bool
}

//NewNULSBlockScanner 创建区块链扫描器
func NewNULSBlockScanner(wm *WalletManager) *NULSBlockScanner {
	bs := NULSBlockScanner{
		BlockScannerBase: openwallet.NewBlockScannerBase(),
	}

	bs.extractingCH = make(chan struct{}, maxExtractingSize)
	bs.wm = wm
	bs.IsScanMemPool = false
	bs.RescanLastBlockCount = 1

	//设置扫描任务
	bs.SetTask(bs.ScanBlockTask)

	return &bs
}

//SetRescanBlockHeight 重置区块链扫描高度
func (bs *NULSBlockScanner) SetRescanBlockHeight(height uint64) error {
	height = height - 1
	if height < 0 {
		return fmt.Errorf("block height to rescan must greater than 0.")
	}

	hash, err := bs.wm.GetBlockHash(height)
	if err != nil {
		return err
	}

	bs.wm.SaveLocalNewBlock(height, hash)

	return nil
}

//ScanBlockTask 扫描任务
func (bs *NULSBlockScanner) ScanBlockTask() {

	//获取本地区块高度
	blockHeader, err := bs.GetScannedBlockHeader()
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get new block height; unexpected error: %v", err)
		return
	}

	currentHeight := blockHeader.Height
	currentHash := blockHeader.Hash

	for {

		if !bs.Scanning {
			//区块扫描器已暂停，马上结束本次任务
			return
		}

		//获取最大高度
		maxHeight, err := bs.wm.GetBlockHeight()
		if err != nil {
			//下一个高度找不到会报异常
			bs.wm.Log.Std.Info("block scanner can not get rpc-server block height; unexpected error: %v", err)
			break
		}

		//是否已到最新高度
		if currentHeight >= maxHeight {
			bs.wm.Log.Std.Info("block scanner has scanned full chain data. Current height: %d", maxHeight)
			break
		}

		//继续扫描下一个区块
		currentHeight = currentHeight + 1

		bs.wm.Log.Std.Info("block scanner scanning height: %d ...", currentHeight)

		hash, err := bs.wm.GetBlockHash(currentHeight)
		if err != nil {
			//下一个高度找不到会报异常
			bs.wm.Log.Std.Info("block scanner can not get new block hash; unexpected error: %v", err)
			break
		}

		block, err := bs.wm.GetBlock(hash)
		if err != nil {
			bs.wm.Log.Std.Info("block scanner can not get new block data; unexpected error: %v", err)

			//记录未扫区块
			unscanRecord := NewUnscanRecord(currentHeight, "", err.Error())
			bs.SaveUnscanRecord(unscanRecord)
			bs.wm.Log.Std.Info("block height: %d extract failed.", currentHeight)
			continue
		}

		isFork := false

		//判断hash是否上一区块的hash
		if currentHash != block.PreHash {

			bs.wm.Log.Std.Info("block has been fork on height: %d.", currentHeight)
			bs.wm.Log.Std.Info("block height: %d local hash = %s ", currentHeight-1, currentHash)
			bs.wm.Log.Std.Info("block height: %d mainnet hash = %s ", currentHeight-1, block.PreHash)

			bs.wm.Log.Std.Info("delete recharge records on block height: %d.", currentHeight-1)

			//查询本地分叉的区块
			forkBlock, _ := bs.wm.GetLocalBlock(currentHeight - 1)

			//删除上一区块链的所有充值记录
			//bs.DeleteRechargesByHeight(currentHeight - 1)
			//删除上一区块链的未扫记录
			bs.wm.DeleteUnscanRecord(currentHeight - 1)
			currentHeight = currentHeight - 2 //倒退2个区块重新扫描
			if currentHeight <= 0 {
				currentHeight = 1
			}

			localBlock, err := bs.wm.GetLocalBlock(currentHeight)
			if err != nil {
				bs.wm.Log.Std.Error("block scanner can not get local block; unexpected error: %v", err)

				//查找core钱包的RPC
				bs.wm.Log.Info("block scanner prev block height:", currentHeight)

				_, err := bs.wm.GetBlockHash(currentHeight)
				if err != nil {
					bs.wm.Log.Std.Error("block scanner can not get prev block; unexpected error: %v", err)
					break
				}

			}

			//重置当前区块的hash
			currentHash = localBlock.Hash

			bs.wm.Log.Std.Info("rescan block on height: %d, hash: %s .", currentHeight, currentHash)

			//重新记录一个新扫描起点
			bs.wm.SaveLocalNewBlock(uint64(localBlock.Height), localBlock.Hash)

			isFork = true

			if forkBlock != nil {

				//通知分叉区块给观测者，异步处理
				bs.newBlockNotify(forkBlock, isFork)
			}

		} else {

			err = bs.BatchExtractTransaction(uint64(block.Height), block.Hash, block.TxList)
			if err != nil {
				bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
			}

			//重置当前区块的hash
			currentHash = hash

			//保存本地新高度
			bs.wm.SaveLocalNewBlock(currentHeight, currentHash)
			bs.wm.SaveLocalBlock(block)

			isFork = false

			//通知新区块给观测者，异步处理
			bs.newBlockNotify(block, isFork)
		}

	}

	//重扫前N个块，为保证记录找到
	for i := currentHeight - bs.RescanLastBlockCount; i < currentHeight; i++ {
		bs.scanBlock(i)
	}

	//if bs.IsScanMemPool {
	//	//扫描交易内存池
	//	bs.ScanTxMemPool()
	//}

	//重扫失败区块
	bs.RescanFailedRecord()

}

//ScanBlock 扫描指定高度区块
func (bs *NULSBlockScanner) ScanBlock(height uint64) error {

	block, err := bs.scanBlock(height)
	if err != nil {
		return err
	}

	//通知新区块给观测者，异步处理
	bs.newBlockNotify(block, false)

	return nil
}

func (bs *NULSBlockScanner) scanBlock(height uint64) (*NusBlock, error) {

	hash, err := bs.wm.GetBlockHash(height)
	if err != nil {
		//下一个高度找不到会报异常
		bs.wm.Log.Std.Info("block scanner can not get new block hash; unexpected error: %v", err)
		return nil, err
	}

	block, err := bs.wm.GetBlock(hash)
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get new block data; unexpected error: %v", err)

		//记录未扫区块
		unscanRecord := NewUnscanRecord(height, "", err.Error())
		bs.SaveUnscanRecord(unscanRecord)
		bs.wm.Log.Std.Info("block height: %d extract failed.", height)
		return nil, err
	}

	bs.wm.Log.Std.Info("block scanner scanning height: %d ...", block.Height)

	err = bs.BatchExtractTransaction(uint64(block.Height), block.Hash, block.TxList)
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
	}

	return block, nil
}

//rescanFailedRecord 重扫失败记录
func (bs *NULSBlockScanner) RescanFailedRecord() {

	var (
		blockMap = make(map[uint64][]string)
	)

	list, err := bs.wm.GetUnscanRecords()
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get rescan data; unexpected error: %v", err)
	}

	//组合成批处理
	for _, r := range list {

		if _, exist := blockMap[r.BlockHeight]; !exist {
			blockMap[r.BlockHeight] = make([]string, 0)
		}

		if len(r.TxID) > 0 {
			arr := blockMap[r.BlockHeight]
			arr = append(arr, r.TxID)

			blockMap[r.BlockHeight] = arr
		}
	}

	for height, _ := range blockMap {

		if height == 0 {
			continue
		}

		var hash string

		bs.wm.Log.Std.Info("block scanner rescanning height: %d ...", height)

		hash, err := bs.wm.GetBlockHash(height)
		if err != nil {
			//下一个高度找不到会报异常
			bs.wm.Log.Std.Info("block scanner can not get new block hash; unexpected error: %v", err)
			continue
		}

		block, err := bs.wm.GetBlock(hash)
		if err != nil {
			bs.wm.Log.Std.Info("block scanner can not get new block data; unexpected error: %v", err)
			continue
		}

		err = bs.BatchExtractTransaction(height, hash, block.TxList)
		if err != nil {
			bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
			continue
		}

		//删除未扫记录
		bs.wm.DeleteUnscanRecord(height)
	}

	//删除未没有找到交易记录的重扫记录
	bs.wm.DeleteUnscanRecordNotFindTX()
}

//newBlockNotify 获得新区块后，通知给观测者
func (bs *NULSBlockScanner) newBlockNotify(block *NusBlock, isFork bool) {

	header := block.BlockHeader(bs.wm.Symbol())
	header.Fork = isFork
	bs.NewBlockNotify(header)
}

//BatchExtractTransaction 批量提取交易单
//nulscoin 1M的区块链可以容纳3000笔交易，批量多线程处理，速度更快
func (bs *NULSBlockScanner) BatchExtractTransaction(blockHeight uint64, blockHash string, txs []*Tx) error {

	var (
		quit       = make(chan struct{})
		done       = 0 //完成标记
		failed     = 0
		shouldDone = len(txs) //需要完成的总数
	)

	if len(txs) == 0 {
		return fmt.Errorf("BatchExtractTransaction block is nil.")
	}

	//生产通道
	producer := make(chan ExtractResult)
	defer close(producer)

	//消费通道
	worker := make(chan ExtractResult)
	defer close(worker)

	//保存工作
	saveWork := func(height uint64, result chan ExtractResult) {
		//回收创建的地址
		for gets := range result {

			if gets.Success {

				notifyErr := bs.newExtractDataNotify(height, gets.extractData)
				//saveErr := bs.SaveRechargeToWalletDB(height, gets.Recharges)
				if notifyErr != nil {
					failed++ //标记保存失败数
					bs.wm.Log.Std.Info("newExtractDataNotify unexpected error: %v", notifyErr)
				}

			} else {
				//记录未扫区块
				unscanRecord := NewUnscanRecord(height, "", "")
				bs.SaveUnscanRecord(unscanRecord)
				bs.wm.Log.Std.Info("block height: %d extract failed.", height)
				failed++ //标记保存失败数
			}
			//累计完成的线程数
			done++
			if done == shouldDone {
				//bs.wm.Log.Std.Info("done = %d, shouldDone = %d ", done, len(txs))
				close(quit) //关闭通道，等于给通道传入nil
			}
		}
	}

	//提取工作
	extractWork := func(eblockHeight uint64, eBlockHash string, mTxs []*Tx, eProducer chan ExtractResult) {
		for _, tx := range mTxs {
			bs.extractingCH <- struct{}{}
			//shouldDone++
			go func(mBlockHeight uint64, mTxid string, end chan struct{}, mProducer chan<- ExtractResult) {

				//导出提出的交易
				mProducer <- bs.ExtractTransaction(mBlockHeight, eBlockHash, tx, bs.ScanAddressFunc)
				//释放
				<-end

			}(eblockHeight, eBlockHash, bs.extractingCH, eProducer)
		}
	}

	/*	开启导出的线程	*/

	//独立线程运行消费
	go saveWork(blockHeight, worker)

	//独立线程运行生产
	go extractWork(blockHeight, blockHash, txs, producer)

	//以下使用生产消费模式
	bs.extractRuntime(producer, worker, quit)

	if failed > 0 {
		return fmt.Errorf("block scanner saveWork failed")
	} else {
		return nil
	}

	//return nil
}

//extractRuntime 提取运行时
func (bs *NULSBlockScanner) extractRuntime(producer chan ExtractResult, worker chan ExtractResult, quit chan struct{}) {

	var (
		values = make([]ExtractResult, 0)
	)

	for {

		var activeWorker chan<- ExtractResult
		var activeValue ExtractResult

		//当数据队列有数据时，释放顶部，传输给消费者
		if len(values) > 0 {
			activeWorker = worker
			activeValue = values[0]

		}

		select {

		//生成者不断生成数据，插入到数据队列尾部
		case pa := <-producer:
			values = append(values, pa)
		case <-quit:
			//退出
			//bs.wm.Log.Std.Info("block scanner have been scanned!")
			return
		case activeWorker <- activeValue:
			//wm.Log.Std.Info("Get %d", len(activeValue))
			values = values[1:]
		}
	}

}

//ExtractTransaction 提取交易单
func (bs *NULSBlockScanner) ExtractTransaction(blockHeight uint64, blockHash string, tx *Tx, scanAddressFunc openwallet.BlockScanAddressFunc) ExtractResult {

	var (
		result = ExtractResult{
			BlockHeight: blockHeight,
			TxID:        tx.Hash,
			extractData: make(map[string]*openwallet.TxExtractData),
		}
	)

	//优先使用传入的高度
	if blockHeight > 0 && tx.BlockHeight == 0 {
		tx.BlockHeight = int64(blockHeight)
	}

	//bs.wm.Log.Debug("start extractTransaction")
	bs.extractTransaction(tx, blockHash, &result, scanAddressFunc)
	//bs.wm.Log.Debug("end extractTransaction")

	return result

}

//ExtractTransactionData 提取交易单
func (bs *NULSBlockScanner) extractTransaction(trx *Tx, blockHash string, result *ExtractResult, scanAddressFunc openwallet.BlockScanAddressFunc) {

	var (
		success = true
	)

	if trx == nil {
		//记录哪个区块哪个交易单没有完成扫描
		success = false
	} else {

		blocktime := trx.Time

		if success {

			switch trx.Type {
			case 2:
				//提取出账部分记录
				from, totalSpent := bs.extractTxInput(trx, blockHash, result, scanAddressFunc)
				//bs.wm.Log.Debug("from:", from, "totalSpent:", totalSpent)

				//提取入账部分记录
				to, totalReceived := bs.extractTxOutput(trx, blockHash, result, scanAddressFunc)
				//bs.wm.Log.Debug("to:", to, "totalReceived:", totalReceived)

				for _, extractData := range result.extractData {
					tx := &openwallet.Transaction{
						From: from,
						To:   to,
						Fees: totalSpent.Sub(totalReceived).StringFixed(8),
						Coin: openwallet.Coin{
							Symbol:     bs.wm.Symbol(),
							IsContract: false,
						},
						BlockHash:   blockHash,
						BlockHeight: uint64(trx.BlockHeight),
						TxID:        trx.Hash,
						Decimal:     8,
						ConfirmTime: blocktime,
						Status:      openwallet.TxStatusSuccess,
					}
					wxID := openwallet.GenTransactionWxID(tx)
					tx.WxID = wxID
					extractData.Transaction = tx

					//bs.wm.Log.Debug("Transaction:", extractData.Transaction)
				}
				break
			case 101:
				tokenTrans, err := bs.wm.Api.GetTokenByHash(trx.Hash)
				if err != nil {
					bs.wm.Log.Error("Token tokenTrans is nil,hash:", trx.Hash, " ,err:", err.Error())
					break
				}
				if tokenTrans == nil || len(tokenTrans) != 1 {
					bs.wm.Log.Error("Token tokenTrans is nil or len is't 1")
					break
				}
				//提取出账部分记录
				from, totalSpent := bs.extractTokenTxInput(tokenTrans, blockHash, trx.BlockHeight, result, scanAddressFunc)
				//bs.wm.Log.Debug("from:", from, "totalSpent:", totalSpent)
				//提取入账部分记录
				to, totalReceived := bs.extractTokenTxOutput(tokenTrans, blockHash, trx.BlockHeight, int64(trx.ConfirmCount), result, scanAddressFunc)

				for _, extractData := range result.extractData {
					tokenIn := tokenTrans[0]
					contractId := openwallet.GenContractID(bs.wm.Symbol(), tokenIn.ContractAddress)
					tx := &openwallet.Transaction{
						From: from,
						To:   to,
						Fees: totalSpent.Sub(totalReceived).StringFixed(8),
						Coin: openwallet.Coin{
							Symbol:     bs.wm.Symbol(),
							IsContract: false,
							ContractID: contractId,
							Contract: openwallet.SmartContract{
								Address:    tokenIn.ContractAddress,
								Decimals:   uint64(tokenIn.Decimals),
								Name:       tokenIn.Name,
								Symbol:     bs.wm.Symbol(),
								Token:      tokenIn.Symbol,
								Protocol:   "nrc20",
								ContractID: contractId,
							},
						},
						BlockHash:   blockHash,
						BlockHeight: uint64(trx.BlockHeight),
						TxID:        trx.Hash,
						Decimal:     8,
						ConfirmTime: blocktime,
						Status:      openwallet.TxStatusSuccess,
					}
					wxID := openwallet.GenTransactionWxID(tx)
					tx.WxID = wxID
					extractData.Transaction = tx

					//bs.wm.Log.Debug("Transaction:", extractData.Transaction)
				}
				break
			}

		}

		success = true

	}
	result.Success = success
}

//ExtractTxInput 提取交易单输入部分
func (bs *NULSBlockScanner) extractTxInput(trx *Tx, blockHash string, result *ExtractResult, scanAddressFunc openwallet.BlockScanAddressFunc) ([]string, decimal.Decimal) {

	//vin := trx.Get("vin")

	var (
		from        = make([]string, 0)
		totalAmount = decimal.Zero
	)

	createAt := time.Now().Unix()
	for i, output := range trx.Inputs {

		//in := vin[i]

		txid := trx.Hash
		//vout := output.Vout
		//
		//output, err := bs.wm.GetTxOut(txid, vout)
		//if err != nil {
		//	return err
		//}

		amount := common.IntToDecimals(int64(output.Value), bs.wm.Decimal()).String()
		addr := output.Address
		sourceKey, ok := scanAddressFunc(addr)
		if ok {
			input := openwallet.TxInput{}
			input.SourceTxID = txid
			input.SourceIndex = uint64(output.FromIndex)
			input.TxID = txid
			input.Address = addr
			//transaction.AccountID = a.AccountID
			input.Amount = amount
			input.Coin = openwallet.Coin{
				Symbol:     bs.wm.Symbol(),
				IsContract: false,
			}
			input.Index = uint64(i)
			input.Sid = openwallet.GenTxInputSID(txid, bs.wm.Symbol(), "", uint64(i))
			//input.Sid = base64.StdEncoding.EncodeToString(crypto.SHA1([]byte(fmt.Sprintf("input_%s_%d_%s", result.TxID, i, addr))))
			input.CreateAt = createAt
			//在哪个区块高度时消费
			input.BlockHeight = uint64(trx.BlockHeight)
			input.BlockHash = blockHash

			//transactions = append(transactions, &transaction)

			ed := result.extractData[sourceKey]
			if ed == nil {
				ed = openwallet.NewBlockExtractData()
				result.extractData[sourceKey] = ed
			}

			ed.TxInputs = append(ed.TxInputs, &input)

		}

		from = append(from, addr+":"+amount)
		dAmount, _ := decimal.NewFromString(amount)
		totalAmount = totalAmount.Add(dAmount)

	}
	return from, totalAmount
}

//ExtractTxInput 提取交易单输入部分
func (bs *NULSBlockScanner) extractTokenTxInput(nulsTokens []*NulsToken, blockHash string, blockHeight int64, result *ExtractResult, scanAddressFunc openwallet.BlockScanAddressFunc) ([]string, decimal.Decimal) {

	//vin := trx.Get("vin")

	var (
		from        = make([]string, 0)
		totalAmount = decimal.Zero
	)

	createAt := time.Now().Unix()
	for i, tokenIn := range nulsTokens {

		//in := vin[i]

		txid := tokenIn.Hash
		amount, err := decimal.NewFromString(tokenIn.Value)
		if err != nil {
			bs.wm.Log.Error("nulsTokens value can't be int,err:", err.Error())
			continue
		}
		amount = amount.Shift(int32(-uint64(tokenIn.Decimals)))
		addr := tokenIn.From
		sourceKey, ok := scanAddressFunc(addr)
		if ok {
			input := openwallet.TxInput{}
			input.SourceTxID = txid
			//input.SourceIndex = uint64(tokenIn.FromIndex)
			input.TxID = txid
			input.Address = addr
			//transaction.AccountID = a.AccountID
			input.Amount = amount.String()
			contractId := openwallet.GenContractID(bs.wm.Symbol(), tokenIn.ContractAddress)
			input.Coin = openwallet.Coin{
				Symbol:     bs.wm.Symbol(),
				IsContract: true,
				ContractID: contractId,
				Contract: openwallet.SmartContract{
					Address:    tokenIn.ContractAddress,
					Decimals:   uint64(tokenIn.Decimals),
					Name:       tokenIn.Name,
					Symbol:     bs.wm.Symbol(),
					Token:      tokenIn.Symbol,
					Protocol:   "nrc20",
					ContractID: contractId,
				},
			}
			input.Index = uint64(i)
			input.Sid = openwallet.GenTxInputSID(txid, bs.wm.Symbol(), contractId, uint64(i))
			//input.Sid = base64.StdEncoding.EncodeToString(crypto.SHA1([]byte(fmt.Sprintf("input_%s_%d_%s", result.TxID, i, addr))))
			input.CreateAt = createAt
			//在哪个区块高度时消费
			input.BlockHeight = uint64(blockHeight)
			input.BlockHash = blockHash

			//transactions = append(transactions, &transaction)

			ed := result.extractData[sourceKey]
			if ed == nil {
				ed = openwallet.NewBlockExtractData()
				result.extractData[sourceKey] = ed
			}

			ed.TxInputs = append(ed.TxInputs, &input)

		}

		from = append(from, addr+":"+amount.String())
		dAmount, _ := decimal.NewFromString(amount.String())
		totalAmount = totalAmount.Add(dAmount)

	}
	return from, totalAmount
}

//ExtractTxInput 提取交易单输入部分
func (bs *NULSBlockScanner) extractTxOutput(trx *Tx, blockHash string, result *ExtractResult, scanAddressFunc openwallet.BlockScanAddressFunc) ([]string, decimal.Decimal) {

	var (
		to          = make([]string, 0)
		totalAmount = decimal.Zero
	)

	confirmations := trx.ConfirmCount
	vout := trx.Outputs
	txid := trx.Hash
	//bs.wm.Log.Debug("vout:", vout.Array())
	createAt := time.Now().Unix()
	for n, output := range vout {

		if output.LockTime > trx.BlockHeight{
			bs.wm.Log.Error("nuls lockTime over Than now,err:")
			continue
		}

		amount := common.IntToDecimals(int64(output.Value), bs.wm.Decimal()).String()
		addr := output.Address
		sourceKey, ok := scanAddressFunc(addr)
		if ok {

			//a := wallet.GetAddress(addr)
			//if a == nil {
			//	continue
			//}

			outPut := openwallet.TxOutPut{}
			outPut.TxID = txid
			outPut.Address = addr
			//transaction.AccountID = a.AccountID
			outPut.Amount = amount
			outPut.Coin = openwallet.Coin{
				Symbol:     bs.wm.Symbol(),
				IsContract: false,
			}
			outPut.Index = uint64(n)
			outPut.Sid = openwallet.GenTxOutPutSID(txid, bs.wm.Symbol(), "", uint64(n))
			//outPut.Sid = base64.StdEncoding.EncodeToString(crypto.SHA1([]byte(fmt.Sprintf("output_%s_%d_%s", txid, n, addr))))

			//保存utxo到扩展字段
			//outPut.SetExtParam("scriptPubKey", output.ScriptPubKey)
			outPut.CreateAt = createAt
			outPut.BlockHeight = uint64(trx.BlockHeight)
			outPut.BlockHash = blockHash
			outPut.Confirm = int64(confirmations)

			//transactions = append(transactions, &transaction)

			ed := result.extractData[sourceKey]
			if ed == nil {
				ed = openwallet.NewBlockExtractData()
				result.extractData[sourceKey] = ed
			}

			ed.TxOutputs = append(ed.TxOutputs, &outPut)

		}

		to = append(to, addr+":"+amount)
		dAmount, _ := decimal.NewFromString(amount)
		totalAmount = totalAmount.Add(dAmount)

	}

	return to, totalAmount
}

//ExtractTxInput 提取交易单输入部分
func (bs *NULSBlockScanner) extractTokenTxOutput(nulsTokens []*NulsToken, blockHash string, blockHeight int64, confirmation int64, result *ExtractResult, scanAddressFunc openwallet.BlockScanAddressFunc) ([]string, decimal.Decimal) {

	var (
		to          = make([]string, 0)
		totalAmount = decimal.Zero
	)

	confirmations := confirmation

	createAt := time.Now().Unix()
	for i, tokenIn := range nulsTokens {


		
		txid := tokenIn.Hash
		amount, err := decimal.NewFromString(tokenIn.Value)
		if err != nil {
			bs.wm.Log.Error("nulsTokens value can't be int,err:", err.Error())
			continue
		}
		amount = amount.Shift(int32(-uint64(tokenIn.Decimals)))
		addr := tokenIn.To
		sourceKey, ok := scanAddressFunc(addr)
		if ok {

			//a := wallet.GetAddress(addr)
			//if a == nil {
			//	continue
			//}

			outPut := openwallet.TxOutPut{}
			outPut.TxID = txid
			outPut.Address = addr
			//transaction.AccountID = a.AccountID
			outPut.Amount = amount.String()
			contractId := openwallet.GenContractID(bs.wm.Symbol(), tokenIn.ContractAddress)
			outPut.Coin = openwallet.Coin{
				Symbol:     bs.wm.Symbol(),
				IsContract: true,
				ContractID: contractId,
				Contract: openwallet.SmartContract{
					ContractID: contractId,
					Address:    tokenIn.ContractAddress,
					Decimals:   uint64(tokenIn.Decimals),
					Name:       tokenIn.Name,
					Symbol:     bs.wm.Symbol(),
					Token:      tokenIn.Symbol,
					Protocol:   "nrc20",
				},
			}
			outPut.Index = uint64(i)
			outPut.Sid = openwallet.GenTxOutPutSID(txid, bs.wm.Symbol(), contractId, uint64(i))
			//outPut.Sid = base64.StdEncoding.EncodeToString(crypto.SHA1([]byte(fmt.Sprintf("output_%s_%d_%s", txid, n, addr))))

			//保存utxo到扩展字段
			//outPut.SetExtParam("scriptPubKey", output.ScriptPubKey)
			outPut.CreateAt = createAt
			outPut.BlockHeight = uint64(blockHeight)
			outPut.BlockHash = blockHash
			outPut.Confirm = int64(confirmations)

			//transactions = append(transactions, &transaction)

			ed := result.extractData[sourceKey]
			if ed == nil {
				ed = openwallet.NewBlockExtractData()
				result.extractData[sourceKey] = ed
			}

			ed.TxOutputs = append(ed.TxOutputs, &outPut)

		}

		to = append(to, addr+":"+amount.String())
		dAmount, _ := decimal.NewFromString(amount.String())
		totalAmount = totalAmount.Add(dAmount)

	}

	return to, totalAmount
}

//newExtractDataNotify 发送通知
func (bs *NULSBlockScanner) newExtractDataNotify(height uint64, extractData map[string]*openwallet.TxExtractData) error {

	for o, _ := range bs.Observers {
		for key, data := range extractData {
			err := o.BlockExtractDataNotify(key, data)
			if err != nil {
				bs.wm.Log.Error("BlockExtractDataNotify unexpected error:", err)
				//记录未扫区块
				unscanRecord := NewUnscanRecord(height, "", "ExtractData Notify failed.")
				err = bs.SaveUnscanRecord(unscanRecord)
				if err != nil {
					bs.wm.Log.Std.Error("block height: %d, save unscan record failed. unexpected error: %v", height, err.Error())
				}

			}
		}
	}

	return nil
}

//DeleteUnscanRecordNotFindTX 删除未没有找到交易记录的重扫记录
func (wm *WalletManager) DeleteUnscanRecordNotFindTX() error {

	//删除找不到交易单
	reason := "[-5]No information available about transaction"

	//获取本地区块高度
	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return err
	}
	defer db.Close()

	var list []*UnscanRecord
	err = db.All(&list)
	if err != nil {
		return err
	}

	tx, err := db.Begin(true)
	if err != nil {
		return err
	}
	for _, r := range list {
		if strings.HasPrefix(r.Reason, reason) {
			tx.DeleteStruct(r)
		}
	}
	return tx.Commit()
}

//SaveRechargeToWalletDB 保存交易单内的充值记录到钱包数据库
//func (bs *NULSBlockScanner) SaveRechargeToWalletDB(height uint64, list []*openwallet.Recharge) error {
//
//	for _, r := range list {
//
//		//accountID := "W4ruoAyS5HdBMrEeeHQTBxo4XtaAixheXQ"
//		wallet, ok := bs.GetWalletByAddress(r.Address)
//		if ok {
//
//			//a := wallet.GetAddress(r.Address)
//			//if a == nil {
//			//	continue
//			//}
//			//
//			//r.AccountID = a.AccountID
//
//			err := wallet.SaveUnreceivedRecharge(r)
//			//如果blockHash没有值，添加到重扫，避免遗留
//			if err != nil || len(r.BlockHash) == 0 {
//
//				//记录未扫区块
//				unscanRecord := NewUnscanRecord(height, r.TxID, "save to wallet failed.")
//				err = bs.SaveUnscanRecord(unscanRecord)
//				if err != nil {
//					bs.wm.Log.Std.Error("block height: %d, txID: %s save unscan record failed. unexpected error: %v", height, r.TxID, err.Error())
//				}
//
//			} else {
//				bs.wm.Log.Info("block scanner save blockHeight:", height, "txid:", r.TxID, "address:", r.Address, "successfully.")
//			}
//		} else {
//			return errors.New("address in wallet is not found")
//		}
//
//	}
//
//	return nil
//}

//GetScannedBlockHeader 获取当前扫描的区块头
func (bs *NULSBlockScanner) GetScannedBlockHeader() (*openwallet.BlockHeader, error) {

	var (
		blockHeight uint64 = 0
		hash        string
		err         error
	)

	blockHeight, hash = bs.wm.GetLocalNewBlock()

	//如果本地没有记录，查询接口的高度
	if blockHeight == 0 {
		blockHeight, err = bs.wm.GetBlockHeight()
		if err != nil {

			return nil, err
		}

		//就上一个区块链为当前区块
		blockHeight = blockHeight - 1

		hash, err = bs.wm.GetBlockHash(blockHeight)
		if err != nil {
			return nil, err
		}
	}

	return &openwallet.BlockHeader{Height: blockHeight, Hash: hash}, nil
}

//GetCurrentBlockHeader 获取当前区块高度
func (bs *NULSBlockScanner) GetCurrentBlockHeader() (*openwallet.BlockHeader, error) {

	var (
		blockHeight uint64 = 0
		hash        string
		err         error
	)

	blockHeight, err = bs.wm.GetBlockHeight()
	if err != nil {

		return nil, err
	}

	hash, err = bs.wm.GetBlockHash(blockHeight)
	if err != nil {
		return nil, err
	}

	return &openwallet.BlockHeader{Height: blockHeight, Hash: hash}, nil
}

func (bs *NULSBlockScanner) GetGlobalMaxBlockHeight() uint64 {
	maxHeight, err := bs.wm.GetBlockHeight()
	if err != nil {
		bs.wm.Log.Std.Info("get global max block height error;unexpected error:%v", err)
		return 0
	}
	return maxHeight
}

//GetScannedBlockHeight 获取已扫区块高度
func (bs *NULSBlockScanner) GetScannedBlockHeight() uint64 {
	localHeight, _ := bs.wm.GetLocalNewBlock()
	return localHeight
}

func (bs *NULSBlockScanner) ExtractTransactionData(txid string, scanTargetFunc openwallet.BlockScanTargetFunc) (map[string][]*openwallet.TxExtractData, error) {

	scanAddressFunc := func(address string) (string, bool) {
		target := openwallet.ScanTarget{
			Address:          address,
			BalanceModelType: openwallet.BalanceModelTypeAddress,
		}
		return scanTargetFunc(target)
	}
	tx, err := bs.wm.Api.GetTxByTxId(txid)
	if err != nil {
		return nil, fmt.Errorf("can't find the txid,err:" + err.Error())
	}
	result := bs.ExtractTransaction(0, "", tx, scanAddressFunc)
	if !result.Success {
		return nil, fmt.Errorf("extract transaction failed")
	}
	extData := make(map[string][]*openwallet.TxExtractData)
	for key, data := range result.extractData {
		txs := extData[key]
		if txs == nil {
			txs = make([]*openwallet.TxExtractData, 0)
		}
		txs = append(txs, data)
		extData[key] = txs
	}
	return extData, nil
}

//DropRechargeRecords 清楚钱包的全部充值记录
//func (bs *NULSBlockScanner) DropRechargeRecords(accountID string) error {
//	bs.mu.RLock()
//	defer bs.mu.RUnlock()
//
//	wallet, ok := bs.walletInScanning[accountID]
//	if !ok {
//		errMsg := fmt.Sprintf("accountID: %s wallet is not found", accountID)
//		return errors.New(errMsg)
//	}
//
//	return wallet.DropRecharge()
//}

//DeleteRechargesByHeight 删除某区块高度的充值记录
//func (bs *NULSBlockScanner) DeleteRechargesByHeight(height uint64) error {
//
//	bs.mu.RLock()
//	defer bs.mu.RUnlock()
//
//	for _, wallet := range bs.walletInScanning {
//
//		list, err := wallet.GetRecharges(false, height)
//		if err != nil {
//			return err
//		}
//
//		db, err := wallet.OpenDB()
//		if err != nil {
//			return err
//		}
//
//		tx, err := db.Begin(true)
//		if err != nil {
//			return err
//		}
//
//		for _, r := range list {
//			err = db.DeleteStruct(&r)
//			if err != nil {
//				return err
//			}
//		}
//
//		tx.Commit()
//
//		db.Close()
//	}
//
//	return nil
//}

//SaveTxToWalletDB 保存交易记录到钱包数据库
func (bs *NULSBlockScanner) SaveUnscanRecord(record *UnscanRecord) error {

	if record == nil {
		return fmt.Errorf("the unscan record to save is nil")
	}

	if record.BlockHeight == 0 {
		bs.wm.Log.Warn("unconfirmed transaction do not rescan")
		return nil
	}

	//获取本地区块高度
	db, err := storm.Open(filepath.Join(bs.wm.Config.dbPath, bs.wm.Config.BlockchainFile))
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Save(record)
}

//GetWalletByAddress 获取地址对应的钱包
//func (bs *NULSBlockScanner) GetWalletByAddress(address string) (*openwallet.Wallet, bool) {
//	bs.mu.RLock()
//	defer bs.mu.RUnlock()
//
//	account, ok := bs.addressInScanning[address]
//	if ok {
//		wallet, ok := bs.walletInScanning[account]
//		return wallet, ok
//
//	} else {
//		return nil, false
//	}
//}

//GetBlockHeight 获取区块链高度
func (wm *WalletManager) GetBlockHeight() (uint64, error) {

	result, err := wm.Api.GetNewHeight()
	if err != nil {
		return 0, err
	}

	return uint64(result), nil
}

//GetLocalNewBlock 获取本地记录的区块高度和hash
func (wm *WalletManager) GetLocalNewBlock() (uint64, string) {

	var (
		blockHeight uint64 = 0
		blockHash   string = ""
	)

	//获取本地区块高度
	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return 0, ""
	}
	defer db.Close()

	db.Get(blockchainBucket, "blockHeight", &blockHeight)
	db.Get(blockchainBucket, "blockHash", &blockHash)

	return blockHeight, blockHash
}

//SaveLocalNewBlock 记录区块高度和hash到本地
func (wm *WalletManager) SaveLocalNewBlock(blockHeight uint64, blockHash string) {

	//获取本地区块高度
	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return
	}
	defer db.Close()

	db.Set(blockchainBucket, "blockHeight", &blockHeight)
	db.Set(blockchainBucket, "blockHash", &blockHash)
}

//SaveLocalBlock 记录本地新区块
func (wm *WalletManager) SaveLocalBlock(block *NusBlock) {

	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return
	}
	defer db.Close()

	db.Save(block)
}

//GetBlockHash 根据区块高度获得区块hash
func (wm *WalletManager) GetBlockHash(height uint64) (string, error) {

	result, err := wm.Api.GetBlockByHeight(int64(height))
	if err != nil {
		return "", err
	}

	return result.Hash, nil
}

//GetLocalBlock 获取本地区块数据
func (wm *WalletManager) GetLocalBlock(height uint64) (*NusBlock, error) {

	var (
		block NusBlock
	)

	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.One("Height", height, &block)
	if err != nil {
		return nil, err
	}

	return &block, nil
}

//GetBlock 获取区块数据
func (wm *WalletManager) GetBlock(hash string) (*NusBlock, error) {

	result, err := wm.Api.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	return result, nil
}

//GetTxOut 获取交易单输出信息，用于追溯交易单输入源头
//func (wm *WalletManager) GetTxOut(txid string, vout uint64) (*Vout, error) {
//
//	request := []interface{}{
//		txid,
//		vout,
//	}
//
//	result, err := wm.WalletClient.Call("GetTxOut", request)
//	if err != nil {
//		return nil, err
//	}
//
//	output := newTxVoutByCore(result)
//
//	return output, nil
//}

//获取未扫记录
func (wm *WalletManager) GetUnscanRecords() ([]*UnscanRecord, error) {
	//获取本地区块高度
	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var list []*UnscanRecord
	err = db.All(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

//DeleteUnscanRecord 删除指定高度的未扫记录
func (wm *WalletManager) DeleteUnscanRecord(height uint64) error {
	//获取本地区块高度
	db, err := storm.Open(filepath.Join(wm.Config.dbPath, wm.Config.BlockchainFile))
	if err != nil {
		return err
	}
	defer db.Close()

	var list []*UnscanRecord
	err = db.Find("BlockHeight", height, &list)
	if err != nil {
		return err
	}

	for _, r := range list {
		db.DeleteStruct(r)
	}

	return nil
}

//GetAssetsAccountBalanceByAddress 查询账户相关地址的交易记录
func (bs *NULSBlockScanner) GetBalanceByAddress(address ...string) ([]*openwallet.Balance, error) {

	return bs.wm.getBalanceCalUnspent(address...)

}

//getBalanceByExplorer 获取地址余额
func (wm *WalletManager) getBalanceCalUnspent(address ...string) ([]*openwallet.Balance, error) {

	addrBalanceArr := make([]*openwallet.Balance, 0)
	for _, a := range address {

		var obj *openwallet.Balance

		nulsBalance, err := wm.Api.GetAddressBalance(a)
		if err != nil {
			return nil, errors.New("cant get balances:" + err.Error())
		}
		confirmBalance := nulsBalance.UnLockBalance
		balance := nulsBalance.Balance
		unconfirmBalance := nulsBalance.Balance



		obj = &openwallet.Balance{
			Symbol:           wm.Symbol(),
			Address:          a,
			Balance:          common.IntToDecimals(balance.IntPart(), wm.Decimal()).String(),
			UnconfirmBalance: common.IntToDecimals(unconfirmBalance.IntPart(), wm.Decimal()).String(),
			ConfirmBalance:   common.IntToDecimals(confirmBalance.IntPart(), wm.Decimal()).String(),
		}

		addrBalanceArr = append(addrBalanceArr, obj)
	}

	return addrBalanceArr, nil
}
