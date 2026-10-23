package main

import "net/http"

// withBasicAuth wraps a handler with HTTP Basic Auth when enabled.
// ponytail: Basic only, not Digest (real Shelly Gen2 uses Digest for RPC) -
// upgrade path: add digest challenge-response if a consumer requires it.
func withBasicAuth(cfg Config, next http.Handler) http.Handler {
	if !cfg.Auth.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != cfg.Auth.Username || pass != cfg.Auth.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="shelly-simulator"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
