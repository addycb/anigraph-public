package user

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
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

// ---------- GET /api/user/preferences ----------

func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var themeID string
	var includeAdult bool

	err := h.pg.QueryRow(r.Context(),
		`SELECT theme_id, include_adult FROM user_preferences WHERE user_id = $1`,
		userID,
	).Scan(&themeID, &includeAdult)

	if err == pgx.ErrNoRows {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"theme":        "midnight",
			"includeAdult": false,
		})
		return
	}
	if err != nil {
		log.Printf("get preferences: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch preferences")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"theme":        themeID,
		"includeAdult": includeAdult,
	})
}

// ---------- PATCH /api/user/preferences ----------

var validThemes = map[string]bool{
	"midnight": true, "sakura": true, "emerald": true, "amber": true,
	"slate": true, "asiimov": true, "healing": true, "scholar": true,
	"sakura-light": true, "scholar-light": true, "asiimov-light": true,
	"strawberry": true, "birthday": true, "birthday2": true,
}

func (h *Handler) PatchPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Theme        *string `json:"theme"`
		IncludeAdult *bool   `json:"includeAdult"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Theme != nil && !validThemes[*body.Theme] {
		httputil.Error(w, http.StatusBadRequest, "Invalid theme")
		return
	}

	var themeParam, adultParam any
	if body.Theme != nil {
		themeParam = *body.Theme
	}
	if body.IncludeAdult != nil {
		adultParam = *body.IncludeAdult
	}

	_, err := h.pg.Exec(r.Context(),
		`INSERT INTO user_preferences (user_id, theme_id, include_adult)
		 VALUES ($1, COALESCE($2, 'midnight'), COALESCE($3, false))
		 ON CONFLICT (user_id) DO UPDATE SET
		   theme_id = COALESCE($2::varchar, user_preferences.theme_id),
		   include_adult = COALESCE($3::boolean, user_preferences.include_adult),
		   updated_at = NOW()`,
		userID, themeParam, adultParam,
	)
	if err != nil {
		log.Printf("patch preferences: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// ---------- GET /api/user/favorites ----------

func (h *Handler) GetFavorites(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	limitParam := httputil.QueryIntPtr(r, "limit")
	offset := httputil.QueryInt(r, "offset", 0)
	showAdult := httputil.QueryBool(r, "includeAdult")

	params := []any{userID, showAdult}
	limitClause := ""
	if limitParam != nil {
		limit := *limitParam
		if limit > 10000 {
			limit = 10000
		}
		params = append(params, limit, offset)
		limitClause = "LIMIT $3 OFFSET $4"
	}

	rows, err := h.pg.Query(r.Context(), fmt.Sprintf(`
		SELECT
			a.anilist_id,
			a.title,
			a.cover_image,
			a.cover_image_extra_large,
			a.cover_image_large,
			a.cover_image_medium,
			a.banner_image,
			a.episodes,
			a.season,
			a.season_year,
			a.average_score::integer as average_score,
			a.popularity,
			a.format,
			a.status,
			uli.notes,
			uli.added_at
		FROM user_list_items uli
		JOIN user_lists ul ON uli.list_id = ul.id
		JOIN anime a ON uli.anime_id = a.id
		WHERE ul.user_id = $1 AND ul.list_type = 'favorites'
			AND ($2 = true OR a.is_adult IS NULL OR a.is_adult = false)
		ORDER BY uli.added_at DESC
		%s`, limitClause), params...)
	if err != nil {
		log.Printf("get favorites: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch favorites")
		return
	}
	defer rows.Close()

	favorites := make([]map[string]any, 0)
	for rows.Next() {
		var (
			anilistID                                                     int
			title                                                         string
			coverImage, coverImageXL, coverImageLg, coverImageMd          *string
			bannerImage                                                   *string
			episodes                                                      *int
			season                                                        *string
			seasonYear                                                    *int
			averageScore, popularity                                      *int
			format, status                                                *string
			notes                                                         *string
			addedAt                                                       time.Time
		)
		if err := rows.Scan(
			&anilistID, &title,
			&coverImage, &coverImageXL, &coverImageLg, &coverImageMd,
			&bannerImage, &episodes, &season, &seasonYear,
			&averageScore, &popularity, &format, &status,
			&notes, &addedAt,
		); err != nil {
			log.Printf("scan favorite: %v", err)
			continue
		}
		favorites = append(favorites, map[string]any{
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
			"favoritedAt":           addedAt,
		})
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    favorites,
		"total":   len(favorites),
	})
}

// ---------- POST /api/user/favorite-anime ----------

func (h *Handler) FavoriteAnime(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		AnimeID  *int  `json:"animeId"`
		Favorite *bool `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.AnimeID == nil || body.Favorite == nil {
		httputil.Error(w, http.StatusBadRequest, "animeId and favorite are required")
		return
	}

	ctx := r.Context()
	tx, err := h.pg.Begin(ctx)
	if err != nil {
		log.Printf("favorite-anime begin tx: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update favorite")
		return
	}
	defer tx.Rollback(ctx)

	// Ensure user exists.
	_, _ = tx.Exec(ctx,
		`INSERT INTO users (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userID)

	// Get or create favorites list.
	var favListID int
	err = tx.QueryRow(ctx,
		`SELECT id FROM user_lists WHERE user_id = $1 AND list_type = 'favorites'`,
		userID,
	).Scan(&favListID)

	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx,
			`INSERT INTO user_lists (user_id, name, list_type)
			 VALUES ($1, 'Favorites', 'favorites')
			 ON CONFLICT ON CONSTRAINT unique_user_list_name DO UPDATE SET name = 'Favorites'
			 RETURNING id`,
			userID,
		).Scan(&favListID)
	}
	if err != nil {
		log.Printf("favorite-anime get list: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update favorite")
		return
	}

	// Get internal anime ID.
	var internalID int
	err = tx.QueryRow(ctx,
		`SELECT id FROM anime WHERE anilist_id = $1`, *body.AnimeID,
	).Scan(&internalID)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Anime not found")
		return
	}
	if err != nil {
		log.Printf("favorite-anime lookup anime: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update favorite")
		return
	}

	if *body.Favorite {
		_, err = tx.Exec(ctx,
			`INSERT INTO user_list_items (list_id, anime_id) VALUES ($1, $2) ON CONFLICT (list_id, anime_id) DO NOTHING`,
			favListID, internalID)
	} else {
		_, err = tx.Exec(ctx,
			`DELETE FROM user_list_items WHERE list_id = $1 AND anime_id = $2`,
			favListID, internalID)
	}
	if err != nil {
		log.Printf("favorite-anime toggle: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update favorite")
		return
	}

	// Touch updated_at for staleness detection.
	_, _ = tx.Exec(ctx,
		`UPDATE user_lists SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, favListID)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("favorite-anime commit: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to update favorite")
		return
	}

	msg := "Added to favorites"
	if !*body.Favorite {
		msg = "Removed from favorites"
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": msg})
}

// ---------- GET /api/user/taste-profile ----------

func (h *Handler) GetTasteProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Get taste profile.
	var (
		tasteSummary     *string
		totalFavorites   int
		preferredGenres  []string
		preferredTags    []string
		preferredEra     *string
		hiddenPatterns   json.RawMessage
		genreVector      json.RawMessage
		tagVector        json.RawMessage
		lastComputed     *time.Time
		staffIDs         []int
		studioIDs        []int
	)

	err := h.pg.QueryRow(ctx,
		`SELECT taste_summary, total_favorites, preferred_genre_names, preferred_tag_names,
				preferred_era, hidden_patterns, genre_vector, tag_vector,
				last_computed, preferred_staff_ids, preferred_studio_ids
		 FROM user_taste_profiles WHERE user_id = $1`,
		userID,
	).Scan(&tasteSummary, &totalFavorites, &preferredGenres, &preferredTags,
		&preferredEra, &hiddenPatterns, &genreVector, &tagVector,
		&lastComputed, &staffIDs, &studioIDs)

	if err == pgx.ErrNoRows {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"exists":  false,
			"message": "No taste profile found. Rate some anime first!",
		})
		return
	}
	if err != nil {
		log.Printf("get taste profile: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch taste profile")
		return
	}

	// Get staff details.
	var staffDetails []map[string]any
	if len(staffIDs) > 0 {
		staffRows, err := h.pg.Query(ctx,
			`SELECT id, staff_id as anilist_id, name_en, image_medium, primary_occupations
			 FROM staff WHERE id = ANY($1) LIMIT 10`,
			staffIDs)
		if err == nil {
			defer staffRows.Close()
			for staffRows.Next() {
				var id, anilistID int
				var nameEn *string
				var imageMed *string
				var occupations []string
				if err := staffRows.Scan(&id, &anilistID, &nameEn, &imageMed, &occupations); err == nil {
					staffDetails = append(staffDetails, map[string]any{
						"id":                   id,
						"anilist_id":           anilistID,
						"name_en":              nameEn,
						"image_medium":         imageMed,
						"primary_occupations":  occupations,
					})
				}
			}
		}
	}

	// Get studio details.
	var studioDetails []map[string]any
	if len(studioIDs) > 0 {
		studioRows, err := h.pg.Query(ctx,
			`SELECT id, name FROM studio WHERE id = ANY($1) LIMIT 10`,
			studioIDs)
		if err == nil {
			defer studioRows.Close()
			for studioRows.Next() {
				var id int
				var name string
				if err := studioRows.Scan(&id, &name); err == nil {
					studioDetails = append(studioDetails, map[string]any{
						"id":   id,
						"name": name,
					})
				}
			}
		}
	}

	// Get favorites.
	favRows, err := h.pg.Query(ctx,
		`SELECT uli.added_at as created_at, a.anilist_id, a.title, a.cover_image_medium, a.average_score
		 FROM user_list_items uli
		 JOIN user_lists ul ON uli.list_id = ul.id
		 JOIN anime a ON uli.anime_id = a.id
		 WHERE ul.user_id = $1 AND ul.list_type = 'favorites'
		 ORDER BY uli.added_at DESC`,
		userID)

	var favorites []map[string]any
	if err == nil {
		defer favRows.Close()
		for favRows.Next() {
			var createdAt time.Time
			var anilistID int
			var title string
			var coverMed *string
			var avgScore *int
			if err := favRows.Scan(&createdAt, &anilistID, &title, &coverMed, &avgScore); err == nil {
				favorites = append(favorites, map[string]any{
					"created_at":        createdAt,
					"anilist_id":        anilistID,
					"title":             title,
					"cover_image_medium": coverMed,
					"average_score":     avgScore,
				})
			}
		}
	}

	if staffDetails == nil {
		staffDetails = []map[string]any{}
	}
	if studioDetails == nil {
		studioDetails = []map[string]any{}
	}
	if favorites == nil {
		favorites = []map[string]any{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"exists": true,
		"profile": map[string]any{
			"tasteSummary":   tasteSummary,
			"totalFavorites": totalFavorites,
			"preferredGenres": preferredGenres,
			"preferredTags":   preferredTags,
			"preferredEra":    preferredEra,
			"hiddenPatterns":  hiddenPatterns,
			"topStaff":        staffDetails,
			"topStudios":      studioDetails,
			"genreVector":     genreVector,
			"tagVector":       tagVector,
			"lastComputed":    lastComputed,
		},
		"favorites": favorites,
	})
}

// ---------- POST /api/user/compute-taste-profile ----------

type hiddenPattern struct {
	Pattern    string   `json:"pattern"`
	Confidence int      `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Insight    string   `json:"insight"`
	Surprise   string   `json:"surprise"`
}

func (h *Handler) ComputeTasteProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Rate limit: 5 per hour per user.
	if !h.limiter.Allow("compute-taste:"+userID, 5, time.Hour) {
		httputil.Error(w, http.StatusTooManyRequests, "Too many requests. Please try again later.")
		return
	}

	// Fetch favorites.
	rows, err := h.pg.Query(ctx,
		`SELECT a.id, a.anilist_id, a.season_year, a.format, a.genre_names, a.tag_names, a.studio_names, a.average_score, uli.added_at
		 FROM user_list_items uli
		 JOIN user_lists ul ON uli.list_id = ul.id
		 JOIN anime a ON uli.anime_id = a.id
		 WHERE ul.user_id = $1 AND ul.list_type = 'favorites'
		 ORDER BY uli.added_at DESC`,
		userID)
	if err != nil {
		log.Printf("compute taste: fetch favorites: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute taste profile")
		return
	}
	defer rows.Close()

	type favorite struct {
		animeID    int
		seasonYear *int
		format     *string
		genres     []string
		tags       []string
	}

	var favorites []favorite
	var animeIDs []int

	for rows.Next() {
		var f favorite
		var anilistID int
		var studioNames []string
		var avgScore *float64
		var addedAt time.Time
		if err := rows.Scan(&f.animeID, &anilistID, &f.seasonYear, &f.format,
			&f.genres, &f.tags, &studioNames, &avgScore, &addedAt); err != nil {
			log.Printf("compute taste: scan: %v", err)
			continue
		}
		favorites = append(favorites, f)
		animeIDs = append(animeIDs, f.animeID)
	}

	if len(favorites) < 3 {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"error":          "Need at least 3 favorites to compute taste profile",
			"favoritesCount": len(favorites),
		})
		return
	}

	totalFavorites := len(favorites)

	// Count genres, tags, eras.
	genreCount := map[string]int{}
	tagCount := map[string]int{}
	eraCount := map[string]int{}

	for _, f := range favorites {
		for _, g := range f.genres {
			genreCount[g]++
		}
		for _, t := range f.tags {
			tagCount[t]++
		}
		if f.seasonYear != nil {
			decade := (*f.seasonYear / 10) * 10
			era := fmt.Sprintf("%ds", decade)
			eraCount[era]++
		}
	}

	topGenres := topN(genreCount, 5)
	topTags := topN(tagCount, 10)

	preferredEra := "2010s"
	if e := topN(eraCount, 1); len(e) > 0 {
		preferredEra = e[0]
	}

	// Fetch staff from favorited anime.
	type staffRow struct {
		staffID    int
		nameEn     string
		appearances int
		roles      []string
	}

	staffRows, err := h.pg.Query(ctx,
		`SELECT s.id, s.name_en, COUNT(DISTINCT ast.anime_id)::int, array_agg(DISTINCT r)
		 FROM anime_staff ast
		 CROSS JOIN LATERAL unnest(ast.role) as r
		 JOIN staff s ON ast.staff_id = s.id
		 WHERE ast.anime_id = ANY($1)
		 GROUP BY s.id, s.name_en
		 HAVING COUNT(DISTINCT ast.anime_id) >= 2
		 ORDER BY COUNT(DISTINCT ast.anime_id) DESC
		 LIMIT 20`,
		animeIDs)
	if err != nil {
		log.Printf("compute taste: fetch staff: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute taste profile")
		return
	}
	defer staffRows.Close()

	var staffResults []staffRow
	var preferredStaffIDs []int
	for staffRows.Next() {
		var s staffRow
		if err := staffRows.Scan(&s.staffID, &s.nameEn, &s.appearances, &s.roles); err != nil {
			log.Printf("compute taste: scan staff: %v", err)
			continue
		}
		staffResults = append(staffResults, s)
		preferredStaffIDs = append(preferredStaffIDs, s.staffID)
	}

	// Fetch studios.
	type studioRow struct {
		studioID    int
		name        string
		appearances int
	}

	studioDBRows, err := h.pg.Query(ctx,
		`SELECT st.id, st.name, COUNT(*)::int
		 FROM anime_studio ast
		 JOIN studio st ON ast.studio_id = st.id
		 WHERE ast.anime_id = ANY($1)
		 GROUP BY st.id, st.name
		 HAVING COUNT(*) >= 2
		 ORDER BY COUNT(*) DESC
		 LIMIT 10`,
		animeIDs)
	if err != nil {
		log.Printf("compute taste: fetch studios: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute taste profile")
		return
	}
	defer studioDBRows.Close()

	var studioResults []studioRow
	var preferredStudioIDs []int
	for studioDBRows.Next() {
		var s studioRow
		if err := studioDBRows.Scan(&s.studioID, &s.name, &s.appearances); err != nil {
			continue
		}
		studioResults = append(studioResults, s)
		preferredStudioIDs = append(preferredStudioIDs, s.studioID)
	}

	// Build normalized vectors.
	genreVector := normalizeMap(genreCount)
	tagVector := normalizeMap(tagCount)

	staffVector := map[string]float64{}
	for _, s := range staffResults {
		staffVector[strconv.Itoa(s.staffID)] = float64(s.appearances) / float64(totalFavorites)
	}

	// Detect hidden patterns.
	var patterns []hiddenPattern

	// Staff affinity.
	for _, s := range staffResults {
		if s.appearances < 3 {
			continue
		}
		confidence := int(math.Min(float64(s.appearances)/float64(totalFavorites)*100, 100))
		if confidence >= 40 {
			roles := strings.Join(s.roles, ", ")
			surprise := "low"
			if confidence >= 70 {
				surprise = "high"
			} else if confidence >= 50 {
				surprise = "medium"
			}
			patterns = append(patterns, hiddenPattern{
				Pattern:    fmt.Sprintf("You frequently enjoy %s's work", s.nameEn),
				Confidence: confidence,
				Evidence: []string{
					fmt.Sprintf("%d of your %d favorites feature their work", s.appearances, totalFavorites),
					fmt.Sprintf("Primary roles: %s", roles),
				},
				Insight:  fmt.Sprintf("%d of your favorites feature %s. Their work appears frequently in your favorites.", s.appearances, s.nameEn),
				Surprise: surprise,
			})
		}
	}

	// Genre clustering.
	if len(genreCount) > 0 {
		domGenre, domCount := topEntry(genreCount)
		if domCount >= totalFavorites*4/10 {
			pct := domCount * 100 / totalFavorites
			patterns = append(patterns, hiddenPattern{
				Pattern:    fmt.Sprintf("Strong %s preference", domGenre),
				Confidence: pct,
				Evidence: []string{
					fmt.Sprintf("%d of %d favorites are %s", domCount, totalFavorites, domGenre),
					fmt.Sprintf("%d%% of your favorites share this genre", pct),
				},
				Insight:  fmt.Sprintf("%s appears most frequently in your favorites.", domGenre),
				Surprise: "low",
			})
		}
	}

	// Studio loyalty.
	if len(studioResults) > 0 && studioResults[0].appearances >= 3 {
		s := studioResults[0]
		pct := s.appearances * 100 / totalFavorites
		patterns = append(patterns, hiddenPattern{
			Pattern:    fmt.Sprintf("%s loyalty", s.name),
			Confidence: pct,
			Evidence: []string{
				fmt.Sprintf("%d of your favorites are from this studio", s.appearances),
			},
			Insight:  fmt.Sprintf("You've favorited %d %s productions.", s.appearances, s.name),
			Surprise: "medium",
		})
	}

	// Sort patterns by confidence descending.
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Confidence > patterns[j].Confidence
	})

	// Taste summary.
	topGenre := "Varied"
	if len(topGenres) > 0 {
		topGenre = topGenres[0]
	}
	tasteSummary := topGenre + " Enthusiast"
	if len(topGenres) > 1 {
		tasteSummary = topGenre + " & " + topGenres[1] + " Enthusiast"
	}

	// Save to database.
	tx, err := h.pg.Begin(ctx)
	if err != nil {
		log.Printf("compute taste: begin tx: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute taste profile")
		return
	}
	defer tx.Rollback(ctx)

	genreVectorJSON, _ := json.Marshal(genreVector)
	tagVectorJSON, _ := json.Marshal(tagVector)
	staffVectorJSON, _ := json.Marshal(staffVector)
	patternsJSON, _ := json.Marshal(patterns)

	_, err = tx.Exec(ctx,
		`INSERT INTO user_taste_profiles (
			user_id, list_id, preferred_staff_ids, preferred_studio_ids,
			preferred_genre_names, preferred_tag_names, preferred_era,
			genre_vector, tag_vector, staff_vector,
			total_favorites, taste_summary, hidden_patterns,
			last_computed
		) VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, COALESCE(list_id, 0)) DO UPDATE SET
			preferred_staff_ids = $2,
			preferred_studio_ids = $3,
			preferred_genre_names = $4,
			preferred_tag_names = $5,
			preferred_era = $6,
			genre_vector = $7,
			tag_vector = $8,
			staff_vector = $9,
			total_favorites = $10,
			taste_summary = $11,
			hidden_patterns = $12,
			last_computed = CURRENT_TIMESTAMP`,
		userID,
		preferredStaffIDs, preferredStudioIDs,
		topGenres, topTags, preferredEra,
		string(genreVectorJSON), string(tagVectorJSON), string(staffVectorJSON),
		totalFavorites, tasteSummary, string(patternsJSON),
	)
	if err != nil {
		log.Printf("compute taste: upsert profile: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute taste profile")
		return
	}

	// Invalidate cached recommendations.
	_, _ = tx.Exec(ctx,
		`DELETE FROM user_anime_predictions WHERE user_id = $1 AND list_id IS NULL`, userID)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("compute taste: commit: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute taste profile")
		return
	}

	// Build response.
	topStaffResp := make([]map[string]any, 0)
	limit := 5
	if limit > len(staffResults) {
		limit = len(staffResults)
	}
	for _, s := range staffResults[:limit] {
		topStaffResp = append(topStaffResp, map[string]any{
			"name":        s.nameEn,
			"appearances": s.appearances,
			"roles":       s.roles,
		})
	}

	topStudiosResp := make([]map[string]any, 0)
	limit = 3
	if limit > len(studioResults) {
		limit = len(studioResults)
	}
	for _, s := range studioResults[:limit] {
		topStudiosResp = append(topStudiosResp, map[string]any{
			"id":          s.studioID,
			"name":        s.name,
			"appearances": s.appearances,
		})
	}

	tagsResp := topTags
	if len(tagsResp) > 5 {
		tagsResp = tagsResp[:5]
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"profile": map[string]any{
			"tasteSummary":   tasteSummary,
			"totalFavorites": totalFavorites,
			"preferredGenres": topGenres,
			"preferredTags":   tagsResp,
			"preferredEra":    preferredEra,
			"hiddenPatterns":  patterns,
			"topStaff":        topStaffResp,
			"topStudios":      topStudiosResp,
		},
	})
}

// ---------- GET /api/user/recommendations ----------

func (h *Handler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	limit := httputil.QueryInt(r, "limit", 24)
	if limit > 10000 {
		limit = 10000
	}
	offset := httputil.QueryInt(r, "offset", 0)
	minScore := httputil.QueryIntPtr(r, "minScore")

	// Check staleness.
	var computedAt, favoritesUpdatedAt *time.Time
	var favoritesCount int

	err := h.pg.QueryRow(ctx,
		`SELECT
			(SELECT computed_at FROM user_anime_predictions WHERE user_id = $1 AND list_id IS NULL ORDER BY computed_at DESC LIMIT 1),
			(SELECT updated_at FROM user_lists WHERE user_id = $1::text AND list_type = 'favorites' LIMIT 1),
			(SELECT COUNT(*) FROM user_list_items uli JOIN user_lists ul ON uli.list_id = ul.id WHERE ul.user_id = $1::text AND ul.list_type = 'favorites')::integer`,
		userID,
	).Scan(&computedAt, &favoritesUpdatedAt, &favoritesCount)
	if err != nil {
		log.Printf("get recommendations: staleness check: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch recommendations")
		return
	}

	needsComputation := computedAt == nil || (favoritesUpdatedAt != nil && favoritesUpdatedAt.After(*computedAt))

	if needsComputation {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success":          false,
			"needsComputation": true,
			"favoritesCount":   favoritesCount,
			"message":          "Recommendations need to be computed. Please click \"Generate Recommendations\" to create them.",
			"data":             []any{},
			"total":            0,
		})
		return
	}

	// Fetch recommendations.
	params := []any{userID, limit, offset}
	scoreFilter := ""
	if minScore != nil {
		scoreFilter = "AND uap.match_score >= $4"
		params = append(params, *minScore)
	}

	rows, err := h.pg.Query(ctx, fmt.Sprintf(`
		SELECT
			a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium,
			a.episodes, a.season, a.season_year,
			a.average_score::integer, a.popularity, a.format, a.status,
			a.description, uap.match_score, uap.reasons
		FROM user_anime_predictions uap
		JOIN anime a ON uap.anime_id = a.id
		WHERE uap.user_id = $1 AND uap.list_id IS NULL %s
		ORDER BY uap.match_score DESC
		LIMIT $2 OFFSET $3`, scoreFilter), params...)
	if err != nil {
		log.Printf("get recommendations: query: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch recommendations")
		return
	}
	defer rows.Close()

	recs := make([]map[string]any, 0)
	for rows.Next() {
		var (
			anilistID                                                int
			title                                                    string
			coverImage, coverImageXL, coverImageLg, coverImageMd     *string
			episodes                                                 *int
			season                                                   *string
			seasonYear                                               *int
			averageScore, popularity                                 *int
			format, status, description                              *string
			matchScore                                               int
			reasons                                                  json.RawMessage
		)
		if err := rows.Scan(
			&anilistID, &title, &coverImage, &coverImageXL,
			&coverImageLg, &coverImageMd,
			&episodes, &season, &seasonYear,
			&averageScore, &popularity, &format, &status,
			&description, &matchScore, &reasons,
		); err != nil {
			log.Printf("scan recommendation: %v", err)
			continue
		}

		// Parse reasons from JSONB.
		var matchReasons []string
		if reasons != nil {
			_ = json.Unmarshal(reasons, &matchReasons)
		}
		if matchReasons == nil {
			matchReasons = []string{}
		}

		recs = append(recs, map[string]any{
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
			"description":           description,
			"matchScore":            matchScore,
			"matchReasons":          matchReasons,
		})
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"favoritesCount": favoritesCount,
		"data":           recs,
		"total":          len(recs),
	})
}

// ---------- POST /api/user/compute-recommendations ----------

func (h *Handler) ComputeRecommendations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Fetch taste profile.
	var genreVectorJSON, tagVectorJSON, staffVectorJSON json.RawMessage
	var preferredStudioIDs []int

	err := h.pg.QueryRow(ctx,
		`SELECT genre_vector, tag_vector, staff_vector, preferred_studio_ids
		 FROM user_taste_profiles WHERE user_id = $1`,
		userID,
	).Scan(&genreVectorJSON, &tagVectorJSON, &staffVectorJSON, &preferredStudioIDs)

	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "User taste profile not found. Please favorite at least 3 anime first.")
		return
	}
	if err != nil {
		log.Printf("compute recs: fetch profile: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute recommendations")
		return
	}

	userGenreVector := parseFloatMap(genreVectorJSON)
	userTagVector := parseFloatMap(tagVectorJSON)
	userStaffVector := parseFloatMap(staffVectorJSON)

	// Fetch preferred studio names for matching.
	var preferredStudioNames []string
	if len(preferredStudioIDs) > 0 {
		sRows, err := h.pg.Query(ctx,
			`SELECT name FROM studio WHERE id = ANY($1)`, preferredStudioIDs)
		if err == nil {
			defer sRows.Close()
			for sRows.Next() {
				var name string
				if sRows.Scan(&name) == nil {
					preferredStudioNames = append(preferredStudioNames, name)
				}
			}
		}
	}

	// Fetch candidate anime (unwatched, score > 60).
	animeRows, err := h.pg.Query(ctx,
		`SELECT a.id, a.anilist_id, a.average_score, a.genre_names, a.tag_names
		 FROM anime a
		 WHERE a.average_score > 60
			AND a.id NOT IN (
				SELECT uli.anime_id FROM user_list_items uli
				JOIN user_lists ul ON uli.list_id = ul.id
				WHERE ul.user_id = $1 AND ul.list_type = 'favorites'
			)
		 ORDER BY a.average_score DESC
		 LIMIT 1000`,
		userID)
	if err != nil {
		log.Printf("compute recs: fetch candidates: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute recommendations")
		return
	}
	defer animeRows.Close()

	type candidateAnime struct {
		id        int
		anilistID int
		genres    []string
		tags      []string
	}

	var candidates []candidateAnime
	var candidateIDs []int

	for animeRows.Next() {
		var c candidateAnime
		var avgScore *float64
		if err := animeRows.Scan(&c.id, &c.anilistID, &avgScore, &c.genres, &c.tags); err != nil {
			continue
		}
		candidates = append(candidates, c)
		candidateIDs = append(candidateIDs, c.id)
	}

	if len(candidates) == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "Computed 0 recommendations",
			"count":   0,
		})
		return
	}

	// Bulk fetch staff for candidates.
	type staffBulkRow struct {
		animeID int
		staffID int
		nameEn  string
	}
	staffByAnime := map[int][]staffBulkRow{}

	sbRows, err := h.pg.Query(ctx,
		`SELECT ast.anime_id, s.id, s.name_en
		 FROM anime_staff ast
		 JOIN staff s ON ast.staff_id = s.id
		 WHERE ast.anime_id = ANY($1)`,
		candidateIDs)
	if err == nil {
		defer sbRows.Close()
		for sbRows.Next() {
			var sb staffBulkRow
			if sbRows.Scan(&sb.animeID, &sb.staffID, &sb.nameEn) == nil {
				staffByAnime[sb.animeID] = append(staffByAnime[sb.animeID], sb)
			}
		}
	}

	// Bulk fetch studios for candidates.
	type studioBulkRow struct {
		animeID  int
		studioID int
		name     string
	}
	studiosByAnime := map[int][]studioBulkRow{}

	stuRows, err := h.pg.Query(ctx,
		`SELECT ast.anime_id, st.id, st.name
		 FROM anime_studio ast
		 JOIN studio st ON ast.studio_id = st.id
		 WHERE ast.anime_id = ANY($1)`,
		candidateIDs)
	if err == nil {
		defer stuRows.Close()
		for stuRows.Next() {
			var sb studioBulkRow
			if stuRows.Scan(&sb.animeID, &sb.studioID, &sb.name) == nil {
				studiosByAnime[sb.animeID] = append(studiosByAnime[sb.animeID], sb)
			}
		}
	}

	// Compute scores.
	userStaffKeys := make(map[string]bool, len(userStaffVector))
	for k := range userStaffVector {
		userStaffKeys[k] = true
	}
	userStaffCount := len(userStaffVector)

	type prediction struct {
		animeID    int
		matchScore int
		genreScore float64
		tagScore   float64
		staffScore float64
		studioScore float64
		reasons    []string
	}

	predictions := make([]prediction, 0, len(candidates))

	for _, c := range candidates {
		// Build anime vectors.
		animeGenreVec := map[string]float64{}
		for _, g := range c.genres {
			animeGenreVec[g] = 1
		}
		animeTagVec := map[string]float64{}
		for _, t := range c.tags {
			animeTagVec[t] = 1
		}

		genreScore := cosineSimilarity(userGenreVector, animeGenreVec)
		tagScore := cosineSimilarity(userTagVector, animeTagVec)

		// Staff score.
		animeStaff := staffByAnime[c.id]
		var staffOverlap []staffBulkRow
		for _, s := range animeStaff {
			if userStaffKeys[strconv.Itoa(s.staffID)] {
				staffOverlap = append(staffOverlap, s)
			}
		}
		var staffScore float64
		if userStaffCount > 0 {
			staffScore = float64(len(staffOverlap)) / float64(userStaffCount)
		}

		// Studio score.
		animeStudios := studiosByAnime[c.id]
		var studioScore float64
		var matchedStudio string
		for _, s := range animeStudios {
			for _, ps := range preferredStudioNames {
				if s.name == ps {
					studioScore = 1.0
					matchedStudio = s.name
					break
				}
			}
			if studioScore > 0 {
				break
			}
		}

		matchScore := int(math.Round(
			genreScore*0.40*100 +
				tagScore*0.30*100 +
				staffScore*0.20*100 +
				studioScore*0.10*100,
		))

		// Generate reasons.
		var reasons []string

		// Top matching genres.
		matchedGenres := 0
		for _, g := range c.genres {
			if matchedGenres >= 3 {
				break
			}
			if v, ok := userGenreVector[g]; ok && v > 0.3 {
				reasons = append(reasons, g+" genre match")
				matchedGenres++
			}
		}

		// Top matching tags.
		matchedTags := 0
		for _, t := range c.tags {
			if matchedTags >= 2 {
				break
			}
			if v, ok := userTagVector[t]; ok && v > 0.3 {
				reasons = append(reasons, t+" tag match")
				matchedTags++
			}
		}

		// Staff matches.
		staffLimit := 2
		if staffLimit > len(staffOverlap) {
			staffLimit = len(staffOverlap)
		}
		for _, s := range staffOverlap[:staffLimit] {
			reasons = append(reasons, s.nameEn+" worked on this")
		}

		// Studio match.
		if matchedStudio != "" {
			reasons = append(reasons, "By "+matchedStudio)
		}

		predictions = append(predictions, prediction{
			animeID:    c.id,
			matchScore: matchScore,
			genreScore: genreScore,
			tagScore:   tagScore,
			staffScore: staffScore,
			studioScore: studioScore,
			reasons:    reasons,
		})
	}

	// Batch insert.
	tx, err := h.pg.Begin(ctx)
	if err != nil {
		log.Printf("compute recs: begin tx: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute recommendations")
		return
	}
	defer tx.Rollback(ctx)

	_, _ = tx.Exec(ctx,
		`DELETE FROM user_anime_predictions WHERE user_id = $1 AND list_id IS NULL`, userID)

	for _, p := range predictions {
		reasonsJSON, _ := json.Marshal(p.reasons)
		_, err = tx.Exec(ctx,
			`INSERT INTO user_anime_predictions
				(user_id, anime_id, match_score, genre_score, tag_score, staff_score, studio_score, reasons)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			userID, p.animeID, p.matchScore, p.genreScore, p.tagScore, p.staffScore, p.studioScore, string(reasonsJSON),
		)
		if err != nil {
			log.Printf("compute recs: insert prediction: %v", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("compute recs: commit: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to compute recommendations")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Computed %d recommendations", len(predictions)),
		"count":   len(predictions),
	})
}

// ======================== helpers ========================

func topN(m map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	out := make([]string, 0, n)
	for i, p := range pairs {
		if i >= n {
			break
		}
		out = append(out, p.k)
	}
	return out
}

func topEntry(m map[string]int) (string, int) {
	var maxK string
	var maxV int
	for k, v := range m {
		if v > maxV {
			maxK = k
			maxV = v
		}
	}
	return maxK, maxV
}

func normalizeMap(m map[string]int) map[string]float64 {
	maxVal := 1
	for _, v := range m {
		if v > maxVal {
			maxVal = v
		}
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = float64(v) / float64(maxVal)
	}
	return out
}

func cosineSimilarity(v1, v2 map[string]float64) float64 {
	var dot, mag1, mag2 float64

	// Collect all keys.
	keys := make(map[string]bool, len(v1)+len(v2))
	for k := range v1 {
		keys[k] = true
	}
	for k := range v2 {
		keys[k] = true
	}

	for k := range keys {
		a := v1[k]
		b := v2[k]
		dot += a * b
		mag1 += a * a
		mag2 += b * b
	}

	if mag1 == 0 || mag2 == 0 {
		return 0
	}
	return dot / (math.Sqrt(mag1) * math.Sqrt(mag2))
}

func parseFloatMap(raw json.RawMessage) map[string]float64 {
	m := map[string]float64{}
	if raw != nil {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// InvalidateListCache deletes the computation cache entry for a given set of anime IDs.
func InvalidateListCache(pg *pgxpool.Pool, ctx context.Context, animeIDs []int) {
	if len(animeIDs) == 0 {
		return
	}
	sorted := make([]int, len(animeIDs))
	copy(sorted, animeIDs)
	sort.Ints(sorted)

	parts := make([]string, len(sorted))
	for i, id := range sorted {
		parts[i] = strconv.Itoa(id)
	}
	hash := md5.Sum([]byte(strings.Join(parts, ",")))
	cacheKey := hex.EncodeToString(hash[:])

	_, err := pg.Exec(ctx, `DELETE FROM list_computation_cache WHERE cache_key = $1`, cacheKey)
	if err != nil {
		log.Printf("invalidate list cache: %v", err)
	}
}
