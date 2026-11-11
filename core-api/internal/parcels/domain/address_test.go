package domain

import (
	"encoding/json"
	"errors"
	"strings"
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
		ZoneID:       "ZONE_WESTERN_01",
		Instructions: "Call upon arrival at gate 2",
	}
}

func TestValidateZoneID(t *testing.T) {
	validZones := []string{"ZONE_AB", "ZONE_COLOMBO_01", "ZONE_" + strings.Repeat("A", 27)}
	for _, zone := range validZones {
		if err := ValidateZoneID(zone); err != nil {
			t.Errorf("expected ValidateZoneID(%q) to succeed, got %v", zone, err)
		}
	}

	invalidZones := []string{
		"",
		"   ",
		"ZONE_A",
		"ZONE_" + strings.Repeat("A", 28),
		"ZONE-WESTERN-01",
		"zone_western_01",
		" ZONE_WESTERN_01 ",
		"WESTERN_01",
	}
	for _, zone := range invalidZones {
		err := ValidateZoneID(zone)
		if !errors.Is(err, ErrInvalidZone) {
			t.Errorf("expected ValidateZoneID(%q) to fail with ErrInvalidZone, got %v", zone, err)
		}
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
		{
			name: "invalid zone id format with hyphen",
			mutate: func(a *Address) {
				a.ZoneID = "ZONE-WESTERN-01"
			},
			wantErr: true,
		},
		{
			name: "invalid zone id format with surrounding spaces",
			mutate: func(a *Address) {
				a.ZoneID = " ZONE_WESTERN_01 "
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

func TestAddressValidate_InvalidZoneWrapsErrInvalidZone(t *testing.T) {
	addr := validTestAddress()
	addr.ZoneID = "ZONE-WESTERN-01"
	err := addr.Validate()
	if !errors.Is(err, ErrInvalidAddress) || !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("expected error wrapping ErrInvalidAddress and ErrInvalidZone, got %v", err)
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
		"ZONE_WESTERN_01",
		" Leave with security guard ",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.ContactName != "Kasun Perera" {
		t.Errorf("expected trimmed contact name, got %q", addr.ContactName)
	}
	if addr.Instructions != "Leave with security guard" {
		t.Errorf("expected trimmed instructions, got %q", addr.Instructions)
	}

	// Leading/trailing spaces in ZoneID must fail ValidateZoneID rather than being masked.
	_, err = NewAddress(
		"Kasun Perera",
		"+94771234567",
		"No 45, Galle Road",
		"",
		"Colombo",
		"00300",
		" ZONE_WESTERN_01 ",
		"",
	)
	if !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("expected ErrInvalidZone for untrimmed zone id, got %v", err)
	}
}

func TestAddress_JSONSerializationSnakeCase(t *testing.T) {
	addr, err := NewAddress(
		"Kasun Perera",
		"+94771234567",
		"No 45, Galle Road",
		"Kollupitiya",
		"Colombo",
		"00300",
		"ZONE_WESTERN_01",
		"Ring bell twice",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := json.Marshal(addr)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal map failed: %v", err)
	}

	expectedKeys := map[string]string{
		"contact_name":  "Kasun Perera",
		"contact_phone": "+94771234567",
		"address_line1": "No 45, Galle Road",
		"address_line2": "Kollupitiya",
		"city":          "Colombo",
		"postal_code":   "00300",
		"zone_id":       "ZONE_WESTERN_01",
		"instructions":  "Ring bell twice",
	}
	for k, want := range expectedKeys {
		got, ok := m[k]
		if !ok {
			t.Errorf("expected snake_case JSON key %q to be present", k)
			continue
		}
		if got != want {
			t.Errorf("JSON key %q = %v, want %q", k, got, want)
		}
	}

	var decoded Address
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal Address failed: %v", err)
	}
	if decoded != addr {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, addr)
	}
	if err := decoded.Validate(); err != nil {
		t.Errorf("round-trip decoded address failed Validate(): %v", err)
	}

	// Instructions must be omitted when empty.
	addrNoInstructions, err := NewAddress(
		"Kasun Perera",
		"+94771234567",
		"No 45, Galle Road",
		"",
		"Colombo",
		"00300",
		"ZONE_WESTERN_01",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rawNoInstr, err := json.Marshal(addrNoInstructions)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var mNoInstr map[string]any
	if err := json.Unmarshal(rawNoInstr, &mNoInstr); err != nil {
		t.Fatalf("json.Unmarshal map failed: %v", err)
	}
	if _, exists := mNoInstr["instructions"]; exists {
		t.Errorf("expected empty instructions to be omitted from JSON, got %s", string(rawNoInstr))
	}
}
