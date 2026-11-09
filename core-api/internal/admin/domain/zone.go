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
	Type    EventType
	ZoneID  string
	Changes map[string][2]any // field -> [old, new]
}

// Zone is the aggregate root. It owns its postal code prefixes. It holds no
// money: fees, including any per-zone charge, are Member 2's pricing rules.
type Zone struct {
	id          ZoneID
	code        ZoneCode
	name        string
	description string
	prefixes    []PostalPrefix
	enabled     bool
	version     int
	bumped      bool // version already raised for the changes not yet saved
	events      []Event
}

func NewZone(id ZoneID, code ZoneCode, name, description string, prefixes []PostalPrefix) (*Zone, error) {
	name = strings.TrimSpace(name)
	if err := checkName(name); err != nil {
		return nil, err
	}
	description = strings.TrimSpace(description)
	if err := checkDescription(description); err != nil {
		return nil, err
	}
	if err := checkPrefixes(prefixes); err != nil {
		return nil, err
	}
	z := &Zone{id: id, code: code, name: name, description: description, prefixes: copyPrefixes(prefixes), enabled: true, version: 1}
	z.record(ZoneCreated, nil)
	return z, nil
}

func (z *Zone) ID() ZoneID               { return z.id }
func (z *Zone) Code() ZoneCode           { return z.code }
func (z *Zone) Name() string             { return z.name }
func (z *Zone) Description() string      { return z.description }
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

func (z *Zone) ChangeCode(c ZoneCode) error {
	if c == z.code {
		return nil
	}
	old := z.code.String()
	z.code = c
	z.changed(ZoneChanged, "code", old, c.String())
	return nil
}

func (z *Zone) ChangeDescription(d string) error {
	d = strings.TrimSpace(d)
	if err := checkDescription(d); err != nil {
		return err
	}
	if d == z.description {
		return nil
	}
	old := z.description
	z.description = d
	z.changed(ZoneChanged, "description", old, d)
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

// PullEvents returns recorded events once and clears them. The use case calls it
// when it saves, so the next change starts a new version.
func (z *Zone) PullEvents() []Event {
	e := z.events
	z.events, z.bumped = nil, false
	return e
}

// changed raises the version once per save, however many fields one request changes,
// so a PATCH with version N always leaves the zone at N+1.
func (z *Zone) changed(t EventType, field string, old, new any) {
	if !z.bumped {
		z.version++
		z.bumped = true
	}
	z.record(t, map[string][2]any{field: {old, new}})
}

func (z *Zone) record(t EventType, changes map[string][2]any) {
	z.events = append(z.events, Event{Type: t, ZoneID: z.id.String(), Changes: changes})
}

func checkName(name string) error {
	if n := len([]rune(name)); n < 1 || n > 120 {
		return fmt.Errorf("%w: length must be 1 to 120", ErrInvalidZoneName)
	}
	return nil
}

func checkDescription(d string) error {
	if len([]rune(d)) > 500 {
		return fmt.Errorf("%w: at most 500 characters", ErrInvalidDescription)
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
