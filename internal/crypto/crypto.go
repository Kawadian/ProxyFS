package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
	nonceLen     = 12
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidPassword   = errors.New("invalid password")
)

// Encrypt encrypts plaintext with AES-256-GCM using the provided 32-byte key.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("encryption key must be at least 32 bytes")
	}
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-256-GCM ciphertext produced by Encrypt.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("decryption key must be at least 32 bytes")
	}
	if len(ciphertext) < nonceLen {
		return nil, ErrInvalidCiphertext
	}
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := ciphertext[:nonceLen]
	data := ciphertext[nonceLen:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}

// HashPassword hashes a password with Argon2id and returns a base64-encoded record.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	record := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return record, nil
}

// VerifyPassword checks a password against an Argon2id hash record.
func VerifyPassword(password, encoded string) (bool, error) {
	if !stringsHasPrefix(encoded, "$argon2id$") {
		return false, ErrInvalidPassword
	}
	parts := splitArgonRecord(encoded)
	if len(parts) != 6 {
		return false, ErrInvalidPassword
	}
	var version int
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidPassword
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrInvalidPassword
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidPassword
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidPassword
	}
	actual := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expected)))
	if subtle.ConstantTimeCompare(actual, expected) == 1 {
		return true, nil
	}
	return false, nil
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func splitArgonRecord(encoded string) []string {
	var parts []string
	current := ""
	for i := 0; i < len(encoded); i++ {
		if encoded[i] == '$' {
			parts = append(parts, current)
			current = ""
			continue
		}
		current += string(encoded[i])
	}
	parts = append(parts, current)
	return parts
}
