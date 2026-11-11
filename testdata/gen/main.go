package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	type pair struct {
		eng, rus string
		n        int
	}
	pairs := []pair{
		{"hello", "привет", 40},
		{"yes", "да", 36},
		{"no", "нет", 32},
		{"thanks", "спасибо", 28},
		{"water", "вода", 24},
		{"book", "книга", 16},
		{"house", "дом", 14},
		{"friend", "друг", 12},
		{"city", "город", 8},
		{"bread", "хлеб", 6},
		{"window", "окно", 4},
		{"umbrella", "зонт", 3},
		{"hedgehog", "ёж", 2},
		{"curiosity", "любопытство", 1},
		{"silence", "тишина", 1},
	}
	var eng, rus, links []string
	eid, rid := 1, 100001
	for _, p := range pairs {
		for i := 0; i < p.n; i++ {
			et, rt := p.eng, p.rus
			if p.eng == "hello" {
				if i%2 == 0 {
					et = "Hello"
				} else {
					et = "hello"
				}
			}
			if p.eng == "hedgehog" {
				if i == 0 {
					rt = "Ёж"
				} else {
					rt = "Еж"
				}
			}
			eng = append(eng, fmt.Sprintf("%d\teng\t%s", eid, et))
			rus = append(rus, fmt.Sprintf("%d\trus\t%s", rid, rt))
			links = append(links, fmt.Sprintf("%d\t%d", eid, rid))
			eid++
			rid++
		}
	}
	eng = append(eng, "900001\teng\thello!")
	links = append(links, "900001\t100001")
	for i := 0; i < 3; i++ {
		id := 200001 + i
		rus = append(rus, fmt.Sprintf("%d\trus\tздравствуй", id))
		links = append(links, fmt.Sprintf("%d\t%d", i+1, id))
	}
	eng = append(eng, "900010\teng\tgood morning")
	rus = append(rus, "9000100\trus\tдоброе утро")
	links = append(links, "900010\t9000100")
	eng = append(eng, "900020\teng\tone two three four five six seven eight nine ten eleven twelve thirteen")
	eng = append(eng, "900030\teng\ta1")
	rus = append(rus, "9000300\trus\tтест")
	links = append(links, "900030\t9000300")
	eng = append(eng, "900031\teng\tI")
	rus = append(rus, "9000310\trus\tя")
	links = append(links, "900031\t9000310")

	dir := filepath.Join("testdata", "tatoeba")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	write(filepath.Join(dir, "eng_sentences.tsv"), eng)
	write(filepath.Join(dir, "rus_sentences.tsv"), rus)
	write(filepath.Join(dir, "links.tsv"), links)
}

func write(path string, lines []string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			panic(err)
		}
	}
}
