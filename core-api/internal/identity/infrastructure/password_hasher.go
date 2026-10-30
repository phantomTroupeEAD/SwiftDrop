package infrastructure

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
)

// StandardPasswordHasher implements application.PasswordHasher using salted SHA-256 with key derivation.
// It format-encodes hashes as: "$sd$sha256$<salt_hex>$<hash_hex>"
// This provides secure, timing-attack resistant password verification with zero external C-dependencies.
type StandardPasswordHasher struct {
	iterations int
}

// NewStandardPasswordHasher constructs a new password hasher.
func NewStandardPasswordHasher() *StandardPasswordHasher {
	return &StandardPasswordHasher{
		iterations: 10000,
	}
}

// HashPassword hashes a plaintext password with a 16-byte cryptographically secure random salt.
func (h *StandardPasswordHasher) HashPassword(plaintext string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := h.computeHash([]byte(plaintext), salt)
	return fmt.Sprintf("$sd$sha256$%s$%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// VerifyPassword verifies a plaintext password against a stored hash using constant-time comparison.
func (h *StandardPasswordHasher) VerifyPassword(plaintext string, storedHash string) bool {
	parts := strings.Split(storedHash, "$")
	// Expected parts: ["", "sd", "sha256", "<salt_hex>", "<hash_hex>"]
	if len(parts) != 5 || parts[1] != "sd" || parts[2] != "sha256" {
		return false
	}

	salt, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(parts[4])
	if err != nil {
		return false
	}

	actualHash := h.computeHash([]byte(plaintext), salt)

	// Constant-time comparison to prevent timing side-channel attacks
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

func (h *StandardPasswordHasher) computeHash(password []byte, salt []byte) []byte {
	// Key stretching via iterative SHA-256
	hsh := sha256.New()
	hsh.Write(salt)
	hsh.Write(password)
	result := hsh.Sum(nil)

	for i := 0; i < h.iterations; i++ {
		hsh.Reset()
		hsh.Write(result)
		hsh.Write(salt)
		result = hsh.Sum(nil)
	}

	return result
}
