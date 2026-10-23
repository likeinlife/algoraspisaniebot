package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"algobot/internal/store/sqlcgen"
)

type Store struct {
	db      *sqlx.DB
	queries *sqlcgen.Queries
}

//go:embed schema.sql
var schema string

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("sqlite path is empty")
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;`); err != nil {
		return nil, closeAfterFailure(db, fmt.Errorf("configure database: %w", err))
	}
	if err := migrate(context.Background(), db); err != nil {
		return nil, closeAfterFailure(db, err)
	}
	return &Store{db: db, queries: sqlcgen.New(db)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func closeAfterFailure(db *sqlx.DB, cause error) error {
	if err := db.Close(); err != nil {
		return errors.Join(cause, fmt.Errorf("close database: %w", err))
	}
	return cause
}

func migrate(ctx context.Context, db *sqlx.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

func (s *Store) ChatID(ctx context.Context) (int64, error) {
	raw, err := s.queries.GetSetting(ctx, "chat_id")
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var id int64
	_, err = fmt.Sscan(raw, &id)
	return id, err
}

func (s *Store) SetChatID(ctx context.Context, id int64) error {
	return s.queries.SetSetting(ctx, sqlcgen.SetSettingParams{Key: "chat_id", Value: fmt.Sprint(id)})
}
