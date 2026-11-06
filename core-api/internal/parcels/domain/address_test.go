package domain

import (
	"errors"
	"testing"
)

func validTestAddress() Address {
	return Address{
		ContactName:  "Kasun Perera",
		ContactPhone: "+94771234567",
		StreetLine1:  "No 45, Galle Road",
		StreetLine2:  "Kollupitiya",
		City:         "Colombo",
		PostalCode:   "00300",
		ZoneID:       "ZONE-WESTERN-01",
	}
}

func TestAddressValidate_Valid(t *testing.T) {
	addr := validTestAddress()
	if err := addr.Validate(); err != nil {
		t.Fatalf("expected valid address, got: %v", err)
	}
}

func TestAddressValidate_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Address)
		wantErr bool
	}{
		{
			name: "empty contact name",
			mutate: func(a *Address) {
				a.ContactName = "   "
			},
			wantErr: true,
		},
		{
			name: "empty contact phone",
			mutate: func(a *Address) {
				a.ContactPhone = ""
			},
			wantErr: true,
		},
		{
			name: "empty street line 1",
			mutate: func(a *Address) {
				a.StreetLine1 = " "
			},
			wantErr: true,
		},
		{
			name: "empty city",
			mutate: func(a *Address) {
				a.City = ""
			},
			wantErr: true,
		},
		{
			name: "empty postal code",
			mutate: func(a *Address) {
				a.PostalCode = ""
			},
			wantErr: true,
		},
		{
			name: "empty zone id",
			mutate: func(a *Address) {
				a.ZoneID = ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := validTestAddress()
			tt.mutate(&addr)
			err := addr.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidAddress) {
				t.Errorf("expected ErrInvalidAddress, got %v", err)
			}
		})
	}
}

func TestNewAddress(t *testing.T) {
	addr, err := NewAddress(
		" Kasun Perera ",
		"+94771234567",
		"No 45, Galle Road",
		"",
		"Colombo",
		"00300",
		"ZONE-WESTERN-01",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.ContactName != "Kasun Perera" {
		t.Errorf("expected trimmed contact name, got %q", addr.ContactName)
	}
}

