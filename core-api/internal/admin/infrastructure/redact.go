package infrastructure

import "strings"

var secretWords = []string{"password", "secret", "token", "api_key", "apikey", "authorization", "cookie", "signature"}

// redactForAudit returns a copy of a before/after snapshot that is safe to store in
// audit_logs: values under secret-looking keys are replaced.
func redactForAudit(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isSecretKey(k) {
			out[k] = "[REDACTED]"
			continue
		}
		switch t := v.(type) {
		case map[string]any:
			out[k] = redactForAudit(t)
		case []any:
			list := make([]any, len(t))
			for i, item := range t {
				if m, ok := item.(map[string]any); ok {
					list[i] = redactForAudit(m)
				} else {
					list[i] = item
				}
			}
			out[k] = list
		default:
			out[k] = v
		}
	}
	return out
}

func isSecretKey(k string) bool {
	k = strings.ToLower(k)
	for _, w := range secretWords {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}
