package main

// Backfill MAL IDs from two public GitHub-hosted JSON databases:
//
//  1. anime-offline-database (AOD) — manami-project (~40k entries, primary)
//  2. arm — kawaiioverflow (~35k entries, fills gaps)
//
// No API keys, no proxies, no rate limiting. Just two HTTP GETs.
//
// Reads anilist_ids from stdin (one per line), downloads both JSON files,
// builds anilist_id → mal_id mapping, and outputs CSV to stdout.
//
// Usage:
//
//	echo "1" | ./backfill_mal_ids
//	psql "$DATABASE_URL" -t -A -c "SELECT anilist_id FROM anime WHERE mal_id IS NULL" \
//	  | ./backfill_mal_ids > results.csv

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	aodReleasesAPI = "https://api.github.com/repos/manami-project/anime-offline-database/releases/latest"
	aodAssetName   = "anime-offline-database-minified.json"
	armURL         = "https://raw.githubusercontent.com/kawaiioverflow/arm/master/arm.json"
)

// AOD structs
type aodDatabase struct {
	Data []aodEntry `json:"data"`
}

type aodEntry struct {
	Sources []string `json:"sources"`
}

// ARM struct
type armEntry struct {
	MalID    *int `json:"mal_id"`
	AnilistID *int `json:"anilist_id"`
}

var (
	anilistRe = regexp.MustCompile(`https://anilist\.co/anime/(\d+)`)
	malRe     = regexp.MustCompile(`https://myanimelist\.net/anime/(\d+)`)
)

func downloadJSON(url string) ([]byte, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// resolveAODURL fetches the latest release from GitHub and finds the download
// URL for anime-offline-database-minified.json.
func resolveAODURL() (string, error) {
	type ghAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	type ghRelease struct {
		TagName string    `json:"tag_name"`
		Assets  []ghAsset `json:"assets"`
	}

	data, err := downloadJSON(aodReleasesAPI)
	if err != nil {
		return "", fmt.Errorf("fetch AOD releases: %w", err)
	}
	var rel ghRelease
	if err := json.Unmarshal(data, &rel); err != nil {
		return "", fmt.Errorf("parse AOD releases: %w", err)
	}
	for _, a := range rel.Assets {
		if a.Name == aodAssetName {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("asset %q not found in release %s", aodAssetName, rel.TagName)
}

func buildMapFromAOD(data []byte) (map[int]int, error) {
	var db aodDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("parse AOD: %w", err)
	}

	m := make(map[int]int)
	for _, entry := range db.Data {
		var anilistID, malID int
		var hasAnilist, hasMal bool
		for _, src := range entry.Sources {
			if match := anilistRe.FindStringSubmatch(src); match != nil {
				anilistID, _ = strconv.Atoi(match[1])
				hasAnilist = true
			}
			if match := malRe.FindStringSubmatch(src); match != nil {
				malID, _ = strconv.Atoi(match[1])
				hasMal = true
			}
		}
		if hasAnilist && hasMal && anilistID > 0 && malID > 0 {
			m[anilistID] = malID
		}
	}
	return m, nil
}

func buildMapFromARM(data []byte) (map[int]int, error) {
	var entries []armEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse ARM: %w", err)
	}

	m := make(map[int]int)
	for _, e := range entries {
		if e.AnilistID != nil && e.MalID != nil && *e.AnilistID > 0 && *e.MalID > 0 {
			m[*e.AnilistID] = *e.MalID
		}
	}
	return m, nil
}

func main() {
	// Read anilist_ids from stdin.
	var ids []int
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "anilist_id" {
			continue
		}
		id, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stdin read error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anilist_ids to look up\n", len(ids))

	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "Nothing to do.")
		os.Exit(0)
	}

	// Resolve AOD release URL.
	fmt.Fprintln(os.Stderr, "Resolving latest AOD release...")
	aodURL, err := resolveAODURL()
	if err != nil {
		fmt.Fprintf(os.Stderr, "AOD URL resolve failed: %v\n", err)
		os.Exit(1)
	}

	// Download AOD.
	fmt.Fprintf(os.Stderr, "Downloading AOD from %s ...\n", aodURL)
	aodData, err := downloadJSON(aodURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AOD download failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "AOD downloaded (%d bytes)\n", len(aodData))

	aodMap, err := buildMapFromAOD(aodData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AOD parse failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "AOD: %d anilist→mal mappings\n", len(aodMap))

	// Download ARM.
	fmt.Fprintf(os.Stderr, "Downloading ARM from %s ...\n", armURL)
	armData, err := downloadJSON(armURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ARM download failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ARM downloaded (%d bytes)\n", len(armData))

	armMap, err := buildMapFromARM(armData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ARM parse failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ARM: %d anilist→mal mappings\n", len(armMap))

	// Merge: AOD first, ARM fills gaps.
	merged := make(map[int]int)
	for k, v := range armMap {
		merged[k] = v
	}
	for k, v := range aodMap {
		merged[k] = v // AOD overwrites ARM
	}
	fmt.Fprintf(os.Stderr, "Merged: %d total anilist→mal mappings\n\n", len(merged))

	// Look up each requested ID and write CSV.
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"anilist_id", "mal_id"})

	var found, missing int
	for _, id := range ids {
		if malID, ok := merged[id]; ok {
			w.Write([]string{strconv.Itoa(id), strconv.Itoa(malID)})
			found++
		} else {
			missing++
		}
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "Done: found=%d missing=%d total=%d\n", found, missing, found+missing)
}
