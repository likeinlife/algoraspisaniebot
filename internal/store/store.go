package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type PollAnswer struct {
	PollID    string
	UserID    int64
	Option    int
	UpdatedAt time.Time
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("sqlite path is empty")
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure database: %w", err)
	}
	if err := migrate(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func migrate(ctx context.Context, db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS polls (
    poll_id TEXT PRIMARY KEY,
    chat_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS poll_answers (
    poll_id TEXT NOT NULL REFERENCES polls(poll_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    option_id INTEGER NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (poll_id, user_id)
);`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

func (s *Store) ChatID(ctx context.Context) (int64, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'chat_id'`).Scan(&raw)
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES('chat_id', ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprint(id))
	return err
}

func (s *Store) SavePoll(ctx context.Context, pollID string, chatID, messageID int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO polls(poll_id, chat_id, message_id, created_at)
VALUES(?, ?, ?, ?)`, pollID, chatID, messageID, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) SavePollAnswer(ctx context.Context, answer PollAnswer) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO poll_answers(poll_id, user_id, option_id, updated_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(poll_id, user_id) DO UPDATE SET option_id = excluded.option_id, updated_at = excluded.updated_at`,
		answer.PollID, answer.UserID, answer.Option, answer.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) RemovePollAnswer(ctx context.Context, pollID string, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM poll_answers WHERE poll_id = ? AND user_id = ?`, pollID, userID)
	return err
}
