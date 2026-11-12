package domain

// GeoCoordinate represents doorstep GPS coordinates captured upon delivery attempt.
type GeoCoordinate struct {
	Latitude  float64
	Longitude float64
}

// NewGeoCoordinate creates and validates geographic coordinates.
func NewGeoCoordinate(lat, lng float64) (*GeoCoordinate, error) {
	if lat < -90.0 || lat > 90.0 || lng < -180.0 || lng > 180.0 {
		return nil, ErrInvalidCoordinates
	}
	return &GeoCoordinate{
		Latitude:  lat,
		Longitude: lng,
	}, nil
}
