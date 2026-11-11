package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	dsn, err := dsn(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func dsn(path string) (string, error) {
	if path == ":memory:" {
		return "file::memory:?_pragma=foreign_keys(1)", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	u := url.URL{
		Scheme:   "file",
		Path:     "/" + filepath.ToSlash(abs),
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
	}
	return u.String(), nil
}

func MigrateContent(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sentences (
			id INTEGER PRIMARY KEY,
			lang TEXT NOT NULL,
			text TEXT NOT NULL,
			token_count INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sentences_lang ON sentences(lang)`,
		`CREATE TABLE IF NOT EXISTS links (
			sentence_id INTEGER NOT NULL,
			translation_id INTEGER NOT NULL,
			PRIMARY KEY (sentence_id, translation_id)
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY,
			lang TEXT NOT NULL,
			text TEXT NOT NULL,
			normalized TEXT NOT NULL,
			frequency INTEGER NOT NULL,
			band INTEGER NOT NULL,
			token_count INTEGER NOT NULL,
			UNIQUE(lang, normalized)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_pick ON items(lang, band, token_count)`,
		`CREATE TABLE IF NOT EXISTS item_answers (
			item_id INTEGER NOT NULL,
			answer_lang TEXT NOT NULL,
			answer_text TEXT NOT NULL,
			normalized TEXT NOT NULL,
			UNIQUE(item_id, answer_lang, normalized)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_item_answers_item ON item_answers(item_id)`,
	}
	return execAll(db, stmts)
}

func MigrateProfile(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY,
			mode TEXT NOT NULL,
			activity TEXT NOT NULL,
			question_count INTEGER NOT NULL,
			asked INTEGER NOT NULL DEFAULT 0,
			score REAL NOT NULL DEFAULT 0,
			finished INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS prompts (
			session_id INTEGER PRIMARY KEY,
			item_id INTEGER NOT NULL,
			prompt_lang TEXT NOT NULL,
			answer_lang TEXT NOT NULL,
			prompt_text TEXT NOT NULL,
			input_mode TEXT NOT NULL,
			choices_json TEXT NOT NULL,
			accepted_json TEXT NOT NULL,
			answered INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS attempts (
			id INTEGER PRIMARY KEY,
			session_id INTEGER NOT NULL,
			item_id INTEGER NOT NULL,
			prompt_text TEXT NOT NULL,
			given TEXT NOT NULL,
			distance INTEGER NOT NULL,
			points REAL NOT NULL,
			outcome TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_attempts_session ON attempts(session_id)`,
	}
	return execAll(db, stmts)
}

func execAll(db *sql.DB, stmts []string) error {
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", strings.TrimSpace(stmt[:40]), err)
		}
	}
	return nil
}
