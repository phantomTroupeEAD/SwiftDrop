package domain

import (
	"fmt"
	"strings"
)

// Address represents an immutable snapshot of a physical location at booking time.
type Address struct {
	ContactName  string
	ContactPhone string
	StreetLine1  string
	StreetLine2  string
	City         string
	PostalCode   string
	ZoneID       string
}

// NewAddress creates and validates an Address value object.
func NewAddress(
	contactName, contactPhone, streetLine1, streetLine2, city, postalCode, zoneID string,
) (Address, error) {
	addr := Address{
		ContactName:  strings.TrimSpace(contactName),
		ContactPhone: strings.TrimSpace(contactPhone),
		StreetLine1:  strings.TrimSpace(streetLine1),
		StreetLine2:  strings.TrimSpace(streetLine2),
		City:         strings.TrimSpace(city),
		PostalCode:   strings.TrimSpace(postalCode),
		ZoneID:       strings.TrimSpace(zoneID),
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

	return nil
}

