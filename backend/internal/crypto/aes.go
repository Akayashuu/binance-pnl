// Package crypto provides AES-GCM symmetric encryption used to protect
// Binance API secrets at rest in the settings table.
//
// The key is supplied via the ENCRYPTION_KEY env var as base64. Generate one
// with: openssl rand -base64 32
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKey is returned when the supplied key is not 32 raw bytes.
var ErrInvalidKey = errors.New("encryption key must be 32 bytes (base64-encoded)")

// AESGCM encrypts and decrypts strings using AES-256-GCM.
type AESGCM struct {
	gcm cipher.AEAD
}

// NewAESGCM constructs an AESGCM from a base64-encoded 32-byte key.
func NewAESGCM(base64Key string) (*AESGCM, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}
	return &AESGCM{gcm: gcm}, nil
}

// Encrypt returns base64(nonce || ciphertext).
func (a *AESGCM) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, a.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := a.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt accepts a value previously produced by Encrypt.
func (a *AESGCM) Decrypt(payload string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(raw) < a.gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := raw[:a.gcm.NonceSize()], raw[a.gcm.NonceSize():]
	pt, err := a.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}
