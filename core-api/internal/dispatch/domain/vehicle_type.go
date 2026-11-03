package domain

type VehicleType string

const (
	VehicleTypeMotorcycle VehicleType = "MOTORCYCLE"
	VehicleTypeVan        VehicleType = "VAN"
	VehicleTypeTruck      VehicleType = "TRUCK"
)

// DefaultCapacity returns the standard system default active parcel capacity for this vehicle type.
func (v VehicleType) DefaultCapacity() int {
	switch v {
	case VehicleTypeMotorcycle:
		return 25
	case VehicleTypeVan:
		return 60
	case VehicleTypeTruck:
		return 120
	default:
		return 25
	}
}

// Validate checks whether the vehicle classification matches approved motorized fleet types.
func (v VehicleType) Validate() error {
	switch v {
	case VehicleTypeMotorcycle, VehicleTypeVan, VehicleTypeTruck:
		return nil
	default:
		return ErrInvalidVehicleType
	}
}

func (v VehicleType) String() string {
	return string(v)
}

