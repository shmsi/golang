package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const urlPrefix = "http://localhost:8080" // could be in env var

// clickRecorder is the one thing the handlers need for counting clicks.
type clickRecorder interface {
	RecordClick(code string)
}

type server struct {
	store  Store
	clicks clickRecorder
}

type CreateRequest struct {
	URL string `json:"url"`
}

type CreateResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	stats, err := s.store.Stats(ctx, code)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("get stats", "code", code, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)

}

func (s *server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	target, err := s.store.Get(ctx, code)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("get link", "code", code, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.clicks.RecordClick(code)
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	ctx := r.Context()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if !validURL(req.URL) {
		http.Error(w, "url must be an absolute http or https URL", http.StatusBadRequest)
		return
	}

	code, existed, err := s.store.Put(ctx, req.URL)
	if err != nil {
		slog.Error("create link", "url", req.URL, "err", err)
		http.Error(w, "could not shorten url", http.StatusInternalServerError)
		return
	}

	short, err := shortURL(code)
	if err != nil {
		slog.Error("build short url", "code", code, "err", err)
		http.Error(w, "could not shorten url", http.StatusInternalServerError)
		return
	}

	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(CreateResponse{Code: code, ShortURL: short})
}

// validURL accepts only absolute http(s) URLs with a host, so the server
// never redirects visitors to javascript:, file: or similar.
func validURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func shortURL(code string) (string, error) {
	return url.JoinPath(urlPrefix, code)
}
