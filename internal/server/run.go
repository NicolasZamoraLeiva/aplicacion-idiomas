package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/NicolasZamoraLeiva/aplicacion-idiomas/internal/storage"
)

func Run(addr, dataDir string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	addr = net.JoinHostPort(host, port)
	contentDB, err := storage.Open(filepath.Join(dataDir, "content.db"))
	if err != nil {
		return err
	}
	defer contentDB.Close()
	profileDB, err := storage.Open(filepath.Join(dataDir, "profile.db"))
	if err != nil {
		return err
	}
	defer profileDB.Close()
	if err := storage.MigrateContent(contentDB); err != nil {
		return err
	}
	if err := storage.MigrateProfile(profileDB); err != nil {
		return err
	}
	app, err := New(contentDB, profileDB)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(ln)
	}()
	fmt.Printf("Escuchando en http://%s\n", ln.Addr())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		log.Printf("servidor: %v", err)
		return err
	}
}
