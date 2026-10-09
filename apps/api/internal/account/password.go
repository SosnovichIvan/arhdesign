package account

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	argonMemory      = 19_456
	argonIterations  = 2
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
)

var ErrInvalidPassword = errors.New("password does not satisfy policy")

type PasswordHash struct {
	PHC        string
	Algorithm  string
	Parameters map[string]any
	Version    int
}

type PasswordHasher struct{}

func (PasswordHasher) Hash(password, login, email string) (PasswordHash, error) {
	normalized, err := validatePassword(password, login, email)
	if err != nil {
		return PasswordHash{}, err
	}
	return hashNormalizedPassword(normalized)
}

func (PasswordHasher) HashBootstrap(password string) (PasswordHash, error) {
	normalized := norm.NFC.String(password)
	length := len([]rune(normalized))
	if length < 1 || length > 128 {
		return PasswordHash{}, ErrInvalidPassword
	}
	for _, character := range normalized {
		if unicode.IsControl(character) {
			return PasswordHash{}, ErrInvalidPassword
		}
	}
	return hashNormalizedPassword(normalized)
}

func hashNormalizedPassword(normalized string) (PasswordHash, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return PasswordHash{}, fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(normalized), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	phc := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonIterations, argonParallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
	return PasswordHash{
		PHC:       phc,
		Algorithm: "argon2id",
		Parameters: map[string]any{
			"memoryKiB": argonMemory, "iterations": argonIterations, "parallelism": argonParallelism, "saltBytes": argonSaltLength, "keyBytes": argonKeyLength,
		},
		Version: 1,
	}, nil
}

func (PasswordHasher) Compare(phc, password string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid password hash")
	}
	version, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v="))
	if err != nil || version != argon2.Version {
		return false, errors.New("unsupported password hash version")
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || memory < 8*1024 || memory > 1024*1024 || iterations == 0 || iterations > 10 || parallelism == 0 || parallelism > 16 {
		return false, errors.New("invalid password hash parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false, errors.New("invalid password salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 64 {
		return false, errors.New("invalid password digest")
	}
	got := argon2.IDKey([]byte(norm.NFC.String(password)), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func validatePassword(password, login, email string) (string, error) {
	normalized := norm.NFC.String(password)
	length := len([]rune(normalized))
	if length < 15 || length > 128 {
		return "", ErrInvalidPassword
	}
	for _, character := range normalized {
		if unicode.IsControl(character) {
			return "", ErrInvalidPassword
		}
	}
	lower := strings.ToLower(normalized)
	blocked := map[string]struct{}{
		"passwordpassword": {}, "qwertyqwerty123": {}, "designer-svetlana": {}, "designer-svetlana.ru": {}, "архдизайнархдизайн": {},
	}
	login = strings.ToLower(norm.NFC.String(login))
	emailLocal := strings.ToLower(strings.SplitN(norm.NFC.String(email), "@", 2)[0])
	if len([]rune(login)) >= 5 {
		blocked[login+login] = struct{}{}
	}
	if len([]rune(emailLocal)) >= 5 {
		blocked[emailLocal+emailLocal] = struct{}{}
	}
	if _, exists := blocked[lower]; exists {
		return "", ErrInvalidPassword
	}
	return normalized, nil
}
