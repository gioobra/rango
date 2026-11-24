// Comando rango: diário alimentar local.
//
//	rango serve   sobe a interface web
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gioobra/rango/internal/core"
	"github.com/gioobra/rango/internal/store"
	"github.com/gioobra/rango/internal/web"
)

const uso = `uso: rango <comando>

comandos:
  serve   sobe a interface web

variáveis de ambiente:
  RANGO_SENHA   senha da web (obrigatória)
  RANGO_ADDR    endereço de escuta (padrão 127.0.0.1:8080; aceita localhost ou IP da Tailscale)
  RANGO_DB      caminho do banco (padrão ~/.local/share/rango/rango.db)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, uso)
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:], log)
	case "-h", "--help", "help":
		fmt.Print(uso)
	default:
		fmt.Fprintf(os.Stderr, "comando desconhecido: %s\n\n%s", os.Args[1], uso)
		os.Exit(2)
	}
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func env(nome, padrao string) string {
	if v := os.Getenv(nome); v != "" {
		return v
	}
	return padrao
}

func abrirBanco(ctx context.Context) (*store.Store, error) {
	caminho := os.Getenv("RANGO_DB")
	if caminho == "" {
		var err error
		if caminho, err = store.CaminhoPadrao(); err != nil {
			return nil, err
		}
	}
	return store.Abrir(ctx, caminho)
}

func serve(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", env("RANGO_ADDR", "127.0.0.1:8080"), "endereço de escuta")
	_ = fs.Parse(args)

	if err := web.ValidarEndereco(*addr); err != nil {
		return err
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	db, err := abrirBanco(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Fechar() }()

	srv, err := web.Novo(core.Novo(db), os.Getenv("RANGO_SENHA"), log)
	if err != nil {
		return err
	}
	h := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Shutdown(c)
	}()
	log.Info("Rango no ar", "endereco", "http://"+*addr)
	if err := h.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
