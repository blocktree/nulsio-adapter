package nulsio_trans

import "github.com/blocktree/nulsio-adapter/nulsio_addrdec"

type TxToken struct {
	Sender          string
	ContractAddress string
	Value           uint64
	GasLimit        uint64
	Price           uint64
	MethodName      string
	ArgsCount       int64
	Args            []string
}

func newTxTokenToBytes(tx *TxToken) ([]byte, error) {
	ret := make([]byte, 0)
	sendBytes := nulsio_addrdec.Base58Decode([]byte(tx.Sender))
	ret = append(ret, sendBytes...)
	contractAddress := nulsio_addrdec.Base58Decode([]byte(tx.ContractAddress))
	ret = append(ret, contractAddress...)
	valueBytes := uint64ToLittleEndianBytes(tx.Value)
	ret = append(ret, valueBytes...)
	gasLimitBytes := uint64ToLittleEndianBytes(tx.GasLimit)
	ret = append(ret, gasLimitBytes...)
	price := uint64ToLittleEndianBytes(tx.Price)
	ret = append(ret, price...)
	methodName, _ := GetBytesWithLength([]byte(tx.MethodName))
	ret = append(ret, methodName...)
	ret = append(ret, 0)
	ret = append(ret, byte(tx.ArgsCount))
	for _, v := range tx.Args {
		arg, _ := GetBytesWithLength([]byte(v))
		ret = append(ret, 1)
		ret = append(ret, arg...)
	}
	return ret, nil
}
