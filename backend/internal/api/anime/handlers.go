package anime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
)

type Handler struct {
	pg *pgxpool.Pool

	// Caches
	randomCacheMu        sync.RWMutex
	randomCache          map[string]*randomCacheEntry
	lastRandomRefresh    time.Time
	filterMetaCacheMu    sync.RWMutex
	filterMetaCache      []byte // pre-encoded JSON
	filterMetaCacheTime  time.Time
	genresTagsCacheMu    sync.RWMutex
	genresTagsCache      map[string][]byte // "safe" or "adult" -> pre-encoded JSON
	genresTagsCacheTime  map[string]time.Time
}

type randomCacheEntry struct {
	data      []byte // pre-encoded JSON
	timestamp time.Time
	key       string
}

func NewHandler(pg *pgxpool.Pool) *Handler {
	return &Handler{
		pg:                pg,
		randomCache:       make(map[string]*randomCacheEntry),
		genresTagsCache:   make(map[string][]byte),
		genresTagsCacheTime: make(map[string]time.Time),
	}
}

// GetByID handles GET /api/anime/{id} — full anime detail.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	anilistID, err := strconv.Atoi(idStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid anime ID")
		return
	}

	ctx := r.Context()

	// Main anime query.
	var row animeRow
	err = h.pg.QueryRow(ctx, `
		SELECT
			a.title, a.title_ja, a.title_native, a.title_romaji, a.title_english,
			a.anilist_id, a.type, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium, a.cover_image_color,
			a.banner_image, a.episodes, a.duration, a.season_year, a.season,
			a.format, a.status, a.average_score::integer, a.mean_score::integer,
			a.description, a.source, a.country_of_origin, a.is_adult, a.synonyms,
			a.updated_at, a.keyframe_link, a.mal_id, a.sakugabooru_tag,
			a.wikipedia_en, a.wikipedia_ja, a.wikipedia_production_html,
			a.wikidata_qid, a.livechart_id, a.tvdb_id, a.tmdb_movie_id, a.tmdb_tv_id,
			a.trailer_id, a.trailer_site, a.trailer_thumbnail,
			f.id, f.title
		FROM anime a
		LEFT JOIN franchise f ON a.franchise_id = f.id
		WHERE a.anilist_id = $1`, anilistID).Scan(
		&row.Title, &row.TitleJA, &row.TitleNative, &row.TitleRomaji, &row.TitleEnglish,
		&row.AnilistID, &row.Type, &row.CoverImage, &row.CoverImageExtraLarge,
		&row.CoverImageLarge, &row.CoverImageMedium, &row.CoverImageColor,
		&row.BannerImage, &row.Episodes, &row.Duration, &row.SeasonYear, &row.Season,
		&row.Format, &row.Status, &row.AverageScore, &row.MeanScore,
		&row.Description, &row.Source, &row.CountryOfOrigin, &row.IsAdult, &row.Synonyms,
		&row.UpdatedAt, &row.KeyframeLink, &row.MalID, &row.SakugabooruTag,
		&row.WikipediaEn, &row.WikipediaJa, &row.WikipediaProductionHTML,
		&row.WikidataQID, &row.LivechartID, &row.TvdbID, &row.TmdbMovieID, &row.TmdbTvID,
		&row.TrailerID, &row.TrailerSite, &row.TrailerThumbnail,
		&row.FranchiseID, &row.FranchiseTitle,
	)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Anime not found")
		return
	}
	if err != nil {
		log.Printf("anime get error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch anime details")
		return
	}

	// Parallel queries for related data.
	type studiosResult struct {
		data []map[string]any
		err  error
	}
	type genresResult struct {
		data []string
		err  error
	}
	type tagsResult struct {
		data []map[string]any
		err  error
	}
	type staffResult struct {
		data []map[string]any
		err  error
	}
	type sakuResult struct {
		data []map[string]any
		err  error
	}
	type similarOpsResult struct {
		data    []map[string]any
		ownOps  []map[string]any
		err     error
	}

	studiosCh := make(chan studiosResult, 1)
	genresCh := make(chan genresResult, 1)
	tagsCh := make(chan tagsResult, 1)
	staffCh := make(chan staffResult, 1)
	sakuCh := make(chan sakuResult, 1)
	similarOpsCh := make(chan similarOpsResult, 1)

	go func() {
		rows, err := h.pg.Query(ctx, `
			SELECT s.name FROM anime_studio ast
			JOIN studio s ON ast.studio_id = s.id
			WHERE ast.anime_id = (SELECT id FROM anime WHERE anilist_id = $1) AND ast.is_main = true`, anilistID)
		if err != nil {
			studiosCh <- studiosResult{err: err}
			return
		}
		defer rows.Close()
		var studios []map[string]any
		for rows.Next() {
			var name string
			rows.Scan(&name)
			studios = append(studios, map[string]any{"name": name})
		}
		studiosCh <- studiosResult{data: studios}
	}()

	go func() {
		rows, err := h.pg.Query(ctx, `
			SELECT g.name FROM anime_genre ag
			JOIN genre g ON ag.genre_id = g.id
			WHERE ag.anime_id = (SELECT id FROM anime WHERE anilist_id = $1)`, anilistID)
		if err != nil {
			genresCh <- genresResult{err: err}
			return
		}
		defer rows.Close()
		var genres []string
		for rows.Next() {
			var name string
			rows.Scan(&name)
			genres = append(genres, name)
		}
		genresCh <- genresResult{data: genres}
	}()

	go func() {
		rows, err := h.pg.Query(ctx, `
			SELECT t.name, t.category, at.rank FROM anime_tag at
			JOIN tag t ON at.tag_id = t.id
			WHERE at.anime_id = (SELECT id FROM anime WHERE anilist_id = $1)
			ORDER BY at.rank ASC NULLS LAST`, anilistID)
		if err != nil {
			tagsCh <- tagsResult{err: err}
			return
		}
		defer rows.Close()
		var tags []map[string]any
		for rows.Next() {
			var name string
			var category *string
			var rank *int
			rows.Scan(&name, &category, &rank)
			tags = append(tags, map[string]any{"name": name, "category": category, "rank": rank})
		}
		tagsCh <- tagsResult{data: tags}
	}()

	go func() {
		rows, err := h.pg.Query(ctx, `
			SELECT s.staff_id, s.name_en, s.name_ja, COALESCE(s.image_large, s.image_medium) as image,
				asf.role, asf.weight, s.primary_occupations
			FROM anime_staff asf
			JOIN staff s ON asf.staff_id = s.id
			WHERE asf.anime_id = (SELECT id FROM anime WHERE anilist_id = $1)
			ORDER BY asf.weight DESC NULLS LAST`, anilistID)
		if err != nil {
			staffCh <- staffResult{err: err}
			return
		}
		defer rows.Close()
		var staff []map[string]any
		for rows.Next() {
			var staffID int
			var nameEn, nameJa, image *string
			var role *[]string
			var weight *float64
			var occupations *[]string
			if err := rows.Scan(&staffID, &nameEn, &nameJa, &image, &role, &weight, &occupations); err != nil {
				log.Printf("anime staff scan error (staffID so far: %d): %v", staffID, err)
				continue
			}
			var occ []string
			if occupations != nil {
				occ = *occupations
			}
			staff = append(staff, map[string]any{
				"staff": map[string]any{
					"staff_id":            staffID,
					"name_en":             nameEn,
					"name_ja":             nameJa,
					"image":               image,
					"primaryOccupations":  occ,
				},
				"role":   role,
				"weight": weight,
			})
		}
		staffCh <- staffResult{data: staff}
	}()

	go func() {
		rows, err := h.pg.Query(ctx, `
			SELECT sp.post_id, sp.file_url, sp.preview_url, sp.source, sp.file_ext, sp.rating
			FROM anime_sakugabooru_post asp
			JOIN sakugabooru_post sp ON asp.post_id = sp.post_id
			WHERE asp.anime_id = (SELECT id FROM anime WHERE anilist_id = $1)
				AND LOWER(REVERSE(SPLIT_PART(REVERSE(sp.file_url), '.', 1))) IN ('mp4', 'webm')
			ORDER BY sp.post_id DESC`, anilistID)
		if err != nil {
			sakuCh <- sakuResult{data: nil}
			return
		}
		defer rows.Close()
		var posts []map[string]any
		for rows.Next() {
			var postID int
			var fileURL string
			var previewURL, source, fileExt, rating *string
			rows.Scan(&postID, &fileURL, &previewURL, &source, &fileExt, &rating)
			posts = append(posts, map[string]any{
				"postId": postID, "fileUrl": fileURL, "previewUrl": previewURL,
				"source": source, "fileExt": fileExt, "rating": rating,
			})
		}
		sakuCh <- sakuResult{data: posts}
	}()

	go func() {
		rows, err := h.pg.Query(ctx, `
			SELECT so.op_number, so.similar_op_number, sim_emb.title_op,
				rec.anilist_id, COALESCE(NULLIF(rec.title_english,''), rec.title_romaji) as title,
				rec.cover_image_large, rec.cover_image_extra_large,
				so.similarity, rec.average_score::integer, rec.format, rec.season_year
			FROM anime_similar_op so
			JOIN anime a ON so.anime_id = a.id
			JOIN anime rec ON so.similar_anime_id = rec.id
			JOIN anime_op_embedding sim_emb ON sim_emb.anime_id = so.similar_anime_id AND sim_emb.op_number = so.similar_op_number
			WHERE a.anilist_id = $1
			ORDER BY so.op_number, so.rank`, anilistID)
		if err != nil {
			similarOpsCh <- similarOpsResult{data: nil}
			return
		}
		defer rows.Close()
		var ops []map[string]any
		for rows.Next() {
			var opNumber, similarOpNumber, recAnilistID int
			var title, similarTitleOP string
			var coverLarge, coverExtraLarge *string
			var similarity float64
			var avgScore *int
			var format, seasonYear *string
			if err := rows.Scan(&opNumber, &similarOpNumber, &similarTitleOP, &recAnilistID, &title,
				&coverLarge, &coverExtraLarge, &similarity, &avgScore, &format, &seasonYear); err != nil {
				continue
			}
			ops = append(ops, map[string]any{
				"opNumber":               opNumber,
				"similarOpNumber":        similarOpNumber,
				"similarTitleOp":         similarTitleOP,
				"anilistId":              recAnilistID,
				"title":                  title,
				"coverImage_large":       coverLarge,
				"coverImage_extraLarge":  coverExtraLarge,
				"similarity":             similarity,
				"averageScore":           avgScore,
				"format":                 format,
				"seasonYear":             seasonYear,
			})
		}
		// Fetch this anime's own OPs
		var ownOps []map[string]any
		ownRows, ownErr := h.pg.Query(ctx, `
			SELECT e.op_number, e.title_op
			FROM anime_op_embedding e
			JOIN anime a ON e.anime_id = a.id
			WHERE a.anilist_id = $1
			ORDER BY e.op_number`, anilistID)
		if ownErr == nil {
			defer ownRows.Close()
			for ownRows.Next() {
				var opNum int
				var titleOP string
				if err := ownRows.Scan(&opNum, &titleOP); err == nil {
					ownOps = append(ownOps, map[string]any{
						"opNumber": opNum,
						"titleOp":  titleOP,
					})
				}
			}
		}
		similarOpsCh <- similarOpsResult{data: ops, ownOps: ownOps}
	}()

	studiosRes := <-studiosCh
	genresRes := <-genresCh
	tagsRes := <-tagsCh
	staffRes := <-staffCh
	sakuRes := <-sakuCh
	similarOpsRes := <-similarOpsCh

	studios := studiosRes.data
	if studios == nil {
		studios = []map[string]any{}
	}
	genres := genresRes.data
	if genres == nil {
		genres = []string{}
	}
	tags := tagsRes.data
	if tags == nil {
		tags = []map[string]any{}
	}
	staff := staffRes.data
	if staff == nil {
		staff = []map[string]any{}
	}
	sakuPosts := sakuRes.data
	if sakuPosts == nil {
		sakuPosts = []map[string]any{}
	}
	similarOps := similarOpsRes.data
	if similarOps == nil {
		similarOps = []map[string]any{}
	}
	animeOps := similarOpsRes.ownOps
	if animeOps == nil {
		animeOps = []map[string]any{}
	}
	synonyms := row.Synonyms
	if synonyms == nil {
		synonyms = []string{}
	}

	var franchise any
	if row.FranchiseID != nil {
		franchise = map[string]any{"id": *row.FranchiseID, "title": row.FranchiseTitle}
	}

	data := map[string]any{
		"title": row.Title, "title_ja": row.TitleJA, "title_native": row.TitleNative,
		"title_romaji": row.TitleRomaji, "title_english": row.TitleEnglish,
		"anilistId": row.AnilistID, "type": row.Type,
		"coverImage": row.CoverImage, "coverImage_extraLarge": row.CoverImageExtraLarge,
		"coverImage_large": row.CoverImageLarge, "coverImage_medium": row.CoverImageMedium,
		"coverImage_color": row.CoverImageColor, "bannerImage": row.BannerImage,
		"episodes": row.Episodes, "duration": row.Duration,
		"seasonYear": row.SeasonYear, "season": row.Season,
		"format": row.Format, "status": row.Status,
		"averageScore": row.AverageScore, "meanScore": row.MeanScore,
		"description": row.Description, "source": row.Source,
		"countryOfOrigin": row.CountryOfOrigin, "isAdult": row.IsAdult,
		"synonyms": synonyms, "updatedAt": row.UpdatedAt,
		"keyframeLink": row.KeyframeLink, "malId": row.MalID,
		"trailer_id": row.TrailerID, "trailer_site": row.TrailerSite, "trailer_thumbnail": row.TrailerThumbnail,
		"studios": studios, "genres": genres, "tags": tags, "staff": staff,
		"sakugabooruTag": row.SakugabooruTag,
		"wikipediaEn": row.WikipediaEn, "wikipediaJa": row.WikipediaJa,
		"wikipediaProductionHtml": row.WikipediaProductionHTML,
		"wikidataQid": row.WikidataQID, "livechartId": row.LivechartID,
		"tvdbId": row.TvdbID, "tmdbMovieId": row.TmdbMovieID, "tmdbTvId": row.TmdbTvID,
		"sakugabooruPosts": sakuPosts,
		"similarOps":      similarOps,
		"animeOps":        animeOps,
		"franchise":        franchise,
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// Popular handles GET /api/anime/popular — popular anime with sorting/filtering.
func (h *Handler) Popular(w http.ResponseWriter, r *http.Request) {
	limit := httputil.QueryInt(r, "limit", 20)
	offset := httputil.QueryInt(r, "offset", 0)
	eras := httputil.QueryStringSlice(r, "eras")
	genres := httputil.QueryCSV(r, "genres")
	tags := httputil.QueryCSV(r, "tags")
	includeAdult := httputil.QueryBool(r, "includeAdult")
	sort := httputil.QueryString(r, "sort", "random")
	sortOrderParam := "DESC"
	if strings.ToUpper(httputil.QueryString(r, "order", "DESC")) == "ASC" {
		sortOrderParam = "ASC"
	}
	format := r.URL.Query().Get("format")
	mediaType := r.URL.Query().Get("type")
	hasRating := httputil.QueryBool(r, "hasRating")
	releasedOnly := httputil.QueryBool(r, "releasedOnly")
	minStaff := httputil.QueryInt(r, "minStaff", 0)

	ctx := r.Context()
	var conditions []string
	var params []any
	paramIdx := 1

	// Type filter.
	if format == "" {
		if mediaType == "manga" {
			conditions = append(conditions, `a.format IN ('MANGA', 'NOVEL', 'ONE_SHOT')`)
		} else if mediaType == "anime" {
			conditions = append(conditions, `a.format IN ('TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC')`)
		}
	}

	// Era filter.
	if len(eras) > 0 {
		var yearConds []string
		for _, era := range eras {
			switch era {
			case "pre-1960":
				yearConds = append(yearConds, "a.season_year < 1960")
			case "1960s-1980s":
				yearConds = append(yearConds, "(a.season_year >= 1960 AND a.season_year < 1990)")
			case "1990s-2000s":
				yearConds = append(yearConds, "(a.season_year >= 1990 AND a.season_year < 2010)")
			case "2010s":
				yearConds = append(yearConds, "(a.season_year >= 2010 AND a.season_year < 2020)")
			case "2020s":
				yearConds = append(yearConds, "a.season_year >= 2020")
			}
		}
		if len(yearConds) > 0 {
			conditions = append(conditions, "("+strings.Join(yearConds, " OR ")+")")
		}
	}

	// Adult filter.
	if !includeAdult {
		conditions = append(conditions, "(a.is_adult IS NULL OR a.is_adult = false)")
		conditions = append(conditions, `NOT EXISTS (
			SELECT 1 FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id
			WHERE ag.anime_id = a.id AND g.name = 'Ecchi')`)
	}

	// Format filter.
	if format != "" {
		conditions = append(conditions, fmt.Sprintf("a.format = $%d", paramIdx))
		params = append(params, format)
		paramIdx++
	}

	// Score filter.
	if sort == "top" || sort == "score" || hasRating {
		conditions = append(conditions, "a.average_score IS NOT NULL")
	}

	// Genre filter (ALL must match).
	if len(genres) > 0 {
		placeholders := make([]string, len(genres))
		for i, g := range genres {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			params = append(params, g)
			paramIdx++
		}
		conditions = append(conditions, fmt.Sprintf(`(
			SELECT COUNT(DISTINCT g.name) FROM anime_genre ag
			JOIN genre g ON ag.genre_id = g.id
			WHERE ag.anime_id = a.id AND g.name IN (%s)) = %d`,
			strings.Join(placeholders, ", "), len(genres)))
	}

	// Tag filter (ALL must match).
	if len(tags) > 0 {
		placeholders := make([]string, len(tags))
		for i, t := range tags {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			params = append(params, t)
			paramIdx++
		}
		conditions = append(conditions, fmt.Sprintf(`(
			SELECT COUNT(DISTINCT t.name) FROM anime_tag at
			JOIN tag t ON at.tag_id = t.id
			WHERE at.anime_id = a.id AND t.name IN (%s)) = %d`,
			strings.Join(placeholders, ", "), len(tags)))
	}

	// Min staff filter.
	if minStaff > 0 {
		conditions = append(conditions, fmt.Sprintf("(SELECT COUNT(DISTINCT staff_id) FROM anime_staff WHERE anime_id = a.id) >= $%d", paramIdx))
		params = append(params, minStaff)
		paramIdx++
	}

	// Released only filter.
	if releasedOnly {
		currentDate := time.Now().Format("2006-01-02")
		conditions = append(conditions, fmt.Sprintf(`(
			(a.status IS NULL OR a.status != 'NOT_YET_RELEASED')
			AND (a.start_date IS NULL OR a.start_date <= '%s'))`, currentDate))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Check random cache.
	if sort == "random" && limit > 1 {
		cacheKey := fmt.Sprintf("%v-%v-%v-%v-%s-%s-%v-%v-%d-%d",
			eras, genres, tags, includeAdult, format, mediaType, hasRating, releasedOnly, limit, offset)
		h.randomCacheMu.RLock()
		entry := h.randomCache[cacheKey]
		h.randomCacheMu.RUnlock()
		if entry != nil && time.Since(entry.timestamp) < time.Minute {
			w.Header().Set("Content-Type", "application/json")
			w.Write(entry.data)
			return
		}
	}

	// Refresh random ranks if needed.
	if sort == "random" {
		h.refreshRandomRanks(ctx)
	}

	// Sort order.
	var orderBy string
	switch sort {
	case "top":
		orderBy = "ORDER BY a.average_score DESC NULLS LAST, a.popularity DESC NULLS LAST"
	case "new":
		orderBy = `ORDER BY a.season_year DESC NULLS LAST,
			CASE a.season WHEN 'FALL' THEN 4 WHEN 'SUMMER' THEN 3 WHEN 'SPRING' THEN 2 WHEN 'WINTER' THEN 1 ELSE 0 END DESC,
			a.popularity DESC NULLS LAST`
	case "newest-id":
		orderBy = "ORDER BY a.anilist_id DESC"
	case "score":
		orderBy = fmt.Sprintf("ORDER BY a.average_score %s NULLS LAST, a.popularity DESC NULLS LAST", sortOrderParam)
	case "year":
		orderBy = fmt.Sprintf(`ORDER BY a.season_year %s NULLS LAST,
			CASE a.season WHEN 'FALL' THEN 4 WHEN 'SUMMER' THEN 3 WHEN 'SPRING' THEN 2 WHEN 'WINTER' THEN 1 ELSE 0 END %s NULLS LAST,
			a.popularity DESC NULLS LAST`, sortOrderParam, sortOrderParam)
	case "title":
		inverted := "ASC"
		if sortOrderParam == "ASC" {
			inverted = "DESC"
		}
		orderBy = fmt.Sprintf("ORDER BY a.title %s", inverted)
	case "trending", "popular":
		orderBy = "ORDER BY a.popularity DESC NULLS LAST"
	case "most-staff":
		orderBy = "ORDER BY staff_count DESC NULLS LAST, a.average_score DESC NULLS LAST"
	default:
		orderBy = "ORDER BY a.random_rank"
	}

	// Random start for single-item requests.
	randomStartCondition := ""
	if sort == "random" && limit <= 1 {
		randomStart := rand.Float64()
		params = append(params, randomStart)
		if len(conditions) > 0 {
			randomStartCondition = fmt.Sprintf(" AND a.random_rank >= $%d", paramIdx)
		} else {
			randomStartCondition = fmt.Sprintf(" WHERE a.random_rank >= $%d", paramIdx)
		}
		paramIdx++
	}

	params = append(params, limit)
	limitParam := fmt.Sprintf("$%d", paramIdx)
	paramIdx++
	params = append(params, offset)
	offsetParam := fmt.Sprintf("$%d", paramIdx)
	paramIdx++

	sql := fmt.Sprintf(`
		SELECT a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium, a.episodes, a.season,
			a.season_year, a.average_score::integer as average_score, a.popularity,
			a.format, a.description,
			(SELECT COUNT(DISTINCT staff_id) FROM anime_staff WHERE anime_id = a.id) as staff_count
		FROM anime a %s%s %s LIMIT %s OFFSET %s`,
		whereClause, randomStartCondition, orderBy, limitParam, offsetParam)

	rows, err := h.pg.Query(ctx, sql, params...)
	if err != nil {
		log.Printf("popular query error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch anime")
		return
	}
	defer rows.Close()

	animeList := h.scanPopularRows(rows, sort)

	// Wraparound for random with no results.
	if len(animeList) == 0 && randomStartCondition != "" {
		fallbackParams := append(params[:len(params)-3], params[len(params)-2:]...)
		fallbackSQL := fmt.Sprintf(`
			SELECT a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
				a.cover_image_large, a.cover_image_medium, a.episodes, a.season,
				a.season_year, a.average_score::integer as average_score, a.popularity,
				a.format, a.description,
				(SELECT COUNT(DISTINCT staff_id) FROM anime_staff WHERE anime_id = a.id) as staff_count
			FROM anime a %s %s LIMIT %s OFFSET %s`,
			whereClause, orderBy, limitParam, offsetParam)
		rows2, err := h.pg.Query(ctx, fallbackSQL, fallbackParams...)
		if err == nil {
			defer rows2.Close()
			animeList = h.scanPopularRows(rows2, sort)
		}
	}

	result := map[string]any{"success": true, "data": animeList}

	// Cache random results.
	if sort == "random" && limit > 1 {
		if encoded, err := json.Marshal(result); err == nil {
			cacheKey := fmt.Sprintf("%v-%v-%v-%v-%s-%s-%v-%v-%d-%d",
				eras, genres, tags, includeAdult, format, mediaType, hasRating, releasedOnly, limit, offset)
			h.randomCacheMu.Lock()
			h.randomCache[cacheKey] = &randomCacheEntry{data: encoded, timestamp: time.Now(), key: cacheKey}
			if len(h.randomCache) > 100 {
				h.pruneRandomCache()
			}
			h.randomCacheMu.Unlock()
		}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// AdvancedSearch handles GET /api/anime/advanced-search.
func (h *Handler) AdvancedSearch(w http.ResponseWriter, r *http.Request) {
	textQuery := r.URL.Query().Get("q")
	studios := httputil.QueryCSV(r, "studios")
	genres := httputil.QueryCSV(r, "genres")
	tags := httputil.QueryCSV(r, "tags")
	formats := httputil.QueryCSV(r, "formats")
	excludeAnimeID := httputil.QueryIntPtr(r, "excludeAnimeId")
	excludeRelated := httputil.QueryCSVInts(r, "excludeRelatedAnilistIds")
	excludeFranchiseID := httputil.QueryIntPtr(r, "excludeFranchiseId")
	yearMin := httputil.QueryIntPtr(r, "yearMin")
	yearMax := httputil.QueryIntPtr(r, "yearMax")
	scoreMin := httputil.QueryIntPtr(r, "scoreMin")
	scoreMax := httputil.QueryIntPtr(r, "scoreMax")
	format := r.URL.Query().Get("format")
	season := r.URL.Query().Get("season")
	episodesMin := httputil.QueryIntPtr(r, "episodesMin")
	episodesMax := httputil.QueryIntPtr(r, "episodesMax")
	sortBy := httputil.QueryString(r, "sort", "score")
	sortOrder := httputil.QueryString(r, "order", "desc")
	mediaType := r.URL.Query().Get("type")
	includeAdult := httputil.QueryBool(r, "includeAdult")
	page := httputil.QueryInt(r, "page", 1)
	limit := httputil.QueryInt(r, "limit", 18)
	skip := (page - 1) * limit

	ctx := r.Context()
	var conditions []string
	var params []any
	paramIdx := 1

	if mediaType == "manga" {
		conditions = append(conditions, `a.format IN ('MANGA', 'NOVEL', 'ONE_SHOT')`)
	} else if mediaType == "anime" {
		conditions = append(conditions, `a.format IN ('TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC')`)
	}

	if textQuery != "" {
		conditions = append(conditions, fmt.Sprintf(`(a.title ILIKE $%d OR a.title_english ILIKE $%d OR a.title_romaji ILIKE $%d)`, paramIdx, paramIdx, paramIdx))
		params = append(params, "%"+textQuery+"%")
		paramIdx++
	}

	if len(studios) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.studio_names @> $%d", paramIdx))
		params = append(params, studios)
		paramIdx++
	}
	if len(genres) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.genre_names @> $%d", paramIdx))
		params = append(params, genres)
		paramIdx++
	}
	if len(tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.tag_names @> $%d", paramIdx))
		params = append(params, tags)
		paramIdx++
	}

	if yearMin != nil {
		conditions = append(conditions, fmt.Sprintf("a.season_year IS NOT NULL AND a.season_year >= $%d", paramIdx))
		params = append(params, *yearMin)
		paramIdx++
	}
	if yearMax != nil {
		conditions = append(conditions, fmt.Sprintf("a.season_year IS NOT NULL AND a.season_year <= $%d", paramIdx))
		params = append(params, *yearMax)
		paramIdx++
	}
	if scoreMin != nil {
		conditions = append(conditions, fmt.Sprintf("a.average_score IS NOT NULL AND a.average_score >= $%d", paramIdx))
		params = append(params, *scoreMin)
		paramIdx++
	}
	if scoreMax != nil {
		conditions = append(conditions, fmt.Sprintf("a.average_score IS NOT NULL AND a.average_score <= $%d", paramIdx))
		params = append(params, *scoreMax)
		paramIdx++
	}

	if format != "" {
		conditions = append(conditions, fmt.Sprintf("a.format = $%d", paramIdx))
		params = append(params, format)
		paramIdx++
	} else if len(formats) > 0 {
		placeholders := make([]string, len(formats))
		for i, f := range formats {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			params = append(params, f)
			paramIdx++
		}
		conditions = append(conditions, fmt.Sprintf("a.format IN (%s)", strings.Join(placeholders, ", ")))
	}

	if season != "" {
		conditions = append(conditions, fmt.Sprintf("a.season = $%d", paramIdx))
		params = append(params, season)
		paramIdx++
	}

	if episodesMin != nil {
		conditions = append(conditions, fmt.Sprintf("a.episodes IS NOT NULL AND a.episodes >= $%d", paramIdx))
		params = append(params, *episodesMin)
		paramIdx++
	}
	if episodesMax != nil {
		conditions = append(conditions, fmt.Sprintf("a.episodes IS NOT NULL AND a.episodes <= $%d", paramIdx))
		params = append(params, *episodesMax)
		paramIdx++
	}

	if excludeAnimeID != nil {
		conditions = append(conditions, fmt.Sprintf("a.anilist_id <> $%d", paramIdx))
		params = append(params, *excludeAnimeID)
		paramIdx++
	}
	if len(excludeRelated) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.anilist_id <> ALL($%d)", paramIdx))
		params = append(params, excludeRelated)
		paramIdx++
	}
	if excludeFranchiseID != nil {
		conditions = append(conditions, fmt.Sprintf("(a.franchise_id IS NULL OR a.franchise_id <> $%d)", paramIdx))
		params = append(params, *excludeFranchiseID)
		paramIdx++
	}

	if sortBy == "year" {
		conditions = append(conditions, "a.season_year IS NOT NULL")
	}

	if !includeAdult {
		conditions = append(conditions, "(a.is_adult IS NULL OR a.is_adult = false)")
		conditions = append(conditions, `NOT EXISTS (
			SELECT 1 FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id
			WHERE ag.anime_id = a.id AND g.name = 'Ecchi')`)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query.
	countParams := make([]any, len(params))
	copy(countParams, params)
	var total int
	err := h.pg.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM anime a %s", whereClause), countParams...).Scan(&total)
	if err != nil {
		log.Printf("advanced search count error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to perform advanced search")
		return
	}
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	// Sort.
	order := "DESC"
	if strings.ToUpper(sortOrder) == "ASC" {
		order = "ASC"
	}
	var orderByClause string
	switch sortBy {
	case "score":
		orderByClause = fmt.Sprintf("ORDER BY a.average_score %s NULLS LAST, a.popularity DESC NULLS LAST", order)
	case "year":
		orderByClause = fmt.Sprintf(`ORDER BY a.season_year %s NULLS LAST,
			CASE a.season WHEN 'WINTER' THEN 1 WHEN 'SPRING' THEN 2 WHEN 'SUMMER' THEN 3 WHEN 'FALL' THEN 4 ELSE 0 END %s NULLS LAST,
			a.popularity DESC NULLS LAST`, order, order)
	case "title":
		orderByClause = fmt.Sprintf("ORDER BY a.title %s", order)
	case "relevance":
		if textQuery != "" {
			params = append(params, strings.ToLower(textQuery))
			textParamIdx := paramIdx
			paramIdx++
			orderByClause = fmt.Sprintf(`ORDER BY
				CASE
					WHEN LOWER(a.title) = $%d OR LOWER(a.title_english) = $%d OR LOWER(a.title_romaji) = $%d THEN 0
					WHEN LOWER(a.title) LIKE $%d || '%%' OR LOWER(a.title_english) LIKE $%d || '%%' OR LOWER(a.title_romaji) LIKE $%d || '%%' THEN 1
					ELSE 2
				END ASC, a.average_score DESC NULLS LAST`,
				textParamIdx, textParamIdx, textParamIdx, textParamIdx, textParamIdx, textParamIdx)
		} else {
			orderByClause = "ORDER BY a.average_score DESC NULLS LAST"
		}
	default:
		orderByClause = "ORDER BY a.average_score DESC NULLS LAST"
	}

	params = append(params, limit)
	limitParam := fmt.Sprintf("$%d", paramIdx)
	paramIdx++
	params = append(params, skip)
	offsetParam := fmt.Sprintf("$%d", paramIdx)

	sql := fmt.Sprintf(`
		SELECT a.title, a.title_english, a.title_romaji, a.anilist_id,
			a.cover_image, a.cover_image_extra_large, a.cover_image_large, a.cover_image_medium,
			a.format, a.season_year, a.season, a.average_score::integer as average_score,
			a.episodes, a.description, a.popularity,
			(SELECT array_agg(DISTINCT s.name) FROM anime_studio ast JOIN studio s ON ast.studio_id = s.id WHERE ast.anime_id = a.id AND ast.is_main = true) as studios,
			(SELECT array_agg(DISTINCT g.name) FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id WHERE ag.anime_id = a.id) as genres
		FROM anime a %s %s LIMIT %s OFFSET %s`, whereClause, orderByClause, limitParam, offsetParam)

	rows, err := h.pg.Query(ctx, sql, params...)
	if err != nil {
		log.Printf("advanced search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to perform advanced search")
		return
	}
	defer rows.Close()

	var animeList []map[string]any
	for rows.Next() {
		var title string
		var titleEn, titleRomaji, coverImage, coverImageXL, coverImageL, coverImageM *string
		var anilistID int
		var formatVal, seasonVal, description *string
		var seasonYear, averageScore, episodes, popularity *int
		var pStudioNames, pGenreNames *[]string

		if err := rows.Scan(&title, &titleEn, &titleRomaji, &anilistID,
			&coverImage, &coverImageXL, &coverImageL, &coverImageM,
			&formatVal, &seasonYear, &seasonVal, &averageScore,
			&episodes, &description, &popularity, &pStudioNames, &pGenreNames); err != nil {
			log.Printf("recommendation list scan error: %v", err)
			continue
		}

		studioNames := []string{}
		if pStudioNames != nil {
			studioNames = *pStudioNames
		}
		genreNames := []string{}
		if pGenreNames != nil {
			genreNames = *pGenreNames
		}

		animeList = append(animeList, map[string]any{
			"title": title, "title_english": titleEn, "title_romaji": titleRomaji,
			"anilistId": anilistID, "coverImage": coverImage,
			"coverImage_extraLarge": coverImageXL, "coverImage_large": coverImageL, "coverImage_medium": coverImageM,
			"format": formatVal, "seasonYear": seasonYear, "season": seasonVal,
			"averageScore": averageScore, "episodes": episodes, "description": description,
			"popularity": popularity, "studios": studioNames, "genres": genreNames,
		})
	}
	if animeList == nil {
		animeList = []map[string]any{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"total":       total,
		"page":        page,
		"totalPages":  totalPages,
		"hasNextPage": page < totalPages,
		"hasPrevPage": page > 1,
		"data":        animeList,
	})
}

// Recommendations handles GET /api/anime/{id}/recommendations.
func (h *Handler) Recommendations(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	anilistID, err := strconv.Atoi(idStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid anime ID format")
		return
	}

	page := httputil.QueryInt(r, "page", 1)
	limit := httputil.QueryInt(r, "limit", 12)
	skip := (page - 1) * limit
	ctx := r.Context()

	var total int
	err = h.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM anime_recommendation ar
		JOIN anime a ON ar.anime_id = a.id WHERE a.anilist_id = $1`, anilistID).Scan(&total)
	if err != nil {
		log.Printf("recommendations count error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch recommended anime")
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	rows, err := h.pg.Query(ctx, `
		SELECT rec.anilist_id, COALESCE(NULLIF(rec.title_english, ''), rec.title_romaji) as title,
			rec.cover_image_large, rec.cover_image_extra_large, ar.similarity,
			rec.average_score::integer as average_score, rec.description, rec.format, rec.season_year
		FROM anime_recommendation ar
		JOIN anime a ON ar.anime_id = a.id
		JOIN anime rec ON ar.recommended_anime_id = rec.id
		WHERE a.anilist_id = $1
		ORDER BY ar.similarity DESC LIMIT $2 OFFSET $3`, anilistID, limit, skip)
	if err != nil {
		log.Printf("recommendations query error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch recommended anime")
		return
	}
	defer rows.Close()

	var recs []map[string]any
	for rows.Next() {
		var recAnilistID int
		var title string
		var coverImageL, coverImageXL, description, formatVal *string
		var similarity float64
		var averageScore, seasonYear *int

		rows.Scan(&recAnilistID, &title, &coverImageL, &coverImageXL, &similarity,
			&averageScore, &description, &formatVal, &seasonYear)

		var coverImage any
		if coverImageXL != nil {
			coverImage = *coverImageXL
		} else if coverImageL != nil {
			coverImage = *coverImageL
		}

		recs = append(recs, map[string]any{
			"anilistId": recAnilistID, "title": title, "coverImage": coverImage,
			"similarity": similarity, "averageScore": averageScore,
			"description": description, "format": formatVal, "seasonYear": seasonYear,
		})
	}
	if recs == nil {
		recs = []map[string]any{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"recommendations": recs,
			"pagination": map[string]any{
				"page": page, "limit": limit, "total": total,
				"totalPages": totalPages, "hasMore": page < totalPages,
			},
		},
	})
}

// Relations handles GET /api/anime/{id}/relations.
func (h *Handler) Relations(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	anilistID, err := strconv.Atoi(idStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Anime ID is required")
		return
	}
	ctx := r.Context()

	var animeID int
	err = h.pg.QueryRow(ctx, "SELECT id FROM anime WHERE anilist_id = $1", anilistID).Scan(&animeID)
	if err == pgx.ErrNoRows {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
		return
	}
	if err != nil {
		log.Printf("relations lookup error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch anime relations")
		return
	}

	rows, err := h.pg.Query(ctx, `
		SELECT related.anilist_id, related.title, related.title_romaji, related.title_english,
			related.title_native, related.cover_image, related.cover_image_extra_large,
			related.cover_image_large, related.cover_image_medium, related.cover_image_color,
			related.banner_image, related.format, related.status,
			related.average_score::integer as average_score, related.season_year, related.season,
			related.episodes, ar.relation_type, ar.relation_type_rank
		FROM anime_relation ar
		JOIN anime related ON ar.related_anime_id = related.id
		WHERE ar.anime_id = $1
		ORDER BY ar.relation_type_rank ASC,
			related.season_year DESC NULLS LAST,
			CASE related.season WHEN 'FALL' THEN 4 WHEN 'SUMMER' THEN 3 WHEN 'SPRING' THEN 2 WHEN 'WINTER' THEN 1 ELSE 0 END DESC NULLS LAST`, animeID)
	if err != nil {
		log.Printf("relations query error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch anime relations")
		return
	}
	defer rows.Close()

	var relations []map[string]any
	for rows.Next() {
		var relAnilistID int
		var title, relationType *string
		var titleRomaji, titleEn, titleNative *string
		var coverImage, coverImageXL, coverImageL, coverImageM, coverImageColor, bannerImage *string
		var formatVal, status, seasonVal *string
		var averageScore, seasonYear, episodes *int
		var relationTypeRank *int

		rows.Scan(&relAnilistID, &title, &titleRomaji, &titleEn, &titleNative,
			&coverImage, &coverImageXL, &coverImageL, &coverImageM, &coverImageColor,
			&bannerImage, &formatVal, &status, &averageScore, &seasonYear, &seasonVal,
			&episodes, &relationType, &relationTypeRank)

		relations = append(relations, map[string]any{
			"anilistId": relAnilistID, "title": title,
			"title_romaji": titleRomaji, "title_english": titleEn, "title_native": titleNative,
			"coverImage": coverImage, "coverImage_extraLarge": coverImageXL,
			"coverImage_large": coverImageL, "coverImage_medium": coverImageM,
			"coverImage_color": coverImageColor, "bannerImage": bannerImage,
			"format": formatVal, "status": status, "averageScore": averageScore,
			"seasonYear": seasonYear, "season": seasonVal, "episodes": episodes,
			"relationType": relationType,
		})
	}
	if relations == nil {
		relations = []map[string]any{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": relations})
}

// Bulk handles GET /api/anime/bulk — batch fetch anime by IDs.
func (h *Handler) Bulk(w http.ResponseWriter, r *http.Request) {
	ids := httputil.QueryCSVInts(r, "ids")
	if len(ids) == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
		return
	}

	rows, err := h.pg.Query(r.Context(),
		"SELECT anilist_id, description FROM anime WHERE anilist_id = ANY($1::int[])", ids)
	if err != nil {
		log.Printf("bulk query error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch anime")
		return
	}
	defer rows.Close()

	var data []map[string]any
	for rows.Next() {
		var anilistID int
		var description *string
		rows.Scan(&anilistID, &description)
		data = append(data, map[string]any{"id": anilistID, "description": description})
	}
	if data == nil {
		data = []map[string]any{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// FilterCounts handles GET /api/anime/filter-counts.
func (h *Handler) FilterCounts(w http.ResponseWriter, r *http.Request) {
	checkStudios := httputil.QueryCSV(r, "checkStudios")
	checkGenres := httputil.QueryCSV(r, "checkGenres")
	checkTags := httputil.QueryCSV(r, "checkTags")
	excludeAnimeID := httputil.QueryIntPtr(r, "excludeAnimeId")

	ctx := r.Context()
	counts := map[string]any{
		"studios": map[string]int{},
		"genres":  map[string]int{},
		"tags":    map[string]int{},
	}

	studioCounts := counts["studios"].(map[string]int)
	genreCounts := counts["genres"].(map[string]int)
	tagCounts := counts["tags"].(map[string]int)

	for _, s := range checkStudios {
		studioCounts[s] = 0
	}
	for _, g := range checkGenres {
		genreCounts[g] = 0
	}
	for _, t := range checkTags {
		tagCounts[t] = 0
	}

	if len(checkStudios) > 0 {
		placeholders := make([]string, len(checkStudios))
		params := make([]any, len(checkStudios))
		for i, s := range checkStudios {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			params[i] = s
		}
		rows, err := h.pg.Query(ctx, fmt.Sprintf(
			"SELECT name, COALESCE(anime_count, 0) as anime_count FROM studio WHERE name IN (%s)",
			strings.Join(placeholders, ", ")), params...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var count int
				rows.Scan(&name, &count)
				studioCounts[name] = count
			}
		}
	}

	if len(checkGenres) > 0 {
		placeholders := make([]string, len(checkGenres))
		params := make([]any, len(checkGenres))
		for i, g := range checkGenres {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			params[i] = g
		}
		rows, err := h.pg.Query(ctx, fmt.Sprintf(
			"SELECT name, COALESCE(anime_count, 0) as anime_count FROM genre WHERE name IN (%s)",
			strings.Join(placeholders, ", ")), params...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var count int
				rows.Scan(&name, &count)
				genreCounts[name] = count
			}
		}
	}

	if len(checkTags) > 0 {
		placeholders := make([]string, len(checkTags))
		params := make([]any, len(checkTags))
		for i, t := range checkTags {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			params[i] = t
		}
		rows, err := h.pg.Query(ctx, fmt.Sprintf(
			"SELECT name, COALESCE(anime_count, 0) as anime_count FROM tag WHERE name IN (%s)",
			strings.Join(placeholders, ", ")), params...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var count int
				rows.Scan(&name, &count)
				tagCounts[name] = count
			}
		}
	}

	// Exclude anime.
	if excludeAnimeID != nil {
		var exStudios, exGenres, exTags []string
		h.pg.QueryRow(ctx, `
			SELECT
				(SELECT array_agg(s.name) FROM anime_studio ast JOIN studio s ON ast.studio_id = s.id WHERE ast.anime_id = a.id),
				(SELECT array_agg(g.name) FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id WHERE ag.anime_id = a.id),
				(SELECT array_agg(t.name) FROM anime_tag at JOIN tag t ON at.tag_id = t.id WHERE at.anime_id = a.id)
			FROM anime a WHERE a.anilist_id = $1`, *excludeAnimeID).Scan(&exStudios, &exGenres, &exTags)

		for _, s := range exStudios {
			if _, ok := studioCounts[s]; ok {
				studioCounts[s] = max(0, studioCounts[s]-1)
			}
		}
		for _, g := range exGenres {
			if _, ok := genreCounts[g]; ok {
				genreCounts[g] = max(0, genreCounts[g]-1)
			}
		}
		for _, t := range exTags {
			if _, ok := tagCounts[t]; ok {
				tagCounts[t] = max(0, tagCounts[t]-1)
			}
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "counts": counts})
}

// FilterMetadata handles GET /api/anime/filter-metadata — lightweight metadata for ALL anime.
func (h *Handler) FilterMetadata(w http.ResponseWriter, r *http.Request) {
	// Check cache.
	h.filterMetaCacheMu.RLock()
	cached := h.filterMetaCache
	cacheTime := h.filterMetaCacheTime
	h.filterMetaCacheMu.RUnlock()

	if cached != nil && time.Since(cacheTime) < time.Hour {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	ctx := r.Context()

	type idName struct {
		ID   int
		Name string
	}

	var studioRows, genreRows, tagRows []idName

	// Get lookup tables.
	query := func(sql string) ([]idName, error) {
		rows, err := h.pg.Query(ctx, sql)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []idName
		for rows.Next() {
			var r idName
			rows.Scan(&r.ID, &r.Name)
			out = append(out, r)
		}
		return out, nil
	}

	var wg sync.WaitGroup
	var studioErr, genreErr, tagErr error
	wg.Add(3)
	go func() { defer wg.Done(); studioRows, studioErr = query("SELECT id, name FROM studio ORDER BY name") }()
	go func() { defer wg.Done(); genreRows, genreErr = query("SELECT id, name FROM genre ORDER BY name") }()
	go func() { defer wg.Done(); tagRows, tagErr = query("SELECT id, name FROM tag ORDER BY name") }()
	wg.Wait()

	if studioErr != nil || genreErr != nil || tagErr != nil {
		httputil.Error(w, http.StatusInternalServerError, "Failed to get filter metadata")
		return
	}

	// Build index maps.
	studioIdx := make(map[int]int, len(studioRows))
	for i, r := range studioRows {
		studioIdx[r.ID] = i
	}
	genreIdx := make(map[int]int, len(genreRows))
	for i, r := range genreRows {
		genreIdx[r.ID] = i
	}
	tagIdx := make(map[int]int, len(tagRows))
	for i, r := range tagRows {
		tagIdx[r.ID] = i
	}

	// Get anime metadata.
	rows, err := h.pg.Query(ctx, `
		SELECT a.anilist_id,
			array_agg(DISTINCT ast.studio_id) FILTER (WHERE ast.studio_id IS NOT NULL) as studio_ids,
			array_agg(DISTINCT ag.genre_id) FILTER (WHERE ag.genre_id IS NOT NULL) as genre_ids,
			array_agg(DISTINCT at.tag_id) FILTER (WHERE at.tag_id IS NOT NULL) as tag_ids
		FROM anime a
		LEFT JOIN anime_studio ast ON a.id = ast.anime_id
		LEFT JOIN anime_genre ag ON a.id = ag.anime_id
		LEFT JOIN anime_tag at ON a.id = at.anime_id
		GROUP BY a.id, a.anilist_id ORDER BY a.anilist_id`)
	if err != nil {
		log.Printf("filter metadata query error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to get filter metadata")
		return
	}
	defer rows.Close()

	type compactAnime struct {
		ID      int   `json:"id"`
		Studios []int `json:"s"`
		Genres  []int `json:"g"`
		Tags    []int `json:"t"`
	}

	var animeMetadata []compactAnime
	for rows.Next() {
		var anilistID int
		var pSIDs, pGIDs, pTIDs *[]int32
		if err := rows.Scan(&anilistID, &pSIDs, &pGIDs, &pTIDs); err != nil {
			log.Printf("filter metadata scan error: %v", err)
			continue
		}
		var sIDs, gIDs, tIDs []int32
		if pSIDs != nil { sIDs = *pSIDs }
		if pGIDs != nil { gIDs = *pGIDs }
		if pTIDs != nil { tIDs = *pTIDs }

		s := mapIDs(sIDs, studioIdx)
		g := mapIDs(gIDs, genreIdx)
		t := mapIDs(tIDs, tagIdx)

		animeMetadata = append(animeMetadata, compactAnime{ID: anilistID, Studios: s, Genres: g, Tags: t})
	}

	studioNames := make([]string, len(studioRows))
	for i, r := range studioRows {
		studioNames[i] = r.Name
	}
	genreNames := make([]string, len(genreRows))
	for i, r := range genreRows {
		genreNames[i] = r.Name
	}
	tagNames := make([]string, len(tagRows))
	for i, r := range tagRows {
		tagNames[i] = r.Name
	}

	now := time.Now().UnixMilli()
	response := map[string]any{
		"success":    true,
		"count":      len(animeMetadata),
		"data":       animeMetadata,
		"useBitmaps": true,
		"lookups": map[string]any{
			"studios": studioNames,
			"genres":  genreNames,
			"tags":    tagNames,
		},
		"cached":    false,
		"timestamp": now,
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Failed to encode filter metadata")
		return
	}

	// Cache it.
	h.filterMetaCacheMu.Lock()
	h.filterMetaCache = encoded
	h.filterMetaCacheTime = time.Now()
	h.filterMetaCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded)
}

// GenresTags handles GET /api/anime/genres-tags.
func (h *Handler) GenresTags(w http.ResponseWriter, r *http.Request) {
	includeAdult := httputil.QueryBool(r, "includeAdult")
	cacheKey := "safe"
	if includeAdult {
		cacheKey = "adult"
	}

	h.genresTagsCacheMu.RLock()
	cached := h.genresTagsCache[cacheKey]
	cacheTime := h.genresTagsCacheTime[cacheKey]
	h.genresTagsCacheMu.RUnlock()

	if cached != nil && time.Since(cacheTime) < time.Hour {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	ctx := r.Context()
	adultGenres := map[string]bool{"Hentai": true, "Ecchi": true}

	genreRows, err := h.pg.Query(ctx, "SELECT name FROM genre WHERE name IS NOT NULL AND name <> '' ORDER BY name")
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch genres and tags")
		return
	}
	defer genreRows.Close()

	var genres []string
	for genreRows.Next() {
		var name string
		genreRows.Scan(&name)
		if !includeAdult && adultGenres[name] {
			continue
		}
		genres = append(genres, name)
	}

	tagRows, err := h.pg.Query(ctx, "SELECT name FROM tag WHERE name IS NOT NULL AND name <> '' ORDER BY name")
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch genres and tags")
		return
	}
	defer tagRows.Close()

	var tagsArr []string
	for tagRows.Next() {
		var name string
		tagRows.Scan(&name)
		tagsArr = append(tagsArr, name)
	}

	allFilters := make([]string, 0, len(genres)+len(tagsArr))
	allFilters = append(allFilters, genres...)
	allFilters = append(allFilters, tagsArr...)
	sortStrings(allFilters)

	response := map[string]any{
		"success":    true,
		"genres":     genres,
		"tags":       tagsArr,
		"allFilters": allFilters,
	}

	if encoded, err := json.Marshal(response); err == nil {
		h.genresTagsCacheMu.Lock()
		h.genresTagsCache[cacheKey] = encoded
		h.genresTagsCacheTime[cacheKey] = time.Now()
		h.genresTagsCacheMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(encoded)
		return
	}

	httputil.JSON(w, http.StatusOK, response)
}

// --- internal helpers ---

type animeRow struct {
	Title, TitleJA, TitleNative, TitleRomaji, TitleEnglish *string
	AnilistID                                              int
	Type                                                   *string
	CoverImage, CoverImageExtraLarge, CoverImageLarge      *string
	CoverImageMedium, CoverImageColor, BannerImage         *string
	Episodes, Duration                                     *int
	SeasonYear                                             *int
	Season, Format, Status                                 *string
	AverageScore, MeanScore                                *int
	Description, Source, CountryOfOrigin                    *string
	IsAdult                                                *bool
	Synonyms                                               []string
	UpdatedAt, KeyframeLink                                *string
	MalID                                                  *int
	SakugabooruTag                                         *string
	WikipediaEn, WikipediaJa, WikipediaProductionHTML      *string
	WikidataQID, LivechartID, TvdbID                       *string
	TmdbMovieID, TmdbTvID                                  *string
	TrailerID, TrailerSite, TrailerThumbnail                *string
	FranchiseID                                            *int
	FranchiseTitle                                         *string
}

func (h *Handler) scanPopularRows(rows pgx.Rows, sort string) []map[string]any {
	var list []map[string]any
	for rows.Next() {
		var anilistID int
		var title string
		var coverImage, coverImageXL, coverImageL, coverImageM *string
		var episodes *int
		var season *string
		var seasonYear, averageScore, popularity *int
		var formatVal, description *string
		var staffCount *int64

		rows.Scan(&anilistID, &title, &coverImage, &coverImageXL,
			&coverImageL, &coverImageM, &episodes, &season,
			&seasonYear, &averageScore, &popularity, &formatVal, &description, &staffCount)

		item := map[string]any{
			"id": anilistID, "anilistId": anilistID, "title": title,
			"coverImage": coverImage, "coverImage_extraLarge": coverImageXL,
			"coverImage_large": coverImageL, "coverImage_medium": coverImageM,
			"episodes": episodes, "season": season, "seasonYear": seasonYear,
			"averageScore": averageScore, "popularity": popularity,
			"format": formatVal, "description": description,
		}
		if sort == "most-staff" && staffCount != nil {
			item["staffCount"] = *staffCount
		}
		list = append(list, item)
	}
	if list == nil {
		list = []map[string]any{}
	}
	return list
}

func (h *Handler) refreshRandomRanks(ctx context.Context) {
	if time.Since(h.lastRandomRefresh) < time.Minute {
		return
	}
	go func() {
		_, err := h.pg.Exec(context.Background(), "SELECT refresh_random_ranks()")
		if err != nil {
			log.Printf("[random] failed to refresh random ranks: %v", err)
		} else {
			log.Println("[random] refreshed random_rank values")
			h.randomCacheMu.Lock()
			h.randomCache = make(map[string]*randomCacheEntry)
			h.lastRandomRefresh = time.Now()
			h.randomCacheMu.Unlock()
		}
	}()
}

func (h *Handler) pruneRandomCache() {
	if len(h.randomCache) <= 100 {
		return
	}
	var oldest string
	var oldestTime time.Time
	for k, v := range h.randomCache {
		if oldest == "" || v.timestamp.Before(oldestTime) {
			oldest = k
			oldestTime = v.timestamp
		}
	}
	delete(h.randomCache, oldest)
}

func mapIDs(ids []int32, indexMap map[int]int) []int {
	if ids == nil {
		return []int{}
	}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if idx, ok := indexMap[int(id)]; ok {
			out = append(out, idx)
		}
	}
	return out
}

func sortStrings(s []string) {
	// Simple sort for string slices.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
