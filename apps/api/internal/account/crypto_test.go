package account

import (
	"bytes"
	"errors"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

func TestTokenManagerAndPayloadCipher(t *testing.T) {
	manager, err := NewTokenManager(bytes.Repeat([]byte{1}, 32), 2)
	if err != nil {
		t.Fatal(err)
	}
	token, hash, version, err := manager.New()
	if err != nil || len(token) != 43 || len(hash) != 32 || version != 2 {
		t.Fatalf("token length = %d, hash length = %d, version = %d, error = %v", len(token), len(hash), version, err)
	}
	if bytes.Equal([]byte(token), hash) || bytes.Equal(hash, manager.Hash(token+"x")) {
		t.Fatal("raw token must not equal storage hash and distinct values must hash differently")
	}

	cipher, err := NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 3)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyVersion, err := cipher.Encrypt([]byte("secret payload"), "message.type")
	if err != nil || keyVersion != 3 || bytes.Contains(ciphertext, []byte("secret payload")) {
		t.Fatalf("ciphertext = %q, version = %d, error = %v", ciphertext, keyVersion, err)
	}
	plaintext, err := cipher.Decrypt(ciphertext, "message.type", 3)
	if err != nil || string(plaintext) != "secret payload" {
		t.Fatalf("plaintext = %q, error = %v", plaintext, err)
	}
	if _, err := cipher.Decrypt(ciphertext, "other.type", 3); err == nil {
		t.Fatal("associated message type mismatch must fail")
	}
	if _, err := cipher.Decrypt(ciphertext, "message.type", 4); err == nil {
		t.Fatal("unknown key version must fail")
	}
}

func TestCryptoConstructorsRejectWeakConfiguration(t *testing.T) {
	if _, err := NewTokenManager([]byte("short"), 1); err == nil {
		t.Fatal("short HMAC key must fail")
	}
	if _, err := NewPayloadCipher([]byte("short"), 1); err == nil {
		t.Fatal("short encryption key must fail")
	}
	manager, _ := NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
	manager.random = failingReader{}
	if _, _, _, err := manager.New(); err == nil {
		t.Fatal("token entropy failure must propagate")
	}
	cipher, _ := NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	cipher.random = failingReader{}
	if _, _, err := cipher.Encrypt([]byte("payload"), "message.type"); err == nil {
		t.Fatal("nonce entropy failure must propagate")
	}
}

func TestTokenPurposesAreSeparated(t *testing.T) {
	manager, err := NewTokenManager(bytes.Repeat([]byte{7}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	token := "same-raw-token"
	access := manager.HashForPurpose(token, accessPurpose)
	refresh := manager.HashForPurpose(token, refreshPurpose)
	csrf := manager.HashForPurpose(token, csrfPurpose)
	if bytes.Equal(access, refresh) || bytes.Equal(access, csrf) || bytes.Equal(refresh, csrf) {
		t.Fatal("purpose-separated hashes must differ")
	}
}
