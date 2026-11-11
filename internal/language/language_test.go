package language

import "testing"

func TestNormalize(t *testing.T) {
	if got := Russian.Normalize("  Ёлка! "); got != "елка" {
		t.Fatalf("Normalize(Ёлка) = %q", got)
	}
	if got := Russian.Normalize("Елка"); got != "елка" {
		t.Fatalf("Normalize(Елка) = %q", got)
	}
	if got := English.Normalize("Hello,"); got != "hello" {
		t.Fatalf("Normalize(Hello,) = %q", got)
	}
	if Russian.Normalize("Ёж") != Russian.Normalize("Еж") {
		t.Fatal("ё and е should fold together")
	}
}

func TestEligibleWord(t *testing.T) {
	if English.EligibleWord("I") || English.EligibleWord("a1") || English.EligibleWord("good morning") {
		t.Fatal("short, numeric, or multi-token text should be rejected")
	}
	if !English.EligibleWord("hello!") || !Russian.EligibleWord("Ёж") {
		t.Fatal("single words of at least two letters should be eligible")
	}
	if English.TokenCount("good morning") != 2 || Russian.TokenCount("привет") != 1 {
		t.Fatal("unexpected token count")
	}
}
