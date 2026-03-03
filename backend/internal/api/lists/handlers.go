package lists

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
	"anigraph/backend/internal/api/middleware"
	"anigraph/backend/internal/api/user"
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

// ---------- GET /api/user/lists ----------

func (h *Handler) GetLists(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	showAdult := httputil.QueryBool(r, "includeAdult")

	rows, err := h.pg.Query(r.Context(), `
		SELECT
			ul.id, ul.name, ul.description, ul.is_public, ul.share_token,
			ul.list_type, ul.created_at, ul.updated_at,
			COUNT(uli.id) FILTER (WHERE $2 = true OR a.is_adult IS NULL OR a.is_adult = false)::integer as item_count,
			ARRAY_AGG(a.anilist_id) FILTER (WHERE a.anilist_id IS NOT NULL AND ($2 = true OR a.is_adult IS NULL OR a.is_adult = false)) as items
		FROM user_lists ul
		LEFT JOIN user_list_items uli ON ul.id = uli.list_id
		LEFT JOIN anime a ON uli.anime_id = a.id
		WHERE ul.user_id = $1
		GROUP BY ul.id
		ORDER BY ul.created_at DESC`,
		userID, showAdult)
	if err != nil {
		log.Printf("get lists: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch lists")
		return
	}
	defer rows.Close()

	data := make([]map[string]any, 0)
	for rows.Next() {
		var (
			id        int
			name      string
			desc      *string
			isPublic  bool
			shareToken *string
			listType  string
			createdAt time.Time
			updatedAt time.Time
			itemCount int
			items     []int
		)
		if err := rows.Scan(&id, &name, &desc, &isPublic, &shareToken,
			&listType, &createdAt, &updatedAt, &itemCount, &items); err != nil {
			log.Printf("scan list: %v", err)
			continue
		}
		if items == nil {
			items = []int{}
		}
		data = append(data, map[string]any{
			"id":         id,
			"name":       name,
			"description": desc,
			"isPublic":   isPublic,
			"shareToken": shareToken,
			"listType":   listType,
			"createdAt":  createdAt,
			"updatedAt":  updatedAt,
			"itemCount":  itemCount,
			"items":      items,
		})
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// ---------- POST /api/user/lists ----------

func (h *Handler) CreateList(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		IsPublic    bool    `json:"isPublic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required and must be a string")
		return
	}
	if len(body.Name) > 255 {
		httputil.Error(w, http.StatusBadRequest, "List name must be 255 characters or less")
		return
	}
	if body.Description != nil {
		trimmed := strings.TrimSpace(*body.Description)
		if len(trimmed) > 1000 {
			httputil.Error(w, http.StatusBadRequest, "Description must be 1000 characters or less")
			return
		}
		if trimmed == "" {
			body.Description = nil
		} else {
			body.Description = &trimmed
		}
	}

	var (
		id        int
		name      string
		desc      *string
		isPublic  bool
		shareToken *string
		createdAt time.Time
		updatedAt time.Time
	)

	err := h.pg.QueryRow(r.Context(),
		`INSERT INTO user_lists (user_id, name, description, is_public, share_token)
		 VALUES ($1, $2, $3, $4, gen_random_uuid()::text)
		 RETURNING id, name, description, is_public, share_token, created_at, updated_at`,
		userID, body.Name, body.Description, body.IsPublic,
	).Scan(&id, &name, &desc, &isPublic, &shareToken, &createdAt, &updatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			httputil.Error(w, http.StatusBadRequest, "You already have a list with this name")
			return
		}
		log.Printf("create list: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to create list")
		return
	}

	// Audit log (non-blocking).
	go h.auditLog(userID, "list.create", "list", strconv.Itoa(id), map[string]any{
		"name": name, "isPublic": isPublic,
	})

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"id": id, "name": name, "description": desc,
			"isPublic": isPublic, "shareToken": shareToken,
			"createdAt": createdAt, "updatedAt": updatedAt,
			"itemCount": 0, "items": []int{},
		},
	})
}

// ---------- PATCH /api/user/lists/{id} ----------

func (h *Handler) UpdateList(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		IsPublic    *bool   `json:"isPublic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate.
	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if len(trimmed) > 255 {
			httputil.Error(w, http.StatusBadRequest, "name must be a string with max 255 characters")
			return
		}
		body.Name = &trimmed
	}
	if body.Description != nil {
		trimmed := strings.TrimSpace(*body.Description)
		if len(trimmed) > 1000 {
			httputil.Error(w, http.StatusBadRequest, "description must be 1000 characters or less")
			return
		}
		if trimmed == "" {
			body.Description = nil
		} else {
			body.Description = &trimmed
		}
	}

	ctx := r.Context()

	// Verify ownership.
	var currentName string
	var currentPublic bool
	err = h.pg.QueryRow(ctx,
		`SELECT name, is_public FROM user_lists WHERE id = $1 AND user_id = $2`,
		listID, userID,
	).Scan(&currentName, &currentPublic)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "List not found or access denied")
		return
	}
	if err != nil {
		log.Printf("update list verify: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update list")
		return
	}

	// Build dynamic update.
	updates := []string{}
	values := []any{userID, listID}
	paramN := 2

	if body.Name != nil {
		paramN++
		updates = append(updates, fmt.Sprintf("name = $%d", paramN))
		values = append(values, *body.Name)
	}
	if body.Description != nil {
		paramN++
		updates = append(updates, fmt.Sprintf("description = $%d", paramN))
		values = append(values, *body.Description)
	}
	if body.IsPublic != nil {
		paramN++
		updates = append(updates, fmt.Sprintf("is_public = $%d", paramN))
		values = append(values, *body.IsPublic)
		if *body.IsPublic {
			updates = append(updates, "share_token = COALESCE(share_token, gen_random_uuid()::text)")
		}
	}

	if len(updates) == 0 {
		httputil.Error(w, http.StatusBadRequest, "No fields to update")
		return
	}

	var (
		id         int
		name       string
		desc       *string
		isPublic   bool
		shareToken *string
		listType   string
		createdAt  time.Time
		updatedAt  time.Time
	)

	err = h.pg.QueryRow(ctx,
		fmt.Sprintf(`UPDATE user_lists SET %s WHERE user_id = $1 AND id = $2
		 RETURNING id, name, description, is_public, share_token, list_type, created_at, updated_at`,
			strings.Join(updates, ", ")),
		values...,
	).Scan(&id, &name, &desc, &isPublic, &shareToken, &listType, &createdAt, &updatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			httputil.Error(w, http.StatusBadRequest, "You already have a list with this name")
			return
		}
		log.Printf("update list: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update list")
		return
	}

	// Get item count and items.
	showAdult := httputil.QueryBool(r, "includeAdult")
	var itemCount int
	var items []int

	_ = h.pg.QueryRow(ctx,
		`SELECT
			COUNT(uli.id) FILTER (WHERE $2 = true OR a.is_adult IS NULL OR a.is_adult = false)::integer,
			ARRAY_AGG(a.anilist_id) FILTER (WHERE a.anilist_id IS NOT NULL AND ($2 = true OR a.is_adult IS NULL OR a.is_adult = false))
		 FROM user_list_items uli
		 LEFT JOIN anime a ON uli.anime_id = a.id
		 WHERE uli.list_id = $1`,
		listID, showAdult,
	).Scan(&itemCount, &items)
	if items == nil {
		items = []int{}
	}

	// Audit log.
	go h.auditLog(userID, "list.update", "list", strconv.Itoa(listID), map[string]any{
		"name": currentName,
	})

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"id": id, "name": name, "description": desc,
			"isPublic": isPublic, "shareToken": shareToken, "listType": listType,
			"createdAt": createdAt, "updatedAt": updatedAt,
			"itemCount": itemCount, "items": items,
		},
	})
}

// ---------- DELETE /api/user/lists/{id} ----------

func (h *Handler) DeleteList(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	ctx := r.Context()

	// Verify list exists.
	var listName string
	err = h.pg.QueryRow(ctx,
		`SELECT name FROM user_lists WHERE id = $1`, listID,
	).Scan(&listName)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "List not found")
		return
	}
	if err != nil {
		log.Printf("delete list check: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to delete list")
		return
	}

	// Delete only if owned.
	var deletedID int
	err = h.pg.QueryRow(ctx,
		`DELETE FROM user_lists WHERE user_id = $1 AND id = $2 RETURNING id`,
		userID, listID,
	).Scan(&deletedID)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusForbidden, "You don't have permission to delete this list")
		return
	}
	if err != nil {
		log.Printf("delete list: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to delete list")
		return
	}

	go h.auditLog(userID, "list.delete", "list", strconv.Itoa(listID), map[string]any{
		"name": listName,
	})

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "List deleted successfully",
	})
}

// ---------- GET /api/user/lists/{id}/items ----------

func (h *Handler) GetListItems(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	ctx := r.Context()
	showAdult := httputil.QueryBool(r, "includeAdult")

	// Check list exists and access.
	var listUserID string
	var isPublic bool
	err = h.pg.QueryRow(ctx,
		`SELECT user_id, is_public FROM user_lists WHERE id = $1`, listID,
	).Scan(&listUserID, &isPublic)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "List not found")
		return
	}
	if err != nil {
		log.Printf("get list items check: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch list items")
		return
	}

	authUserID := middleware.GetUserID(ctx)
	isOwner := authUserID != "" && listUserID == authUserID
	if !isPublic && !isOwner {
		httputil.Error(w, http.StatusForbidden, "Access denied. This list is private.")
		return
	}

	items := h.fetchListItems(ctx, listID, showAdult)
	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": items})
}

// ---------- POST /api/user/lists/{id}/items ----------

func (h *Handler) AddListItem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	var body struct {
		AnimeID *int    `json:"animeId"`
		Notes   *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.AnimeID == nil {
		httputil.Error(w, http.StatusBadRequest, "animeId is required")
		return
	}
	if body.Notes != nil && len(*body.Notes) > 1000 {
		httputil.Error(w, http.StatusBadRequest, "Notes must be 1000 characters or less")
		return
	}

	ctx := r.Context()

	// Verify ownership.
	var listCheck int
	err = h.pg.QueryRow(ctx,
		`SELECT id FROM user_lists WHERE id = $1 AND user_id = $2`,
		listID, userID,
	).Scan(&listCheck)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "List not found or you don't have permission to modify it")
		return
	}
	if err != nil {
		log.Printf("add list item check: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to add to list")
		return
	}

	// Get internal anime ID.
	var internalID int
	err = h.pg.QueryRow(ctx,
		`SELECT id FROM anime WHERE anilist_id = $1`, *body.AnimeID,
	).Scan(&internalID)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Anime not found")
		return
	}
	if err != nil {
		log.Printf("add list item anime lookup: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to add to list")
		return
	}

	var notes *string
	if body.Notes != nil {
		trimmed := strings.TrimSpace(*body.Notes)
		if trimmed != "" {
			notes = &trimmed
		}
	}

	// Insert/upsert.
	_, err = h.pg.Exec(ctx,
		`INSERT INTO user_list_items (list_id, anime_id, notes)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (list_id, anime_id) DO UPDATE SET notes = EXCLUDED.notes`,
		listID, internalID, notes)
	if err != nil {
		log.Printf("add list item insert: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to add to list")
		return
	}

	// Touch updated_at.
	_, _ = h.pg.Exec(ctx,
		`UPDATE user_lists SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, listID)

	// Invalidate cache.
	go h.invalidateCache(listID)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Added to list successfully",
	})
}

// ---------- DELETE /api/user/lists/{id}/items ----------

func (h *Handler) RemoveListItem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	var body struct {
		AnimeID *int `json:"animeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.AnimeID == nil {
		httputil.Error(w, http.StatusBadRequest, "animeId is required")
		return
	}

	ctx := r.Context()

	// Verify ownership.
	var listCheck int
	err = h.pg.QueryRow(ctx,
		`SELECT id FROM user_lists WHERE id = $1 AND user_id = $2`,
		listID, userID,
	).Scan(&listCheck)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "List not found or you don't have permission to modify it")
		return
	}
	if err != nil {
		log.Printf("remove list item check: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to remove from list")
		return
	}

	// Get internal anime ID.
	var internalID int
	err = h.pg.QueryRow(ctx,
		`SELECT id FROM anime WHERE anilist_id = $1`, *body.AnimeID,
	).Scan(&internalID)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Anime not found")
		return
	}
	if err != nil {
		log.Printf("remove list item anime lookup: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to remove from list")
		return
	}

	_, err = h.pg.Exec(ctx,
		`DELETE FROM user_list_items WHERE list_id = $1 AND anime_id = $2`,
		listID, internalID)
	if err != nil {
		log.Printf("remove list item delete: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to remove from list")
		return
	}

	// Touch updated_at.
	_, _ = h.pg.Exec(ctx,
		`UPDATE user_lists SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, listID)

	// Invalidate cache.
	go h.invalidateCache(listID)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Removed from list successfully",
	})
}

// ---------- GET /api/lists/public ----------

func (h *Handler) PublicLists(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	if !h.limiter.Allow("public-lists:"+clientIP, 100, time.Minute) {
		httputil.Error(w, http.StatusTooManyRequests, "Too many requests")
		return
	}

	limit := httputil.QueryInt(r, "limit", 50)
	if limit > 500 {
		limit = 500
	}
	search := httputil.QueryString(r, "search", "")
	showAdult := httputil.QueryBool(r, "includeAdult")

	if len(search) > 100 {
		httputil.Error(w, http.StatusBadRequest, "Search query too long (max 100 characters)")
		return
	}

	params := []any{showAdult}
	query := `
		SELECT
			ul.id, ul.name, ul.description, ul.share_token, ul.created_at,
			COUNT(uli.id)::integer as item_count,
			ARRAY_AGG(a.cover_image_large ORDER BY uli.added_at DESC)
				FILTER (WHERE a.cover_image_large IS NOT NULL AND ($1 = true OR a.is_adult IS NULL OR a.is_adult = false)) as previews
		FROM user_lists ul
		LEFT JOIN user_list_items uli ON ul.id = uli.list_id
		LEFT JOIN anime a ON uli.anime_id = a.id
		WHERE ul.is_public = true AND ul.list_type != 'favorites'`

	if search != "" {
		params = append(params, "%"+search+"%")
		query += fmt.Sprintf(` AND (ul.name ILIKE $%d OR ul.description ILIKE $%d)`, len(params), len(params))
	}

	params = append(params, limit)
	query += fmt.Sprintf(`
		GROUP BY ul.id
		ORDER BY ul.created_at DESC
		LIMIT $%d`, len(params))

	rows, err := h.pg.Query(r.Context(), query, params...)
	if err != nil {
		log.Printf("public lists: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch public lists")
		return
	}
	defer rows.Close()

	data := make([]map[string]any, 0)
	for rows.Next() {
		var (
			id         int
			name       string
			desc       *string
			shareToken *string
			createdAt  time.Time
			itemCount  int
			previews   []string
		)
		if err := rows.Scan(&id, &name, &desc, &shareToken, &createdAt, &itemCount, &previews); err != nil {
			log.Printf("scan public list: %v", err)
			continue
		}
		if previews == nil {
			previews = []string{}
		}
		if len(previews) > 4 {
			previews = previews[:4]
		}
		data = append(data, map[string]any{
			"id":         id,
			"name":       name,
			"description": desc,
			"shareToken": shareToken,
			"createdAt":  createdAt,
			"itemCount":  itemCount,
			"previews":   previews,
		})
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// ---------- GET /api/lists/share/{token} ----------

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (h *Handler) ShareToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		httputil.Error(w, http.StatusBadRequest, "Share token is required")
		return
	}

	if !uuidRegex.MatchString(strings.ToLower(token)) {
		httputil.Error(w, http.StatusBadRequest, "Invalid share token format")
		return
	}

	clientIP := getClientIP(r)
	if !h.limiter.Allow("share-token:"+clientIP, 60, time.Minute) {
		httputil.Error(w, http.StatusTooManyRequests, "Too many requests")
		return
	}

	ctx := r.Context()
	showAdult := httputil.QueryBool(r, "includeAdult")

	// Get list.
	var (
		listID    int
		name      string
		desc      *string
		shareToken string
		listType  string
		createdAt time.Time
		listUserID string
	)

	err := h.pg.QueryRow(ctx,
		`SELECT id, name, description, share_token, list_type, created_at, user_id
		 FROM user_lists WHERE share_token = $1 AND is_public = true`,
		token,
	).Scan(&listID, &name, &desc, &shareToken, &listType, &createdAt, &listUserID)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "List not found or is not public")
		return
	}
	if err != nil {
		log.Printf("share token lookup: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch list")
		return
	}

	items := h.fetchShareItems(ctx, listID, showAdult)

	// Audit log (non-blocking).
	go h.auditLog(listUserID, "list.share_access", "list", strconv.Itoa(listID), map[string]any{
		"listName":    name,
		"accessedVia": "share_token",
		"itemCount":   len(items),
	})

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"list": map[string]any{
				"id": listID, "name": name, "description": desc,
				"shareToken": shareToken, "listType": listType,
				"createdAt": createdAt, "itemCount": len(items),
			},
			"items": items,
		},
	})
}

// ======================== helpers ========================

func (h *Handler) fetchListItems(ctx context.Context, listID int, showAdult bool) []map[string]any {
	rows, err := h.pg.Query(ctx, `
		SELECT
			a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium, a.banner_image,
			a.episodes, a.season, a.season_year,
			a.average_score::integer, a.popularity, a.format, a.status,
			uli.notes, uli.added_at
		FROM user_list_items uli
		JOIN anime a ON uli.anime_id = a.id
		WHERE uli.list_id = $1
			AND ($2 = true OR a.is_adult IS NULL OR a.is_adult = false)
		ORDER BY uli.added_at DESC`,
		listID, showAdult)
	if err != nil {
		log.Printf("fetch list items: %v", err)
		return []map[string]any{}
	}
	defer rows.Close()

	return scanAnimeItems(rows, true)
}

func (h *Handler) fetchShareItems(ctx context.Context, listID int, showAdult bool) []map[string]any {
	rows, err := h.pg.Query(ctx, `
		SELECT
			a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium,
			a.episodes, a.season, a.season_year,
			a.average_score::integer, a.popularity, a.format, a.status,
			uli.added_at
		FROM user_list_items uli
		JOIN anime a ON uli.anime_id = a.id
		WHERE uli.list_id = $1
			AND ($2 = true OR a.is_adult IS NULL OR a.is_adult = false)
		ORDER BY uli.added_at DESC`,
		listID, showAdult)
	if err != nil {
		log.Printf("fetch share items: %v", err)
		return []map[string]any{}
	}
	defer rows.Close()

	return scanAnimeItems(rows, false)
}

func scanAnimeItems(rows pgx.Rows, includeNotes bool) []map[string]any {
	items := make([]map[string]any, 0)
	for rows.Next() {
		var (
			anilistID                                                int
			title                                                    string
			coverImage, coverImageXL, coverImageLg, coverImageMd     *string
			episodes                                                 *int
			season                                                   *string
			seasonYear                                               *int
			averageScore, popularity                                 *int
			format, status                                           *string
			addedAt                                                  time.Time
		)

		if includeNotes {
			var bannerImage *string
			var notes *string
			if err := rows.Scan(
				&anilistID, &title, &coverImage, &coverImageXL,
				&coverImageLg, &coverImageMd, &bannerImage,
				&episodes, &season, &seasonYear,
				&averageScore, &popularity, &format, &status,
				&notes, &addedAt,
			); err != nil {
				log.Printf("scan anime item: %v", err)
				continue
			}
			items = append(items, map[string]any{
				"id":                    anilistID,
				"anilistId":             anilistID,
				"title":                 title,
				"coverImage":            coverImage,
				"coverImage_extraLarge": coverImageXL,
				"coverImage_large":      coverImageLg,
				"coverImage_medium":     coverImageMd,
				"bannerImage":           bannerImage,
				"episodes":              episodes,
				"season":                season,
				"seasonYear":            seasonYear,
				"averageScore":          averageScore,
				"popularity":            popularity,
				"format":                format,
				"status":                status,
				"notes":                 notes,
				"addedAt":               addedAt,
			})
		} else {
			if err := rows.Scan(
				&anilistID, &title, &coverImage, &coverImageXL,
				&coverImageLg, &coverImageMd,
				&episodes, &season, &seasonYear,
				&averageScore, &popularity, &format, &status,
				&addedAt,
			); err != nil {
				log.Printf("scan anime item: %v", err)
				continue
			}
			items = append(items, map[string]any{
				"id":                    anilistID,
				"anilistId":             anilistID,
				"title":                 title,
				"coverImage":            coverImage,
				"coverImage_extraLarge": coverImageXL,
				"coverImage_large":      coverImageLg,
				"coverImage_medium":     coverImageMd,
				"episodes":              episodes,
				"season":                season,
				"seasonYear":            seasonYear,
				"averageScore":          averageScore,
				"popularity":            popularity,
				"format":                format,
				"status":                status,
				"addedAt":               addedAt,
			})
		}
	}
	return items
}

func (h *Handler) invalidateCache(listID int) {
	ctx := context.Background()
	rows, err := h.pg.Query(ctx,
		`SELECT a.anilist_id FROM user_list_items uli JOIN anime a ON uli.anime_id = a.id WHERE uli.list_id = $1`,
		listID)
	if err != nil {
		return
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	user.InvalidateListCache(h.pg, ctx, ids)
}

func (h *Handler) auditLog(userID, action, resourceType, resourceID string, metadata map[string]any) {
	ctx := context.Background()
	metaJSON, _ := json.Marshal(metadata)
	_, err := h.pg.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata)
		 VALUES ($1, $2, $3, $4, $5::jsonb)`,
		userID, action, resourceType, resourceID, string(metaJSON))
	if err != nil {
		log.Printf("audit log: %v", err)
	}
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}
