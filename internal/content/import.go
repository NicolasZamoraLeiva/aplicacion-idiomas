package content

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/language"
)

const maxImportedTokens = 12

type Stats struct {
	Sentences int
	Links     int
	Items     int
	Answers   int
}

type sentence struct {
	id     int64
	lang   string
	text   string
	tokens int
	norm   string
}

// Import reads Tatoeba sentence files and links.csv.
// Sentences longer than 12 tokens are dropped. Practice items are the
// one-token sentences that also have a one-token translation.
func Import(db *sql.DB, engPath, rusPath, linksPath string, logw io.Writer) (Stats, error) {
	if logw == nil {
		logw = io.Discard
	}
	sentences := map[int64]sentence{}
	if err := readSentences(engPath, language.English.Code, sentences, logw); err != nil {
		return Stats{}, err
	}
	if err := readSentences(rusPath, language.Russian.Code, sentences, logw); err != nil {
		return Stats{}, err
	}
	links, err := readLinks(linksPath, sentences, logw)
	if err != nil {
		return Stats{}, err
	}
	items := buildItems(sentences, links)
	stats := Stats{Sentences: len(sentences), Items: len(items)}
	for _, it := range items {
		stats.Answers += len(it.answers)
	}
	tx, err := db.Begin()
	if err != nil {
		return Stats{}, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`DELETE FROM item_answers`,
		`DELETE FROM items`,
		`DELETE FROM links`,
		`DELETE FROM sentences`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return Stats{}, err
		}
	}
	insertSentence, err := tx.Prepare(`INSERT INTO sentences(id, lang, text, token_count) VALUES(?,?,?,?)`)
	if err != nil {
		return Stats{}, err
	}
	defer insertSentence.Close()
	for _, s := range sentences {
		if _, err := insertSentence.Exec(s.id, s.lang, s.text, s.tokens); err != nil {
			return Stats{}, err
		}
	}
	insertLink, err := tx.Prepare(`INSERT OR IGNORE INTO links(sentence_id, translation_id) VALUES(?,?)`)
	if err != nil {
		return Stats{}, err
	}
	defer insertLink.Close()
	for from, tos := range links {
		for _, to := range tos {
			if _, err := insertLink.Exec(from, to); err != nil {
				return Stats{}, err
			}
			stats.Links++
		}
	}
	insertItem, err := tx.Prepare(`
		INSERT INTO items(lang, text, normalized, frequency, band, token_count)
		VALUES(?,?,?,?,?,1)`)
	if err != nil {
		return Stats{}, err
	}
	defer insertItem.Close()
	insertAnswer, err := tx.Prepare(`
		INSERT INTO item_answers(item_id, answer_lang, answer_text, normalized)
		VALUES(?,?,?,?)`)
	if err != nil {
		return Stats{}, err
	}
	defer insertAnswer.Close()
	for _, it := range items {
		res, err := insertItem.Exec(it.lang, it.text, it.norm, it.freq, it.band)
		if err != nil {
			return Stats{}, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return Stats{}, err
		}
		for _, ans := range it.answers {
			if _, err := insertAnswer.Exec(id, ans.lang, ans.text, ans.norm); err != nil {
				return Stats{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return Stats{}, err
	}
	return stats, nil
}

func readSentences(path, defaultLang string, dest map[int64]sentence, logw io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("oraciones %s: %w", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	n := 0
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			line = strings.TrimPrefix(line, "\uFEFF")
			first = false
		}
		n++
		if n%200000 == 0 {
			fmt.Fprintf(logw, "leídas %d líneas de %s\n", n, path)
		}
		id, lang, text, ok := parseSentence(line, defaultLang)
		if !ok {
			continue
		}
		langProf, known := language.ByCode(lang)
		if !known {
			continue
		}
		tokens := langProf.TokenCount(text)
		if tokens < 1 || tokens > maxImportedTokens {
			continue
		}
		if _, exists := dest[id]; exists {
			continue
		}
		dest[id] = sentence{
			id:     id,
			lang:   lang,
			text:   text,
			tokens: tokens,
			norm:   langProf.Normalize(text),
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("oraciones %s: %w", path, err)
	}
	return nil
}

func parseSentence(line, defaultLang string) (int64, string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", "", false
	}
	parts := strings.Split(line, "\t")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", "", false
	}
	if len(parts) >= 3 && looksLikeLang(parts[1]) {
		text := strings.Join(parts[2:], "\t")
		if strings.TrimSpace(text) == "" {
			return 0, "", "", false
		}
		return id, parts[1], text, true
	}
	if len(parts) >= 2 && defaultLang != "" {
		text := strings.Join(parts[1:], "\t")
		if strings.TrimSpace(text) == "" {
			return 0, "", "", false
		}
		return id, defaultLang, text, true
	}
	return 0, "", "", false
}

func looksLikeLang(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func readLinks(path string, sentences map[int64]sentence, logw io.Writer) (map[int64][]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("enlaces %s: %w", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	out := map[int64][]int64{}
	seen := map[[2]int64]struct{}{}
	n := 0
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			line = strings.TrimPrefix(line, "\uFEFF")
			first = false
		}
		n++
		if n%500000 == 0 {
			fmt.Fprintf(logw, "leídos %d enlaces\n", n)
		}
		parts := strings.Split(strings.TrimSpace(line), "\t")
		if len(parts) < 2 {
			continue
		}
		from, err1 := strconv.ParseInt(parts[0], 10, 64)
		to, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || err2 != nil || from == to {
			continue
		}
		if _, ok := sentences[from]; !ok {
			continue
		}
		if _, ok := sentences[to]; !ok {
			continue
		}
		addLink(out, seen, from, to)
		addLink(out, seen, to, from)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("enlaces %s: %w", path, err)
	}
	return out, nil
}

func addLink(out map[int64][]int64, seen map[[2]int64]struct{}, from, to int64) {
	key := [2]int64{from, to}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	out[from] = append(out[from], to)
}

type builtAnswer struct {
	lang     string
	norm     string
	text     string
	surfaces map[string]int
}

type builtItem struct {
	lang     string
	norm     string
	text     string
	freq     int
	band     int
	surfaces map[string]int
	answers  map[string]*builtAnswer
}

func buildItems(sentences map[int64]sentence, links map[int64][]int64) []builtItem {
	grouped := map[string]*builtItem{}
	for _, s := range sentences {
		if s.tokens != 1 {
			continue
		}
		prof, ok := language.ByCode(s.lang)
		if !ok || !prof.EligibleWord(s.text) || s.norm == "" {
			continue
		}
		key := s.lang + "\x00" + s.norm
		it := grouped[key]
		if it == nil {
			it = &builtItem{
				lang:     s.lang,
				norm:     s.norm,
				surfaces: map[string]int{},
				answers:  map[string]*builtAnswer{},
			}
			grouped[key] = it
		}
		it.freq++
		it.surfaces[s.text]++
		for _, tid := range links[s.id] {
			other, ok := sentences[tid]
			if !ok || other.lang == s.lang || other.tokens != 1 {
				continue
			}
			otherProf, ok := language.ByCode(other.lang)
			if !ok || !otherProf.EligibleWord(other.text) || other.norm == "" {
				continue
			}
			akey := other.lang + "\x00" + other.norm
			ans := it.answers[akey]
			if ans == nil {
				ans = &builtAnswer{lang: other.lang, norm: other.norm, surfaces: map[string]int{}}
				it.answers[akey] = ans
			}
			ans.surfaces[other.text]++
		}
	}
	byLang := map[string][]*builtItem{}
	for _, it := range grouped {
		if len(it.answers) == 0 {
			continue
		}
		it.text = bestSurface(it.surfaces)
		for _, ans := range it.answers {
			ans.text = bestSurface(ans.surfaces)
		}
		byLang[it.lang] = append(byLang[it.lang], it)
	}
	var out []builtItem
	for _, list := range byLang {
		freqs := make([]int, len(list))
		for i, it := range list {
			freqs[i] = it.freq
		}
		bands := BandsByFrequency(freqs)
		for i, it := range list {
			it.band = bands[i]
			out = append(out, *it)
		}
	}
	return out
}
