package domain

import (
	"fmt"
	"math"
)

// Dimensions represents the physical three-dimensional measurements of a parcel in centimeters.
type Dimensions struct {
	LengthCm float64
	WidthCm  float64
	HeightCm float64
}

// NewDimensions creates and validates a Dimensions value object.
func NewDimensions(lengthCm, widthCm, heightCm float64) (Dimensions, error) {
	d := Dimensions{
		LengthCm: lengthCm,
		WidthCm:  widthCm,
		HeightCm: heightCm,
	}
	if err := d.Validate(); err != nil {
		return Dimensions{}, err
	}
	return d, nil
}

// Validate ensures dimensions are positive, non-zero, finite numbers.
func (d Dimensions) Validate() error {
	values := []struct {
		name  string
		value float64
	}{
		{"length", d.LengthCm},
		{"width", d.WidthCm},
		{"height", d.HeightCm},
	}

	for _, v := range values {
		if math.IsNaN(v.value) || math.IsInf(v.value, 0) || v.value <= 0 {
			return fmt.Errorf("%w: %s must be a positive finite number, got %v", ErrInvalidDimensions, v.name, v.value)
		}
	}

	return nil
}
