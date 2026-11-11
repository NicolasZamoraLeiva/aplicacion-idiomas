package grade

const (
	Exact   = "exact"
	Partial = "partial"
	Wrong   = "wrong"
)

type Result struct {
	Outcome  string
	Points   float64
	Distance int
	Matched  string
}

// Evaluate compares an already normalized answer with the accepted forms.
// An empty answer is wrong even when the tolerance would cover the distance.
// Distance is -1 when nothing was compared.
func Evaluate(given string, accepted []string, tolerance int) Result {
	if given == "" {
		return Result{Outcome: Wrong, Points: 0, Distance: -1}
	}
	if tolerance < 0 {
		tolerance = 0
	}
	best := -1
	matched := ""
	for _, candidate := range accepted {
		if candidate == "" {
			continue
		}
		d := Distance(given, candidate)
		if best < 0 || d < best {
			best = d
			matched = candidate
		}
	}
	if best < 0 {
		return Result{Outcome: Wrong, Points: 0, Distance: -1}
	}
	out := Result{Distance: best, Matched: matched}
	switch {
	case best == 0:
		out.Outcome = Exact
		out.Points = 1
	case best <= tolerance:
		out.Outcome = Partial
		out.Points = 0.5
	default:
		out.Outcome = Wrong
		out.Points = 0
	}
	return out
}

// Distance is the Levenshtein edit distance over runes, so one Cyrillic
// letter counts as one edit.
func Distance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = min(del, ins, sub)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}
