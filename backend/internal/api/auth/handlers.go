package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
	"anigraph/backend/internal/api/middleware"
)

type Handler struct {
	pg      *pgxpool.Pool
	limiter *middleware.RateLimiter
}

func NewHandler(pg *pgxpool.Pool) *Handler {
	return &Handler{
		pg:      pg,
		limiter: middleware.NewRateLimiter(),
	}
}

// googleUserInfo mirrors the response from Google's userinfo endpoint.
type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// ---------- GET /api/auth/google/login ----------

func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		httputil.Error(w, http.StatusInternalServerError, "Google OAuth not configured")
		return
	}

	state, err := randomHex(32)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Failed to generate state")
		return
	}

	localhost := isLocalhost(r)

	// Store state in cookie for CSRF validation in callback.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   !localhost,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURI := buildRedirectURI(r, localhost)

	authURL := "https://accounts.google.com/o/oauth2/v2/auth" +
		"?client_id=" + url.QueryEscape(clientID) +
		"&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&response_type=code" +
		"&scope=" + url.QueryEscape("email profile") +
		"&access_type=offline" +
		"&prompt=consent" +
		"&state=" + url.QueryEscape(state)

	http.Redirect(w, r, authURL, http.StatusFound)
}

// ---------- GET /api/auth/google/callback ----------

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		httputil.Error(w, http.StatusBadRequest, "Authorization code missing")
		return
	}

	// Validate state parameter (CSRF).
	storedState := cookieVal(r, "oauth_state")
	if state == "" || storedState == "" || state != storedState {
		// If state cookie was already consumed but user has a valid session,
		// this is a duplicate/replayed callback — just redirect home.
		if sessionID := cookieVal(r, "anigraph_session"); sessionID != "" {
			var exists bool
			_ = h.pg.QueryRow(r.Context(),
				`SELECT EXISTS(SELECT 1 FROM user_sessions WHERE session_id = $1 AND expires_at > NOW())`,
				sessionID,
			).Scan(&exists)
			if exists {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		httputil.Error(w, http.StatusForbidden, "Invalid state parameter - possible CSRF attack")
		return
	}

	// Clear state cookie.
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", MaxAge: -1, Path: "/"})

	// Rate limit OAuth callbacks.
	clientIP := getClientIP(r)
	if !h.limiter.Allow("oauth-callback:"+clientIP, 10, 5*time.Minute) {
		httputil.Error(w, http.StatusTooManyRequests, "Too many requests. Please try again later.")
		return
	}

	localhost := isLocalhost(r)
	redirectURI := buildRedirectURI(r, localhost)

	// Exchange code for access token.
	tokenData, err := exchangeCodeForToken(code, redirectURI)
	if err != nil {
		log.Printf("oauth token exchange: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "OAuth failed: "+err.Error())
		return
	}

	accessToken, _ := tokenData["access_token"].(string)
	if accessToken == "" {
		httputil.Error(w, http.StatusInternalServerError, "OAuth failed: no access token")
		return
	}

	// Fetch Google user info.
	googleUser, err := fetchGoogleUserInfo(accessToken)
	if err != nil {
		log.Printf("oauth user info: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "OAuth failed: "+err.Error())
		return
	}

	// Find or create user in database.
	userID, err := h.findOrCreateGoogleUser(r.Context(), googleUser)
	if err != nil {
		log.Printf("oauth find/create user: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "OAuth failed: "+err.Error())
		return
	}

	// Merge anonymous user if cookie present.
	anonymousID := cookieVal(r, "anigraph_anonymous_id")
	if anonymousID != "" && anonymousID != userID {
		if h.limiter.Allow("user-merge:"+clientIP, 3, time.Hour) {
			if err := h.mergeAnonymousUser(r.Context(), anonymousID, userID); err != nil {
				log.Printf("anonymous merge: %v", err)
				// Non-critical — continue with login.
			}
		}
	}

	// Create session.
	sessionID, err := h.createSession(r.Context(), userID, 30)
	if err != nil {
		log.Printf("oauth create session: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "OAuth failed: session creation")
		return
	}

	// Set session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "anigraph_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   !localhost,
		SameSite: http.SameSiteLaxMode,
	})

	// Set CSRF token.
	csrfToken, _ := middleware.GenerateCSRFToken()
	middleware.SetCSRFCookie(w, r, csrfToken)

	// Clear anonymous cookie.
	if anonymousID != "" {
		http.SetCookie(w, &http.Cookie{Name: "anigraph_anonymous_id", Value: "", MaxAge: -1, Path: "/"})
	}

	// Redirect to return URL or home.
	returnURL := cookieVal(r, "oauth_return_url")
	redirectPath := "/"
	if returnURL != "" {
		if decoded, err := url.QueryUnescape(returnURL); err == nil {
			redirectPath = decoded
		} else {
			redirectPath = returnURL
		}
		// Prevent open redirects: must be a relative path
		if !strings.HasPrefix(redirectPath, "/") || strings.Contains(redirectPath, "://") {
			redirectPath = "/"
		}
		http.SetCookie(w, &http.Cookie{Name: "oauth_return_url", Value: "", MaxAge: -1, Path: "/"})
	}

	protocol := "https"
	if localhost {
		protocol = "http"
	}
	host := r.Host
	sep := "?"
	if strings.Contains(redirectPath, "?") {
		sep = "&"
	}
	http.Redirect(w, r, fmt.Sprintf("%s://%s%s%slogin=success", protocol, host, redirectPath, sep), http.StatusFound)
}

// ---------- GET /api/auth/me ----------

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	sessionID := cookieVal(r, "anigraph_session")
	if sessionID == "" {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"user":          nil,
		})
		return
	}

	// Validate session.
	var userID string
	err := h.pg.QueryRow(r.Context(),
		`SELECT user_id FROM user_sessions
		 WHERE session_id = $1 AND expires_at > NOW()`,
		sessionID,
	).Scan(&userID)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Expired or invalid — clear cookie.
			http.SetCookie(w, &http.Cookie{Name: "anigraph_session", Value: "", MaxAge: -1, Path: "/"})
		}
		httputil.JSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"user":          nil,
		})
		return
	}

	// Get user info.
	var email, name, picture *string
	var isAnonymous bool
	var createdAt time.Time

	err = h.pg.QueryRow(r.Context(),
		`SELECT email, name, picture, is_anonymous, created_at
		 FROM users WHERE user_id = $1`,
		userID,
	).Scan(&email, &name, &picture, &isAnonymous, &createdAt)

	if err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"user":          nil,
		})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user": map[string]any{
			"userId":      userID,
			"email":       email,
			"name":        name,
			"picture":     picture,
			"isAnonymous": isAnonymous,
			"createdAt":   createdAt,
		},
	})
}

// ---------- GET /api/auth/csrf-token ----------

func (h *Handler) CSRFToken(w http.ResponseWriter, r *http.Request) {
	token := cookieVal(r, "anigraph_csrf")
	if token == "" {
		var err error
		token, err = middleware.GenerateCSRFToken()
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, "Failed to generate CSRF token")
			return
		}
		middleware.SetCSRFCookie(w, r, token)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"token": token})
}

// ---------- POST /api/auth/logout ----------

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// CSRF is validated by RequireCSRF middleware on the route group.
	sessionID := cookieVal(r, "anigraph_session")
	if sessionID != "" {
		_, err := h.pg.Exec(r.Context(),
			`DELETE FROM user_sessions WHERE session_id = $1`, sessionID)
		if err != nil {
			log.Printf("logout delete session: %v", err)
		}
	}

	http.SetCookie(w, &http.Cookie{Name: "anigraph_session", Value: "", MaxAge: -1, Path: "/"})
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Logged out successfully",
	})
}

// ======================== helpers ========================

func (h *Handler) createSession(ctx context.Context, userID string, days int) (string, error) {
	sessionID, err := randomHex(32)
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().AddDate(0, 0, days)
	_, err = h.pg.Exec(ctx,
		`INSERT INTO user_sessions (session_id, user_id, expires_at)
		 VALUES ($1, $2, $3)`,
		sessionID, userID, expiresAt,
	)
	return sessionID, err
}

func (h *Handler) findOrCreateGoogleUser(ctx context.Context, gu *googleUserInfo) (string, error) {
	tx, err := h.pg.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check by Google ID.
	var userID string
	err = tx.QueryRow(ctx,
		`SELECT user_id FROM users WHERE google_id = $1`, gu.ID,
	).Scan(&userID)

	if err == nil {
		// Existing user — update info.
		_, _ = tx.Exec(ctx,
			`UPDATE users SET name = $1, email = $2, picture = $3, last_active = CURRENT_TIMESTAMP
			 WHERE google_id = $4`,
			gu.Name, gu.Email, gu.Picture, gu.ID,
		)
		h.ensureFavoritesList(ctx, tx, userID)
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return userID, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("lookup by google_id: %w", err)
	}

	// Check by email.
	err = tx.QueryRow(ctx,
		`SELECT user_id FROM users WHERE email = $1`, gu.Email,
	).Scan(&userID)

	if err == nil {
		// Link Google account to existing user.
		_, _ = tx.Exec(ctx,
			`UPDATE users SET google_id = $1, name = $2, picture = $3, is_anonymous = FALSE, last_active = CURRENT_TIMESTAMP
			 WHERE email = $4`,
			gu.ID, gu.Name, gu.Picture, gu.Email,
		)
		h.ensureFavoritesList(ctx, tx, userID)
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return userID, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("lookup by email: %w", err)
	}

	// Create new user.
	err = tx.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name, picture, is_anonymous)
		 VALUES ($1, $2, $3, $4, FALSE) RETURNING user_id`,
		gu.ID, gu.Email, gu.Name, gu.Picture,
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("insert user: %w", err)
	}

	_, _ = tx.Exec(ctx,
		`INSERT INTO user_lists (user_id, name, description, list_type, is_public, share_token)
		 VALUES ($1, 'Favorites', 'Your favorite anime', 'favorites', FALSE, gen_random_uuid()::text)`,
		userID,
	)

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

func (h *Handler) ensureFavoritesList(ctx context.Context, tx pgx.Tx, userID string) {
	_, _ = tx.Exec(ctx,
		`INSERT INTO user_lists (user_id, name, description, list_type, is_public, share_token)
		 SELECT $1::varchar, 'Favorites', 'Your favorite anime', 'favorites', FALSE, gen_random_uuid()::text
		 WHERE NOT EXISTS (
		   SELECT 1 FROM user_lists WHERE user_id = $1 AND list_type = 'favorites'
		 )`,
		userID,
	)
}

func (h *Handler) mergeAnonymousUser(ctx context.Context, anonymousID, authenticatedID string) error {
	tx, err := h.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Validate anonymous user.
	var isAnonymous bool
	var createdAt time.Time
	err = tx.QueryRow(ctx,
		`SELECT is_anonymous, created_at FROM users WHERE user_id = $1`, anonymousID,
	).Scan(&isAnonymous, &createdAt)
	if err != nil {
		return fmt.Errorf("anonymous user not found")
	}
	if !isAnonymous {
		log.Printf("[SECURITY] Attempted to merge non-anonymous user %s", anonymousID)
		return fmt.Errorf("cannot merge non-anonymous user")
	}
	if time.Since(createdAt).Hours() > 7*24 {
		return fmt.Errorf("anonymous user is too old to merge")
	}
	if anonymousID == authenticatedID {
		return fmt.Errorf("cannot merge user with itself")
	}

	// Get or create authenticated user's favorites list.
	var authFavListID int
	err = tx.QueryRow(ctx,
		`SELECT id FROM user_lists WHERE user_id = $1 AND list_type = 'favorites'`, authenticatedID,
	).Scan(&authFavListID)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx,
			`INSERT INTO user_lists (user_id, name, list_type)
			 VALUES ($1, 'Favorites', 'favorites')
			 ON CONFLICT ON CONSTRAINT unique_user_list_name DO UPDATE SET name = 'Favorites'
			 RETURNING id`,
			authenticatedID,
		).Scan(&authFavListID)
	}
	if err != nil {
		return fmt.Errorf("get auth favorites list: %w", err)
	}

	// Check anonymous user's favorites list.
	var anonFavListID int
	var transferredCount int
	err = tx.QueryRow(ctx,
		`SELECT id FROM user_lists WHERE user_id = $1 AND list_type = 'favorites'`, anonymousID,
	).Scan(&anonFavListID)

	if err == nil {
		// Count items for audit.
		_ = tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM user_list_items WHERE list_id = $1`, anonFavListID,
		).Scan(&transferredCount)

		// Transfer items (skip duplicates).
		_, _ = tx.Exec(ctx,
			`INSERT INTO user_list_items (list_id, anime_id, added_at)
			 SELECT $1, anime_id, added_at FROM user_list_items WHERE list_id = $2
			 ON CONFLICT (list_id, anime_id) DO NOTHING`,
			authFavListID, anonFavListID,
		)

		// Delete anonymous user's list items and list.
		_, _ = tx.Exec(ctx, `DELETE FROM user_list_items WHERE list_id = $1`, anonFavListID)
		_, _ = tx.Exec(ctx, `DELETE FROM user_lists WHERE id = $1`, anonFavListID)
	}

	// Touch updated_at.
	_, _ = tx.Exec(ctx,
		`UPDATE user_lists SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, authFavListID)

	// Delete anonymous user.
	_, _ = tx.Exec(ctx,
		`DELETE FROM users WHERE user_id = $1 AND is_anonymous = TRUE`, anonymousID)

	// Audit log.
	metadata := fmt.Sprintf(`{"anonymousUserId":"%s","favoritesTransferred":%d,"timestamp":"%s"}`,
		anonymousID, transferredCount, time.Now().UTC().Format(time.RFC3339))
	_, _ = tx.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata)
		 VALUES ($1, 'user.merge', 'user', $2, $3::jsonb)`,
		authenticatedID, anonymousID, metadata,
	)

	return tx.Commit(ctx)
}

// exchangeCodeForToken calls Google's token endpoint.
func exchangeCodeForToken(code, redirectURI string) (map[string]any, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {os.Getenv("GOOGLE_CLIENT_ID")},
		"client_secret": {os.Getenv("GOOGLE_CLIENT_SECRET")},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	resp, err := http.Post("https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token response %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("token decode: %w", err)
	}
	return result, nil
}

// fetchGoogleUserInfo calls Google's userinfo endpoint.
func fetchGoogleUserInfo(accessToken string) (*googleUserInfo, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo response %d: %s", resp.StatusCode, body)
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("userinfo decode: %w", err)
	}
	return &info, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func cookieVal(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func isLocalhost(r *http.Request) bool {
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	return strings.Contains(host, "localhost")
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP).
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}

func buildRedirectURI(r *http.Request, localhost bool) string {
	protocol := "https"
	host := r.Host
	if localhost {
		protocol = "http"
	}
	return protocol + "://" + host + "/api/auth/google/callback"
}
