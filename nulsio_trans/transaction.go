package nulsio_trans

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/blocktree/go-owcrypt"
)

type Vin struct {
	TxID     string
	Vout     uint32
	Amount   uint64
	LockTime uint64
}

type Vout struct {
	Address  string
	Amount   uint64
	LockTime uint64
}

type TxUnlock struct {
	PrivateKey   []byte
	LockScript   string
	RedeemScript string
	Amount       uint64
	Address      string
}

type Token struct {
	Sender          string
	ContractAddress string
	Value           int64
	GasLimit        int64
	MethodName      string
	ArgsCount       int64
	Args            []interface{}
}

const (
	DefaultTxVersion = uint32(2)
	DefaultHashType  = uint32(1)
)

func CreateEmptyRawTransaction(vins []Vin, vouts []Vout, remark string, lockTime uint32, replaceable bool,txData *TxToken) (string, []byte, error) {
	emptyTrans, err := newTransaction(vins, vouts, nil, lockTime, txData,replaceable)
	if err != nil {
		return "", nil, err
	}

	txBytes, err := emptyTrans.encodeToBytes()
	if err != nil {
		return "", nil, err
	}

	result, _ := json.Marshal(emptyTrans)

	return hex.EncodeToString(txBytes), result, nil
}

func SignTransactionMessage(message string, prikey []byte) ([]byte, error) {

	if len(message) == 0 {
		return nil, errors.New("No message to sign!")
	}
	//fmt.Println("trc", message)

	if prikey == nil || len(prikey) != 32 {
		return nil, errors.New("Invalid private key!")
	}

	data, err := hex.DecodeString(message)
	if err != nil {
		return nil, errors.New("Invalid message to sign!")
	}

	data = Sha256Twice(data) //sha256

	signature, retCode := owcrypt.Signature(prikey, nil, 0, data, 32, owcrypt.ECC_CURVE_SECP256K1)
	if retCode != owcrypt.SUCCESS {
		return nil, errors.New("Failed to sign message!")
	}

	pub, ret := owcrypt.GenPubkey(prikey, owcrypt.ECC_CURVE_SECP256K1)
	if ret != owcrypt.SUCCESS {
		return nil, errors.New("Get Pubkey failed!")
	}
	pub = owcrypt.PointCompress(pub, owcrypt.ECC_CURVE_SECP256K1)

	sigPub := &SigPub{
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

	return result, nil
}

type SigPub struct {
	PublicKey []byte
	Signature []byte
}



func (sp SigPub) ToBytes() []byte {
	r := sp.Signature[:32]
	s := sp.Signature[32:]
	if r[0]&0x80 == 0x80 {
		r = append([]byte{0x00}, r...)
	} else {
		for i := 0; i < 32; i++ {
			if r[i] == 0 && r[i+1]&0x80 != 0x80 {
				r = r[1:]
			} else {
				break
			}
		}
	}
	if s[0]&0x80 == 0x80 {
		s = append([]byte{0}, s...)
	} else {
		for i := 0; i < 32; i++ {
			if s[i] == 0 && s[i+1]&0x80 != 0x80 {
				s = s[1:]
			} else {
				break
			}
		}
	}

	r = append([]byte{byte(len(r))}, r...)
	r = append([]byte{0x02}, r...)
	s = append([]byte{byte(len(s))}, s...)
	s = append([]byte{0x02}, s...)

	rs := append(r, s...)
	rs = append([]byte{byte(len(rs))}, rs...)
	rs = append([]byte{0x30}, rs...)
	rs = append([]byte{byte(len(rs))}, rs...)

	return rs
}



func CreateRawTransactionHashForSig(txHex string, unlockData []TxUnlock) ([]string, error) {
	//txBytes, err := hex.DecodeString(txHex)
	//if err != nil {
	//	return nil, errors.New("Invalid transaction hex string!")
	//}
	//emptyTrans, err := DecodeRawTransaction(txBytes)
	//if err != nil {
	//	return nil, err
	//}
	//
	//hashes, err := emptyTrans.getHashesForSig(unlockData)
	//if err != nil {
	//	return nil, err
	//}

	//ret := []string{}
	//
	//for _, h := range hashes {
	//	ret = append(ret, hex.EncodeToString(h))
	//}

	return nil, nil
}

//
//func SignEmptyRawTransaction(txHex string, unlockData []TxUnlock) (string, error) {
//	txBytes, err := hex.DecodeString(txHex)
//	if err != nil {
//		return "", errors.New("Invalid transaction hex string!")
//	}
//
//
//	txBytes, err = emptyTrans.encodeToBytes()
//	if err != nil {
//		return "", err
//	}
//	return hex.EncodeToString(txBytes), nil
//}
//
//func SignRawTransactionHash(txHash []string, unlockData []TxUnlock) ([]SignaturePubkey, error) {
//	hashes := [][]byte{}
//	for _, h := range txHash {
//		hash, err := hex.DecodeString(h)
//		if err != nil {
//			return nil, errors.New("Invalid transaction hash data!")
//		}
//		hashes = append(hashes, hash)
//	}
//
//	return calcSignaturePubkey(hashes, unlockData)
//}
//
//func InsertSignatureIntoEmptyTransaction(txHex string, sigPub []SignaturePubkey, unlockData []TxUnlock) (string, error) {
//	txBytes, err := hex.DecodeString(txHex)
//	if err != nil {
//		return "", errors.New("Invalid transaction hex data!")
//	}
//
//	emptyTrans, err := DecodeRawTransaction(txBytes)
//	if err != nil {
//		return "", err
//	}
//
//	if unlockData == nil || len(unlockData) == 0 {
//		return "", errors.New("No unlock data found!")
//	}
//
//	if sigPub == nil || len(sigPub) == 0 {
//		return "", errors.New("No signature data found!")
//	}
//
//	if emptyTrans.Vins == nil || len(emptyTrans.Vins) == 0 {
//		return "", errors.New("Invalid empty transaction,no input found!")
//	}
//
//	if emptyTrans.Vouts == nil || len(emptyTrans.Vouts) == 0 {
//		return "", errors.New("Invalid empty transaction,no output found!")
//	}
//
//	if len(emptyTrans.Vins) != len(unlockData) {
//		return "", errors.New("The number of transaction inputs and the unlock data are not match!")
//	}
//
//	if isMultiSig(unlockData[0].LockScript, unlockData[0].RedeemScript) {
//		redeemBytes, _ := hex.DecodeString(unlockData[0].RedeemScript)
//		emptyTrans.Vins[0].ScriptPubkeySignature = redeemBytes
//		for i := 0; i < len(sigPub); i++ {
//			emptyTrans.Witness = append(emptyTrans.Witness, TxWitness{sigPub[i].Signature, sigPub[i].Pubkey})
//		}
//	} else {
//		for i := 0; i < len(emptyTrans.Vins); i++ {
//
//			if sigPub[i].Signature == nil || len(sigPub[i].Signature) != 64 {
//				return "", errors.New("Invalid signature data!")
//			}
//			if sigPub[i].Pubkey == nil || len(sigPub[i].Pubkey) != 33 {
//				return "", errors.New("Invalid pubkey data!")
//			}
//
//			// bech32 branch
//			if unlockData[i].RedeemScript == "" && strings.Index(unlockData[i].LockScript, "0014") == 0 {
//				unlockData[i].RedeemScript = unlockData[i].LockScript
//				unlockData[i].LockScript = "00"
//			}
//
//			if unlockData[i].RedeemScript == "" {
//
//				emptyTrans.Vins[i].ScriptPubkeySignature = sigPub[i].encodeToScript(SigHashAll)
//				if emptyTrans.Witness != nil {
//					emptyTrans.Witness = append(emptyTrans.Witness, TxWitness{})
//				}
//			} else {
//				if emptyTrans.Witness == nil {
//					for j := 0; j < i; j++ {
//						emptyTrans.Witness = append(emptyTrans.Witness, TxWitness{})
//					}
//				}
//				emptyTrans.Witness = append(emptyTrans.Witness, TxWitness{sigPub[i].Signature, sigPub[i].Pubkey})
//				if unlockData[i].RedeemScript == "" {
//					return "", errors.New("Missing redeem script for a P2SH input!")
//				}
//
//				if unlockData[i].LockScript == "00" {
//					emptyTrans.Vins[i].ScriptPubkeySignature = nil
//				} else {
//					redeem, err := hex.DecodeString(unlockData[i].RedeemScript)
//					if err != nil {
//						return "", errors.New("Invlalid redeem script!")
//					}
//					redeem = append([]byte{byte(len(redeem))}, redeem...)
//					emptyTrans.Vins[i].ScriptPubkeySignature = redeem
//				}
//			}
//		}
//	}
//
//	txBytes, err = emptyTrans.encodeToBytes()
//	if err != nil {
//		return "", err
//	}
//	return hex.EncodeToString(txBytes), nil
//}
//
//func VerifyRawTransaction(txHex string, unlockData []TxUnlock) bool {
//	txBytes, err := hex.DecodeString(txHex)
//	if err != nil {
//		return false
//	}
//
//	signedTrans, err := DecodeRawTransaction(txBytes)
//	if err != nil {
//		return false
//	}
//
//	if len(signedTrans.Vins) != len(unlockData) {
//		return false
//	}
//
//	var sigAndPub []SignaturePubkey
//	if signedTrans.Witness == nil {
//		for _, sp := range signedTrans.Vins {
//			tmp, err := decodeFromScriptBytes(sp.ScriptPubkeySignature)
//			if err != nil {
//				return false
//			}
//			sigAndPub = append(sigAndPub, *tmp)
//		}
//	} else {
//		for i := 0; i < len(signedTrans.Vins); i++ {
//			if signedTrans.Witness[i].Signature == nil {
//				tmp, err := decodeFromScriptBytes(signedTrans.Vins[i].ScriptPubkeySignature)
//				if err != nil {
//					return false
//				}
//				sigAndPub = append(sigAndPub, *tmp)
//			} else {
//				sigAndPub = append(sigAndPub, SignaturePubkey{signedTrans.Witness[i].Signature, signedTrans.Witness[i].Pubkey})
//				if strings.Index(unlockData[i].LockScript, "0014") == 0 {
//					continue
//				}
//				unlockData[i].RedeemScript = hex.EncodeToString(signedTrans.Vins[i].ScriptPubkeySignature[1:])
//			}
//		}
//	}
//
//	signedTrans.Witness = nil
//
//	hashes, err := signedTrans.getHashesForSig(unlockData)
//	if err != nil {
//		return false
//	}
//
//	return verifyHashes(hashes, sigAndPub)
//}
