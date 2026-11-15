package infrastructure

import (
	"strings"
	"testing"
)

func TestStandardPasswordHasher(t *testing.T) {
	hasher := NewStandardPasswordHasher()

	t.Run("hashes and verifies matching password", func(t *testing.T) {
		hash1, err := hasher.HashPassword("StrongPass123!")
		if err != nil {
			t.Fatalf("unexpected hash error: %v", err)
		}
		if !strings.HasPrefix(hash1, "$sd$sha256$") {
			t.Errorf("expected hash prefix $sd$sha256$, got %q", hash1)
		}

		// Unique salt per invocation
		hash2, err := hasher.HashPassword("StrongPass123!")
		if err != nil {
			t.Fatalf("unexpected hash error: %v", err)
		}
		if hash1 == hash2 {
			t.Errorf("expected distinct hashes due to random salt")
		}

		if !hasher.VerifyPassword("StrongPass123!", hash1) {
			t.Errorf("expected password verification to succeed")
		}
		if !hasher.VerifyPassword("StrongPass123!", hash2) {
			t.Errorf("expected password verification to succeed for second hash")
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		hash, err := hasher.HashPassword("CorrectPass1!")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasher.VerifyPassword("WrongPass1!", hash) {
			t.Errorf("expected wrong password to fail verification")
		}
	})

	t.Run("rejects malformed stored hashes", func(t *testing.T) {
		cases := []string{
			"",
			"not-a-hash",
			"$bcrypt$sha256$abcd$ef01",
			"$sd$md5$abcd$ef01",
			"$sd$sha256$invalidhex$ef01",
			"$sd$sha256$abcd$invalidhex",
			"$sd$sha256$abcd",
		}
		for _, tc := range cases {
			if hasher.VerifyPassword("anything", tc) {
				t.Errorf("expected malformed hash %q to return false", tc)
			}
		}
	})
}
