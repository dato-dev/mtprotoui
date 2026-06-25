package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
)

type SSHCredentials struct {
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

func Encrypt(key []byte, creds SSHCredentials) (ciphertext, nonce []byte, err error) {
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return nil, nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func Decrypt(key, ciphertext, nonce []byte) (SSHCredentials, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return SSHCredentials{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SSHCredentials{}, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return SSHCredentials{}, fmt.Errorf("decrypt credentials: %w", err)
	}

	var creds SSHCredentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return SSHCredentials{}, err
	}
	return creds, nil
}
