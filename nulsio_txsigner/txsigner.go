package nulsio_txsigner

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/blocktree/go-owcrypt"
	"github.com/blocktree/nulsio-adapter/nulsio_trans"
)


var Default = &TransactionSigner{}

type TransactionSigner struct {
}

// SignTransactionHash 交易哈希签名算法
// required
func (singer *TransactionSigner) SignTransactionHash(msg []byte, prikey []byte, eccType uint32) ([]byte, error) {
	msg = nulsio_trans.Sha256Twice(msg) //sha256

	signature, retCode := owcrypt.Signature(prikey, nil, 0, data, 32, owcrypt.ECC_CURVE_SECP256K1)
	if retCode != owcrypt.SUCCESS {
		return nil, errors.New("Failed to sign message!")
	}

	pub, ret := owcrypt.GenPubkey(prikey, owcrypt.ECC_CURVE_SECP256K1)
	if ret != owcrypt.SUCCESS {
		return nil, errors.New("Get Pubkey failed!")
	}
	pub = owcrypt.PointCompress(pub, owcrypt.ECC_CURVE_SECP256K1)

	sigPub := &nulsio_trans.SigPub{
		pub,
		signature,
	}

	fmt.Println(hex.EncodeToString(sigPub.ToBytes()))

	result := make([]byte, 0)
	result = append(result, byte(len(pub)))
	result = append(result, pub...)

	result = append(result, 0)
	resultSig := make([]byte, 0)
	resultSig = append(resultSig, sigPub.ToBytes()...)

	result = append(result, resultSig...)

	return result, nil
}
