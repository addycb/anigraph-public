package httputil

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

// Error writes an error JSON response.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]any{
		"success": false,
		"message": msg,
	})
}

// QueryInt parses an integer query parameter with a default value.
func QueryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// QueryIntPtr parses an optional integer query parameter, returns nil if absent.
func QueryIntPtr(r *http.Request, key string) *int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

// QueryString returns a query parameter or the default.
func QueryString(r *http.Request, key, def string) string {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	return s
}

// QueryBool returns true if query param is "true".
func QueryBool(r *http.Request, key string) bool {
	return r.URL.Query().Get(key) == "true"
}

// QueryCSV splits a comma-separated query parameter, filtering empty strings.
func QueryCSV(r *http.Request, key string) []string {
	s := r.URL.Query().Get(key)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// QueryCSVInts splits a comma-separated query parameter into ints.
func QueryCSVInts(r *http.Request, key string) []int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.Atoi(p)
		if err == nil && v > 0 {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// QueryStringSlice returns all values for a query parameter (handles both
// repeated params and comma-separated).
func QueryStringSlice(r *http.Request, key string) []string {
	values := r.URL.Query()[key]
	if len(values) == 0 {
		return nil
	}
	var out []string
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
