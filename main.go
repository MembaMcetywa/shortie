package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

func main() {
	port := getenv("PORT", "8080")
	baseURL := getenv("BASE_URL", "http://localhost:"+port)

	store := &memStore{
		m: make(map[string]string),
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintln(w, "Welcome to Shortie, Shortie!")
	})

	mux.HandleFunc("/shorten", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			bad(w, "invalid json")
			return
		}
		if !validURL(req.URL) {
			bad(w, "invalid url (must start with http or https)")
			return
		}

		var code string
		var err error
		for range 5 {
			code, err = newCode(7)
			if err != nil {
				srvErr(w)
				return
			}
			if !store.exists(code) {
				break
			}
			code = ""
		}
		if code == "" {
			srvErr(w)
			return
		}

		store.save(code, req.URL)

		writeJSON(w, http.StatusCreated, map[string]string{
			"code":     code,
			"shortUrl": baseURL + "/" + code,
		})
	})
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

/* ---------- helpers ---------- */

type memStore struct {
	mu sync.RWMutex
	m  map[string]string
}

func (s *memStore) save(code, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[code] = target
}

func (s *memStore) exists(code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[code]
	return ok
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func bad(w http.ResponseWriter, msg string) {
	http.Error(w, `{"error":"`+msg+`"}`, http.StatusBadRequest)
}

func srvErr(w http.ResponseWriter) {
	http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
}

func validURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func newCode(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range n {
		x, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[x.Int64()]
	}
	return string(b), nil
}
