package staff

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
)

type Handler struct {
	pg *pgxpool.Pool
	es *elasticsearch.Client
}

func NewHandler(pg *pgxpool.Pool, es *elasticsearch.Client) *Handler {
	return &Handler{pg: pg, es: es}
}

// Search handles GET /api/staff/search — Elasticsearch staff search.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := httputil.QueryString(r, "q", "*")
	limit := httputil.QueryInt(r, "limit", 20)

	exists, _ := h.indexExists("staff")
	if !exists {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true, "total": 0, "data": []any{},
			"message": "No staff data indexed yet. Please run data ingestion first.",
		})
		return
	}

	body := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"should": []any{
					map[string]any{"multi_match": map[string]any{
						"query": q, "fields": []string{"name^4", "name_en^3", "name_ja^2"},
						"type": "best_fields", "fuzziness": "AUTO",
					}},
					map[string]any{"match": map[string]any{
						"name": map[string]any{"query": q, "operator": "and", "boost": 2},
					}},
				},
			},
		},
		"size": limit,
		"highlight": map[string]any{
			"fields": map[string]any{"name": map[string]any{}, "name_en": map[string]any{}},
		},
	}

	hits, total, err := h.doSearch("staff", body)
	if err != nil {
		log.Printf("staff search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to search staff")
		return
	}

	data := make([]map[string]any, len(hits))
	for i, hit := range hits {
		data[i] = map[string]any{
			"id":         orVal(hit.Source["id"], hit.ID),
			"name":       hit.Source["name"],
			"nameEn":     hit.Source["name_en"],
			"nameJa":     hit.Source["name_ja"],
			"picture":    hit.Source["picture"],
			"score":      hit.Score,
			"highlights": hit.Highlight,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "total": total, "data": data})
}

// GetByID handles GET /api/staff/{id} — full staff detail with filmography.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	staffID, err := strconv.Atoi(idStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Staff ID is required")
		return
	}

	ctx := r.Context()

	// Fetch staff details.
	var s staffRow
	err = h.pg.QueryRow(ctx, `
		SELECT id, staff_id, name_en, name_ja, pen_name_en, pen_name_ja,
			image_large, image_medium, language, description, primary_occupations, gender,
			date_of_birth_year, date_of_birth_month, date_of_birth_day,
			date_of_death_year, date_of_death_month, date_of_death_day,
			age, years_active, home_town, blood_type
		FROM staff WHERE staff_id = $1`, staffID).Scan(
		&s.ID, &s.StaffID, &s.NameEN, &s.NameJA, &s.PenNameEN, &s.PenNameJA,
		&s.ImageLarge, &s.ImageMedium, &s.Language, &s.Description,
		&s.PrimaryOccupations, &s.Gender,
		&s.DOBYear, &s.DOBMonth, &s.DOBDay,
		&s.DODYear, &s.DODMonth, &s.DODDay,
		&s.Age, &s.YearsActive, &s.HomeTown, &s.BloodType,
	)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Staff member not found")
		return
	}
	if err != nil {
		log.Printf("staff detail error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch staff details")
		return
	}

	// Filmography.
	filmRows, err := h.pg.Query(ctx, `
		SELECT a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium, a.description, a.episodes,
			a.season_year, a.season, a.format, a.average_score::integer, a.is_adult,
			asf.role, asf.weight,
			(SELECT array_agg(DISTINCT g.name) FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id WHERE ag.anime_id = a.id) as genres,
			(SELECT json_agg(json_build_object('name', t.name, 'rank', at.rank))
			 FROM anime_tag at JOIN tag t ON at.tag_id = t.id WHERE at.anime_id = a.id) as tags
		FROM anime_staff asf
		JOIN anime a ON asf.anime_id = a.id
		WHERE asf.staff_id = $1
		ORDER BY asf.weight DESC NULLS LAST`, s.ID)
	if err != nil {
		log.Printf("filmography error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch staff details")
		return
	}
	defer filmRows.Close()

	var filmography []map[string]any
	genreCounts := map[string]int{}
	tagScores := map[string]*tagAcc{}

	for filmRows.Next() {
		var anilistID int
		var title string
		var coverImage, coverImageXL, coverImageL, coverImageM, description *string
		var episodes, seasonYear, averageScore *int
		var season, formatVal *string
		var role *[]string
		var isAdult *bool
		var weight *float64
		var genres *[]string
		var tagsJSON *[]byte

		if err := filmRows.Scan(&anilistID, &title, &coverImage, &coverImageXL,
			&coverImageL, &coverImageM, &description, &episodes,
			&seasonYear, &season, &formatVal, &averageScore, &isAdult,
			&role, &weight, &genres, &tagsJSON); err != nil {
			log.Printf("filmography scan error: %v", err)
			continue
		}

		if title == "" {
			continue
		}

		var tags []map[string]any
		if tagsJSON != nil {
			json.Unmarshal(*tagsJSON, &tags)
		}
		if tags == nil {
			tags = []map[string]any{}
		}
		var genreList []string
		if genres != nil {
			genreList = *genres
		}
		if genreList == nil {
			genreList = []string{}
		}

		roleStr := ""
		if role != nil && len(*role) > 0 {
			roleStr = (*role)[0]
		}
		category := categorizeRole(roleStr)

		filmography = append(filmography, map[string]any{
			"anime": map[string]any{
				"anilistId": anilistID, "title": title,
				"coverImage": coverImage, "coverImage_extraLarge": coverImageXL,
				"coverImage_large": coverImageL, "coverImage_medium": coverImageM,
				"description": description, "episodes": episodes,
				"seasonYear": seasonYear, "season": season,
				"format": formatVal, "averageScore": averageScore,
				"isAdult": isAdult, "genres": genreList, "tags": tags,
			},
			"role": role, "weight": weight, "category": category,
		})

		// Accumulate stats.
		for _, g := range genreList {
			genreCounts[g]++
		}
		for _, t := range tags {
			name, _ := t["name"].(string)
			rank, _ := t["rank"].(float64)
			if name == "" {
				continue
			}
			acc := tagScores[name]
			if acc == nil {
				acc = &tagAcc{}
				tagScores[name] = acc
			}
			acc.count++
			acc.totalRank += rank
		}
	}

	if filmography == nil {
		filmography = []map[string]any{}
	}

	// Categories.
	catSet := map[string]bool{}
	for _, f := range filmography {
		catSet[f["category"].(string)] = true
	}
	categories := make([]string, 0, len(catSet))
	for c := range catSet {
		categories = append(categories, c)
	}

	// Genre stats.
	genreStats := make([]map[string]any, 0, len(genreCounts))
	for name, count := range genreCounts {
		genreStats = append(genreStats, map[string]any{"name": name, "count": count})
	}
	sort.Slice(genreStats, func(i, j int) bool {
		return genreStats[i]["count"].(int) > genreStats[j]["count"].(int)
	})

	// Tag stats (top 20).
	tagStats := make([]map[string]any, 0, len(tagScores))
	for name, acc := range tagScores {
		avg := 0.0
		if acc.count > 0 {
			avg = acc.totalRank / float64(acc.count)
		}
		tagStats = append(tagStats, map[string]any{"name": name, "count": acc.count, "avgRank": avg})
	}
	sort.Slice(tagStats, func(i, j int) bool {
		ci := tagStats[i]["count"].(int)
		cj := tagStats[j]["count"].(int)
		if ci != cj {
			return ci > cj
		}
		return tagStats[i]["avgRank"].(float64) > tagStats[j]["avgRank"].(float64)
	})
	if len(tagStats) > 20 {
		tagStats = tagStats[:20]
	}

	// Sakugabooru posts.
	sakuRows, err := h.pg.Query(ctx, `
		SELECT sp.post_id, sp.file_url, sp.preview_url, sp.source, sp.file_ext, sp.rating
		FROM staff_sakugabooru_post ssp
		JOIN sakugabooru_post sp ON ssp.post_id = sp.post_id
		WHERE ssp.staff_id = $1
			AND LOWER(REVERSE(SPLIT_PART(REVERSE(sp.file_url), '.', 1))) IN ('mp4', 'webm')
		ORDER BY sp.post_id DESC`, s.ID)
	var sakuPosts []map[string]any
	if err == nil {
		defer sakuRows.Close()
		for sakuRows.Next() {
			var postID int
			var fileURL string
			var previewURL, source, fileExt, rating *string
			sakuRows.Scan(&postID, &fileURL, &previewURL, &source, &fileExt, &rating)
			sakuPosts = append(sakuPosts, map[string]any{
				"postId": postID, "fileUrl": fileURL, "previewUrl": previewURL,
				"source": source, "fileExt": fileExt, "rating": rating,
			})
		}
	}
	if sakuPosts == nil {
		sakuPosts = []map[string]any{}
	}

	data := map[string]any{
		"staff_id": s.StaffID, "name_en": s.NameEN, "name_ja": s.NameJA,
		"pen_name_en": s.PenNameEN, "pen_name_ja": s.PenNameJA,
		"image_large": s.ImageLarge, "image_medium": s.ImageMedium,
		"language": s.Language, "description": s.Description,
		"primaryOccupations": s.PrimaryOccupations, "gender": s.Gender,
		"dateOfBirth_year": s.DOBYear, "dateOfBirth_month": s.DOBMonth, "dateOfBirth_day": s.DOBDay,
		"dateOfDeath_year": s.DODYear, "dateOfDeath_month": s.DODMonth, "dateOfDeath_day": s.DODDay,
		"age": s.Age, "yearsActive": s.YearsActive, "homeTown": s.HomeTown, "bloodType": s.BloodType,
		"filmography": filmography, "categories": categories,
		"genreStats": genreStats, "tagStats": tagStats,
		"sakugabooruTag": nil, "sakugabooruPosts": sakuPosts,
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// --- types and helpers ---

type staffRow struct {
	ID                 int
	StaffID            int
	NameEN, NameJA     *string
	PenNameEN, PenNameJA *string
	ImageLarge, ImageMedium *string
	Language, Description *string
	PrimaryOccupations *[]string
	Gender             *string
	DOBYear, DOBMonth, DOBDay *int
	DODYear, DODMonth, DODDay *int
	Age                *int
	YearsActive        *[]int32
	HomeTown           *string
	BloodType          *string
}

type tagAcc struct {
	count     int
	totalRank float64
}

type searchHit struct {
	ID        string         `json:"_id"`
	Score     float64        `json:"_score"`
	Source    map[string]any `json:"_source"`
	Highlight map[string]any `json:"highlight,omitempty"`
}

func (h *Handler) doSearch(index string, body map[string]any) ([]searchHit, int, error) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	res, err := h.es.Search(
		h.es.Search.WithIndex(index),
		h.es.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Total json.RawMessage `json:"total"`
			Hits  []searchHit     `json:"hits"`
		} `json:"hits"`
	}
	json.NewDecoder(res.Body).Decode(&result)

	var total int
	var obj struct{ Value int }
	if json.Unmarshal(result.Hits.Total, &obj) == nil {
		total = obj.Value
	} else {
		json.Unmarshal(result.Hits.Total, &total)
	}

	return result.Hits.Hits, total, nil
}

func (h *Handler) indexExists(index string) (bool, error) {
	res, err := h.es.Indices.Exists([]string{index})
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode == 200, nil
}

func orVal(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
