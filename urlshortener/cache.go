package main

import (
	"context"
	"log/slog"
	"sync"
)

type cachedStore struct {
	next       Store
	mu         sync.RWMutex
	urls       map[string]string
	urlsToCode map[string]string
}

var _ Store = (*cachedStore)(nil)

func newCachedStore(next Store) *cachedStore {
	urls := make(map[string]string)
	urlsToCode := make(map[string]string)
	return &cachedStore{next: next, urls: urls, urlsToCode: urlsToCode}
}

func (c *cachedStore) Get(ctx context.Context, code string) (string, error) {
	c.mu.RLock()
	url, ok := c.urls[code]
	c.mu.RUnlock()

	if ok {
		slog.DebugContext(ctx, "cache hit", "op", "get", "code", code)
		return url, nil
	}

	url, err := c.next.Get(ctx, code)

	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.urls[code] = url
	c.urlsToCode[url] = code
	c.mu.Unlock()

	return url, nil

}

func (c *cachedStore) Put(ctx context.Context, longURL string) (code string, existed bool, err error) {
	c.mu.RLock()
	code, exists := c.urlsToCode[longURL]
	c.mu.RUnlock()
	if exists {
		slog.DebugContext(ctx, "cache hit", "op", "put", "code", code)
		return code, true, nil
	}
	code, existed, err = c.next.Put(ctx, longURL)
	if err != nil {
		return "", false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.urls[code] = longURL
	c.urlsToCode[longURL] = code

	return code, existed, err
}

func (c *cachedStore) Stats(ctx context.Context, code string) (URLStats, error) {
	return c.next.Stats(ctx, code)
}
