package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/activity/translate"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/content"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/profile"
)

func (a *App) handleTranslateGET(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.showTranslate(w, r, false, nil)
}

func (a *App) handleTranslatePOST(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fragment := r.URL.Query().Get("fragment") == "1"
	if err := r.ParseForm(); err != nil {
		a.fail(w, err)
		return
	}
	n, err := a.countItems(r)
	if err != nil {
		a.fail(w, err)
		return
	}
	if n == 0 {
		a.render(w, a.emptyView(), fragment)
		return
	}
	sessionID, err1 := strconv.ParseInt(r.FormValue("session_id"), 10, 64)
	itemID, err2 := strconv.ParseInt(r.FormValue("item_id"), 10, 64)
	if err1 != nil || err2 != nil {
		a.showTranslate(w, r, fragment, nil)
		return
	}
	sess, err := profile.LoadSession(r.Context(), a.profile, sessionID)
	if err != nil {
		a.showTranslate(w, r, fragment, nil)
		return
	}
	if sess.Finished {
		view := a.playView(sess, translate.Prompt{}, nil)
		view.HasResult = false
		a.render(w, view, fragment)
		return
	}
	row, ok, err := profile.CurrentPrompt(r.Context(), a.profile, sess.ID)
	if err != nil {
		a.fail(w, err)
		return
	}
	if !ok || row.ItemID != itemID {
		a.showTranslate(w, r, fragment, nil)
		return
	}
	prompt, err := decodePrompt(row)
	if err != nil {
		a.fail(w, err)
		return
	}
	if row.Answered {
		a.showAnsweredAgain(w, r, fragment, sess, prompt)
		return
	}
	settings, err := a.translateSettings(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	raw := r.FormValue("answer")
	result := a.player.Grade(prompt, raw, atoi(settings["tolerance"], 1))
	given := displayGiven(prompt, raw)
	updated, recorded, err := profile.RecordAnswer(r.Context(), a.profile, sess, prompt.ItemID, profile.Attempt{
		PromptText: prompt.Text,
		Given:      given,
		Distance:   result.Distance,
		Points:     result.Points,
		Outcome:    result.Outcome,
	}, sess.Mode == "quiz" && sess.Asked+1 >= sess.QuestionCount)
	if err != nil {
		a.fail(w, err)
		return
	}
	if !recorded {
		a.showAnsweredAgain(w, r, fragment, updated, prompt)
		return
	}
	state := &resultState{
		Outcome:    result.Outcome,
		Given:      given,
		Accepted:   prompt.Accepted,
		AnswerLang: prompt.AnswerLang,
		Points:     result.Points,
	}
	if updated.Finished {
		a.render(w, a.playView(updated, translate.Prompt{}, state), fragment)
		return
	}
	next, err := a.ensurePrompt(r.Context(), updated, settings)
	if errors.Is(err, content.ErrNone) {
		view := a.playView(updated, translate.Prompt{}, state)
		view.HasPrompt = false
		a.render(w, view, fragment)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, a.playView(updated, next, state), fragment)
}

func (a *App) showAnsweredAgain(w http.ResponseWriter, r *http.Request, fragment bool, sess profile.Session, prompt translate.Prompt) {
	attempt, ok, err := profile.LastAttempt(r.Context(), a.profile, sess.ID, prompt.ItemID)
	if err != nil {
		a.fail(w, err)
		return
	}
	var state *resultState
	if ok {
		state = &resultState{
			Outcome:    attempt.Outcome,
			Given:      attempt.Given,
			Accepted:   prompt.Accepted,
			AnswerLang: prompt.AnswerLang,
			Points:     attempt.Points,
		}
	}
	if sess.Finished {
		a.render(w, a.playView(sess, translate.Prompt{}, state), fragment)
		return
	}
	settings, err := a.translateSettings(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	next, err := a.ensurePrompt(r.Context(), sess, settings)
	if errors.Is(err, content.ErrNone) {
		a.render(w, a.noItemsView(), fragment)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	fresh, err := profile.LoadSession(r.Context(), a.profile, sess.ID)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, a.playView(fresh, next, state), fragment)
}

func (a *App) showTranslate(w http.ResponseWriter, r *http.Request, fragment bool, state *resultState) {
	n, err := a.countItems(r)
	if err != nil {
		a.fail(w, err)
		return
	}
	if n == 0 {
		a.render(w, a.emptyView(), fragment)
		return
	}
	settings, err := a.translateSettings(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	sess, err := a.ensureSession(r.Context(), settings)
	if err != nil {
		a.fail(w, err)
		return
	}
	prompt, err := a.ensurePrompt(r.Context(), sess, settings)
	if errors.Is(err, content.ErrNone) {
		a.render(w, a.noItemsView(), fragment)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	fresh, err := profile.LoadSession(r.Context(), a.profile, sess.ID)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, a.playView(fresh, prompt, state), fragment)
}

func (a *App) ensureSession(ctx context.Context, settings map[string]string) (profile.Session, error) {
	mode := settings["mode"]
	if mode != "quiz" {
		mode = "instant"
	}
	if sess, ok, err := profile.FindOpen(ctx, a.profile, a.player.Info().ID, mode); err != nil || ok {
		return sess, err
	}
	questions := 0
	if mode == "quiz" {
		questions = atoi(settings["quiz_size"], 10)
		if questions < 1 {
			questions = 1
		}
	}
	return profile.CreateSession(ctx, a.profile, a.player.Info().ID, mode, questions)
}

func (a *App) ensurePrompt(ctx context.Context, sess profile.Session, settings map[string]string) (translate.Prompt, error) {
	if row, ok, err := profile.CurrentPrompt(ctx, a.profile, sess.ID); err != nil {
		return translate.Prompt{}, err
	} else if ok && !row.Answered {
		return decodePrompt(row)
	}
	exclude, err := profile.AttemptItemIDs(ctx, a.profile, sess.ID)
	if err != nil {
		return translate.Prompt{}, err
	}
	req := translate.Request{
		UserLang:    settings["user_lang"],
		TargetLang:  settings["target_lang"],
		Direction:   settings["direction"],
		Band:        atoi(settings["difficulty"], 0),
		MaxTokens:   atoi(settings["max_tokens"], 1),
		Input:       settings["input"],
		OptionCount: atoi(settings["option_count"], 4),
		Distractors: settings["distractors"],
		Exclude:     exclude,
	}
	prompt, err := a.player.Next(ctx, req)
	if err != nil {
		return translate.Prompt{}, err
	}
	choices, err := json.Marshal(prompt.Choices)
	if err != nil {
		return translate.Prompt{}, err
	}
	accepted := make([]storedAnswer, len(prompt.Accepted))
	for i := range prompt.Accepted {
		accepted[i] = storedAnswer{Text: prompt.Accepted[i], Normalized: prompt.AcceptedNorm[i]}
	}
	rawAccepted, err := json.Marshal(accepted)
	if err != nil {
		return translate.Prompt{}, err
	}
	err = profile.SavePrompt(ctx, a.profile, profile.Prompt{
		SessionID:    sess.ID,
		ItemID:       prompt.ItemID,
		PromptLang:   prompt.PromptLang,
		AnswerLang:   prompt.AnswerLang,
		Text:         prompt.Text,
		Input:        prompt.Input,
		ChoicesJSON:  string(choices),
		AcceptedJSON: string(rawAccepted),
	})
	return prompt, err
}

type storedAnswer struct {
	Text       string `json:"text"`
	Normalized string `json:"normalized"`
}

func decodePrompt(row profile.Prompt) (translate.Prompt, error) {
	var choices []translate.Choice
	if row.ChoicesJSON != "" && row.ChoicesJSON != "null" {
		if err := json.Unmarshal([]byte(row.ChoicesJSON), &choices); err != nil {
			return translate.Prompt{}, err
		}
	}
	var accepted []storedAnswer
	if row.AcceptedJSON != "" {
		if err := json.Unmarshal([]byte(row.AcceptedJSON), &accepted); err != nil {
			return translate.Prompt{}, err
		}
	}
	p := translate.Prompt{
		ItemID:     row.ItemID,
		PromptLang: row.PromptLang,
		AnswerLang: row.AnswerLang,
		Text:       row.Text,
		Input:      row.Input,
		Choices:    choices,
	}
	for _, ans := range accepted {
		p.Accepted = append(p.Accepted, ans.Text)
		p.AcceptedNorm = append(p.AcceptedNorm, ans.Normalized)
	}
	return p, nil
}

func displayGiven(p translate.Prompt, raw string) string {
	if p.Input == "choice" {
		for _, choice := range p.Choices {
			if choice.Value == raw {
				return choice.Label
			}
		}
	}
	return raw
}
