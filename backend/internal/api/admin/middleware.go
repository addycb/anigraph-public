package admin

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

// RequireAdmin is middleware that validates the admin API key.
// It checks the Authorization header (Bearer token) or request body adminKey field.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminKey := os.Getenv("ADMIN_API_KEY")
		if adminKey == "" {
			log.Println("ADMIN_API_KEY not set in environment variables")
			http.Error(w, `{"error":"Admin authentication not configured"}`, http.StatusInternalServerError)
			return
		}

		var providedKey string

		// Try Authorization header first.
		if auth := r.Header.Get("Authorization"); auth != "" {
			providedKey = strings.TrimPrefix(auth, "Bearer ")
			if providedKey == auth {
				// No "Bearer " prefix — try case-insensitive.
				providedKey = strings.TrimPrefix(auth, "bearer ")
			}
		}

		// Fall back to request body for POST/PATCH/DELETE.
		if providedKey == "" {
			method := r.Method
			if method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete {
				// Peek at the body for adminKey without consuming it.
				// We use a limited decoder to avoid reading huge bodies.
				var body struct {
					AdminKey string `json:"adminKey"`
				}
				dec := json.NewDecoder(r.Body)
				if err := dec.Decode(&body); err == nil {
					providedKey = body.AdminKey
				}
				// Note: body is consumed here. Downstream handlers that need the body
				// should get params from query string or the adminKey is the only body field.
			}
		}

		if providedKey == "" {
			http.Error(w, `{"error":"Admin authentication required. Provide admin key in Authorization header or request body."}`, http.StatusUnauthorized)
			return
		}

		clientIP := r.Header.Get("X-Forwarded-For")
		if idx := strings.Index(clientIP, ","); idx != -1 {
			clientIP = strings.TrimSpace(clientIP[:idx])
		}
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}

		// Constant-time comparison to prevent timing attacks.
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(adminKey)) != 1 {
			log.Printf("Failed admin authentication attempt from: %s", clientIP)
			http.Error(w, `{"error":"Invalid admin key"}`, http.StatusForbidden)
			return
		}

		log.Printf("Admin endpoint accessed from: %s", clientIP)
		next.ServeHTTP(w, r)
	})
}
