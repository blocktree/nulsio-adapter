package nulsio_addrdec

import "encoding/hex"

func uint32ToByteArrayLE(val int64, out []byte, offset int) {
	out[offset] = (byte)(0xFF & val)
	out[offset+1] = (byte)(0xFF & (val >> 8))
	out[offset+2] = (byte)(0xFF & (val >> 16))
	out[offset+3] = (byte)(0xFF & (val >> 24))
}

func uint64ToByteArrayLE(val int64, out []byte, offset int) {
	out[offset] = (byte)(0xFF & val)
	out[offset+1] = (byte)(0xFF & (val >> 8))
	out[offset+2] = (byte)(0xFF & (val >> 16))
	out[offset+3] = (byte)(0xFF & (val >> 24))
	out[offset+4] = (byte)(0xFF & (val >> 32))
	out[offset+5] = (byte)(0xFF & (val >> 40))
	out[offset+6] = (byte)(0xFF & (val >> 48))
	out[offset+7] = (byte)(0xFF & (val >> 56))
}

func sizeOf(value int64) int64 {
	// if negative, it's actually a very large unsigned long value
	if value < 0 {
		// 1 marker + 8 data bytes
		return 9
	}
	if value < 253 {
		// 1 data byte
		return 1
	}
	if value <= 0xFFFF {
		// 1 marker + 2 data bytes
		return 3
	}
	if value <= 0xFFFFFFFF {
		// 1 marker + 4 data bytes
		return 5
	}
	// 1 marker + 8 data bytes
	return 9
}

func encode(value int64) []byte {
	bytes := make([]byte, 0)
	switch sizeOf(value) {
	case 1:
		bytes = append(bytes, byte(value))
		return bytes
	case 3:
		bytes = append(bytes, 253)
		bytes = append(bytes, byte(value))
		bytes = append(bytes, byte((value >> 8)))
		return bytes
	case 5:
		bytes = append(bytes, 254)
		bytes = append(bytes, 0, 0, 0, 0)
		uint32ToByteArrayLE(value, bytes, 1)
		return bytes
	default:
		bytes = append(bytes, 255)
		bytes = append(bytes, 0, 0, 0, 0, 0, 0, 0, 0)
		uint64ToByteArrayLE(value, bytes, 1)
		return bytes
	}
}


func GetInputOwnerKey(hexStr string, index int64) ([]byte, error) {
	scriptPubkey, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	resByte := encode(index)
	scriptPubkey = append(scriptPubkey, resByte...)
	return scriptPubkey,nil
}
