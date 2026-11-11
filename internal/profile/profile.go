package profile

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Session struct {
	ID            int64
	Mode          string
	Activity      string
	QuestionCount int
	Asked         int
	Score         float64
	Finished      bool
	CreatedAt     string
}

type Prompt struct {
	SessionID    int64
	ItemID       int64
	PromptLang   string
	AnswerLang   string
	Text         string
	Input        string
	ChoicesJSON  string
	AcceptedJSON string
	Answered     bool
}

type Attempt struct {
	ID         int64
	SessionID  int64
	ItemID     int64
	PromptText string
	Given      string
	Distance   int
	Points     float64
	Outcome    string
	CreatedAt  string
}

func LoadSettings(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func SaveSettings(ctx context.Context, db *sql.DB, values map[string]string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for k, v := range values {
		if _, err := stmt.ExecContext(ctx, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func FindOpen(ctx context.Context, db *sql.DB, activity, mode string) (Session, bool, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, mode, activity, question_count, asked, score, finished, created_at
		FROM sessions
		WHERE activity = ? AND mode = ? AND finished = 0
		ORDER BY id DESC LIMIT 1`, activity, mode)
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	return s, err == nil, err
}

func CreateSession(ctx context.Context, db *sql.DB, activity, mode string, questions int) (Session, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO sessions(mode, activity, question_count, asked, score, finished, created_at)
		VALUES(?,?,?,0,0,0,?)`, mode, activity, questions, now)
	if err != nil {
		return Session{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Session{}, err
	}
	return LoadSession(ctx, db, id)
}

func LoadSession(ctx context.Context, db *sql.DB, id int64) (Session, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, mode, activity, question_count, asked, score, finished, created_at
		FROM sessions WHERE id = ?`, id)
	s, err := scanSession(row)
	return s, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(row scanner) (Session, error) {
	var s Session
	var finished int
	err := row.Scan(&s.ID, &s.Mode, &s.Activity, &s.QuestionCount, &s.Asked, &s.Score, &finished, &s.CreatedAt)
	s.Finished = finished != 0
	return s, err
}

func CurrentPrompt(ctx context.Context, db *sql.DB, sessionID int64) (Prompt, bool, error) {
	var p Prompt
	var answered int
	err := db.QueryRowContext(ctx, `
		SELECT session_id, item_id, prompt_lang, answer_lang, prompt_text, input_mode,
		       choices_json, accepted_json, answered
		FROM prompts WHERE session_id = ?`, sessionID).Scan(
		&p.SessionID, &p.ItemID, &p.PromptLang, &p.AnswerLang, &p.Text, &p.Input,
		&p.ChoicesJSON, &p.AcceptedJSON, &answered,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Prompt{}, false, nil
	}
	p.Answered = answered != 0
	return p, err == nil, err
}

func SavePrompt(ctx context.Context, db *sql.DB, p Prompt) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO prompts(session_id, item_id, prompt_lang, answer_lang, prompt_text, input_mode, choices_json, accepted_json, answered)
		VALUES(?,?,?,?,?,?,?,?,0)
		ON CONFLICT(session_id) DO UPDATE SET
			item_id = excluded.item_id,
			prompt_lang = excluded.prompt_lang,
			answer_lang = excluded.answer_lang,
			prompt_text = excluded.prompt_text,
			input_mode = excluded.input_mode,
			choices_json = excluded.choices_json,
			accepted_json = excluded.accepted_json,
			answered = 0`,
		p.SessionID, p.ItemID, p.PromptLang, p.AnswerLang, p.Text, p.Input, p.ChoicesJSON, p.AcceptedJSON,
	)
	return err
}

func AttemptItemIDs(ctx context.Context, db *sql.DB, sessionID int64) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT item_id FROM attempts WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// RecordAnswer scores a prompt once. recorded is false when it was already answered.
func RecordAnswer(ctx context.Context, db *sql.DB, session Session, itemID int64, attempt Attempt, finish bool) (Session, bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
		UPDATE prompts SET answered = 1
		WHERE session_id = ? AND item_id = ? AND answered = 0`, session.ID, itemID)
	if err != nil {
		return Session{}, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Session{}, false, err
	}
	if n == 0 {
		current, err := loadSessionTx(ctx, tx, session.ID)
		return current, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO attempts(session_id, item_id, prompt_text, given, distance, points, outcome, created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		session.ID, itemID, attempt.PromptText, attempt.Given, attempt.Distance, attempt.Points, attempt.Outcome, now,
	); err != nil {
		return Session{}, false, err
	}
	finished := 0
	if finish {
		finished = 1
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions
		SET asked = asked + 1, score = score + ?, finished = ?
		WHERE id = ?`, attempt.Points, finished, session.ID); err != nil {
		return Session{}, false, err
	}
	updated, err := loadSessionTx(ctx, tx, session.ID)
	if err != nil {
		return Session{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, false, err
	}
	return updated, true, nil
}

func loadSessionTx(ctx context.Context, tx *sql.Tx, id int64) (Session, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, mode, activity, question_count, asked, score, finished, created_at
		FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

func LastAttempt(ctx context.Context, db *sql.DB, sessionID, itemID int64) (Attempt, bool, error) {
	var a Attempt
	err := db.QueryRowContext(ctx, `
		SELECT id, session_id, item_id, prompt_text, given, distance, points, outcome, created_at
		FROM attempts
		WHERE session_id = ? AND item_id = ?
		ORDER BY id DESC LIMIT 1`, sessionID, itemID).Scan(
		&a.ID, &a.SessionID, &a.ItemID, &a.PromptText, &a.Given, &a.Distance, &a.Points, &a.Outcome, &a.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Attempt{}, false, nil
	}
	return a, err == nil, err
}

func Attempts(ctx context.Context, db *sql.DB, sessionID int64) ([]Attempt, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, session_id, item_id, prompt_text, given, distance, points, outcome, created_at
		FROM attempts WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(&a.ID, &a.SessionID, &a.ItemID, &a.PromptText, &a.Given, &a.Distance, &a.Points, &a.Outcome, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
