package adminsec

import "net/http"

// RequireRole lets the request through only when the verified caller has one of the
// roles. It fails closed: no principal means 401, any other role means 403.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())
			if !ok {
				WriteError(w, r, ErrUnauthenticated)
				return
			}
			if !allowed[p.Role] {
				WriteError(w, r, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
