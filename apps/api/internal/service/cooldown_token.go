package service

import (
	"crypto/hmac"
	"crypto/sha256"
)

func CooldownTokenHash(secret []byte, token string) []byte {
	hasher := hmac.New(sha256.New, secret)
	_, _ = hasher.Write([]byte(token))
	return hasher.Sum(nil)
}
