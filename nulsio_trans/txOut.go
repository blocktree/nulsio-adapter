package nulsio_trans

type TxOut struct {
	Owner    []byte
	Na       []byte
	LockTime []byte
}

func newTxOutForEmptyTrans(vout []Vout) ([]TxOut, error) {
	var ret []TxOut

	for _, v := range vout {
		owner := Base58Decode(v.Address)
		owner = owner[:23]
		ownerFinal,_ := GetBytesWithLength(owner)
		na := uint64ToLittleEndianBytes(v.Amount)
		lockTime := uint48ToLittleEndianBytes(v.LockTime)
		ret = append(ret, TxOut{ownerFinal, na, lockTime})
	}
	return ret, nil
}
