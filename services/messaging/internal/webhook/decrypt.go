package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/swiftdrop/messaging/internal/config"
)

var (
	ErrCiphertextTooShort = errors.New("webhook: ciphertext is shorter than minimum required length (28 bytes)")
	ErrKeyUnavailable     = errors.New("webhook: encryption key unavailable for requested version")
	ErrDecryptFailed      = errors.New("webhook: failed to decrypt secret ciphertext (wrong AAD or corrupted tag)")
)

// Decryptor manages AES-256-GCM secret decryption using versioned encryption keys.
type Decryptor struct {
	keys map[int][]byte
}

// NewDecryptor initializes Decryptor with primary key from WEBHOOK_SECRET_ENCRYPTION_KEY.
// Accepts 64-character hexadecimal or base64-encoded 32-byte AES-256 keys.
// Also loads WEBHOOK_SECRET_ENCRYPTION_KEY_V2, V3 etc. if present for key rotation (Decision D3).
func NewDecryptor(primaryKey string) (*Decryptor, error) {
	key1, err := config.DecodeAES256Key(primaryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode primary encryption key: %w", err)
	}

	keys := map[int][]byte{
		1: key1,
	}

	// Look for additional versioned keys in environment
	for v := 2; v <= 10; v++ {
		envName := fmt.Sprintf("WEBHOOK_SECRET_ENCRYPTION_KEY_V%d", v)
		val := strings.TrimSpace(os.Getenv(envName))
		if val != "" {
			if kBytes, err := config.DecodeAES256Key(val); err == nil {
				keys[v] = kBytes
			}
		}
	}

	return &Decryptor{keys: keys}, nil
}

// Decrypt decrypts an AES-256-GCM payload using the specified key version and AAD.
// Ciphertext layout: Nonce (12 bytes) || Ciphertext || Tag (16 bytes). Minimum 28 bytes.
// AAD: endpoint_id string.
func (d *Decryptor) Decrypt(data []byte, endpointID string, keyVersion int) ([]byte, error) {
	if len(data) < 28 {
		return nil, ErrCiphertextTooShort
	}

	if keyVersion <= 0 {
		keyVersion = 1
	}

	key, ok := d.keys[keyVersion]
	if !ok {
		return nil, fmt.Errorf("%w: version %d", ErrKeyUnavailable, keyVersion)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize() // 12 bytes
	if len(data) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce := data[:nonceSize]
	ciphertextAndTag := data[nonceSize:]

	aad := []byte(endpointID)
	plaintext, err := gcm.Open(nil, nonce, ciphertextAndTag, aad)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	return plaintext, nil
}
