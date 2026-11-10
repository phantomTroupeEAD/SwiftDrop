package domain

// ServiceLevel represents the chosen courier delivery SLA tier.
type ServiceLevel string

const (
	ServiceLevelStandard ServiceLevel = "STANDARD"
	ServiceLevelExpress  ServiceLevel = "EXPRESS"
	ServiceLevelSameDay  ServiceLevel = "SAME_DAY"
)

// Validate ensures the service level is one of the supported tiers.
func (s ServiceLevel) Validate() error {
	switch s {
	case ServiceLevelStandard, ServiceLevelExpress, ServiceLevelSameDay:
		return nil
	default:
		return ErrInvalidServiceLevel
	}
}
