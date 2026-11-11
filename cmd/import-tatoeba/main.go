package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/content"
	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/storage"
)

func main() {
	eng := flag.String("eng", "", "export de oraciones en inglés (TSV de Tatoeba)")
	rus := flag.String("rus", "", "export de oraciones en ruso (TSV de Tatoeba)")
	links := flag.String("links", "", "links.csv de Tatoeba")
	dbPath := flag.String("db", "data/content.db", "base SQLite de contenido")
	flag.Parse()
	if *eng == "" || *rus == "" || *links == "" {
		fmt.Fprintln(os.Stderr, "Uso: import-tatoeba -eng eng_sentences.tsv -rus rus_sentences.tsv -links links.tsv [-db data/content.db]")
		os.Exit(2)
	}
	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	if err := storage.MigrateContent(db); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stats, err := content.Import(db, *eng, *rus, *links, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Oraciones: %d\nEnlaces: %d\nPalabras: %d\nTraducciones: %d\n", stats.Sentences, stats.Links, stats.Items, stats.Answers)
}
