package main

import (
	"log"
	"os"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/server"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	data := os.Getenv("APP_DATA")
	if data == "" {
		data = "data"
	}
	if err := server.Run(addr, data); err != nil {
		log.Fatal(err)
	}
}
