package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Alimento tem valores por 100 g.
type Alimento struct {
	ID        int64
	Nome      string
	Categoria string
	Fonte     string // taco | usuario | receita
	Kcal      float64
	Prot      float64
	Carb      float64
	Gord      float64
	Fibra     float64
}

// Porcao é uma medida caseira do usuário ("1 concha" = 90 g).
type Porcao struct {
	ID         int64   `json:"id"`
	AlimentoID int64   `json:"alimento_id"`
	Nome       string  `json:"nome"`
	Gramas     float64 `json:"gramas"`
}

// Lancamento é um item registrado no diário, com macros congelados.
type Lancamento struct {
	ID           int64   `json:"id"`
	Data         string  `json:"data"`
	Refeicao     string  `json:"refeicao"`
	AlimentoID   int64   `json:"alimento_id"`
	AlimentoNome string  `json:"alimento_nome"`
	Gramas       float64 `json:"gramas"`
	Kcal         float64 `json:"kcal"`
	Prot         float64 `json:"prot"`
	Carb         float64 `json:"carb"`
	Gord         float64 `json:"gord"`
	Origem       string  `json:"origem"`
	Estimado     bool    `json:"estimado"`
	CriadoEm     string  `json:"criado_em"`
	RemovidoEm   *string `json:"removido_em,omitempty"`
}

// Meta diária, válida a partir de VigenteDesde.
type Meta struct {
	VigenteDesde string
	Kcal         float64
	Prot         float64
	Carb         float64
	Gord         float64
}

// Auditoria registra cada mudança em lançamentos.
type Auditoria struct {
	ID           int64
	LancamentoID int64
	Acao         string // criar | editar | remover | desfazer
	Antes        *string
	Depois       *string
	Origem       string
	Em           string
}

func achar(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNaoEncontrado
	}
	return err
}

const colsAlimento = `id, nome, categoria, fonte, kcal, prot, carb, gord, fibra`

func scanAlimento(sc interface{ Scan(...any) error }) (Alimento, error) {
	var a Alimento
	err := sc.Scan(&a.ID, &a.Nome, &a.Categoria, &a.Fonte, &a.Kcal, &a.Prot, &a.Carb, &a.Gord, &a.Fibra)
	return a, err
}

// BuscarAlimentos devolve alimentos cujo nome contém todos os termos
// (já normalizados). Os do usuário vêm primeiro, os crus por último, e depois os nomes mais curtos.
func (r *Repo) BuscarAlimentos(ctx context.Context, termos []string, limite int) ([]Alimento, error) {
	var where []string
	var args []any
	for _, t := range termos {
		where = append(where, `nome_busca LIKE ? ESCAPE '\'`)
		t = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(t)
		args = append(args, "%"+t+"%")
	}
	q := `SELECT ` + colsAlimento + ` FROM alimentos`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	// cru por último: no dia a dia, o que se registra é o alimento pronto
	q += ` ORDER BY fonte = 'taco', nome_busca LIKE '%cru%', length(nome), nome LIMIT ?`
	args = append(args, limite)
	rows, err := r.q.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Alimento
	for rows.Next() {
		a, err := scanAlimento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Alimento busca um alimento pelo id.
func (r *Repo) Alimento(ctx context.Context, id int64) (Alimento, error) {
	a, err := scanAlimento(r.q.QueryRowContext(ctx, `SELECT `+colsAlimento+` FROM alimentos WHERE id = ?`, id))
	return a, achar(err)
}

// Porcoes lista as porções salvas de um alimento.
func (r *Repo) Porcoes(ctx context.Context, alimentoID int64) ([]Porcao, error) {
	rows, err := r.q.QueryContext(ctx,
		`SELECT id, alimento_id, nome, gramas FROM porcoes WHERE alimento_id = ? ORDER BY gramas`, alimentoID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Porcao
	for rows.Next() {
		var p Porcao
		if err := rows.Scan(&p.ID, &p.AlimentoID, &p.Nome, &p.Gramas); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SalvarPorcao cria a porção ou atualiza os gramas se o nome já existe.
func (r *Repo) SalvarPorcao(ctx context.Context, p Porcao) (Porcao, error) {
	err := r.q.QueryRowContext(ctx, `INSERT INTO porcoes (alimento_id, nome, gramas) VALUES (?, ?, ?)
		ON CONFLICT (alimento_id, nome) DO UPDATE SET gramas = excluded.gramas
		RETURNING id`, p.AlimentoID, p.Nome, p.Gramas).Scan(&p.ID)
	return p, err
}

const colsLancamento = `l.id, l.data, l.refeicao, l.alimento_id, a.nome, l.gramas, l.kcal, l.prot, l.carb, l.gord,
	l.origem, l.estimado, l.criado_em, l.removido_em`

func scanLancamento(sc interface{ Scan(...any) error }) (Lancamento, error) {
	var l Lancamento
	err := sc.Scan(&l.ID, &l.Data, &l.Refeicao, &l.AlimentoID, &l.AlimentoNome, &l.Gramas,
		&l.Kcal, &l.Prot, &l.Carb, &l.Gord, &l.Origem, &l.Estimado, &l.CriadoEm, &l.RemovidoEm)
	return l, err
}

// InserirLancamento grava um lançamento novo e devolve o id.
func (r *Repo) InserirLancamento(ctx context.Context, l Lancamento) (int64, error) {
	var id int64
	err := r.q.QueryRowContext(ctx, `INSERT INTO lancamentos
		(data, refeicao, alimento_id, gramas, kcal, prot, carb, gord, origem, estimado, criado_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		l.Data, l.Refeicao, l.AlimentoID, l.Gramas, l.Kcal, l.Prot, l.Carb, l.Gord, l.Origem, l.Estimado, l.CriadoEm,
	).Scan(&id)
	return id, err
}

// Lancamento busca um lançamento pelo id, inclusive se removido.
func (r *Repo) Lancamento(ctx context.Context, id int64) (Lancamento, error) {
	l, err := scanLancamento(r.q.QueryRowContext(ctx, `SELECT `+colsLancamento+`
		FROM lancamentos l JOIN alimentos a ON a.id = l.alimento_id WHERE l.id = ?`, id))
	return l, achar(err)
}

// AtualizarLancamento grava todos os campos mutáveis do lançamento.
func (r *Repo) AtualizarLancamento(ctx context.Context, l Lancamento) error {
	_, err := r.q.ExecContext(ctx, `UPDATE lancamentos SET data = ?, refeicao = ?, gramas = ?,
		kcal = ?, prot = ?, carb = ?, gord = ?, estimado = ?, removido_em = ? WHERE id = ?`,
		l.Data, l.Refeicao, l.Gramas, l.Kcal, l.Prot, l.Carb, l.Gord, l.Estimado, l.RemovidoEm, l.ID)
	return err
}

// LancamentosDoDia lista os lançamentos ativos de uma data, na ordem de registro.
func (r *Repo) LancamentosDoDia(ctx context.Context, data string) ([]Lancamento, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+colsLancamento+`
		FROM lancamentos l JOIN alimentos a ON a.id = l.alimento_id
		WHERE l.data = ? AND l.removido_em IS NULL ORDER BY l.id`, data)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Lancamento
	for rows.Next() {
		l, err := scanLancamento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// MetaVigente devolve a meta válida na data. ErrNaoEncontrado se não houver nenhuma.
func (r *Repo) MetaVigente(ctx context.Context, data string) (Meta, error) {
	var m Meta
	err := r.q.QueryRowContext(ctx, `SELECT vigente_desde, kcal, prot, carb, gord FROM metas
		WHERE vigente_desde <= ? ORDER BY vigente_desde DESC LIMIT 1`, data).
		Scan(&m.VigenteDesde, &m.Kcal, &m.Prot, &m.Carb, &m.Gord)
	return m, achar(err)
}

// SalvarMeta grava a meta; a mesma data de vigência é sobrescrita.
func (r *Repo) SalvarMeta(ctx context.Context, m Meta) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO metas (vigente_desde, kcal, prot, carb, gord) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (vigente_desde) DO UPDATE SET kcal = excluded.kcal, prot = excluded.prot,
		carb = excluded.carb, gord = excluded.gord`, m.VigenteDesde, m.Kcal, m.Prot, m.Carb, m.Gord)
	return err
}

// InserirAuditoria registra uma mudança.
func (r *Repo) InserirAuditoria(ctx context.Context, a Auditoria) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO auditoria (lancamento_id, acao, antes, depois, origem, em)
		VALUES (?, ?, ?, ?, ?, ?)`, a.LancamentoID, a.Acao, a.Antes, a.Depois, a.Origem, a.Em)
	return err
}

// UltimaAuditoriaDesfazivel devolve a mudança mais recente que ainda pode ser desfeita.
func (r *Repo) UltimaAuditoriaDesfazivel(ctx context.Context) (Auditoria, error) {
	var a Auditoria
	err := r.q.QueryRowContext(ctx, `SELECT id, lancamento_id, acao, antes, depois, origem, em FROM auditoria
		WHERE acao <> 'desfazer' AND desfeito = 0 ORDER BY id DESC LIMIT 1`).
		Scan(&a.ID, &a.LancamentoID, &a.Acao, &a.Antes, &a.Depois, &a.Origem, &a.Em)
	return a, achar(err)
}

// MarcarDesfeita marca a mudança como desfeita.
func (r *Repo) MarcarDesfeita(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE auditoria SET desfeito = 1 WHERE id = ?`, id)
	return err
}
