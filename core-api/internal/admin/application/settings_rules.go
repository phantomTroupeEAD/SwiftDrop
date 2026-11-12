package application

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxSettingValueBytes caps one system_settings value (Doc 5 4.3: jsonb).
const MaxSettingValueBytes = 4096

type rule func(json.RawMessage) error

func intBetween(lo, hi int) rule {
	return func(v json.RawMessage) error {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return fmt.Errorf("%w: must be a whole number", ErrValidation)
		}
		if n < lo || n > hi {
			return fmt.Errorf("%w: must be %d to %d", ErrValidation, lo, hi)
		}
		return nil
	}
}

// numberBetween accepts a JSON number (whole or decimal) from lo to hi. Used for
// kilograms, which are not money; money settings use intBetween (minor units).
func numberBetween(lo, hi float64) rule {
	return func(v json.RawMessage) error {
		var n float64
		if err := json.Unmarshal(v, &n); err != nil {
			return fmt.Errorf("%w: must be a number", ErrValidation)
		}
		if n < lo || n > hi {
			return fmt.Errorf("%w: must be %g to %g", ErrValidation, lo, hi)
		}
		return nil
	}
}

func stringMax(maxLen int) rule {
	return func(v json.RawMessage) error {
		var str string
		if err := json.Unmarshal(v, &str); err != nil {
			return fmt.Errorf("%w: must be text", ErrValidation)
		}
		if utf8.RuneCountInString(str) > maxLen {
			return fmt.Errorf("%w: at most %d characters", ErrValidation, maxLen)
		}
		return nil
	}
}

// settingRules is the allowlist, matching the seed list in Doc 5 section 10.1.
// A key that is not here cannot be written through the API, so an admin cannot
// create arbitrary or secret-looking settings. [Proposed]
var settingRules = map[string]rule{
	"reports.max_range_days":    intBetween(1, 366),
	"ops.board_refresh_seconds": intBetween(10, 300),
	"ops.maintenance_banner":    stringMax(200),
	// Parcel limits read by the AI assistant and booking screens (Doc 4 section 8).
	// The weight cap matches pricing's hard limit of 30 kg. COD is LKR cents, never a float.
	"parcels.max_weight_kg": numberBetween(0.5, 30),
	"parcels.max_cod_cents": intBetween(0, 10_000_000),
}

// ValidateSetting checks one key/value pair before it is saved.
func ValidateSetting(key string, value json.RawMessage) error {
	r, ok := settingRules[key]
	if !ok {
		return ErrUnknownSetting
	}
	if len(value) == 0 || len(value) > MaxSettingValueBytes {
		return fmt.Errorf("%w: value size", ErrValidation)
	}
	return r(value)
}

// ValidateReason is required for changes that affect many parcels
// (disable zone, settings). 10 to 500 characters.
func ValidateReason(reason string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(reason))
	if n < 10 || n > 500 {
		return ErrReasonRequired
	}
	return nil
}
