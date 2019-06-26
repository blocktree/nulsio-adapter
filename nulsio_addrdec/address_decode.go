package nulsio_addrdec

import (
	"crypto/sha256"
	"github.com/blocktree/go-owcdrivers/addressEncoder"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ripemd160"
)

const (
	btcAlphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	addType = 1
)

var (
	NULSIO_mainnetAddressP2PKH         = addressEncoder.AddressType{EncodeType: "base58", Alphabet: btcAlphabet, ChecksumType: "doubleSHA256", HashType: "h160", HashLen: 20, Prefix: []byte{0x04, 0x00, 0x01}, Suffix: nil}
	NULSIO_mainnetPrivateWIFCompressed = addressEncoder.AddressType{EncodeType: "base58", Alphabet: btcAlphabet, ChecksumType: "doubleSHA256", HashType: "", HashLen: 32, Prefix: []byte{0x80}, Suffix: []byte{0x01}}

	Default = AddressDecoderV2{}
)

//AddressDecoderV2
type AddressDecoderV2 struct {
	IsTestNet bool
}

//AddressDecode 地址解析
func (dec *AddressDecoderV2) AddressDecode(addr string, opts ...interface{}) ([]byte, error) {

	cfg := NULSIO_mainnetAddressP2PKH

	if len(opts) > 0 {
		for _, opt := range opts {
			if at, ok := opt.(addressEncoder.AddressType); ok {
				cfg = at
			}
		}
	}

	return addressEncoder.AddressDecode(addr, cfg)
}

//AddressEncode 地址编码
func (dec *AddressDecoderV2) AddressEncode(hash []byte, opts ...interface{}) (string, error) {

	cfg := NULSIO_mainnetAddressP2PKH

	if len(opts) > 0 {
		for _, opt := range opts {
			if at, ok := opt.(addressEncoder.AddressType); ok {
				cfg = at
			}
		}
	}

	address := addressEncoder.AddressEncode(hash, cfg)
	return address, nil
}

//sha256之后hash160
func Sha256hash160(pub []byte) []byte{
	h := sha256.New()
	h.Write(pub)
	hasher := ripemd160.New()
	hasher.Write(h.Sum(nil))
	hashBytes := hasher.Sum(nil)
	return hashBytes
}

func GetAddressByPub(pub []byte) (string,error){
	pubPart := Sha256hash160(pub)
	if len(pubPart)!= 20 {
		return "",errors.New("pubPart len not 20")
	}
	chainPart := ShortToBytes(8964)
	resultPart1 := make([]byte,23)
	for index,v := range chainPart{
		resultPart1[index] = v
	}
	resultPart1[2] = addType
	for index,v := range pubPart{
		resultPart1[index + 3] = v
	}
	xor := GetXor(resultPart1)
	resultPart2 := make([]byte,24)
	for index,v := range resultPart1{
		resultPart2[index] = v
	}
	resultPart2[23] = xor
	resultBytes := Base58Encode(resultPart2)
	return string(resultBytes),nil
}

//异或方法
func GetXor(body []byte) byte {
	var xor byte
	xor = 0x00
	for i := 0; i < len(body); i++ {
		xor = byte(xor) ^ body[i]
	}
	return xor
}

//chainid转换
func ShortToBytes(val int) []byte {
	bytes := make([]byte, 2)
	bytes[1] = (byte)(0xFF & (val >> 8))
	bytes[0] = (byte)(0xFF & (val >> 0))
	return bytes
}
