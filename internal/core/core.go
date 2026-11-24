// Package core tem as regras do Rango: busca, cálculo de macros, diário, metas e desfazer.
// Não conhece HTTP, MCP nem SQL.
package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gioobra/rango/internal/store"
)

// FormatoData é o formato das datas guardadas no banco.
const FormatoData = "2006-01-02"

// Origens de uma mudança.
const (
	OrigemManual = "manual"
	OrigemAgente = "agente"
)

// Refeicao é uma das refeições do dia.
type Refeicao struct {
	Chave string
	Nome  string
}

// Refeicoes em ordem ao longo do dia.
var Refeicoes = []Refeicao{
	{"cafe", "Café da manhã"},
	{"almoco", "Almoço"},
	{"lanche", "Lanche"},
	{"jantar", "Jantar"},
}

// NomeRefeicao devolve o nome legível da refeição.
func NomeRefeicao(chave string) string {
	for _, r := range Refeicoes {
		if r.Chave == chave {
			return r.Nome
		}
	}
	return chave
}

// ErroValidacao é um erro causado por entrada inválida; a mensagem pode ir ao usuário.
type ErroValidacao struct{ Msg string }

func (e *ErroValidacao) Error() string { return e.Msg }

func invalido(format string, args ...any) error {
	return &ErroValidacao{Msg: fmt.Sprintf(format, args...)}
}

// EhValidacao diz se o erro é de entrada inválida.
func EhValidacao(err error) bool {
	var v *ErroValidacao
	return errors.As(err, &v)
}

// ErrNaoEncontrado indica que o registro pedido não existe.
var ErrNaoEncontrado = store.ErrNaoEncontrado

// Servico reúne as operações do Rango.
type Servico struct {
	db    *store.Store
	agora func() time.Time
}

// Novo cria o serviço sobre um banco aberto.
func Novo(db *store.Store) *Servico {
	return &Servico{db: db, agora: time.Now}
}

// Macros de uma quantidade de alimento.
type Macros struct {
	Kcal  float64
	Prot  float64
	Carb  float64
	Gord  float64
	Fibra float64
}

func arred(v float64) float64 { return math.Round(v*10) / 10 }

// MacrosDe calcula os macros de gramas de um alimento (que tem valores por 100 g),
// arredondados a uma casa decimal.
func MacrosDe(a store.Alimento, gramas float64) Macros {
	f := gramas / 100
	return Macros{
		Kcal:  arred(a.Kcal * f),
		Prot:  arred(a.Prot * f),
		Carb:  arred(a.Carb * f),
		Gord:  arred(a.Gord * f),
		Fibra: arred(a.Fibra * f),
	}
}

// BuscarAlimentos procura pelo nome, ignorando acentos e maiúsculas.
func (s *Servico) BuscarAlimentos(ctx context.Context, texto string, limite int) ([]store.Alimento, error) {
	termos := strings.FieldsFunc(store.Normalizar(texto), func(r rune) bool {
		return r == ' ' || r == ',' || r == '.'
	})
	if len(termos) == 0 {
		return nil, nil
	}
	if limite <= 0 || limite > 50 {
		limite = 20
	}
	return s.db.BuscarAlimentos(ctx, termos, limite)
}

// Alimento devolve um alimento e suas porções salvas.
func (s *Servico) Alimento(ctx context.Context, id int64) (store.Alimento, []store.Porcao, error) {
	a, err := s.db.Alimento(ctx, id)
	if err != nil {
		return a, nil, err
	}
	p, err := s.db.Porcoes(ctx, id)
	return a, p, err
}

// DefinirPorcao salva uma medida caseira ("1 concha" = 90 g) para o alimento.
func (s *Servico) DefinirPorcao(ctx context.Context, alimentoID int64, nome string, gramas float64) (store.Porcao, error) {
	nome = strings.TrimSpace(nome)
	if nome == "" {
		return store.Porcao{}, invalido("dê um nome à porção")
	}
	if gramas <= 0 || gramas > 5000 {
		return store.Porcao{}, invalido("a porção precisa ter entre 0 e 5000 g")
	}
	if _, err := s.db.Alimento(ctx, alimentoID); err != nil {
		return store.Porcao{}, err
	}
	return s.db.SalvarPorcao(ctx, store.Porcao{AlimentoID: alimentoID, Nome: nome, Gramas: gramas})
}

// Metas devolve a meta vigente na data; ok é falso se nenhuma foi definida.
func (s *Servico) Metas(ctx context.Context, data time.Time) (m store.Meta, ok bool, err error) {
	m, err = s.db.MetaVigente(ctx, data.Format(FormatoData))
	if errors.Is(err, store.ErrNaoEncontrado) {
		return m, false, nil
	}
	return m, err == nil, err
}

// DefinirMetas grava a meta diária, valendo a partir de hoje.
func (s *Servico) DefinirMetas(ctx context.Context, kcal, prot, carb, gord float64) (store.Meta, error) {
	for _, v := range []float64{kcal, prot, carb, gord} {
		if v < 0 || v > 20000 {
			return store.Meta{}, invalido("valores de meta fora do razoável")
		}
	}
	if kcal == 0 {
		return store.Meta{}, invalido("a meta de calorias é obrigatória")
	}
	m := store.Meta{VigenteDesde: s.agora().Format(FormatoData), Kcal: kcal, Prot: prot, Carb: carb, Gord: gord}
	return m, s.db.SalvarMeta(ctx, m)
}
