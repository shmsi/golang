package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	db, err := sql.Open("sqlite", "file:test.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), CreateTableQuery); err != nil {
		log.Fatal(err)
	}

	dbStore := newDBStore(db)

	cache := newCachedStore(dbStore)
	s := &server{store: cache, clicks: dbStore}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", s.handleRedirect)
	mux.HandleFunc("GET /links/{code}/stats", s.handleStats)
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
	slog.Info("listening", "addr", ":8080")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	slog.Info("shutting down")

	// 1. Stop the server: returns once every in-flight request has finished.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Timed out: handlers may still be sending clicks, so closing the
		// channel now could panic. Give up on the final flush instead.
		slog.Error("shutdown", "err", err)
		return
	}

	// 2. No handler can send anymore, so it is safe to close the channel.
	close(dbStore.clickChan)

	// 3. Wait for the final flush. The deferred db.Close runs after this.
	<-counterDone
}
