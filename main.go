// main.go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	// "strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

// Structures match the JSON from the frontend.
type Answer struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Value   *string `json:"value"`   // can be null
	Comment *string `json:"comment"` // can be null
}

type Checklist struct {
	ChildName  *string  `json:"childName"`
	Date       *string  `json:"date"` // expected YYYY-MM-DD or omitted
	Specialist *string  `json:"specialist"`
	CreatedAt  *string  `json:"createdAt"`
	Answers    []Answer `json:"answers"`
}

var db *sql.DB

func main() {
	// Read DSN from env
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		// Example: export PG_DSN="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
		log.Fatal("PG_DSN environment variable is required (e.g. postgres://user:pass@localhost:5432/dbname?sslmode=disable)")
	}

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Set reasonable connection pool limits
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 30)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	// Create tables if not exist
	if err := prepareSchema(db); err != nil {
		log.Fatalf("failed to prepare schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/checklist", checklistHandler)

	srv := &http.Server{
		Addr:         ":8081",
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		log.Println("shutdown signal received, shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Println("server listening on :8080")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http server error: %v", err)
	}

	<-idleConnsClosed
	log.Println("server stopped")
}

// loggingMiddleware - simple request logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// checklistHandler handles POST /api/checklist
func checklistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var in Checklist
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return
	}

	// Basic validation: at least one answer provided
	if len(in.Answers) == 0 {
		http.Error(w, "answers must be provided", http.StatusBadRequest)
		return
	}

	// Normalize date: try to parse provided date or set today if missing
	var date sql.NullTime
	if in.Date != nil && strings.TrimSpace(*in.Date) != "" {
		// accept YYYY-MM-DD
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(*in.Date)); err == nil {
			date = sql.NullTime{Time: t, Valid: true}
		} else {
			// try RFC3339
			if t2, err2 := time.Parse(time.RFC3339, strings.TrimSpace(*in.Date)); err2 == nil {
				date = sql.NullTime{Time: t2, Valid: true}
			} else {
				http.Error(w, "date must be YYYY-MM-DD or RFC3339", http.StatusBadRequest)
				return
			}
		}
	} else {
		// default to today (date only)
		t := time.Now().Truncate(24 * time.Hour)
		date = sql.NullTime{Time: t, Valid: true}
	}

	// parse createdAt if provided
	var createdAt time.Time
	if in.CreatedAt != nil && *in.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, *in.CreatedAt); err == nil {
			createdAt = t
		} else {
			createdAt = time.Now().UTC()
		}
	} else {
		createdAt = time.Now().UTC()
	}

	// Save to DB in transaction
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, "failed to begin tx", http.StatusInternalServerError)
		log.Printf("begin tx error: %v", err)
		return
	}
	defer func() {
		// if still pending, rollback
		_ = tx.Rollback()
	}()

	var checklistID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO checklists (child_name, date_of_check, specialist, created_at)
         VALUES ($1, $2, $3, $4) RETURNING id`,
		nullStringPtr(in.ChildName), nullTime(date), nullStringPtr(in.Specialist), createdAt).Scan(&checklistID)
	if err != nil {
		http.Error(w, "failed to insert checklist", http.StatusInternalServerError)
		log.Printf("insert checklist error: %v", err)
		return
	}

	// Insert answers
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO answers (checklist_id, key_name, label, value, comment) VALUES ($1,$2,$3,$4,$5)`)
	if err != nil {
		http.Error(w, "failed to prepare answer insert", http.StatusInternalServerError)
		log.Printf("prepare answer insert: %v", err)
		return
	}
	defer stmt.Close()

	for i := range in.Answers {
		a := in.Answers[i]
		_, err := stmt.ExecContext(ctx, checklistID, a.Key, a.Label, a.Value, a.Comment)
		if err != nil {
			http.Error(w, "failed to insert answers", http.StatusInternalServerError)
			log.Printf("insert answer %v error: %v", a, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		log.Printf("commit error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	resp := map[string]interface{}{"id": checklistID}
	_ = json.NewEncoder(w).Encode(resp)
}

// prepareSchema creates tables if they do not exist.
func prepareSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS checklists (
  id BIGSERIAL PRIMARY KEY,
  child_name TEXT,
  date_of_check DATE,
  specialist TEXT,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS answers (
  id BIGSERIAL PRIMARY KEY,
  checklist_id BIGINT NOT NULL REFERENCES checklists(id) ON DELETE CASCADE,
  key_name TEXT NOT NULL,
  label TEXT,
  value TEXT,
  comment TEXT
);

CREATE INDEX IF NOT EXISTS idx_answers_checklist ON answers(checklist_id);
`
	_, err := db.Exec(schema)
	return err
}

// helpers for null handling
func nullStringPtr(s *string) interface{} {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return strings.TrimSpace(*s)
}

func nullTime(t sql.NullTime) interface{} {
	if t.Valid {
		// store date only (without time) as date column accepts time.Time as date
		return t.Time
	}
	return nil
}
