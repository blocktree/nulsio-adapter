package nulsio_addrdec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestSha256hash160(t *testing.T) {
	Sha256hash160([]byte("03ee8e9ed5440849f0704f067e4f0f7ba29da3f53051973b5babb81c78313e1139"))

	s := ShortToBytes(8964)
	fmt.Println(s)

	res,_ := hex.DecodeString("03ee8e9ed5440849f0704f067e4f0f7ba29da3f53051973b5babb81c78313e1139")
	result, _ := GetAddressByPub(res)

	fmt.Print(result)
}

func TestHex(t *testing.T) {
	result,_ :=GetInputOwnerKey("002082e51bfa483e246177c6d66a3e62d864ad380ecc98d31fed217724a3f83b162e",257)
	fmt.Print(result)
	data,_ := hex.DecodeString("02008cc986706b0100ffffffff01230020ed9d26bb973ac4c2e59ecf8a7bb53c4c35ee9b6f933fc71f40817ddbc98a390e0000c2eb0b000000000000000000000217042301ab142cb6d65838132af4223089a267ef93c456d8a08601000000000000000000000017042301c7d2c6f4d63c50d1ed812d677a7685b59e21d7e8409ae20b0000000000000000000000")
	h := sha256.New()
	h.Write(data)
	h2 := sha256.New()
	h2.Write(h.Sum(nil))
	data = h2.Sum(nil)
	fmt.Println(hex.EncodeToString(data))
}

