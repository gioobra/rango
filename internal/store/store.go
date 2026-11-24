// Package store guarda os dados do Rango em SQLite: migrations, seed da TACO e repositórios.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // driver database/sql
)

//go:embed migrations/*.sql
var migrations embed.FS

// ErrNaoEncontrado indica que o registro pedido não existe.
var ErrNaoEncontrado = errors.New("não encontrado")

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Repo expõe as consultas. Fora de transação usa o banco; dentro, a transação.
type Repo struct {
	q querier
}

// Store é o banco aberto.
type Store struct {
	*Repo
	db *sql.DB
}

// CaminhoPadrao devolve o caminho do banco seguindo o XDG.
func CaminhoPadrao() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "rango", "rango.db"), nil
}

// Abrir abre (ou cria) o banco, aplica as migrations e carrega a TACO na primeira execução.
// Use ":memory:" para um banco temporário.
func Abrir(ctx context.Context, caminho string) (*Store, error) {
	if caminho != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
			return nil, err
		}
	}
	dsn := "file:" + caminho + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Uso pessoal: uma conexão evita disputa de escrita e mantém o :memory: único.
	db.SetMaxOpenConns(1)
	s := &Store{Repo: &Repo{q: db}, db: db}
	if err := s.migrar(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	if err := s.semear(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed da TACO: %w", err)
	}
	return s, nil
}

// Fechar fecha o banco.
func (s *Store) Fechar() error { return s.db.Close() }

// Tx roda fn numa transação. Erro em fn desfaz tudo.
func (s *Store) Tx(ctx context.Context, fn func(r *Repo) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&Repo{q: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) migrar(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		versao TEXT PRIMARY KEY, aplicada_em TEXT NOT NULL)`); err != nil {
		return err
	}
	nomes, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(nomes)
	for _, nome := range nomes {
		versao := strings.TrimSuffix(filepath.Base(nome), ".sql")
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE versao = ?`, versao).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		sqlTexto, err := migrations.ReadFile(nome)
		if err != nil {
			return err
		}
		err = s.Tx(ctx, func(r *Repo) error {
			if _, err := r.q.ExecContext(ctx, string(sqlTexto)); err != nil {
				return fmt.Errorf("%s: %w", versao, err)
			}
			_, err := r.q.ExecContext(ctx, `INSERT INTO schema_migrations (versao, aplicada_em) VALUES (?, ?)`,
				versao, time.Now().Format(time.RFC3339))
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Normalizar deixa o texto minúsculo e sem acentos, para comparar na busca.
func Normalizar(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'â', 'ã', 'ä':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'í', 'ì', 'î', 'ï':
			r = 'i'
		case 'ó', 'ò', 'ô', 'õ', 'ö':
			r = 'o'
		case 'ú', 'ù', 'û', 'ü':
			r = 'u'
		case 'ç':
			r = 'c'
		case 'ñ':
			r = 'n'
		}
		b.WriteRune(r)
	}
	return b.String()
}
