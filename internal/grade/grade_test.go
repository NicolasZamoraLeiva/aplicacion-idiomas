package grade

import "testing"

func TestDistance(t *testing.T) {
	if Distance("kitten", "sitting") != 3 {
		t.Fatalf("kitten/sitting = %d", Distance("kitten", "sitting"))
	}
	if Distance("привет", "привт") != 1 {
		t.Fatalf("one missing cyrillic letter = %d", Distance("привет", "привт"))
	}
	if Distance("same", "same") != 0 || Distance("", "ab") != 2 {
		t.Fatal("unexpected distance")
	}
}

func TestEvaluate(t *testing.T) {
	accepted := []string{"привет", "здравствуй"}
	exact := Evaluate("здравствуй", accepted, 0)
	if exact.Outcome != Exact || exact.Points != 1 {
		t.Fatalf("second translation: %+v", exact)
	}
	partial := Evaluate("привт", accepted, 1)
	if partial.Outcome != Partial || partial.Points != 0.5 || partial.Distance != 1 {
		t.Fatalf("partial: %+v", partial)
	}
	wrong := Evaluate("привт", accepted, 0)
	if wrong.Outcome != Wrong || wrong.Points != 0 {
		t.Fatalf("outside tolerance: %+v", wrong)
	}
	empty := Evaluate("", accepted, 5)
	if empty.Outcome != Wrong || empty.Distance != -1 || empty.Points != 0 {
		t.Fatalf("empty answer: %+v", empty)
	}
}
