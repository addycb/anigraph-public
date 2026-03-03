package admin

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
	"anigraph/backend/internal/api/httputil"
)

// enrichmentRunning guards concurrent enrichment invocations.
var enrichmentRunning sync.Mutex
var enrichmentActive bool

// EnrichWikidata enriches anime with Wikidata QIDs via the enrichment service.
func (h *Handler) EnrichWikidata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	force := r.URL.Query().Get("force") == "true"

	if enrichmentActive {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": "Wikidata enrichment is already running. Check server logs for progress.",
		})
		return
	}

	whereClause := "WHERE wikidata_qid IS NULL AND (wikidata_searched_at IS NULL OR wikidata_searched_at < NOW() - INTERVAL '30 days')"
	if force {
		whereClause = "WHERE 1=1"
	}

	rows, err := h.pg.Query(ctx, fmt.Sprintf(`
		SELECT anilist_id,
			COALESCE(mal_id::text, '') AS mal_id,
			COALESCE(title_english, '') AS title_english,
			COALESCE(title_romaji, '') AS title_romaji,
			COALESCE(season_year::text, '') AS season_year,
			COALESCE(type, '') AS type,
			COALESCE(format, '') AS format
		FROM anime %s ORDER BY anilist_id`, whereClause))
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	type animeRow struct {
		AnilistID   int32
		MalID       string
		TitleEng    string
		TitleRom    string
		SeasonYear  string
		Type        string
		Format      string
	}

	var anime []animeRow
	for rows.Next() {
		var a animeRow
		if err := rows.Scan(&a.AnilistID, &a.MalID, &a.TitleEng, &a.TitleRom, &a.SeasonYear, &a.Type, &a.Format); err != nil {
			continue
		}
		anime = append(anime, a)
	}

	total := len(anime)
	if total == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true, "message": "All anime already have Wikidata QIDs.", "updated": 0,
		})
		return
	}

	enrichmentActive = true
	startedAt := time.Now().Format(time.RFC3339)

	// Fire-and-forget: run enrichment in background.
	go func() {
		defer func() { enrichmentActive = false }()
		bgCtx := context.Background()

		// Build entries for bidi-streaming.
		entries := make([]*pb.AnimeWikidataInput, len(anime))
		for i, a := range anime {
			malID, _ := strconv.ParseInt(a.MalID, 10, 32)
			sy, _ := strconv.ParseInt(a.SeasonYear, 10, 32)
			entries[i] = &pb.AnimeWikidataInput{
				AnilistId:    a.AnilistID,
				MalId:        int32(malID),
				TitleEnglish: a.TitleEng,
				TitleRomaji:  a.TitleRom,
				SeasonYear:   int32(sy),
				Type:         a.Type,
				Format:       a.Format,
			}
		}

		// Call enrichment service via bidi-stream client.
		bidiStream := h.enrichment.EnrichWikidata(bgCtx)

		// Send all entries in one request.
		if err := bidiStream.Send(&pb.EnrichWikidataRequest{Entries: entries}); err != nil {
			log.Printf("[enrich-wikidata] Send failed: %v", err)
			return
		}
		if err := bidiStream.CloseRequest(); err != nil {
			log.Printf("[enrich-wikidata] CloseRequest failed: %v", err)
			return
		}

		// Receive all results.
		var resolved []wikidataResult
		for {
			resp, err := bidiStream.Receive()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("[enrich-wikidata] Receive error: %v", err)
				break
			}
			batch := resp.GetResults()
			if batch == nil {
				continue
			}
			for _, r := range batch.Entries {
				resolved = append(resolved, wikidataResult{
					AnilistID:           r.AnilistId,
					WikidataQID:         r.WikidataQid,
					Method:              r.Method,
					WikipediaEn:         r.WikipediaEn,
					WikipediaJa:         r.WikipediaJa,
					LivechartID:         r.LivechartId,
					NotifyID:            r.NotifyId,
					TvdbID:              r.TvdbId,
					TmdbMovieID:         r.TmdbMovieId,
					TmdbTvID:            r.TmdbTvId,
					TvmazeID:            r.TvmazeId,
					MywaifulistID:       r.MywaifulistId,
					UnconsentingMediaID: r.UnconsentingMediaId,
				})
			}
		}

		if len(resolved) > 0 {
			h.applyWikidataResults(bgCtx, resolved, force)
		}
		log.Printf("[enrich-wikidata] Done. updated=%d", len(resolved))
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Wikidata enrichment started for %d anime%s. Monitor server logs for progress.", total, ifStr(force, " (force re-enrich ALL)", "")),
		"total":     total,
		"force":     force,
		"startedAt": startedAt,
	})
}

type wikidataResult struct {
	AnilistID                                                                                              int32
	WikidataQID, Method, WikipediaEn, WikipediaJa                                                         string
	LivechartID, NotifyID, TvdbID, TmdbMovieID, TmdbTvID, TvmazeID, MywaifulistID, UnconsentingMediaID string
}

func (h *Handler) applyWikidataResults(ctx context.Context, results []wikidataResult, force bool) {
	resolved := make([]wikidataResult, 0, len(results))
	var notFoundIDs []int32
	for _, r := range results {
		if r.WikidataQID != "" && r.Method != "not_found" {
			resolved = append(resolved, r)
		} else {
			notFoundIDs = append(notFoundIDs, r.AnilistID)
		}
	}

	if len(resolved) > 0 {
		ids := make([]int32, len(resolved))
		qids := make([]string, len(resolved))
		wpEn := make([]string, len(resolved))
		wpJa := make([]string, len(resolved))
		lc := make([]string, len(resolved))
		nt := make([]string, len(resolved))
		tvdb := make([]string, len(resolved))
		tmdbM := make([]string, len(resolved))
		tmdbT := make([]string, len(resolved))
		tvmz := make([]string, len(resolved))
		mywf := make([]string, len(resolved))
		ucm := make([]string, len(resolved))

		for i, r := range resolved {
			ids[i] = r.AnilistID
			qids[i] = r.WikidataQID
			wpEn[i] = r.WikipediaEn
			wpJa[i] = r.WikipediaJa
			lc[i] = r.LivechartID
			nt[i] = r.NotifyID
			tvdb[i] = r.TvdbID
			tmdbM[i] = r.TmdbMovieID
			tmdbT[i] = r.TmdbTvID
			tvmz[i] = r.TvmazeID
			mywf[i] = r.MywaifulistID
			ucm[i] = r.UnconsentingMediaID
		}

		forceClause := ""
		if !force {
			forceClause = "AND anime.wikidata_qid IS NULL"
		}

		_, err := h.pg.Exec(ctx, fmt.Sprintf(`UPDATE anime SET
			wikidata_qid = data.wikidata_qid,
			wikipedia_en = NULLIF(data.wikipedia_en, ''),
			wikipedia_ja = NULLIF(data.wikipedia_ja, ''),
			livechart_id = NULLIF(data.livechart_id, ''),
			notify_id = NULLIF(data.notify_id, ''),
			tvdb_id = NULLIF(data.tvdb_id, ''),
			tmdb_movie_id = NULLIF(data.tmdb_movie_id, ''),
			tmdb_tv_id = NULLIF(data.tmdb_tv_id, ''),
			tvmaze_id = NULLIF(data.tvmaze_id, ''),
			mywaifulist_id = NULLIF(data.mywaifulist_id, ''),
			unconsenting_media_id = NULLIF(data.unconsenting_media_id, '')
		FROM (SELECT unnest($1::int[]) AS anilist_id, unnest($2::text[]) AS wikidata_qid,
			unnest($3::text[]) AS wikipedia_en, unnest($4::text[]) AS wikipedia_ja,
			unnest($5::text[]) AS livechart_id, unnest($6::text[]) AS notify_id,
			unnest($7::text[]) AS tvdb_id, unnest($8::text[]) AS tmdb_movie_id,
			unnest($9::text[]) AS tmdb_tv_id, unnest($10::text[]) AS tvmaze_id,
			unnest($11::text[]) AS mywaifulist_id, unnest($12::text[]) AS unconsenting_media_id
		) AS data WHERE anime.anilist_id = data.anilist_id %s`, forceClause),
			ids, qids, wpEn, wpJa, lc, nt, tvdb, tmdbM, tmdbT, tvmz, mywf, ucm)
		if err != nil {
			log.Printf("[enrich-wikidata] Bulk update failed: %v", err)
		}
	}

	if len(notFoundIDs) > 0 {
		h.pg.Exec(ctx, "UPDATE anime SET wikidata_searched_at = NOW() WHERE anilist_id = ANY($1::int[])", notFoundIDs)
		log.Printf("[enrich-wikidata] Stamped %d not-found anime with 30-day cooldown.", len(notFoundIDs))
	}
}

// BackfillWikidataProps backfills missing Wikidata properties for anime with QIDs.
func (h *Handler) BackfillWikidataProps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	force := r.URL.Query().Get("force") == "true"

	whereClause := `WHERE wikidata_qid IS NOT NULL AND wikidata_qid != ''
		AND (wikipedia_en IS NULL OR livechart_id IS NULL)`
	if force {
		whereClause = `WHERE wikidata_qid IS NOT NULL AND wikidata_qid != ''`
	}

	rows, err := h.pg.Query(ctx, fmt.Sprintf(
		`SELECT anilist_id, wikidata_qid FROM anime %s ORDER BY anilist_id`, whereClause))
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var ids []int32
	var qids []string
	for rows.Next() {
		var id int32
		var qid string
		if err := rows.Scan(&id, &qid); err == nil {
			ids = append(ids, id)
			qids = append(qids, qid)
		}
	}

	total := len(ids)
	if total == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "No anime need backfilling.", "total": 0})
		return
	}

	log.Printf("[backfill-wikidata-props] Starting for %d anime", total)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Backfill started for %d anime. Monitor server logs.", total),
		"total":     total,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// WikipediaProduction scrapes Wikipedia production HTML for anime.
func (h *Handler) WikipediaProduction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pg.Query(ctx,
		`SELECT anilist_id, wikipedia_en FROM anime
		 WHERE wikipedia_en IS NOT NULL AND wikipedia_en != ''
		   AND wikipedia_production_html IS NULL
		 ORDER BY anilist_id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
	}

	log.Printf("[wikipedia-production] Starting for %d anime", count)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Wikipedia production scrape started for %d anime. Monitor logs.", count),
		"total":     count,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// WikipediaStudioContent scrapes Wikipedia content HTML for studios.
func (h *Handler) WikipediaStudioContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pg.Query(ctx,
		`SELECT id, wikipedia_en FROM studio
		 WHERE wikipedia_en IS NOT NULL AND wikipedia_en != ''
		   AND wikipedia_content_html IS NULL
		 ORDER BY id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
	}

	log.Printf("[wikipedia-studio-content] Starting for %d studios", count)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Wikipedia studio content scrape started for %d studios. Monitor logs.", count),
		"total":     count,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// EnrichStaffAlternativeNames fetches alternative name romanizations for staff.
func (h *Handler) EnrichStaffAlternativeNames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pg.Query(ctx, "SELECT staff_id FROM staff ORDER BY staff_id")
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var staffIDs []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err == nil {
			staffIDs = append(staffIDs, id)
		}
	}

	total := len(staffIDs)
	log.Printf("[enrich-staff-alt-names] Starting for %d staff", total)

	// Fire-and-forget enrichment using bidi-stream client.
	go func() {
		bgCtx := context.Background()
		bidiStream := h.enrichment.EnrichStaffAlternativeNames(bgCtx)

		// Send staff IDs.
		if err := bidiStream.Send(&pb.EnrichStaffAlternativeNamesRequest{
			StaffIds: staffIDs,
		}); err != nil {
			log.Printf("[enrich-staff-alt-names] Send failed: %v", err)
			return
		}
		if err := bidiStream.CloseRequest(); err != nil {
			log.Printf("[enrich-staff-alt-names] CloseRequest failed: %v", err)
			return
		}

		// Collect all results.
		var allResults []*pb.StaffAlternativeNamesResult
		for {
			resp, err := bidiStream.Receive()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("[enrich-staff-alt-names] Receive error: %v", err)
				break
			}
			batch := resp.GetResults()
			if batch != nil {
				allResults = append(allResults, batch.Entries...)
			}
		}

		// Apply results in chunks.
		const chunkSize = 500
		updated := 0
		for i := 0; i < len(allResults); i += chunkSize {
			end := i + chunkSize
			if end > len(allResults) {
				end = len(allResults)
			}
			chunk := allResults[i:end]

			ids := make([]int32, len(chunk))
			names := make([]string, len(chunk))
			for j, r := range chunk {
				ids[j] = r.StaffId
				names[j] = "{" + strings.Join(quoteAll(r.AlternativeNames), ",") + "}"
			}

			_, err := h.pg.Exec(bgCtx,
				`UPDATE staff SET alternative_names = data.names::text[]
				 FROM (SELECT unnest($1::int[]) AS staff_id, unnest($2::text[]) AS names) AS data
				 WHERE staff.staff_id = data.staff_id`, ids, names)
			if err != nil {
				log.Printf("[enrich-staff-alt-names] Chunk update failed: %v", err)
			} else {
				updated += len(chunk)
			}
		}
		log.Printf("[enrich-staff-alt-names] Done. Updated %d staff.", updated)
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Staff alternative names enrichment started for %d staff.", total),
		"total":     total,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// SakugabooruEnrich runs the Sakugabooru enrichment pipeline (tag matching + post fetch).
func (h *Handler) SakugabooruEnrich(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var body struct {
		SkipTagLookup bool `json:"skipTagLookup"`
		SkipPostFetch bool `json:"skipPostFetch"`
	}
	readBodyJSON(w, r, &body)

	mangaFormats := []string{"MANGA", "NOVEL", "ONE_SHOT", "MANHWA", "MANHUA"}

	// Quick counts.
	var animeCount, staffCount, postCount int64
	h.pg.QueryRow(ctx, `SELECT COUNT(*) FROM anime WHERE type='ANIME' AND format NOT IN ('MANGA','NOVEL','ONE_SHOT','MANHWA','MANHUA') AND sakugabooru_tag IS NULL`).Scan(&animeCount)
	h.pg.QueryRow(ctx, `SELECT COUNT(*) FROM staff WHERE sakugabooru_tag IS NULL`).Scan(&staffCount)
	h.pg.QueryRow(ctx, `SELECT COUNT(*) FROM sakugabooru_post`).Scan(&postCount)

	startedAt := time.Now().Format(time.RFC3339)

	// Run in background.
	go func() {
		bgCtx := context.Background()
		results := map[string]any{}

		// Phase 1: Tag matching.
		if !body.SkipTagLookup {
			tagResult, err := h.runSakugabooruTagMatching(bgCtx, mangaFormats)
			if err != nil {
				log.Printf("[sakugabooru] Phase 1 failed: %v", err)
			} else {
				results["tagMatching"] = tagResult
			}
		}

		// Phase 2: Post fetch.
		if !body.SkipPostFetch {
			postResult, err := h.runSakugabooruPostFetch(bgCtx, mangaFormats)
			if err != nil {
				log.Printf("[sakugabooru] Phase 2 failed: %v", err)
			} else {
				results["posts"] = postResult
			}
		}

		log.Printf("[sakugabooru] Pipeline complete.")
	}()

	phases := []string{}
	if !body.SkipTagLookup {
		phases = append(phases, fmt.Sprintf("tag matching (%d anime, %d staff untagged)", animeCount, staffCount))
	}
	if !body.SkipPostFetch {
		phases = append(phases, "post fetch + import")
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":          true,
		"message":          fmt.Sprintf("Sakugabooru enrichment started. Phases: %s. Monitor server logs.", strings.Join(phases, " → ")),
		"startedAt":        startedAt,
		"postsAlreadyInDb": postCount,
		"animeNeedingTags": ifAny(body.SkipTagLookup, "skipped", animeCount),
		"staffNeedingTags": ifAny(body.SkipTagLookup, "skipped", staffCount),
	})
}

func (h *Handler) runSakugabooruTagMatching(ctx context.Context, mangaFormats []string) (map[string]any, error) {
	// Query anime and staff needing tags.
	animeRows, err := h.pg.Query(ctx, `
		SELECT anilist_id, COALESCE(title_english,'') AS title_english,
			COALESCE(title_romaji,'') AS title_romaji, COALESCE(synonyms,'{}') AS synonyms
		FROM anime WHERE type='ANIME' AND format NOT IN ('MANGA','NOVEL','ONE_SHOT','MANHWA','MANHUA')
			AND sakugabooru_tag IS NULL ORDER BY anilist_id`)
	if err != nil {
		return nil, err
	}
	defer animeRows.Close()

	var animeInputs []*pb.AnimeMatchInput
	for animeRows.Next() {
		var id int32
		var titleEng, titleRom string
		var synonyms []string
		if err := animeRows.Scan(&id, &titleEng, &titleRom, &synonyms); err != nil {
			continue
		}
		animeInputs = append(animeInputs, &pb.AnimeMatchInput{
			AnilistId:    id,
			TitleEnglish: titleEng,
			TitleRomaji:  titleRom,
			Synonyms:     synonyms,
		})
	}

	staffRows, err := h.pg.Query(ctx, `
		SELECT staff_id, COALESCE(name_en,'') AS name_en,
			COALESCE(name_ja,'') AS name_ja, COALESCE(alternative_names,'{}') AS alternative_names
		FROM staff WHERE sakugabooru_tag IS NULL ORDER BY staff_id`)
	if err != nil {
		return nil, err
	}
	defer staffRows.Close()

	var staffInputs []*pb.StaffMatchInput
	for staffRows.Next() {
		var id int32
		var nameEn, nameJa string
		var altNames []string
		if err := staffRows.Scan(&id, &nameEn, &nameJa, &altNames); err != nil {
			continue
		}
		staffInputs = append(staffInputs, &pb.StaffMatchInput{
			StaffId:          id,
			NameEn:           nameEn,
			NameJa:           nameJa,
			AlternativeNames: altNames,
		})
	}

	if len(animeInputs) == 0 && len(staffInputs) == 0 {
		return map[string]any{"animeChecked": 0, "animeFound": 0, "staffChecked": 0, "staffFound": 0}, nil
	}

	log.Printf("[sakugabooru] Phase 1: matching tags for %d anime + %d staff", len(animeInputs), len(staffInputs))

	// Call service directly.
	req := connect.NewRequest(&pb.MatchTagsRequest{
		Anime: animeInputs,
		Staff: staffInputs,
	})

	stream, err := h.sakugabooru.MatchTags(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("MatchTags failed: %w", err)
	}

	var animeMatches []*pb.AnimeMatch
	var staffMatches []*pb.StaffMatch
	for stream.Receive() {
		msg := stream.Msg()
		if r := msg.GetResults(); r != nil {
			animeMatches = append(animeMatches, r.AnimeMatches...)
			staffMatches = append(staffMatches, r.StaffMatches...)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("MatchTags stream error: %w", err)
	}

	// Apply anime matches.
	foundAnime := 0
	var aIDs []int32
	var aTags []string
	for _, m := range animeMatches {
		if m.Found && m.SakugabooruTag != "" {
			aIDs = append(aIDs, m.AnilistId)
			aTags = append(aTags, m.SakugabooruTag)
			foundAnime++
		}
	}
	if len(aIDs) > 0 {
		h.pg.Exec(ctx, `UPDATE anime SET sakugabooru_tag = data.tag
			FROM (SELECT unnest($1::int[]) AS anilist_id, unnest($2::text[]) AS tag) AS data
			WHERE anime.anilist_id = data.anilist_id AND anime.sakugabooru_tag IS NULL`, aIDs, aTags)
	}

	// Apply staff matches.
	foundStaff := 0
	var sIDs []int32
	var sTags []string
	for _, m := range staffMatches {
		if m.Found && m.SakugabooruTag != "" {
			sIDs = append(sIDs, m.StaffId)
			sTags = append(sTags, m.SakugabooruTag)
			foundStaff++
		}
	}
	if len(sIDs) > 0 {
		h.pg.Exec(ctx, `UPDATE staff SET sakugabooru_tag = data.tag
			FROM (SELECT unnest($1::int[]) AS staff_id, unnest($2::text[]) AS tag) AS data
			WHERE staff.staff_id = data.staff_id AND staff.sakugabooru_tag IS NULL`, sIDs, sTags)
	}

	return map[string]any{
		"animeChecked": len(animeInputs), "animeFound": foundAnime,
		"staffChecked": len(staffInputs), "staffFound": foundStaff,
	}, nil
}

func (h *Handler) runSakugabooruPostFetch(ctx context.Context, mangaFormats []string) (map[string]any, error) {
	animeRows, err := h.pg.Query(ctx,
		`SELECT anilist_id, sakugabooru_tag FROM anime
		 WHERE sakugabooru_tag IS NOT NULL AND type='ANIME'
		   AND format NOT IN ('MANGA','NOVEL','ONE_SHOT','MANHWA','MANHUA')`)
	if err != nil {
		return nil, err
	}
	defer animeRows.Close()

	var animeTags []*pb.AnimeTagInput
	for animeRows.Next() {
		var id int32
		var tag string
		if err := animeRows.Scan(&id, &tag); err == nil {
			animeTags = append(animeTags, &pb.AnimeTagInput{AnilistId: id, SakugabooruTag: tag})
		}
	}

	staffRows, err := h.pg.Query(ctx,
		`SELECT staff_id, sakugabooru_tag FROM staff WHERE sakugabooru_tag IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer staffRows.Close()

	var staffTags []*pb.StaffTagInput
	for staffRows.Next() {
		var id int32
		var tag string
		if err := staffRows.Scan(&id, &tag); err == nil {
			staffTags = append(staffTags, &pb.StaffTagInput{StaffId: id, SakugabooruTag: tag})
		}
	}

	if len(animeTags) == 0 && len(staffTags) == 0 {
		return map[string]any{"posts": 0, "animeLinks": 0, "staffLinks": 0}, nil
	}

	log.Printf("[sakugabooru] Phase 2: fetching posts for %d anime tags + %d staff tags", len(animeTags), len(staffTags))

	req := connect.NewRequest(&pb.FetchPostsRequest{
		AnimeTags: animeTags,
		StaffTags: staffTags,
	})

	stream, err := h.sakugabooru.FetchPosts(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("FetchPosts failed: %w", err)
	}

	var posts []*pb.SakugabooruPost
	var animeLinks []*pb.AnimePostLink
	var staffLinks []*pb.StaffPostLink
	for stream.Receive() {
		msg := stream.Msg()
		if r := msg.GetResults(); r != nil {
			posts = append(posts, r.Posts...)
			animeLinks = append(animeLinks, r.AnimeLinks...)
			staffLinks = append(staffLinks, r.StaffLinks...)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("FetchPosts stream error: %w", err)
	}

	// Upsert posts.
	if len(posts) > 0 {
		log.Printf("[sakugabooru] Importing %d posts", len(posts))
		for i := 0; i < len(posts); i += 1000 {
			end := i + 1000
			if end > len(posts) {
				end = len(posts)
			}
			batch := posts[i:end]
			var pIDs []int64
			var fileURLs, previewURLs, sources, ratings, fileExts []string
			for _, p := range batch {
				pIDs = append(pIDs, int64(p.PostId))
				fileURLs = append(fileURLs, p.FileUrl)
				previewURLs = append(previewURLs, p.PreviewUrl)
				sources = append(sources, p.Source)
				ratings = append(ratings, p.Rating)
				fileExts = append(fileExts, p.FileExt)
			}
			h.pg.Exec(ctx, `INSERT INTO sakugabooru_post (post_id, file_url, preview_url, source, rating, file_ext)
				SELECT * FROM unnest($1::bigint[], $2::text[], $3::text[], $4::text[], $5::text[], $6::text[])
				ON CONFLICT (post_id) DO UPDATE SET source=EXCLUDED.source, rating=EXCLUDED.rating, file_ext=EXCLUDED.file_ext`,
				pIDs, fileURLs, previewURLs, sources, ratings, fileExts)
		}
	}

	// Insert anime↔post links.
	if len(animeLinks) > 0 {
		log.Printf("[sakugabooru] Importing %d anime↔post links", len(animeLinks))
		var aIDs, pIDs []int64
		for _, l := range animeLinks {
			aIDs = append(aIDs, int64(l.AnilistId))
			pIDs = append(pIDs, int64(l.PostId))
		}
		h.pg.Exec(ctx, `INSERT INTO anime_sakugabooru_post (anime_id, post_id)
			SELECT a.id, data.post_id FROM (SELECT unnest($1::bigint[]) AS anilist_id, unnest($2::bigint[]) AS post_id) data
			JOIN anime a ON a.anilist_id = data.anilist_id
			ON CONFLICT DO NOTHING`, aIDs, pIDs)
	}

	// Insert staff↔post links.
	if len(staffLinks) > 0 {
		log.Printf("[sakugabooru] Importing %d staff↔post links", len(staffLinks))
		var sIDs, pIDs []int64
		for _, l := range staffLinks {
			sIDs = append(sIDs, int64(l.StaffId))
			pIDs = append(pIDs, int64(l.PostId))
		}
		h.pg.Exec(ctx, `INSERT INTO staff_sakugabooru_post (staff_id, post_id)
			SELECT s.id, data.post_id FROM (SELECT unnest($1::bigint[]) AS staff_id, unnest($2::bigint[]) AS post_id) data
			JOIN staff s ON s.staff_id = data.staff_id
			ON CONFLICT DO NOTHING`, sIDs, pIDs)
	}

	return map[string]any{"posts": len(posts), "animeLinks": len(animeLinks), "staffLinks": len(staffLinks)}, nil
}

// FetchStudioImages fetches studio images via the studio service.
func (h *Handler) FetchStudioImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get studios that need images.
	rows, err := h.pg.Query(ctx,
		`SELECT s.name, array_agg(a.mal_id ORDER BY a.popularity ASC NULLS LAST LIMIT 3) as mal_ids
		 FROM studio s
		 JOIN anime_studio ast ON ast.studio_id = s.id
		 JOIN anime a ON a.id = ast.anime_id AND a.mal_id IS NOT NULL
		 WHERE s.image_url IS NULL
		 GROUP BY s.name`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var studioInputs []*pb.StudioImageInput
	for rows.Next() {
		var name string
		var malIDs []int32
		if err := rows.Scan(&name, &malIDs); err == nil {
			studioInputs = append(studioInputs, &pb.StudioImageInput{
				StudioName:  name,
				MalAnimeIds: malIDs,
			})
		}
	}

	total := len(studioInputs)
	if total == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "All studios already have images.", "total": 0})
		return
	}

	log.Printf("[fetch-studio-images] Starting for %d studios", total)

	go func() {
		bgCtx := context.Background()
		req := connect.NewRequest(&pb.FetchStudioImagesRequest{Studios: studioInputs})
		stream, err := h.studio.FetchStudioImages(bgCtx, req)
		if err != nil {
			log.Printf("[fetch-studio-images] Service call failed: %v", err)
			return
		}

		updated := 0
		for stream.Receive() {
			msg := stream.Msg()
			if r := msg.GetResults(); r != nil {
				for _, res := range r.Entries {
					if res.ImageUrl != "" {
						_, err := h.pg.Exec(bgCtx,
							`UPDATE studio SET image_url = $1, description = $2 WHERE name = $3`,
							res.ImageUrl, res.Description, res.StudioName)
						if err == nil {
							updated++
						}
					}
				}
			}
		}
		log.Printf("[fetch-studio-images] Done. Updated %d studios.", updated)
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Studio image fetch started for %d studios. Monitor logs.", total),
		"total":     total,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// EnrichWikidataStudios enriches studios with Wikidata QIDs.
func (h *Handler) EnrichWikidataStudios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pg.Query(ctx,
		`SELECT id, name FROM studio WHERE wikidata_qid IS NULL ORDER BY id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
	}

	log.Printf("[enrich-wikidata-studios] Starting for %d studios", count)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Studio Wikidata enrichment started for %d studios. Monitor logs.", count),
		"total":     count,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// BackfillMalIDs backfills MAL IDs using the enrichment service.
func (h *Handler) BackfillMalIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pg.Query(ctx,
		`SELECT anilist_id FROM anime WHERE mal_id IS NULL ORDER BY anilist_id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var ids []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	total := len(ids)
	if total == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "All anime have MAL IDs.", "total": 0})
		return
	}

	log.Printf("[backfill-mal-ids] Starting for %d anime", total)

	go func() {
		bgCtx := context.Background()
		req := connect.NewRequest(&pb.BackfillMalIdsRequest{AnilistIds: ids})
		resp, err := h.enrichment.BackfillMalIds(bgCtx, req)
		if err != nil {
			log.Printf("[backfill-mal-ids] Service call failed: %v", err)
			return
		}

		mappings := resp.Msg.GetMappings()
		if len(mappings) > 0 {
			aIDs := make([]int32, len(mappings))
			mIDs := make([]int32, len(mappings))
			for i, m := range mappings {
				aIDs[i] = m.AnilistId
				mIDs[i] = m.MalId
			}
			h.pg.Exec(bgCtx,
				`UPDATE anime SET mal_id = data.mal_id
				 FROM (SELECT unnest($1::int[]) AS anilist_id, unnest($2::int[]) AS mal_id) AS data
				 WHERE anime.anilist_id = data.anilist_id AND anime.mal_id IS NULL`,
				aIDs, mIDs)
		}
		log.Printf("[backfill-mal-ids] Done. Mapped %d/%d anime.", len(mappings), total)
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("MAL ID backfill started for %d anime. Monitor logs.", total),
		"total":     total,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// BackfillWikipedia backfills Wikipedia URLs for anime with Wikidata QIDs.
func (h *Handler) BackfillWikipedia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var count int64
	h.pg.QueryRow(ctx,
		`SELECT COUNT(*) FROM anime WHERE wikidata_qid IS NOT NULL AND wikipedia_en IS NULL`).Scan(&count)

	log.Printf("[backfill-wikipedia] Starting for %d anime", count)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Wikipedia backfill started for %d anime. Monitor logs.", count),
		"total":     count,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// TestScraperPath is a diagnostic endpoint for scraper binary paths.
func (h *Handler) TestScraperPath(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]any{
		"paths": map[string]any{
			"note": "Go server handles all scraping in-process via ConnectRPC services",
		},
		"services": map[string]any{
			"scraper":         h.scraper != nil,
			"enrichment":      h.enrichment != nil,
			"sakugabooru":     h.sakugabooru != nil,
			"studio":          h.studio != nil,
			"preprocessor":    h.preprocessor != nil,
			"recommendations": h.recommendations != nil,
		},
	})
}

// quoteAll wraps each string in quotes for PostgreSQL array literal.
func quoteAll(strs []string) []string {
	out := make([]string, len(strs))
	for i, s := range strs {
		out[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `""`) + `"`
	}
	return out
}

func ifStr(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}

func ifAny(cond bool, t string, f int64) any {
	if cond {
		return t
	}
	return f
}

// csvEscape escapes a value for CSV output.
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// unused but kept for reference:
var _ = csv.NewReader
