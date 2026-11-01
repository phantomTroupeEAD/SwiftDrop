package domain

import (
	"fmt"
	"strings"
)

type EventType string

const (
	ZoneCreated       EventType = "ZONE_CREATED"
	ZoneChanged       EventType = "ZONE_CHANGED"
	ZoneStatusChanged EventType = "ZONE_STATUS_CHANGED"
)

// Event is a fact recorded by the aggregate; the use case turns it into an audit row.
type Event struct {
	Type     EventType
	ZoneCode string
	Changes  map[string][2]any // field -> [old, new]
}

// Zone is the aggregate root. It owns its postal code prefixes.
type Zone struct {
	id        string
	code      ZoneCode
	name      string
	surcharge Money
	prefixes  []PostalPrefix
	enabled   bool
	version   int
	events    []Event
}

func NewZone(id string, code ZoneCode, name string, surcharge Money, prefixes []PostalPrefix) (*Zone, error) {
	name = strings.TrimSpace(name)
	if err := checkName(name); err != nil {
		return nil, err
	}
	if err := checkPrefixes(prefixes); err != nil {
		return nil, err
	}
	z := &Zone{id: id, code: code, name: name, surcharge: surcharge, prefixes: copyPrefixes(prefixes), enabled: true, version: 1}
	z.record(ZoneCreated, nil)
	return z, nil
}

func (z *Zone) ID() string               { return z.id }
func (z *Zone) Code() ZoneCode           { return z.code }
func (z *Zone) Name() string             { return z.name }
func (z *Zone) Surcharge() Money         { return z.surcharge }
func (z *Zone) Enabled() bool            { return z.enabled }
func (z *Zone) Version() int             { return z.version }
func (z *Zone) Prefixes() []PostalPrefix { return copyPrefixes(z.prefixes) }

func (z *Zone) Rename(name string) error {
	name = strings.TrimSpace(name)
	if err := checkName(name); err != nil {
		return err
	}
	if name == z.name {
		return nil
	}
	old := z.name
	z.name = name
	z.changed(ZoneChanged, "name", old, name)
	return nil
}

func (z *Zone) ChangeSurcharge(m Money) error {
	if m == z.surcharge {
		return nil
	}
	old := z.surcharge.Cents()
	z.surcharge = m
	z.changed(ZoneChanged, "base_surcharge_cents", old, m.Cents())
	return nil
}

func (z *Zone) ReplacePrefixes(p []PostalPrefix) error {
	if err := checkPrefixes(p); err != nil {
		return err
	}
	old := prefixStrings(z.prefixes)
	z.prefixes = copyPrefixes(p)
	z.changed(ZoneChanged, "prefixes", old, prefixStrings(p))
	return nil
}

func (z *Zone) Disable() error {
	if !z.enabled {
		return ErrAlreadyDisabled
	}
	z.enabled = false
	z.changed(ZoneStatusChanged, "enabled", true, false)
	return nil
}

func (z *Zone) Enable() error {
	if z.enabled {
		return ErrAlreadyEnabled
	}
	z.enabled = true
	z.changed(ZoneStatusChanged, "enabled", false, true)
	return nil
}

// PullEvents returns recorded events once and clears them.
func (z *Zone) PullEvents() []Event {
	e := z.events
	z.events = nil
	return e
}

func (z *Zone) changed(t EventType, field string, old, new any) {
	z.version++
	z.record(t, map[string][2]any{field: {old, new}})
}

func (z *Zone) record(t EventType, changes map[string][2]any) {
	z.events = append(z.events, Event{Type: t, ZoneCode: z.code.String(), Changes: changes})
}

func checkName(name string) error {
	if len(name) < 1 || len(name) > 100 {
		return fmt.Errorf("%w: length must be 1 to 100", ErrInvalidZoneName)
	}
	return nil
}

func checkPrefixes(p []PostalPrefix) error {
	if len(p) == 0 {
		return ErrNoPrefixes
	}
	seen := make(map[string]bool, len(p))
	for _, x := range p {
		if seen[x.v] {
			return fmt.Errorf("%w: %s", ErrDuplicatePrefixInZone, x.v)
		}
		seen[x.v] = true
	}
	return nil
}

func copyPrefixes(p []PostalPrefix) []PostalPrefix { return append([]PostalPrefix(nil), p...) }

func prefixStrings(p []PostalPrefix) []string {
	out := make([]string, len(p))
	for i, x := range p {
		out[i] = x.v
	}
	return out
}
