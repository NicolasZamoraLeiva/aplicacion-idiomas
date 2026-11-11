package translate

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/content"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/grade"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/storage"
)

func TestDirectionAndGrading(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.MigrateContent(db); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO items(lang, text, normalized, frequency, band, token_count) VALUES('eng','hello','hello',10,1,1)`)
	if err != nil {
		t.Fatal(err)
	}
	helloID, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO item_answers(item_id, answer_lang, answer_text, normalized) VALUES(?, 'rus', 'привет', 'привет'), (?, 'rus', 'здравствуй', 'здравствуй')`, helloID, helloID); err != nil {
		t.Fatal(err)
	}
	res, err = db.Exec(`INSERT INTO items(lang, text, normalized, frequency, band, token_count) VALUES('rus','привет','привет',10,1,1)`)
	if err != nil {
		t.Fatal(err)
	}
	privetID, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO item_answers(item_id, answer_lang, answer_text, normalized) VALUES(?, 'eng', 'hello', 'hello')`, privetID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO items(lang, text, normalized, frequency, band, token_count) VALUES('rus','дом','дом',3,2,1),('rus','вода','вода',3,2,1),('rus','книга','книга',3,2,1)`); err != nil {
		t.Fatal(err)
	}

	svc := &Service{Content: content.Store{DB: db}, Rand: rand.New(rand.NewPCG(1, 2))}
	ctx := context.Background()
	toTarget, err := svc.Next(ctx, Request{UserLang: "eng", TargetLang: "rus", Direction: "to_target", MaxTokens: 1, Input: "choice", OptionCount: 4})
	if err != nil {
		t.Fatal(err)
	}
	if toTarget.PromptLang != "eng" || toTarget.AnswerLang != "rus" || len(toTarget.AcceptedNorm) != 2 {
		t.Fatalf("to target: %+v", toTarget)
	}
	if len(toTarget.Choices) < 2 {
		t.Fatalf("choices: %+v", toTarget.Choices)
	}
	toUser, err := svc.Next(ctx, Request{UserLang: "eng", TargetLang: "rus", Direction: "to_user", MaxTokens: 1, Input: "type", OptionCount: 4})
	if err != nil {
		t.Fatal(err)
	}
	if toUser.PromptLang != "rus" || toUser.AnswerLang != "eng" || toUser.Choices != nil {
		t.Fatalf("to user: %+v", toUser)
	}

	typed := toTarget
	typed.Input = "type"
	if got := svc.Grade(typed, "привт", 1); got.Outcome != grade.Partial {
		t.Fatalf("typed partial: %+v", got)
	}
	if got := svc.Grade(typed, "здравствуй", 0); got.Outcome != grade.Exact {
		t.Fatalf("second accepted answer: %+v", got)
	}
	choice := toTarget
	choice.Input = "choice"
	if got := svc.Grade(choice, "привт", 5); got.Outcome != grade.Wrong {
		t.Fatalf("choice should ignore tolerance: %+v", got)
	}

	seenUser, seenTarget := false, false
	for i := 0; i < 40; i++ {
		p, err := svc.Next(ctx, Request{UserLang: "eng", TargetLang: "rus", Direction: "mixed", MaxTokens: 1, Input: "type"})
		if err != nil {
			t.Fatal(err)
		}
		if p.PromptLang == "eng" {
			seenUser = true
		}
		if p.PromptLang == "rus" {
			seenTarget = true
		}
	}
	if !seenUser || !seenTarget {
		t.Fatal("mixed direction did not use both languages")
	}
}

func TestValidateLanguages(t *testing.T) {
	svc := &Service{}
	values := map[string]string{}
	for _, def := range svc.Settings() {
		values[def.Key] = def.Default
	}
	if _, err := svc.Validate(values); err != nil {
		t.Fatal(err)
	}
	values["target_lang"] = values["user_lang"]
	if _, err := svc.Validate(values); !IsSameLanguage(err) {
		t.Fatalf("err = %v", err)
	}
	if svc.Settings()[len(svc.Settings())-1].Max != 1 {
		t.Fatal("text length is capped at one token")
	}
}
