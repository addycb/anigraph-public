package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
)

type contextKey string

const (
	// UserIDKey is the context key for the authenticated user's UUID.
	UserIDKey contextKey = "userId"
	// UserIsAnonymousKey is the context key for whether the user is anonymous.
	UserIsAnonymousKey contextKey = "userIsAnonymous"
	// UserEmailKey is the context key for the authenticated user's email.
	UserEmailKey contextKey = "userEmail"
	// UserNameKey is the context key for the authenticated user's display name.
	UserNameKey contextKey = "userName"
)

// GetUserID returns the authenticated user ID from context, or empty string.
func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

// Auth is middleware that validates the anigraph_session cookie against the
// user_sessions table. On success it sets user info in the request context.
// If required is true, returns 401 when not authenticated.
func Auth(pg *pgxpool.Pool, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := cookieValue(r, "anigraph_session")
			if sessionID == "" {
				if required {
					httputil.Error(w, http.StatusUnauthorized, "Authentication required")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			var userID, email, name string
			var isAnonymous bool

			err := pg.QueryRow(r.Context(),
				`SELECT s.user_id, u.email, u.name, u.is_anonymous
				 FROM user_sessions s
				 JOIN users u ON s.user_id = u.user_id
				 WHERE s.session_id = $1 AND s.expires_at > NOW()`,
				sessionID,
			).Scan(&userID, &email, &name, &isAnonymous)

			if err != nil {
				if err == pgx.ErrNoRows {
					if required {
						httputil.Error(w, http.StatusUnauthorized, "Invalid or expired session")
						return
					}
					next.ServeHTTP(w, r)
					return
				}
				log.Printf("auth middleware: %v", err)
				if required {
					httputil.Error(w, http.StatusInternalServerError, "Session validation failed")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserIsAnonymousKey, isAnonymous)
			ctx = context.WithValue(ctx, UserEmailKey, email)
			ctx = context.WithValue(ctx, UserNameKey, name)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
