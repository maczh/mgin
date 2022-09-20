package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"github.com/maczh/mgin/logs"
)

func PKCS5Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func PKCS5UnPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	unPadding := int(origData[length-1])
	if (length - unPadding) < 0 {
		return origData, errors.New("解密错误")
	} else {
		return origData[:(length - unPadding)], nil
	}
}

func AESBase64Encrypt(origin_data string, key string, iv []byte) (base64_result string, err error) {
	var block cipher.Block
	if block, err = aes.NewCipher([]byte(key)); err != nil {
		logs.Error("AES加密错误:{}", err.Error())
		return
	}
	encrypt := cipher.NewCBCEncrypter(block, iv)
	var source []byte = PKCS5Padding([]byte(origin_data), 16)
	var dst []byte = make([]byte, len(source))
	encrypt.CryptBlocks(dst, source)
	base64_result = base64.StdEncoding.EncodeToString(dst)
	return
}

func AESBase64Decrypt(encrypt_data string, key string, iv []byte) (string, error) {
	var block cipher.Block
	var err error
	if block, err = aes.NewCipher([]byte(key)); err != nil {
		logs.Error("AES加密错误:{}", err.Error())
		return "", err
	}
	encrypt := cipher.NewCBCDecrypter(block, iv)

	var source []byte
	if source, err = base64.StdEncoding.DecodeString(encrypt_data); err != nil {
		logs.Error("AES解密错误:{}", err.Error())
		return "", err
	}
	var dst []byte = make([]byte, len(source))
	encrypt.CryptBlocks(dst, source)
	oridata, err := PKCS5UnPadding(dst)
	if err != nil {
		logs.Error("AES解密错误:{}", err.Error())
	}
	return string(oridata), err
}

func AesEncrypt(origData []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	origData = PKCS5Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	encrypted := make([]byte, len(origData))
	blockMode.CryptBlocks(encrypted, origData)
	return encrypted, nil
}

func AESEncrypt(data string, key string) string {
	encdata, err := AesEncrypt([]byte(data), []byte(key))
	if err != nil {
		logs.Error("AES加密错误:{}", err.Error())
		return ""
	}
	return string(encdata)
}

func AesDecrypt(encrypted []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(encrypted))
	blockMode.CryptBlocks(origData, encrypted)
	origData, err = PKCS5UnPadding(origData)
	return origData, err
}

func AESDecrypt(data string, key string) string {
	text, _ := AesDecrypt([]byte(data), []byte(key))
	return string(text)
}

func AesDecryptEcb(crypted, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		logs.Error("AES密钥错误:{}", err.Error())
		return "", err
	}
	blockMode := NewECBDecrypter(block)
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, []byte(crypted))
	origData, err = PKCS5UnPadding(origData)
	return string(origData), err
}

func AesEncryptEcb(src, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		logs.Error("AES密钥错误:{}", err.Error())
		return "", err
	}
	if src == "" {
		logs.Error("AES原始数据为空")
		return "", errors.New("AES明文数据为空")
	}
	ecb := NewECBEncrypter(block)
	content := []byte(src)
	content = PKCS5Padding(content, block.BlockSize())
	crypted := make([]byte, len(content))
	ecb.CryptBlocks(crypted, content)
	return string(crypted), nil
}

type ecb struct {
	b         cipher.Block
	blockSize int
}

func newECB(b cipher.Block) *ecb {
	return &ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

type ecbEncrypter ecb

// NewECBEncrypter returns a BlockMode which encrypts in electronic code book
// mode, using the given Block.
func NewECBEncrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(b))
}
func (x *ecbEncrypter) BlockSize() int { return x.blockSize }
func (x *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		logs.Error("crypto/cipher: input not full blocks")
		return
	}
	if len(dst) < len(src) {
		logs.Error("crypto/cipher: output smaller than input")
		return
	}
	for len(src) > 0 {
		x.b.Encrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

type ecbDecrypter ecb

// NewECBDecrypter returns a BlockMode which decrypts in electronic code book
// mode, using the given Block.
func NewECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}
func (x *ecbDecrypter) BlockSize() int { return x.blockSize }
func (x *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		logs.Error("crypto/cipher: input not full blocks")
		return
	}
	if len(dst) < len(src) {
		logs.Error("crypto/cipher: output smaller than input")
		return
	}
	for len(src) > 0 {
		x.b.Decrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}
