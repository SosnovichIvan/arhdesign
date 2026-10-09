package account

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordPolicyBoundariesAndComparison(t *testing.T) {
	hasher := PasswordHasher{}
	for name, password := range map[string]string{
		"too short": strings.Repeat("я", 14),
		"too long":  strings.Repeat("я", 129),
		"blocked":   "passwordpassword",
		"control":   "valid length but\nnewline",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := hasher.Hash(password, "customer", "customer@example.com"); !errors.Is(err, ErrInvalidPassword) {
				t.Fatalf("error = %v, want password policy rejection", err)
			}
		})
	}
	for name, password := range map[string]string{
		"minimum": strings.Repeat("я", 15),
		"maximum": strings.Repeat("я", 128),
		"spaces":  "фраза с пробелами 2026",
	} {
		t.Run(name, func(t *testing.T) {
			hash, err := hasher.Hash(password, "customer", "customer@example.com")
			if err != nil || hash.Algorithm != "argon2id" || !strings.HasPrefix(hash.PHC, "$argon2id$") {
				t.Fatalf("hash = %#v, error = %v", hash, err)
			}
			matched, err := hasher.Compare(hash.PHC, password)
			if err != nil || !matched {
				t.Fatalf("correct password matched = %v, error = %v", matched, err)
			}
			matched, err = hasher.Compare(hash.PHC, password+"x")
			if err != nil || matched {
				t.Fatalf("wrong password matched = %v, error = %v", matched, err)
			}
		})
	}
}

func TestPasswordNFCAndMalformedHash(t *testing.T) {
	hasher := PasswordHasher{}
	decomposed := "Cafe\u0301 Cafe\u0301 secure"
	hash, err := hasher.Hash(decomposed, "customer", "customer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	matched, err := hasher.Compare(hash.PHC, "Café Café secure")
	if err != nil || !matched {
		t.Fatalf("NFC equivalent matched = %v, error = %v", matched, err)
	}
	for _, malformed := range []string{"", "$argon2i$v=19$m=1,t=1,p=1$bad$bad", "$argon2id$v=no$m=1,t=1,p=1$bad$bad", "$argon2id$v=19$m=0,t=0,p=0$bad$bad", "$argon2id$v=19$m=999999999,t=2,p=1$c2FsdHNhbHQ$MTIzNDU2Nzg5MDEyMzQ1Ng"} {
		if matched, err := hasher.Compare(malformed, "irrelevant"); err == nil || matched {
			t.Fatalf("malformed hash %q must fail safely", malformed)
		}
	}
}

func TestBootstrapPasswordUsesConfiguredSystemSecret(t *testing.T) {
	hasher := PasswordHasher{}
	hash, err := hasher.HashBootstrap("legacy-pass")
	if err != nil {
		t.Fatal(err)
	}
	matched, err := hasher.Compare(hash.PHC, "legacy-pass")
	if err != nil || !matched {
		t.Fatalf("configured bootstrap password matched = %v, error = %v", matched, err)
	}
	if _, err = hasher.Hash("legacy-pass", "admin", "admin@example.com"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("regular account policy must still reject the legacy password: %v", err)
	}
	for _, password := range []string{"", strings.Repeat("x", 129), "line\nbreak"} {
		if _, err = hasher.HashBootstrap(password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("unsafe bootstrap password accepted: %v", err)
		}
	}
}
