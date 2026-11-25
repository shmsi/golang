package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"
)

const (
	alphabet    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
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
const SelectStats = `SELECT long_url, clicks FROM links WHERE code = ?`

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
	return longURL, nil
}

func (d *DBStore) RecordClick(code string) {
	select {
	case d.clickChan <- code:
	default:
		slog.Warn("click dropped, buffer full", "code", code)
	}
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
				slog.Info("final flush done")
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
		slog.Error("flush clicks", "err", err, "codes", len(counts))
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

func (d *DBStore) Stats(ctx context.Context, code string) (URLStats, error) {
	var urlStat URLStats
	err := d.db.QueryRowContext(ctx, SelectStats, code).Scan(&urlStat.URL, &urlStat.Clicks)
	if errors.Is(err, sql.ErrNoRows) {
		return URLStats{}, ErrNotFound
	}
	if err != nil {
		return URLStats{}, fmt.Errorf("get %q: %w", code, err)
	}
	urlStat.Code = code
	return urlStat, nil

}

func randomCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(b)
}
