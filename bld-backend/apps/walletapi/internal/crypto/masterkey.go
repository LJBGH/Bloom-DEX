package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type MasterKey struct {
	aead cipher.AEAD
}

// NewMasterKeyFromBase64 从 base64 编码的 32 字节密钥创建 MasterKey
func NewMasterKeyFromBase64(b64 string) (*MasterKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(raw) != 32 {
		return nil, errors.New("CustodyMasterKey must be 32 bytes (base64 encoded)")
	}
	// 创建 AES 加密器
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	// 创建 GCM 加密器
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// 返回 MasterKey 实例
	return &MasterKey{aead: aead}, nil
}

// EncryptToBase64 加密明文并返回 base64 编码的字符串
func (k *MasterKey) EncryptToBase64(plaintext []byte) (string, error) {
	// 生成随机 nonce
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// 加密明文
	ciphertext := k.aead.Seal(nil, nonce, plaintext, nil)
	// 将 nonce 和 ciphertext 拼接在一起
	out := append(nonce, ciphertext...)
	// 返回 base64 编码的字符串
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptFromBase64 从 base64 编码的密文解密为明文
func (k *MasterKey) DecryptFromBase64(encB64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		return nil, err
	}
	nonceSize := k.aead.NonceSize()
	if len(raw) < nonceSize {
		return nil, errors.New("invalid ciphertext")
	}
	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]
	plain, err := k.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}
