package domain

import "time"

// ParcelTrackingInfo represents the internal parcel state retrieved from PostgreSQL
// for projection purposes.
type ParcelTrackingInfo struct {
	ID              string
	TrackingNumber  TrackingNumber
	Status          ParcelStatus
	DestinationCity string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MilestoneRecord represents an internal status history entry from PostgreSQL.
type MilestoneRecord struct {
	ID         string
	ParcelID   string
	FromStatus *string
	ToStatus   string
	ActorID    string
	ActorType  string
	ActorRole  string
	Reason     string
	Timestamp  time.Time
}
