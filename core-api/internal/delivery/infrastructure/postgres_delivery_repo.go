package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// PostgresDeliveryRepository implements application.DeliveryAttemptRepository using PostgreSQL.
type PostgresDeliveryRepository struct {
	db *sql.DB
}

// NewPostgresDeliveryRepository constructs the delivery repository.
func NewPostgresDeliveryRepository(db *sql.DB) *PostgresDeliveryRepository {
	return &PostgresDeliveryRepository{db: db}
}

// InsertAttempt writes an immutable delivery attempt row to delivery_attempts.
func (r *PostgresDeliveryRepository) InsertAttempt(ctx context.Context, tx *sql.Tx, a *domain.DeliveryAttempt) error {
	if tx == nil && r.db == nil {
		return fmt.Errorf("delivery repository: %w", db.ErrNilDB)
	}
	query := `
		INSERT INTO delivery_attempts (
			id,
			parcel_id,
			courier_id,
			attempt_number,
			outcome,
			failure_reason_code,
			proof_type,
			proof_attachment_url,
			proof_signature_data,
			received_by_name,
			latitude,
			longitude,
			notes,
			attempted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		);
	`

	var failureReason sql.NullString
	if a.FailureReason != nil {
		failureReason = sql.NullString{String: string(*a.FailureReason), Valid: true}
	}

	var proofType sql.NullString
	var proofURL sql.NullString
	var signatureData sql.NullString
	var receivedByName sql.NullString
	if a.Proof != nil {
		proofType = sql.NullString{String: string(a.Proof.Type), Valid: true}
		if a.Proof.AttachmentURL != "" {
			proofURL = sql.NullString{String: a.Proof.AttachmentURL, Valid: true}
		}
		if a.Proof.SignatureData != "" {
			signatureData = sql.NullString{String: a.Proof.SignatureData, Valid: true}
		}
		if a.Proof.ReceivedByName != "" {
			receivedByName = sql.NullString{String: a.Proof.ReceivedByName, Valid: true}
		}
	}

	var lat sql.NullFloat64
	var lng sql.NullFloat64
	if a.Coordinates != nil {
		lat = sql.NullFloat64{Float64: a.Coordinates.Latitude, Valid: true}
		lng = sql.NullFloat64{Float64: a.Coordinates.Longitude, Valid: true}
	}

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, a.ID, a.ParcelID, a.CourierID, a.AttemptNumber, string(a.Outcome), failureReason, proofType, proofURL, signatureData, receivedByName, lat, lng, a.Notes, a.AttemptedAt)
	} else {
		_, err = r.db.ExecContext(ctx, query, a.ID, a.ParcelID, a.CourierID, a.AttemptNumber, string(a.Outcome), failureReason, proofType, proofURL, signatureData, receivedByName, lat, lng, a.Notes, a.AttemptedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to insert delivery attempt: %w", err)
	}
	return nil
}

// ListAttemptsByParcelID loads the complete chronological attempt history for a parcel.
func (r *PostgresDeliveryRepository) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	query := `
		SELECT 
			id,
			parcel_id,
			courier_id,
			attempt_number,
			outcome,
			failure_reason_code,
			proof_type,
			proof_attachment_url,
			proof_signature_data,
			received_by_name,
			latitude,
			longitude,
			notes,
			attempted_at
		FROM delivery_attempts
		WHERE parcel_id = $1
		ORDER BY attempt_number ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, parcelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.DeliveryAttempt
	for rows.Next() {
		var a domain.DeliveryAttempt
		var outcomeStr string
		var failureReason sql.NullString
		var proofType sql.NullString
		var proofURL sql.NullString
		var signatureData sql.NullString
		var receivedByName sql.NullString
		var lat sql.NullFloat64
		var lng sql.NullFloat64
		var notes sql.NullString
		var attemptedAt time.Time

		err := rows.Scan(
			&a.ID,
			&a.ParcelID,
			&a.CourierID,
			&a.AttemptNumber,
			&outcomeStr,
			&failureReason,
			&proofType,
			&proofURL,
			&signatureData,
			&receivedByName,
			&lat,
			&lng,
			&notes,
			&attemptedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delivery attempt row: %w", err)
		}

		a.Outcome = domain.AttemptOutcome(outcomeStr)
		a.AttemptedAt = attemptedAt
		if notes.Valid {
			a.Notes = notes.String
		}

		if failureReason.Valid {
			rc := domain.FailureReasonCode(failureReason.String)
			a.FailureReason = &rc
		}

		if proofType.Valid {
			a.Proof = &domain.ProofOfDelivery{
				Type:           domain.ProofType(proofType.String),
				AttachmentURL:  proofURL.String,
				SignatureData:  signatureData.String,
				ReceivedByName: receivedByName.String,
			}
		}

		if lat.Valid && lng.Valid {
			a.Coordinates = &domain.GeoCoordinate{
				Latitude:  lat.Float64,
				Longitude: lng.Float64,
			}
		}

		attempts = append(attempts, &a)
	}

	return attempts, rows.Err()
}

// GetAttemptCount returns the total number of attempts recorded for a parcel.
func (r *PostgresDeliveryRepository) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM delivery_attempts WHERE parcel_id = $1;`

	var count int
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, query, parcelID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, query, parcelID).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to query attempt count: %w", err)
	}
	return count, nil
}
