package utils

import (
	"crypto/cipher"
	"crypto/des"
	"errors"
	"github.com/maczh/mgin/logs"
)

func DesEncrypt(origData, key []byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	origData = PKCS5Padding(origData, block.BlockSize())
	// origData = ZeroPadding(origData, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, key)
	crypted := make([]byte, len(origData))
	// 根据CryptBlocks方法的说明，如下方式初始化crypted也可以
	// crypted := origData
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func DesDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, key)
	origData := make([]byte, len(crypted))
	// origData := crypted
	blockMode.CryptBlocks(origData, crypted)
	origData, err = PKCS5UnPadding(origData)
	// origData = ZeroUnPadding(origData)
	return origData, err
}

func DESEncrypt(data string, key string) string {
	encdata, err := DesEncrypt([]byte(data), []byte(key))
	if err != nil {
		logs.Error("DES加密错误:{}", err.Error())
		return ""
	} else {
		return string(encdata)
	}
}

func DESDecrypt(data string, key string) string {
	text, err := DesDecrypt([]byte(data), []byte(key))
	if err != nil {
		logs.Debug("DES解密错误:{}", err.Error())
		return ""
	} else {
		return string(text)
	}
}

func DesEcbEncrypt(src, key []byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	src = PKCS5Padding(src, bs)
	if len(src)%bs != 0 {
		return nil, errors.New("Need a multiple of the blocksize")
	}
	out := make([]byte, len(src))
	dst := out
	for len(src) > 0 {
		block.Encrypt(dst, src[:bs])
		src = src[bs:]
		dst = dst[bs:]
	}
	return out, nil
}

func DesEcbDecrypt(src, key []byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(src))
	dst := out
	bs := block.BlockSize()
	if len(src)%bs != 0 {
		return nil, errors.New("crypto/cipher: input not full blocks")
	}
	for len(src) > 0 {
		block.Decrypt(dst, src[:bs])
		src = src[bs:]
		dst = dst[bs:]
	}
	out, err = PKCS5UnPadding(out)
	return out, err
}

func DESEncryptECB(data string, key string) string {
	encdata, err := DesEcbEncrypt([]byte(data), []byte(key))
	if err != nil {
		logs.Error("DES加密错误:{}", err.Error())
		return ""
	} else {
		return string(encdata)
	}
}

func DESDecryptECB(data string, key string) string {
	text, err := DesEcbDecrypt([]byte(data), []byte(key))
	if err != nil {
		logs.Debug("DES解密错误:{}", err.Error())
		return ""
	} else {
		return string(text)
	}
}
