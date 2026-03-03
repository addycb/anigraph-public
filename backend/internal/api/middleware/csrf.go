package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"anigraph/backend/internal/api/httputil"
)

// GenerateCSRFToken returns a 32-byte hex-encoded random token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetCSRFCookie sets the anigraph_csrf cookie and returns the token value.
func SetCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "anigraph_csrf",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: false,              // must be readable by JS
		Secure:   !isLocalhost(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// RequireCSRF is middleware that validates the double-submit cookie pattern
// for state-changing methods (POST, PATCH, DELETE, PUT).
func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		if method == "POST" || method == "PATCH" || method == "DELETE" || method == "PUT" {
			cookieToken := cookieValue(r, "anigraph_csrf")
			headerToken := r.Header.Get("X-CSRF-Token")

			if cookieToken == "" || headerToken == "" {
				httputil.Error(w, http.StatusForbidden, "CSRF token missing")
				return
			}

			// Timing-safe comparison
			if !hmac.Equal([]byte(cookieToken), []byte(headerToken)) {
				httputil.Error(w, http.StatusForbidden, "Invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalhost(r *http.Request) bool {
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	return strings.Contains(host, "localhost")
}
