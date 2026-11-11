package language

import (
	"strings"
	"unicode"
)

// Language is the profile the rest of the app uses for a study language.
// Adding a language means registering one of these and teaching Normalize about it.
type Language struct {
	Code   string
	Name   string
	Script string
}

var (
	English = Language{Code: "eng", Name: "Inglés", Script: "Latn"}
	Russian = Language{Code: "rus", Name: "Ruso", Script: "Cyrl"}
)

func All() []Language {
	return []Language{English, Russian}
}

func ByCode(code string) (Language, bool) {
	switch code {
	case English.Code:
		return English, true
	case Russian.Code:
		return Russian, true
	default:
		return Language{}, false
	}
}

func (l Language) BCP47() string {
	switch l.Code {
	case Russian.Code:
		return "ru"
	case English.Code:
		return "en"
	default:
		return "und"
	}
}

// TokenCount splits on whitespace. English and Russian both work that way;
// a language without spaces would override this switch.
func (l Language) TokenCount(text string) int {
	switch l.Code {
	default:
		return len(strings.Fields(strings.TrimSpace(text)))
	}
}

// EligibleWord reports whether a single Tatoeba sentence can be a practice item.
func (l Language) EligibleWord(text string) bool {
	if l.TokenCount(text) != 1 || hasDigit(text) {
		return false
	}
	return letterCount(text) >= 2
}

// Normalize folds case and strips everything that is not a letter.
// Russian treats ё and е as the same letter.
func (l Language) Normalize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		r = unicode.ToLower(r)
		if l.Code == Russian.Code && r == 'ё' {
			r = 'е'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func hasDigit(text string) bool {
	for _, r := range text {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func letterCount(text string) int {
	n := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}
