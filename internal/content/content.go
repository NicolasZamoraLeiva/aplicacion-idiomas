package content

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrNone = errors.New("no matching items")

type Item struct {
	ID         int64
	Lang       string
	Text       string
	Normalized string
	Frequency  int
	Band       int
	Tokens     int
}

type Answer struct {
	Lang       string
	Text       string
	Normalized string
}

type Store struct {
	DB *sql.DB
}

func (s Store) CountItems(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&n)
	return n, err
}

func (s Store) ItemByNorm(ctx context.Context, lang, normalized string) (Item, []Answer, error) {
	var item Item
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, lang, text, normalized, frequency, band, token_count
		FROM items WHERE lang = ? AND normalized = ?`, lang, normalized).
		Scan(&item.ID, &item.Lang, &item.Text, &item.Normalized, &item.Frequency, &item.Band, &item.Tokens)
	if err != nil {
		return Item{}, nil, err
	}
	answers, err := s.answers(ctx, item.ID, "")
	return item, answers, err
}

// Pick chooses a random item in promptLang that has an answer in answerLang.
// band 0 means any band. exclude lists item ids already used in the session.
func (s Store) Pick(ctx context.Context, promptLang, answerLang string, band, maxTokens int, exclude []int64) (Item, []Answer, error) {
	if maxTokens < 1 {
		maxTokens = 1
	}
	item, err := s.pick(ctx, promptLang, answerLang, band, maxTokens, exclude)
	if errors.Is(err, ErrNone) && len(exclude) > 0 {
		item, err = s.pick(ctx, promptLang, answerLang, band, maxTokens, nil)
	}
	if err != nil {
		return Item{}, nil, err
	}
	answers, err := s.answers(ctx, item.ID, answerLang)
	if err != nil {
		return Item{}, nil, err
	}
	if len(answers) == 0 {
		return Item{}, nil, ErrNone
	}
	return item, answers, nil
}

func (s Store) pick(ctx context.Context, promptLang, answerLang string, band, maxTokens int, exclude []int64) (Item, error) {
	query := `
		SELECT i.id, i.lang, i.text, i.normalized, i.frequency, i.band, i.token_count
		FROM items i
		WHERE i.lang = ?
		  AND i.token_count <= ?
		  AND (? = 0 OR i.band = ?)
		  AND EXISTS (
			SELECT 1 FROM item_answers a
			WHERE a.item_id = i.id AND a.answer_lang = ?
		  )`
	args := []any{promptLang, maxTokens, band, band, answerLang}
	if len(exclude) > 0 {
		query += ` AND i.id NOT IN (` + placeholders(len(exclude)) + `)`
		for _, id := range exclude {
			args = append(args, id)
		}
	}
	query += ` ORDER BY RANDOM() LIMIT 1`
	var item Item
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Lang, &item.Text, &item.Normalized, &item.Frequency, &item.Band, &item.Tokens,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNone
	}
	return item, err
}

func (s Store) answers(ctx context.Context, itemID int64, lang string) ([]Answer, error) {
	query := `SELECT answer_lang, answer_text, normalized FROM item_answers WHERE item_id = ?`
	args := []any{itemID}
	if lang != "" {
		query += ` AND answer_lang = ?`
		args = append(args, lang)
	}
	query += ` ORDER BY answer_text`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Answer
	for rows.Next() {
		var a Answer
		if err := rows.Scan(&a.Lang, &a.Text, &a.Normalized); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Distractors returns other words in lang, never one of the accepted forms.
// When similar is set, words within one letter of targetLen are preferred.
func (s Store) Distractors(ctx context.Context, lang string, accepted []string, targetLen int, similar bool, limit int) ([]Answer, error) {
	if limit < 1 {
		return nil, nil
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT text, normalized FROM items
		WHERE lang = ? AND token_count = 1
		ORDER BY RANDOM()
		LIMIT 400`, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blocked := map[string]bool{}
	for _, n := range accepted {
		blocked[n] = true
	}
	var pool []Answer
	for rows.Next() {
		var a Answer
		a.Lang = lang
		if err := rows.Scan(&a.Text, &a.Normalized); err != nil {
			return nil, err
		}
		if blocked[a.Normalized] || a.Normalized == "" {
			continue
		}
		pool = append(pool, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var chosen []Answer
	take := func(pred func(Answer) bool) {
		for _, a := range pool {
			if len(chosen) >= limit {
				return
			}
			if seen[a.Normalized] || !pred(a) {
				continue
			}
			seen[a.Normalized] = true
			chosen = append(chosen, a)
		}
	}
	if similar {
		take(func(a Answer) bool {
			return abs(utf8.RuneCountInString(a.Normalized)-targetLen) <= 1
		})
	}
	take(func(Answer) bool { return true })
	return chosen, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func placeholders(n int) string {
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

// BandsByFrequency assigns 1 to the most frequent entries and 5 to the least.
// Ties keep the original order among themselves.
func BandsByFrequency(freqs []int) []int {
	n := len(freqs)
	bands := make([]int, n)
	if n == 0 {
		return bands
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if freqs[idx[a]] != freqs[idx[b]] {
			return freqs[idx[a]] > freqs[idx[b]]
		}
		return idx[a] < idx[b]
	})
	for rank, i := range idx {
		band := rank*5/n + 1
		if band > 5 {
			band = 5
		}
		if band < 1 {
			band = 1
		}
		bands[i] = band
	}
	return bands
}

func bestSurface(counts map[string]int) string {
	best := ""
	bestN := -1
	for text, n := range counts {
		if n > bestN || (n == bestN && surfaceLess(text, best)) {
			best = text
			bestN = n
		}
	}
	return strings.TrimSpace(best)
}

func surfaceLess(a, b string) bool {
	aClean := strings.TrimFunc(a, unicode.IsPunct) == a
	bClean := strings.TrimFunc(b, unicode.IsPunct) == b
	if aClean != bClean {
		return aClean
	}
	ar, br := utf8.RuneCountInString(a), utf8.RuneCountInString(b)
	if ar != br {
		return ar < br
	}
	return a < b
}
