package account

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	verificationPurpose  = "arhdesign/email-verification/v1"
	passwordResetPurpose = "arhdesign/password-reset/v1"
	accessPurpose        = "arhdesign/access-session/v1"
	refreshPurpose       = "arhdesign/refresh-session/v1"
	csrfPurpose          = "arhdesign/csrf-session/v1"
)

type TokenManager struct {
	key     []byte
	version int
	random  io.Reader
}

func NewTokenManager(key []byte, version int) (TokenManager, error) {
	if len(key) < 32 || version < 1 {
		return TokenManager{}, fmt.Errorf("token HMAC key must contain at least 32 bytes and version must be positive")
	}
	return TokenManager{key: append([]byte(nil), key...), version: version, random: rand.Reader}, nil
}

func (manager TokenManager) New() (string, []byte, int, error) {
	return manager.NewForPurpose(verificationPurpose)
}

func (manager TokenManager) NewForPurpose(purpose string) (string, []byte, int, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(manager.random, raw); err != nil {
		return "", nil, 0, fmt.Errorf("generate opaque token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, manager.HashForPurpose(token, purpose), manager.version, nil
}

func (manager TokenManager) Hash(token string) []byte {
	return manager.HashForPurpose(token, verificationPurpose)
}

func (manager TokenManager) HashForPurpose(token, purpose string) []byte {
	mac := hmac.New(sha256.New, manager.key)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(token))
	return mac.Sum(nil)
}

func (manager TokenManager) Version() int { return manager.version }

type PayloadCipher struct {
	aead    cipher.AEAD
	version int
	random  io.Reader
}

func NewPayloadCipher(key []byte, version int) (PayloadCipher, error) {
	if len(key) != 32 || version < 1 {
		return PayloadCipher{}, fmt.Errorf("outbox encryption key must contain exactly 32 bytes and version must be positive")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return PayloadCipher{}, fmt.Errorf("create outbox cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return PayloadCipher{}, fmt.Errorf("create outbox AEAD: %w", err)
	}
	return PayloadCipher{aead: aead, version: version, random: rand.Reader}, nil
}

func (payloadCipher PayloadCipher) Encrypt(plaintext []byte, messageType string) ([]byte, int, error) {
	nonce := make([]byte, payloadCipher.aead.NonceSize())
	if _, err := io.ReadFull(payloadCipher.random, nonce); err != nil {
		return nil, 0, fmt.Errorf("generate outbox nonce: %w", err)
	}
	sealed := payloadCipher.aead.Seal(nil, nonce, plaintext, []byte(messageType))
	return append(nonce, sealed...), payloadCipher.version, nil
}

func (payloadCipher PayloadCipher) Decrypt(ciphertext []byte, messageType string, keyVersion int) ([]byte, error) {
	if keyVersion != payloadCipher.version || len(ciphertext) < payloadCipher.aead.NonceSize() {
		return nil, fmt.Errorf("unsupported or malformed encrypted outbox payload")
	}
	nonce, sealed := ciphertext[:payloadCipher.aead.NonceSize()], ciphertext[payloadCipher.aead.NonceSize():]
	plaintext, err := payloadCipher.aead.Open(nil, nonce, sealed, []byte(messageType))
	if err != nil {
		return nil, fmt.Errorf("decrypt outbox payload: %w", err)
	}
	return plaintext, nil
}
