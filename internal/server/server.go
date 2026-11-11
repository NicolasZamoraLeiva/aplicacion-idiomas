package server

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/activity"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/activity/translate"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/content"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/grade"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/language"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/profile"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/ui"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/web"
)

const importCommand = "go run ./cmd/import-tatoeba -eng testdata/tatoeba/eng_sentences.tsv -rus testdata/tatoeba/rus_sentences.tsv -links testdata/tatoeba/links.tsv -db data/content.db"

type App struct {
	content    *sql.DB
	profile    *sql.DB
	player     *translate.Service
	activities []activity.Activity
	tmpl       *template.Template
	static     http.Handler
	mu         sync.Mutex
}

func New(contentDB, profileDB *sql.DB) (*App, error) {
	tmpl, err := template.New("app").Funcs(template.FuncMap{"t": ui.T}).ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
	}
	player := &translate.Service{Content: content.Store{DB: contentDB}}
	return &App{
		content:    contentDB,
		profile:    profileDB,
		player:     player,
		activities: []activity.Activity{player},
		tmpl:       tmpl,
		static:     http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /activities/translate", a.handleTranslateGET)
	mux.HandleFunc("POST /activities/translate/answer", a.handleTranslatePOST)
	mux.HandleFunc("GET /settings", a.handleSettingsGET)
	mux.HandleFunc("POST /settings", a.handleSettingsPOST)
	mux.HandleFunc("GET /sessions/{id}", a.handleSummary)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("GET /static/", a.static)
	return mux
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/activities/translate", http.StatusSeeOther)
}

func (a *App) handleSettingsGET(w http.ResponseWriter, r *http.Request) {
	values, err := a.translateSettings(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	view := a.settingsView(values, "")
	view.Saved = r.URL.Query().Get("saved") == "1"
	a.render(w, view, false)
}

func (a *App) handleSettingsPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.fail(w, err)
		return
	}
	submitted := map[string]string{}
	for _, def := range a.player.Settings() {
		submitted[def.Key] = r.FormValue(def.Key)
	}
	clean, err := a.player.Validate(submitted)
	if err != nil {
		msg := ui.T("error.invalid")
		if translate.IsSameLanguage(err) {
			msg = ui.T("error.same_language")
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		a.render(w, a.settingsView(submitted, msg), false)
		return
	}
	stored := map[string]string{}
	prefix := a.player.Info().ID + "."
	for k, v := range clean {
		stored[prefix+k] = v
	}
	if err := profile.SaveSettings(r.Context(), a.profile, stored); err != nil {
		a.fail(w, err)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (a *App) handleSummary(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		a.notFound(w)
		return
	}
	sess, err := profile.LoadSession(r.Context(), a.profile, id)
	if err != nil {
		if err == sql.ErrNoRows {
			a.notFound(w)
			return
		}
		a.fail(w, err)
		return
	}
	attempts, err := profile.Attempts(r.Context(), a.profile, id)
	if err != nil {
		a.fail(w, err)
		return
	}
	view := a.base()
	view.Kind = "summary"
	view.Title = ui.T("summary.title")
	view.SessionID = sess.ID
	view.ShowScore = sess.Mode == "quiz"
	view.Total = formatScore(sess.Score)
	view.Max = strconv.Itoa(sess.QuestionCount)
	view.Percent = percent(sess.Score, sess.QuestionCount)
	for _, attempt := range attempts {
		given := attempt.Given
		if strings.TrimSpace(given) == "" {
			given = "—"
		}
		view.Rows = append(view.Rows, rowView{
			Prompt:  attempt.PromptText,
			Given:   given,
			Outcome: outcomeLabel(attempt.Outcome),
			Points:  formatScore(attempt.Points),
		})
	}
	a.render(w, view, false)
}

func (a *App) settingsView(values map[string]string, message string) View {
	view := a.base()
	view.Kind = "settings"
	view.Title = ui.T("settings.title")
	view.SettingsActive = true
	view.Error = message
	if values == nil {
		values = activity.Coerce(a.player.Settings(), nil)
	}
	for _, def := range a.player.Settings() {
		field := fieldView{
			Key:   def.Key,
			Label: def.Label,
			Help:  def.Help,
			Kind:  def.Kind,
			Value: values[def.Key],
			Min:   def.Min,
			Max:   def.Max,
		}
		if field.Value == "" {
			field.Value = def.Default
		}
		for _, opt := range def.Options {
			field.Options = append(field.Options, optionView{
				Value:    opt.Value,
				Label:    opt.Label,
				Selected: opt.Value == field.Value,
			})
		}
		view.Fields = append(view.Fields, field)
	}
	return view
}

func (a *App) translateSettings(ctx context.Context) (map[string]string, error) {
	raw, err := profile.LoadSettings(ctx, a.profile)
	if err != nil {
		return nil, err
	}
	prefix := a.player.Info().ID + "."
	stored := map[string]string{}
	for k, v := range raw {
		rest, ok := strings.CutPrefix(k, prefix)
		if ok {
			stored[rest] = v
		}
	}
	return activity.Coerce(a.player.Settings(), stored), nil
}

func (a *App) base() View {
	view := View{Activities: make([]activity.Info, 0, len(a.activities))}
	for _, act := range a.activities {
		view.Activities = append(view.Activities, act.Info())
	}
	return view
}

func (a *App) emptyView() View {
	view := a.base()
	view.Kind = "empty"
	view.Title = ui.T("activity.translate")
	view.CurrentActivity = a.player.Info().ID
	view.ImportCommand = importCommand
	return view
}

func (a *App) noItemsView() View {
	view := a.base()
	view.Kind = "message"
	view.Title = ui.T("activity.translate")
	view.CurrentActivity = a.player.Info().ID
	view.MessageTitle = ui.T("message.no_items_title")
	view.Message = ui.T("error.no_items")
	view.MessageLink = "/settings"
	view.MessageLinkLabel = ui.T("message.settings_link")
	return view
}

func (a *App) notFound(w http.ResponseWriter) {
	view := a.base()
	view.Kind = "message"
	view.Title = ui.T("summary.title")
	view.MessageTitle = ui.T("error.not_found")
	view.Message = ui.T("error.not_found")
	view.MessageLink = "/activities/translate"
	view.MessageLinkLabel = ui.T("summary.back")
	w.WriteHeader(http.StatusNotFound)
	a.render(w, view, false)
}

func (a *App) render(w http.ResponseWriter, view View, fragment bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	name := "shell"
	if fragment {
		name = "stage"
	}
	if err := a.tmpl.ExecuteTemplate(w, name, view); err != nil {
		log.Printf("plantilla: %v", err)
	}
}

func (a *App) fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, ui.T("error.generic"), http.StatusInternalServerError)
}

func (a *App) countItems(r *http.Request) (int, error) {
	return content.Store{DB: a.content}.CountItems(r.Context())
}

type View struct {
	Kind             string
	Title            string
	CurrentActivity  string
	SettingsActive   bool
	Activities       []activity.Info
	HasResult        bool
	Outcome          string
	OutcomeLabel     string
	Given            string
	Accepted         []string
	ResultBCP47      string
	Points           string
	ShowScore        bool
	Progress         string
	Finished         bool
	HasPrompt        bool
	Word             string
	Instruction      string
	PromptProgress   string
	Input            string
	Choices          []translate.Choice
	SessionID        int64
	ItemID           int64
	PromptBCP47      string
	AnswerBCP47      string
	Total            string
	Max              string
	Percent          string
	Fields           []fieldView
	Error            string
	Saved            bool
	Rows             []rowView
	ImportCommand    string
	MessageTitle     string
	Message          string
	MessageLink      string
	MessageLinkLabel string
}

type fieldView struct {
	Key     string
	Label   string
	Help    string
	Kind    string
	Value   string
	Min     int
	Max     int
	Options []optionView
}

type optionView struct {
	Value    string
	Label    string
	Selected bool
}

type rowView struct {
	Prompt  string
	Given   string
	Outcome string
	Points  string
}

type resultState struct {
	Outcome    string
	Given      string
	Accepted   []string
	AnswerLang string
	Points     float64
}

func (a *App) playView(sess profile.Session, p translate.Prompt, result *resultState) View {
	view := a.base()
	view.Kind = "play"
	view.Title = ui.T("activity.translate")
	view.CurrentActivity = a.player.Info().ID
	view.SessionID = sess.ID
	if result != nil {
		view.HasResult = true
		view.Outcome = result.Outcome
		view.OutcomeLabel = outcomeLabel(result.Outcome)
		view.Given = result.Given
		if strings.TrimSpace(view.Given) == "" {
			view.Given = "—"
		}
		view.Accepted = result.Accepted
		view.ResultBCP47 = bcp(result.AnswerLang)
		view.Points = formatScore(result.Points)
		if sess.Mode == "quiz" {
			view.ShowScore = true
			view.Progress = fmt.Sprintf(ui.T("result.running"), sess.Asked, sess.QuestionCount, formatScore(sess.Score))
		}
	}
	if sess.Finished {
		view.Finished = true
		view.ShowScore = true
		view.Total = formatScore(sess.Score)
		view.Max = strconv.Itoa(sess.QuestionCount)
		view.Percent = percent(sess.Score, sess.QuestionCount)
		return view
	}
	view.HasPrompt = true
	view.Word = p.Text
	view.Instruction = instruction(p.AnswerLang)
	view.Input = p.Input
	view.Choices = p.Choices
	view.ItemID = p.ItemID
	view.PromptBCP47 = bcp(p.PromptLang)
	view.AnswerBCP47 = bcp(p.AnswerLang)
	if sess.Mode == "quiz" {
		view.PromptProgress = fmt.Sprintf(ui.T("prompt.question"), sess.Asked+1, sess.QuestionCount)
	} else {
		view.PromptProgress = ui.T("prompt.instant")
	}
	return view
}

func outcomeLabel(outcome string) string {
	switch outcome {
	case grade.Exact:
		return ui.T("result.exact")
	case grade.Partial:
		return ui.T("result.partial")
	default:
		return ui.T("result.wrong")
	}
}

func instruction(answerLang string) string {
	if answerLang == language.Russian.Code {
		return ui.T("prompt.to_rus")
	}
	return ui.T("prompt.to_eng")
}

func bcp(code string) string {
	if lang, ok := language.ByCode(code); ok {
		return lang.BCP47()
	}
	return "und"
}

func formatScore(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	s = strings.TrimSuffix(s, ".0")
	return strings.ReplaceAll(s, ".", ",")
}

func percent(score float64, n int) string {
	if n <= 0 {
		return "0"
	}
	return strconv.Itoa(int(math.Round(score / float64(n) * 100)))
}

func atoi(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
