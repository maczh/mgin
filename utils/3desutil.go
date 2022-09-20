package utils

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"errors"
	"github.com/maczh/mgin/logs"
)

// 3DES加密
func TripleDesEncrypt(origData, key []byte, pcks5padding bool) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	if pcks5padding {
		origData = PKCS5Padding(origData, block.BlockSize())
	} else {
		origData = ZeroPadding(origData, block.BlockSize())
	}
	blockMode := cipher.NewCBCEncrypter(block, key[:8])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

// 3DES解密
func TripleDesDecrypt(crypted, key []byte, pcks5padding bool) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, key[:8])
	origData := make([]byte, len(crypted))
	// origData := crypted
	blockMode.CryptBlocks(origData, crypted)
	if pcks5padding {
		origData, err = PKCS5UnPadding(origData)
		return origData, err
	} else {
		origData = ZeroUnPadding(origData)
		return origData, nil
	}
}

func ZeroPadding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{0}, padding)
	return append(ciphertext, padtext...)
}

func ZeroUnPadding(origData []byte) []byte {
	return bytes.TrimRightFunc(origData, func(r rune) bool {
		return r == rune(0)
	})
}

func TripleDESEncrypt(data string, key string, pcks5padding bool) string {
	encdata, err := TripleDesEncrypt([]byte(data), []byte(key), pcks5padding)
	if err != nil {
		logs.Error("TripleDES加密错误:{}", err.Error())
		return ""
	} else {
		return string(encdata)
	}
}

func TripleDESDecrypt(data string, key string, pcks5padding bool) string {
	text, err := TripleDesDecrypt([]byte(data), []byte(key), pcks5padding)
	if err != nil {
		logs.Error("TripleDES解密错误:{}", err.Error())
		return ""
	} else {
		return string(text)
	}
}

//Des encryption
func encryptDesEcb(origData, key []byte) ([]byte, error) {
	if len(origData) < 1 || len(key) < 1 {
		return nil, errors.New("wrong data or key")
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(origData)%bs != 0 {
		return nil, errors.New("wrong padding")
	}
	out := make([]byte, len(origData))
	dst := out
	for len(origData) > 0 {
		block.Encrypt(dst, origData[:bs])
		origData = origData[bs:]
		dst = dst[bs:]
	}
	return out, nil
}

//Des decryption
func decryptDesEcb(crypted, key []byte) ([]byte, error) {
	if len(crypted) < 1 || len(key) < 1 {
		return nil, errors.New("wrong data or key")
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(crypted))
	dst := out
	bs := block.BlockSize()
	if len(crypted)%bs != 0 {
		return nil, errors.New("wrong crypted size")
	}

	for len(crypted) > 0 {
		block.Decrypt(dst, crypted[:bs])
		crypted = crypted[bs:]
		dst = dst[bs:]
	}

	return out, nil
}

//[golang ECB 3DES Encrypt]
func TripleEcbDesEncrypt(origData, key []byte) ([]byte, error) {
	tkey := make([]byte, 24, 24)
	copy(tkey, key)
	k1 := tkey[:8]
	k2 := tkey[8:16]
	k3 := tkey[16:]

	block, err := des.NewCipher(k1)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	origData = PKCS5Padding(origData, bs)

	buf1, err := encryptDesEcb(origData, k1)
	if err != nil {
		return nil, err
	}
	buf2, err := decryptDesEcb(buf1, k2)
	if err != nil {
		return nil, err
	}
	out, err := encryptDesEcb(buf2, k3)
	if err != nil {
		return nil, err
	}
	return out, nil
}

//[golang ECB 3DES Decrypt]
func TripleEcbDesDecrypt(crypted, key []byte) ([]byte, error) {
	tkey := make([]byte, 24, 24)
	copy(tkey, key)
	k1 := tkey[:8]
	k2 := tkey[8:16]
	k3 := tkey[16:]
	buf1, err := decryptDesEcb(crypted, k3)
	if err != nil {
		return nil, err
	}
	buf2, err := encryptDesEcb(buf1, k2)
	if err != nil {
		return nil, err
	}
	out, err := decryptDesEcb(buf2, k1)
	if err != nil {
		return nil, err
	}
	out, err = PKCS5UnPadding(out)
	return out, err
}
