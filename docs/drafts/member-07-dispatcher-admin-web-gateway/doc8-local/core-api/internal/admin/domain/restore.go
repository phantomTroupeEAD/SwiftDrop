package domain

// Restore rebuilds a zone that was loaded from storage. It records no events.
func Restore(id ZoneID, code ZoneCode, name, description string, prefixes []PostalPrefix, enabled bool, version int) *Zone {
	return &Zone{id: id, code: code, name: name, description: description, prefixes: copyPrefixes(prefixes), enabled: enabled, version: version}
}
