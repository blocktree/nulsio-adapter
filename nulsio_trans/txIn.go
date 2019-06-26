package nulsio_trans

import (
	"errors"
)

type TxIn struct {
	Owner                 []byte
	Na                    []byte
	LockTime              []byte
	ScriptPubkeySignature []byte
}

func newTxInForEmptyTrans(vin []Vin) ([]TxIn, error) {
	var ret []TxIn

	for _, v := range vin {
		owner, err := GetInputOwnerKey(v.TxID, int64(v.Vout))
		if err != nil {
			return nil, errors.New("Invalid previous txid!" + err.Error())
		}
		ownerFinal, _ := GetBytesWithLength(owner)
		na := uint64ToLittleEndianBytes(v.Amount)
		lockTime := uint48ToLittleEndianBytes(v.LockTime)
		//vout := uint32ToLittleEndianBytes(v.Vout)

		ret = append(ret, TxIn{ownerFinal[:], na, lockTime, nil})
	}
	return ret, nil
}
