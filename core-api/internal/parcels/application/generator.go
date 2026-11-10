package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// CryptoTrackingNumberGenerator implements TrackingNumberGenerator using cryptographically
// secure randomness and canonical Crockford Base32 encoding.
type CryptoTrackingNumberGenerator struct{}

// NewCryptoTrackingNumberGenerator constructs a new CryptoTrackingNumberGenerator.
func NewCryptoTrackingNumberGenerator() *CryptoTrackingNumberGenerator {
	return &CryptoTrackingNumberGenerator{}
}

// Generate creates a canonical tracking number in the format SD-YYYYMMDD-XXXXXX.
func (g *CryptoTrackingNumberGenerator) Generate(ctx context.Context, date time.Time) (domain.TrackingNumber, error) {
	const alphabet = domain.CrockfordBase32Alphabet
	const suffixLen = 6

	var randomBytes [suffixLen]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for tracking number: %w", err)
	}

	suffix := make([]byte, suffixLen)
	for i := 0; i < suffixLen; i++ {
		// Crockford alphabet has exactly 32 characters (0x1F mask)
		suffix[i] = alphabet[randomBytes[i]&0x1F]
	}

	return domain.FormatTrackingNumber(date, string(suffix))
}

// UUIDv7Generator implements IDGenerator by generating RFC 9562 compliant UUIDv7 strings.
type UUIDv7Generator struct{}

// NewUUIDv7Generator constructs a new UUIDv7Generator.
func NewUUIDv7Generator() *UUIDv7Generator {
	return &UUIDv7Generator{}
}

// GenerateID produces a valid UUIDv7 string.
func (g *UUIDv7Generator) GenerateID() (string, error) {
	return events.GenerateUUIDv7()
}

// SystemClock provides current UTC time.
type SystemClock struct{}

// NewSystemClock constructs a SystemClock.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current time in UTC.
func (c *SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// DBTransactionManager implements TransactionManager by delegating to platform/db.WithTx.
type DBTransactionManager struct {
	db *sql.DB
}

// NewDBTransactionManager constructs a DBTransactionManager wrapping a *sql.DB handle.
func NewDBTransactionManager(database *sql.DB) *DBTransactionManager {
	return &DBTransactionManager{db: database}
}

// WithTx executes fn inside an atomic database transaction.
func (m *DBTransactionManager) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if m.db == nil {
		return ErrNilTransactionManager
	}
	return db.WithTx(ctx, m.db, fn)
}
