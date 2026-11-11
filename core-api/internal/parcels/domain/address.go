package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var zoneIDPattern = regexp.MustCompile(`^ZONE_[A-Z0-9_]{2,27}$`)

// ValidateZoneID validates that a zone identifier conforms to the shared ZONE_* format.
func ValidateZoneID(zoneID string) error {
	if !zoneIDPattern.MatchString(zoneID) {
		return fmt.Errorf("%w: %q must match ^ZONE_[A-Z0-9_]{2,27}$", ErrInvalidZone, zoneID)
	}
	return nil
}

// Address represents an immutable snapshot of a physical location at booking time.
type Address struct {
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	StreetLine1  string `json:"address_line1"`
	StreetLine2  string `json:"address_line2"`
	City         string `json:"city"`
	PostalCode   string `json:"postal_code"`
	ZoneID       string `json:"zone_id"`
	Instructions string `json:"instructions,omitempty"`
}

// NewAddress creates and validates an Address value object.
func NewAddress(
	contactName, contactPhone, streetLine1, streetLine2, city, postalCode, zoneID, instructions string,
) (Address, error) {
	addr := Address{
		ContactName:  strings.TrimSpace(contactName),
		ContactPhone: strings.TrimSpace(contactPhone),
		StreetLine1:  strings.TrimSpace(streetLine1),
		StreetLine2:  strings.TrimSpace(streetLine2),
		City:         strings.TrimSpace(city),
		PostalCode:   strings.TrimSpace(postalCode),
		ZoneID:       zoneID,
		Instructions: strings.TrimSpace(instructions),
	}

	if err := addr.Validate(); err != nil {
		return Address{}, err
	}

	return addr, nil
}

// Validate verifies that all mandatory address fields are present and valid.
func (a Address) Validate() error {
	if strings.TrimSpace(a.ContactName) == "" {
		return fmt.Errorf("%w: contact name is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(a.ContactPhone) == "" {
		return fmt.Errorf("%w: contact phone is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(a.StreetLine1) == "" {
		return fmt.Errorf("%w: street line 1 is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(a.City) == "" {
		return fmt.Errorf("%w: city is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(a.PostalCode) == "" {
		return fmt.Errorf("%w: postal code is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(a.ZoneID) == "" {
		return fmt.Errorf("%w: zone id is required", ErrInvalidAddress)
	}
	if err := ValidateZoneID(a.ZoneID); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAddress, err)
	}

	return nil
}
