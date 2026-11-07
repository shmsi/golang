package main

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const urlPrefix = "http://localhost:8080" // could be in env var
type server struct {
	mu    sync.Mutex
	links map[string]string
}

// curl -X POST localhost:8080/links -d '{"url":"https://go.dev/doc/effective_go"}'
// {"code":"aZ3kQ9","short_url":"http://localhost:8080/aZ3kQ9"}
//
// curl -i localhost:8080/aZ3kQ9
// HTTP/1.1 302 Found
// Location: https://go.dev/doc/effective_gotype RedirectRequest struct {
//
//
// type RedirectRequest struct {
// 	Code string `json:"code"`
// }
// type RedirectResponse struct {
// 	// not sure what should be here
// }

type CreateRequest struct {
	URL string `json:"url"`
}

type CreateResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (s *server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	s.mu.Lock()
	url, found := s.links[code]
	s.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func randomCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(b)
}

func shortURL(code string) (string, error) {
	return url.JoinPath(urlPrefix, code)
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	code := randomCode(5)
	s.mu.Lock()
	s.links[code] = req.URL
	s.mu.Unlock()

	srtURL, err := shortURL(code)
	if err != nil {
		http.Error(w, "Could not shorten url", http.StatusInternalServerError)
	}
	resp := CreateResponse{Code: code, ShortURL: srtURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)

}

func main() {
	s := &server{links: make(map[string]string)}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{code}", s.handleRedirect)

	mux.HandleFunc("POST /links", s.handleCreate)

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))

}
