package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrDuplicateShortCode = errors.New("short code already exists")

type URL struct {
	ShortCode   string
	OriginalURL string
	ExpiresAt   sql.NullTime
	CreatedAt   time.Time
}

type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Insert(ctx context.Context, short, orig string, exp *time.Time) error {
	var expires sql.NullTime
	if exp != nil {
		expires = sql.NullTime{Time: *exp, Valid: true}
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO urls (short_code, original_url, expires_at)
     VALUES (?, ?, ?)`,
		short, orig, expires,
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrDuplicateShortCode
	}
	return err
}

func (s *Store) URLInfo(ctx context.Context, url string) (URL, error) {
	var u URL
	row := s.db.QueryRowContext(ctx,
		`SELECT short_code, original_url, expires_at, created_at
     FROM urls WHERE original_url = ?`, url)

	err := row.Scan(&u.ShortCode, &u.OriginalURL, &u.ExpiresAt, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return URL{}, sql.ErrNoRows
	}
	return u, err
}

func (s *Store) Get(ctx context.Context, short string) (URL, error) {
	var u URL
	row := s.db.QueryRowContext(ctx,
		`SELECT short_code, original_url, expires_at, created_at
     FROM urls WHERE short_code = ?`, short)
	err := row.Scan(&u.ShortCode, &u.OriginalURL, &u.ExpiresAt, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return URL{}, sql.ErrNoRows
	}
	return u, err
}

func (s *Store) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM urls WHERE expires_at IS NOT NULL AND expires_at < ?`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// InitDB opens an SQLite database connection and creates the schema if needed.
func InitDB(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS urls (
		short_code   TEXT PRIMARY KEY UNIQUE,
		original_url TEXT NOT NULL,
		expires_at   DATETIME,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_expires_at ON urls(expires_at);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return db, nil
}
