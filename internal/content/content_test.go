package content

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/storage"
)

func TestBandsByFrequency(t *testing.T) {
	bands := BandsByFrequency([]int{1, 50, 3, 40, 2})
	if bands[1] != 1 || bands[0] != 5 {
		t.Fatalf("bands = %v", bands)
	}
	for _, band := range bands {
		if band < 1 || band > 5 {
			t.Fatalf("band out of range: %v", bands)
		}
	}
}

func TestImportSample(t *testing.T) {
	db := openContent(t)
	dir := sampleDir(t)
	stats, err := Import(db, filepath.Join(dir, "eng_sentences.tsv"), filepath.Join(dir, "rus_sentences.tsv"), filepath.Join(dir, "links.tsv"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Items < 15 || stats.Sentences <= stats.Items {
		t.Fatalf("stats %+v", stats)
	}
	again, err := Import(db, filepath.Join(dir, "eng_sentences.tsv"), filepath.Join(dir, "rus_sentences.tsv"), filepath.Join(dir, "links.tsv"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Items != stats.Items {
		t.Fatalf("reimport changed items: %d vs %d", again.Items, stats.Items)
	}

	ctx := context.Background()
	store := Store{DB: db}
	hello, answers, err := store.ItemByNorm(ctx, "eng", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if hello.Band != 1 {
		t.Fatalf("hello band = %d", hello.Band)
	}
	if len(answers) < 2 {
		t.Fatalf("hello answers = %+v", answers)
	}
	silence, _, err := store.ItemByNorm(ctx, "eng", "silence")
	if err != nil {
		t.Fatal(err)
	}
	if silence.Band != 5 {
		t.Fatalf("silence band = %d", silence.Band)
	}
	if _, _, err := store.ItemByNorm(ctx, "rus", "еж"); err != nil {
		t.Fatal("ёж and еж should be one item", err)
	}
	var dropped, keptPhrase, digitItem int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sentences WHERE token_count > 12`).Scan(&dropped); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sentences WHERE text = 'good morning'`).Scan(&keptPhrase); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE text = 'a1' OR text = 'I'`).Scan(&digitItem); err != nil {
		t.Fatal(err)
	}
	if dropped != 0 || keptPhrase != 1 || digitItem != 0 {
		t.Fatalf("dropped=%d phrase=%d digitItem=%d", dropped, keptPhrase, digitItem)
	}
}

func TestPickFiltersAndDistractors(t *testing.T) {
	db := openContent(t)
	ctx := context.Background()
	insert := func(lang, text, norm string, band, tokens int) int64 {
		t.Helper()
		res, err := db.Exec(`INSERT INTO items(lang, text, normalized, frequency, band, token_count) VALUES(?,?,?,?,?,?)`,
			lang, text, norm, 1, band, tokens)
		if err != nil {
			t.Fatal(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	answer := func(itemID int64, lang, text, norm string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO item_answers(item_id, answer_lang, answer_text, normalized) VALUES(?,?,?,?)`,
			itemID, lang, text, norm); err != nil {
			t.Fatal(err)
		}
	}
	easy := insert("eng", "cat", "cat", 1, 1)
	answer(easy, "rus", "кот", "кот")
	hard := insert("eng", "curiosity", "curiosity", 5, 1)
	answer(hard, "rus", "любопытство", "любопытство")
	phrase := insert("eng", "good morning", "goodmorning", 1, 2)
	answer(phrase, "rus", "утро", "утро")
	insert("rus", "кот", "кот", 1, 1)
	insert("rus", "пёс", "пес", 1, 1)
	insert("rus", "дом", "дом", 1, 1)
	insert("rus", "любопытство", "любопытство", 5, 1)

	store := Store{DB: db}
	item, answers, err := store.Pick(ctx, "eng", "rus", 1, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != easy || item.Tokens != 1 || len(answers) != 1 || answers[0].Normalized != "кот" {
		t.Fatalf("band filter: %+v %+v", item, answers)
	}
	excluded, _, err := store.Pick(ctx, "eng", "rus", 0, 1, []int64{easy})
	if err != nil {
		t.Fatal(err)
	}
	if excluded.ID == easy {
		t.Fatal("excluded item was selected")
	}
	fallback, _, err := store.Pick(ctx, "eng", "rus", 1, 1, []int64{easy})
	if err != nil || fallback.ID != easy {
		t.Fatalf("fallback = %+v %v", fallback, err)
	}
	found, err := store.Distractors(ctx, "rus", []string{"кот"}, 3, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("distractors = %+v", found)
	}
	for _, d := range found {
		if d.Normalized == "кот" {
			t.Fatalf("accepted translation used as distractor: %+v", found)
		}
	}
}

func TestTwoColumnSentences(t *testing.T) {
	dir := t.TempDir()
	eng := filepath.Join(dir, "eng.tsv")
	rus := filepath.Join(dir, "rus.tsv")
	links := filepath.Join(dir, "links.tsv")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(eng, "1\thello\n")
	write(rus, "2\trus\tпривет\n")
	write(links, "1\t2\n")
	db := openContent(t)
	if _, err := Import(db, eng, rus, links, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Store{DB: db}).ItemByNorm(context.Background(), "eng", "hello"); err != nil {
		t.Fatal(err)
	}
}

func openContent(t *testing.T) *sql.DB {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.MigrateContent(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func sampleDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "tatoeba")
}
