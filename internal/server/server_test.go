package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/content"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/storage"
)

func TestTranslateFlow(t *testing.T) {
	dir := t.TempDir()
	contentDB, err := storage.Open(filepath.Join(dir, "content.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { contentDB.Close() })
	profileDB, err := storage.Open(filepath.Join(dir, "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { profileDB.Close() })
	if err := storage.MigrateContent(contentDB); err != nil {
		t.Fatal(err)
	}
	if err := storage.MigrateProfile(profileDB); err != nil {
		t.Fatal(err)
	}
	sample := sampleDir(t)
	if _, err := content.Import(contentDB, filepath.Join(sample, "eng_sentences.tsv"), filepath.Join(sample, "rus_sentences.tsv"), filepath.Join(sample, "links.tsv"), nil); err != nil {
		t.Fatal(err)
	}
	app, err := New(contentDB, profileDB)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)

	settings := get(t, srv.URL+"/settings")
	if !strings.Contains(settings, "Idioma del usuario") || !strings.Contains(settings, "Tolerancia al escribir") {
		t.Fatal("settings page is missing generated fields")
	}
	bad := defaultSettings()
	bad.Set("target_lang", "eng")
	res, body := post(t, srv.URL+"/settings", bad)
	if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "tienen que ser distintos") {
		t.Fatalf("same language: %d %s", res.StatusCode, body)
	}

	page := get(t, srv.URL+"/activities/translate")
	if !strings.Contains(page, "fragment=1") || !strings.Contains(page, "Traduce al ruso") {
		t.Fatal("prompt page is missing the form or the direction")
	}
	sessionID := find(t, page, `name="session_id" value="([^"]+)"`)
	itemID := find(t, page, `name="item_id" value="([^"]+)"`)
	rows, err := contentDB.Query(`SELECT normalized FROM item_answers WHERE item_id = ? AND answer_lang = 'rus'`, itemID)
	if err != nil {
		t.Fatal(err)
	}
	var accepted string
	for rows.Next() {
		var norm string
		if err := rows.Scan(&norm); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(page, `value="`+norm+`"`) {
			accepted = norm
		}
	}
	rows.Close()
	if accepted == "" {
		t.Fatal("correct choice is missing from the page")
	}
	answer := url.Values{}
	answer.Set("session_id", sessionID)
	answer.Set("item_id", itemID)
	answer.Set("answer", accepted)
	res, body = post(t, srv.URL+"/activities/translate/answer", answer)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "Correcto") {
		t.Fatalf("answer: %d %s", res.StatusCode, body)
	}

	typed := defaultSettings()
	typed.Set("input", "type")
	typed.Set("tolerance", "1")
	typed.Set("mode", "quiz")
	typed.Set("quiz_size", "1")
	res, _ = post(t, srv.URL+"/settings", typed)
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("save settings: %d", res.StatusCode)
	}
	page = get(t, srv.URL+"/activities/translate")
	sessionID = find(t, page, `name="session_id" value="([^"]+)"`)
	itemID = find(t, page, `name="item_id" value="([^"]+)"`)
	if err := contentDB.QueryRow(`SELECT normalized FROM item_answers WHERE item_id = ? LIMIT 1`, itemID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	_, size := utf8.DecodeRuneInString(accepted)
	almost := accepted[size:]
	answer = url.Values{}
	answer.Set("session_id", sessionID)
	answer.Set("item_id", itemID)
	answer.Set("answer", almost)
	_, body = post(t, srv.URL+"/activities/translate/answer", answer)
	if !strings.Contains(body, "Casi") || !strings.Contains(body, "Resultado") {
		t.Fatalf("quiz result:\n%s", body)
	}
}

func TestEmptyLibrary(t *testing.T) {
	dir := t.TempDir()
	contentDB, err := storage.Open(filepath.Join(dir, "content.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { contentDB.Close() })
	profileDB, err := storage.Open(filepath.Join(dir, "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { profileDB.Close() })
	if err := storage.MigrateContent(contentDB); err != nil {
		t.Fatal(err)
	}
	if err := storage.MigrateProfile(profileDB); err != nil {
		t.Fatal(err)
	}
	app, err := New(contentDB, profileDB)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)
	page := get(t, srv.URL+"/")
	if !strings.Contains(page, "import-tatoeba") {
		t.Fatal("empty state should explain how to import")
	}
}

func defaultSettings() url.Values {
	values := url.Values{}
	values.Set("user_lang", "eng")
	values.Set("target_lang", "rus")
	values.Set("direction", "to_target")
	values.Set("mode", "instant")
	values.Set("quiz_size", "10")
	values.Set("input", "choice")
	values.Set("option_count", "4")
	values.Set("tolerance", "1")
	values.Set("difficulty", "0")
	values.Set("distractors", "random")
	values.Set("max_tokens", "1")
	return values
}

func get(t *testing.T, rawURL string) string {
	t.Helper()
	res, err := http.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", rawURL, res.StatusCode, body)
	}
	return string(body)
}

func post(t *testing.T, rawURL string, values url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func find(t *testing.T, body, pattern string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("missing %s", pattern)
	}
	return m[1]
}

func sampleDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "tatoeba")
}
