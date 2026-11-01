package socket

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

type Vault struct{ aead cipher.AEAD }

func NewVault(encoded string) (*Vault, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("Socket encryption key must be base64 encoded 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	return &Vault{aead: aead}, err
}
func (v *Vault) Seal(token, tenant string) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return v.aead.Seal(nonce, nonce, []byte(token), []byte(tenant)), nil
}
func (v *Vault) Open(data []byte, tenant string) (string, error) {
	n := v.aead.NonceSize()
	if len(data) < n {
		return "", errors.New("Socket token cannot be decrypted")
	}
	plain, err := v.aead.Open(nil, data[:n], data[n:], []byte(tenant))
	if err != nil {
		return "", errors.New("Socket token cannot be decrypted; save it again")
	}
	return string(plain), nil
}
