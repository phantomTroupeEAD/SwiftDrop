package domain

import (
	"errors"
	"testing"
)

func TestValidateContact(t *testing.T) {
	ok := Address{Line1: "12 Temple Rd", City: "Kandy", Country: "LK"}
	cases := []struct {
		name, email, phone string
		addr               Address
		field              string
	}{
		{"valid", "ops@shop.lk", "+94771234567", ok, ""},
		{"bad email", "ops-at-shop", "+94771234567", ok, "contact_email"},
		{"local phone", "ops@shop.lk", "0771234567", ok, "contact_phone"},
		{"missing city", "ops@shop.lk", "+94771234567", Address{Line1: "x", Country: "LK"}, "address.city"},
		{"lowercase country", "ops@shop.lk", "+94771234567", Address{Line1: "x", City: "y", Country: "lk"}, "address.country"},
		{"newline injection", "ops@shop.lk", "+94771234567", Address{Line1: "x\nFAKE LOG", City: "y", Country: "LK"}, "address.line1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateContact(c.email, c.phone, c.addr)
			if c.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var pe ErrInvalidProfile
			if !errors.As(err, &pe) || pe.Field != c.field {
				t.Fatalf("err = %v, want field %s", err, c.field)
			}
		})
	}
}
