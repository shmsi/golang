package main

import (
	"context"
	"errors"
)

// ErrNotFound is returned by a Store when a code does not exist.
var ErrNotFound = errors.New("code not found")

// Store is what the handlers need from storage: SQLite, a cache, or a fake in tests.
type Store interface {
	Get(ctx context.Context, code string) (string, error)
	Put(ctx context.Context, longURL string) (code string, existed bool, err error)
	Stats(ctx context.Context, code string) (URLStats, error)
}

type URLStats struct {
	Code   string `json:"code"`
	URL    string `json:"url"`
	Clicks int    `json:"clicks"`
}
