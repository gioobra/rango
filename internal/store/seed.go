package store

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
)

// taco.csv vem da TACO 4ª edição (NEPA/Unicamp), via a conversão para CSV do
// projeto raulfdm/taco-api. Valores por 100 g; "Tr" e células vazias viraram 0,
// e os alimentos sem kcal na tabela original ficaram de fora.
//
//go:embed seed/taco.csv
var tacoCSV []byte

func (s *Store) semear(ctx context.Context) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM alimentos WHERE fonte = 'taco'`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	linhas, err := csv.NewReader(bytes.NewReader(tacoCSV)).ReadAll()
	if err != nil {
		return err
	}
	return s.Tx(ctx, func(r *Repo) error {
		for i, l := range linhas[1:] { // pula o cabeçalho
			// taco_id, categoria, nome, kcal, prot, carb, gord, fibra
			nums := make([]float64, 5)
			for j := range nums {
				v, err := strconv.ParseFloat(l[3+j], 64)
				if err != nil {
					return fmt.Errorf("linha %d: %w", i+2, err)
				}
				nums[j] = v
			}
			_, err := r.q.ExecContext(ctx, `INSERT INTO alimentos
				(nome, nome_busca, categoria, fonte, taco_id, kcal, prot, carb, gord, fibra)
				VALUES (?, ?, ?, 'taco', ?, ?, ?, ?, ?, ?)`,
				l[2], Normalizar(l[2]), l[1], l[0], nums[0], nums[1], nums[2], nums[3], nums[4])
			if err != nil {
				return fmt.Errorf("linha %d: %w", i+2, err)
			}
		}
		return nil
	})
}
