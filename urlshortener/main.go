package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	alphabet    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	urlPrefix   = "http://localhost:8080" // could be in env var
	codeLength  = 5
	maxAttempts = 10 // tries to find a free code before giving up
)

const CreateTableQuery = `
	CREATE TABLE IF NOT EXISTS links (
	     code TEXT PRIMARY KEY,
	     long_url TEXT NOT NULL UNIQUE,
	     clicks INTEGER NOT NULL DEFAULT 0,
	     created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`
const SelectByCode = `SELECT long_url FROM links WHERE code = ?`
const SelectByURL = `SELECT code FROM links WHERE long_url = ?`
const InsertLink = `INSERT INTO links(code, long_url) VALUES(?, ?) ON CONFLICT DO NOTHING`
const UpdateClicks = `UPDATE links SET clicks = clicks + ? where code = ?`

var ErrNotFound = errors.New("code not found")

type Store interface {
	Get(ctx context.Context, code string) (string, error)
	Put(ctx context.Context, longURL string) (code string, existed bool, err error)
}

type DBStore struct {
	db        *sql.DB
	clickChan chan string
}

// Compile-time check: the build fails if memoryStore stops satisfying Store.
var _ Store = (*DBStore)(nil)

func newDBStore(db *sql.DB) *DBStore {
	c := make(chan string, 1024)
	return &DBStore{db: db, clickChan: c}
}

func (d *DBStore) Get(ctx context.Context, code string) (string, error) {

	var longURL string
	err := d.db.QueryRowContext(ctx, SelectByCode, code).Scan(&longURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get %q: %w", code, err)
	}
	select {
	case d.clickChan <- code:
	default:
		log.Println("Buffer full - dropped the click")
	}
	return longURL, nil
}

// AddClicks counts clicks from clickChan and writes them to the database
// every 5 seconds. It returns after clickChan is closed, with a final flush.
func (d *DBStore) AddClicks() {
	counts := map[string]int{}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case code, ok := <-d.clickChan:
			if !ok { // closed and empty: every buffered click has been counted
				d.flush(counts)
				log.Println("final flush done")
				return
			}
			counts[code]++
		case <-ticker.C:
			d.flush(counts)
		}
	}
}

// flush writes counts to the database and clears them. On failure it keeps
// them, so the next flush retries.
func (d *DBStore) flush(counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.addClicks(ctx, counts); err != nil {
		log.Printf("flush clicks: %v", err)
		return
	}
	clear(counts)
}

func (d *DBStore) addClicks(ctx context.Context, counts map[string]int) error {
	query, args := batchUpdateQuery(counts)
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func batchUpdateQuery(counts map[string]int) (string, []any) {
	rows := make([]string, 0, len(counts))
	args := make([]any, 0, 2*len(counts))
	for code, n := range counts {
		rows = append(rows, "(?, ?)")
		args = append(args, code, n)
	}
	q := `UPDATE links SET clicks = clicks + v.column2
	      FROM (VALUES ` + strings.Join(rows, ", ") + `) AS v
	      WHERE links.code = v.column1`

	fmt.Println(q)
	return q, args
}

func (d *DBStore) Put(ctx context.Context, longURL string) (string, bool, error) {
	for range maxAttempts {
		code := randomCode(codeLength)
		res, err := d.db.ExecContext(ctx, InsertLink, code, longURL)
		if err != nil {
			return "", false, fmt.Errorf("insert link: %w", err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return "", false, fmt.Errorf("insert link: %w", err)
		}
		if n == 1 {
			return code, false, nil // new link
		}
		var existing string
		err = d.db.QueryRowContext(ctx, SelectByURL, longURL).Scan(&existing)
		if err == nil {
			return existing, true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("look up existing link: %w", err)
		}
		// sql.ErrNoRows: code collision, loop and try a new code
	}
	return "", false, errors.New("could not find a free code")
}

type server struct {
	store Store
}

type CreateRequest struct {
	URL string `json:"url"`
}

type CreateResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
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
		log.Printf("get %q: %v", code, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
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
		log.Printf("put %q: %v", req.URL, err)
		http.Error(w, "could not shorten url", http.StatusInternalServerError)
		return
	}

	short, err := shortURL(code)
	if err != nil {
		log.Printf("build short url for %q: %v", code, err)
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

func main() {
	db, err := sql.Open("sqlite", "file:test.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), CreateTableQuery); err != nil {
		log.Fatal(err)
	}

	dbStore := newDBStore(db)
	s := &server{store: dbStore}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", s.handleRedirect)
	mux.HandleFunc("POST /links", s.handleCreate)

	counterDone := make(chan struct{})
	go func() {
		defer close(counterDone)
		dbStore.AddClicks()
	}()

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	log.Println("Listening on :8080")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	log.Println("shutting down")

	// 1. Stop the server: returns once every in-flight request has finished.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Timed out: handlers may still be sending clicks, so closing the
		// channel now could panic. Give up on the final flush instead.
		log.Printf("shutdown: %v", err)
		return
	}

	// 2. No handler can send anymore, so it is safe to close the channel.
	close(dbStore.clickChan)

	// 3. Wait for the final flush. The deferred db.Close runs after this.
	<-counterDone
}
