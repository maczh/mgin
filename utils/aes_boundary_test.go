package utils

import "testing"

func TestAESBase64DecryptRejectsTruncatedCiphertext(t *testing.T) {
	if _, err := AESBase64Decrypt("MTIzNDU=", "1234567890123456", []byte("1234567890123456")); err == nil {
		t.Fatal("expected invalid ciphertext length error")
	}
}
