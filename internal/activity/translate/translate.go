package translate

import (
	"context"
	"math/rand/v2"
	"time"
	"unicode/utf8"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/activity"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/content"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/grade"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/language"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/ui"
)

const activityID = "translate"

type Service struct {
	Content content.Store
	Rand    *rand.Rand
}

type Choice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Prompt struct {
	ItemID       int64
	PromptLang   string
	AnswerLang   string
	Text         string
	Input        string
	Choices      []Choice
	Accepted     []string
	AcceptedNorm []string
}

type Request struct {
	UserLang    string
	TargetLang  string
	Direction   string
	Band        int
	MaxTokens   int
	Input       string
	OptionCount int
	Distractors string
	Exclude     []int64
}

func (s *Service) Info() activity.Info {
	return activity.Info{ID: activityID, Name: ui.T("activity.translate")}
}

func (s *Service) Settings() []activity.Setting {
	langs := []activity.Option{
		{Value: language.English.Code, Label: ui.T("lang.eng")},
		{Value: language.Russian.Code, Label: ui.T("lang.rus")},
	}
	return []activity.Setting{
		{Key: "user_lang", Label: ui.T("setting.user_lang"), Help: ui.T("setting.user_lang.help"), Kind: activity.KindSelect, Options: langs, Default: language.English.Code},
		{Key: "target_lang", Label: ui.T("setting.target_lang"), Help: ui.T("setting.target_lang.help"), Kind: activity.KindSelect, Options: langs, Default: language.Russian.Code},
		{Key: "direction", Label: ui.T("setting.direction"), Help: ui.T("setting.direction.help"), Kind: activity.KindSelect, Default: "to_target", Options: []activity.Option{
			{Value: "to_target", Label: ui.T("setting.direction.to_target")},
			{Value: "to_user", Label: ui.T("setting.direction.to_user")},
			{Value: "mixed", Label: ui.T("setting.direction.mixed")},
		}},
		{Key: "mode", Label: ui.T("setting.mode"), Help: ui.T("setting.mode.help"), Kind: activity.KindSelect, Default: "instant", Options: []activity.Option{
			{Value: "instant", Label: ui.T("setting.mode.instant")},
			{Value: "quiz", Label: ui.T("setting.mode.quiz")},
		}},
		{Key: "quiz_size", Label: ui.T("setting.quiz_size"), Help: ui.T("setting.quiz_size.help"), Kind: activity.KindInt, Min: 1, Max: 50, Default: "10"},
		{Key: "input", Label: ui.T("setting.input"), Kind: activity.KindSelect, Default: "choice", Options: []activity.Option{
			{Value: "choice", Label: ui.T("setting.input.choice")},
			{Value: "type", Label: ui.T("setting.input.type")},
		}},
		{Key: "option_count", Label: ui.T("setting.option_count"), Help: ui.T("setting.option_count.help"), Kind: activity.KindInt, Min: 3, Max: 6, Default: "4"},
		{Key: "tolerance", Label: ui.T("setting.tolerance"), Help: ui.T("setting.tolerance.help"), Kind: activity.KindInt, Min: 0, Max: 3, Default: "1"},
		{Key: "difficulty", Label: ui.T("setting.difficulty"), Help: ui.T("setting.difficulty.help"), Kind: activity.KindSelect, Default: "0", Options: []activity.Option{
			{Value: "0", Label: ui.T("setting.difficulty.any")},
			{Value: "1", Label: ui.T("setting.difficulty.1")},
			{Value: "2", Label: ui.T("setting.difficulty.2")},
			{Value: "3", Label: ui.T("setting.difficulty.3")},
			{Value: "4", Label: ui.T("setting.difficulty.4")},
			{Value: "5", Label: ui.T("setting.difficulty.5")},
		}},
		{Key: "distractors", Label: ui.T("setting.distractors"), Help: ui.T("setting.distractors.help"), Kind: activity.KindSelect, Default: "random", Options: []activity.Option{
			{Value: "random", Label: ui.T("setting.distractors.random")},
			{Value: "length", Label: ui.T("setting.distractors.length")},
		}},
		{Key: "max_tokens", Label: ui.T("setting.max_tokens"), Help: ui.T("setting.max_tokens.help"), Kind: activity.KindInt, Min: 1, Max: 1, Default: "1"},
	}
}

func (s *Service) Validate(values map[string]string) (map[string]string, error) {
	clean, err := activity.Check(s.Settings(), values)
	if err != nil {
		return nil, err
	}
	if clean["user_lang"] == clean["target_lang"] {
		return nil, errSameLanguage
	}
	return clean, nil
}

type sameLanguageError struct{}

func (sameLanguageError) Error() string { return "same language" }

var errSameLanguage = sameLanguageError{}

func IsSameLanguage(err error) bool { return err == errSameLanguage }

func (s *Service) Next(ctx context.Context, req Request) (Prompt, error) {
	promptLang, answerLang := s.direction(req.UserLang, req.TargetLang, req.Direction)
	item, answers, err := s.Content.Pick(ctx, promptLang, answerLang, req.Band, req.MaxTokens, req.Exclude)
	if err != nil {
		return Prompt{}, err
	}
	p := Prompt{
		ItemID:     item.ID,
		PromptLang: item.Lang,
		AnswerLang: answerLang,
		Text:       item.Text,
		Input:      req.Input,
	}
	if p.Input != "type" {
		p.Input = "choice"
	}
	for _, a := range answers {
		p.Accepted = append(p.Accepted, a.Text)
		p.AcceptedNorm = append(p.AcceptedNorm, a.Normalized)
	}
	if p.Input == "choice" {
		p.Choices, err = s.choices(ctx, answers, answerLang, req.OptionCount, req.Distractors == "length")
		if err != nil {
			return Prompt{}, err
		}
	}
	return p, nil
}

func (s *Service) Grade(p Prompt, given string, tolerance int) grade.Result {
	prof, ok := language.ByCode(p.AnswerLang)
	norm := given
	if ok {
		norm = prof.Normalize(given)
	}
	tol := tolerance
	if p.Input == "choice" {
		tol = 0
	}
	return grade.Evaluate(norm, p.AcceptedNorm, tol)
}

func (s *Service) direction(user, target, direction string) (string, string) {
	switch direction {
	case "to_user":
		return target, user
	case "mixed":
		if s.rng().IntN(2) == 0 {
			return user, target
		}
		return target, user
	default:
		return user, target
	}
}

func (s *Service) choices(ctx context.Context, answers []content.Answer, answerLang string, count int, similar bool) ([]Choice, error) {
	if count < 2 {
		count = 2
	}
	rng := s.rng()
	correct := answers[rng.IntN(len(answers))]
	norms := make([]string, len(answers))
	for i, a := range answers {
		norms[i] = a.Normalized
	}
	need := count - 1
	found, err := s.Content.Distractors(ctx, answerLang, norms, utf8.RuneCountInString(correct.Normalized), similar, need)
	if err != nil {
		return nil, err
	}
	out := make([]Choice, 0, 1+len(found))
	out = append(out, Choice{Value: correct.Normalized, Label: correct.Text})
	for _, d := range found {
		out = append(out, Choice{Value: d.Normalized, Label: d.Text})
	}
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out, nil
}

func (s *Service) rng() *rand.Rand {
	if s.Rand != nil {
		return s.Rand
	}
	return rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15))
}
