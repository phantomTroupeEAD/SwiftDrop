package domain

// Restore rebuilds a zone that was loaded from storage. It records no events.
func Restore(id string, code ZoneCode, name string, surcharge Money, prefixes []PostalPrefix, enabled bool, version int) *Zone {
	return &Zone{id: id, code: code, name: name, surcharge: surcharge, prefixes: copyPrefixes(prefixes), enabled: enabled, version: version}
}
