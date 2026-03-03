package main

// sakugabooru_tags.go
//
// Downloads ALL Sakugabooru tags in one shot (limit=0), then writes a single
// JSON file with tags categorized by type.
//
// Output structure:
//   { "general": [...], "artist": [...], "copyright": [...], "character": [...], "meta": [...] }
//
// Usage:
//
//	./sakugabooru_tags -out sakugabooru_tags.json

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	tagsBase  = "https://www.sakugabooru.com"
	tagsAgent = "AniGraph-TagDownload/1.0 (anigraph.xyz)"
)

var tagTypeNames = map[int]string{
	0: "general",
	1: "artist",
	3: "copyright",
	4: "terminology",
	5: "meta",
}

type tagEntry struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Type      int    `json:"type"`
	Ambiguous bool   `json:"ambiguous"`
}

func fetchAllTags() ([]tagEntry, error) {
	url := tagsBase + "/tag.json?limit=0"

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", tagsAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var tags []tagEntry
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func main() {
	outFile := flag.String("out", "sakugabooru_tags.json", "Output JSON file")
	flag.Parse()

	fmt.Fprintln(os.Stderr, "Fetching all tags from Sakugabooru ...")

	tags, err := fetchAllTags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Downloaded %d total tags\n", len(tags))

	categorized := make(map[string][]tagEntry)
	for _, name := range tagTypeNames {
		categorized[name] = []tagEntry{}
	}

	for _, t := range tags {
		name, ok := tagTypeNames[t.Type]
		if !ok {
			name = fmt.Sprintf("unknown_%d", t.Type)
			if categorized[name] == nil {
				categorized[name] = []tagEntry{}
			}
		}
		categorized[name] = append(categorized[name], t)
	}

	for name, list := range categorized {
		fmt.Fprintf(os.Stderr, "  %-12s %d tags\n", name, len(list))
	}

	f, err := os.Create(*outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", *outFile, err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(categorized); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Wrote %s\n", *outFile)
}
