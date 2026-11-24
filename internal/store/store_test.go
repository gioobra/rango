package store

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestMigrationsESeed(t *testing.T) {
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "rango.db")
	arquivos, _ := fs.Glob(migrations, "migrations/*.sql")
	for i := range 2 { // a segunda abertura não pode reaplicar nada
		s, err := Abrir(ctx, caminho)
		if err != nil {
			t.Fatalf("abertura %d: %v", i+1, err)
		}
		var versoes, taco int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&versoes); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM alimentos WHERE fonte = 'taco'`).Scan(&taco); err != nil {
			t.Fatal(err)
		}
		if versoes != len(arquivos) || taco != 591 {
			t.Fatalf("abertura %d: %d migrations, %d alimentos da TACO", i+1, versoes, taco)
		}
		_ = s.Fechar()
	}
}
