package service

import (
	"bytes"
	"testing"
)

func TestCooldownTokenHash(t *testing.T) {
	secret := []byte("test-secret")
	first := CooldownTokenHash(secret, "token-a")
	second := CooldownTokenHash(secret, "token-a")
	different := CooldownTokenHash(secret, "token-b")
	if !bytes.Equal(first, second) {
		t.Fatal("same token and secret must produce the same HMAC")
	}
	if bytes.Equal(first, different) {
		t.Fatal("different tokens must produce different HMACs")
	}
}
