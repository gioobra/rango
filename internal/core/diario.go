package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gioobra/rango/internal/store"
)

// Item é um alimento a registrar.
type Item struct {
	AlimentoID int64
	Gramas     float64
	Estimado   bool
}

// Registro é uma refeição com um ou mais itens.
type Registro struct {
	Data     time.Time
	Refeicao string
	Origem   string
	Itens    []Item
}

// Edicao muda um lançamento; campos nil ficam como estão.
type Edicao struct {
	Gramas   *float64
	Refeicao *string
	Data     *time.Time
}

func validarRefeicao(r string) error {
	for _, x := range Refeicoes {
		if x.Chave == r {
			return nil
		}
	}
	return invalido("refeição desconhecida: %q", r)
}

func validarOrigem(o string) error {
	if o != OrigemManual && o != OrigemAgente {
		return invalido("origem desconhecida: %q", o)
	}
	return nil
}

func validarGramas(g float64) error {
	if g <= 0 || g > 5000 {
		return invalido("a quantidade precisa ficar entre 0 e 5000 g")
	}
	return nil
}

func instantaneo(l store.Lancamento) *string {
	b, _ := json.Marshal(l)
	s := string(b)
	return &s
}

func (s *Servico) auditar(ctx context.Context, r *store.Repo, acao, origem string, antes, depois *store.Lancamento) error {
	a := store.Auditoria{Acao: acao, Origem: origem, Em: s.agora().Format(time.RFC3339)}
	if antes != nil {
		a.LancamentoID = antes.ID
		a.Antes = instantaneo(*antes)
	}
	if depois != nil {
		a.LancamentoID = depois.ID
		a.Depois = instantaneo(*depois)
	}
	return r.InserirAuditoria(ctx, a)
}

// Registrar grava todos os itens de uma vez, com os macros calculados e congelados.
func (s *Servico) Registrar(ctx context.Context, reg Registro) ([]store.Lancamento, error) {
	if err := validarRefeicao(reg.Refeicao); err != nil {
		return nil, err
	}
	if err := validarOrigem(reg.Origem); err != nil {
		return nil, err
	}
	if len(reg.Itens) == 0 {
		return nil, invalido("nada para registrar")
	}
	var out []store.Lancamento
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		for _, it := range reg.Itens {
			if err := validarGramas(it.Gramas); err != nil {
				return err
			}
			a, err := r.Alimento(ctx, it.AlimentoID)
			if err != nil {
				return fmt.Errorf("alimento %d: %w", it.AlimentoID, err)
			}
			m := MacrosDe(a, it.Gramas)
			l := store.Lancamento{
				Data: reg.Data.Format(FormatoData), Refeicao: reg.Refeicao,
				AlimentoID: a.ID, AlimentoNome: a.Nome, Gramas: it.Gramas,
				Kcal: m.Kcal, Prot: m.Prot, Carb: m.Carb, Gord: m.Gord, Fibra: m.Fibra,
				Origem: reg.Origem, Estimado: it.Estimado, CriadoEm: s.agora().Format(time.RFC3339),
			}
			if l.ID, err = r.InserirLancamento(ctx, l); err != nil {
				return err
			}
			if err := s.auditar(ctx, r, "criar", reg.Origem, nil, &l); err != nil {
				return err
			}
			out = append(out, l)
		}
		return nil
	})
	return out, err
}

// Editar muda quantidade, refeição ou data. Os macros são reescalados a partir
// dos valores congelados, sem consultar o alimento de novo.
func (s *Servico) Editar(ctx context.Context, id int64, e Edicao, origem string) (store.Lancamento, error) {
	if err := validarOrigem(origem); err != nil {
		return store.Lancamento{}, err
	}
	var novo store.Lancamento
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		antes, err := r.Lancamento(ctx, id)
		if err != nil {
			return err
		}
		if antes.RemovidoEm != nil {
			return ErrNaoEncontrado
		}
		novo = antes
		if e.Gramas != nil {
			if err := validarGramas(*e.Gramas); err != nil {
				return err
			}
			f := *e.Gramas / antes.Gramas
			novo.Gramas = *e.Gramas
			novo.Kcal, novo.Prot = arred(antes.Kcal*f), arred(antes.Prot*f)
			novo.Carb, novo.Gord = arred(antes.Carb*f), arred(antes.Gord*f)
			novo.Fibra = arred(antes.Fibra * f)
		}
		if e.Refeicao != nil {
			if err := validarRefeicao(*e.Refeicao); err != nil {
				return err
			}
			novo.Refeicao = *e.Refeicao
		}
		if e.Data != nil {
			novo.Data = e.Data.Format(FormatoData)
		}
		if err := r.AtualizarLancamento(ctx, novo); err != nil {
			return err
		}
		return s.auditar(ctx, r, "editar", origem, &antes, &novo)
	})
	return novo, err
}

// Remover tira o lançamento do diário. Dá para voltar com Desfazer.
func (s *Servico) Remover(ctx context.Context, id int64, origem string) (store.Lancamento, error) {
	if err := validarOrigem(origem); err != nil {
		return store.Lancamento{}, err
	}
	var antes store.Lancamento
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		var err error
		antes, err = s.remover(ctx, r, id, origem)
		return err
	})
	return antes, err
}

// remover faz a remoção lógica e audita. ErrNaoEncontrado se o lançamento não existe ou já saiu.
func (s *Servico) remover(ctx context.Context, r *store.Repo, id int64, origem string) (store.Lancamento, error) {
	antes, err := r.Lancamento(ctx, id)
	if err != nil {
		return antes, err
	}
	if antes.RemovidoEm != nil {
		return antes, ErrNaoEncontrado
	}
	depois := antes
	agora := s.agora().Format(time.RFC3339)
	depois.RemovidoEm = &agora
	if err := r.AtualizarLancamento(ctx, depois); err != nil {
		return antes, err
	}
	return antes, s.auditar(ctx, r, "remover", origem, &antes, &depois)
}

// ErrNadaADesfazer indica que não há mudança pendente para desfazer.
var ErrNadaADesfazer = errors.New("nada para desfazer")

// Desfeito descreve o que Desfazer reverteu.
type Desfeito struct {
	Acao       string // a ação revertida: criar | editar | remover
	Lancamento store.Lancamento
}

// Desfazer reverte a mudança mais recente em lançamentos, de qualquer origem.
func (s *Servico) Desfazer(ctx context.Context, origem string) (Desfeito, error) {
	if err := validarOrigem(origem); err != nil {
		return Desfeito{}, err
	}
	var d Desfeito
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		a, err := r.UltimaAuditoriaDesfazivel(ctx)
		if errors.Is(err, store.ErrNaoEncontrado) {
			return ErrNadaADesfazer
		}
		if err != nil {
			return err
		}
		atual, err := r.Lancamento(ctx, a.LancamentoID)
		if err != nil {
			return err
		}
		var alvo store.Lancamento
		switch a.Acao {
		case "criar":
			alvo = atual
			agora := s.agora().Format(time.RFC3339)
			alvo.RemovidoEm = &agora
		case "editar", "remover":
			if a.Antes == nil {
				return fmt.Errorf("auditoria %d sem estado anterior", a.ID)
			}
			if err := json.Unmarshal([]byte(*a.Antes), &alvo); err != nil {
				return err
			}
		default:
			return fmt.Errorf("ação desconhecida na auditoria: %q", a.Acao)
		}
		if err := r.AtualizarLancamento(ctx, alvo); err != nil {
			return err
		}
		if err := r.MarcarDesfeita(ctx, a.ID); err != nil {
			return err
		}
		d = Desfeito{Acao: a.Acao, Lancamento: alvo}
		return s.auditar(ctx, r, "desfazer", origem, &atual, &alvo)
	})
	return d, err
}

// Totais somados de lançamentos.
type Totais struct {
	Kcal  float64
	Prot  float64
	Carb  float64
	Gord  float64
	Fibra float64
}

func (t *Totais) somar(l store.Lancamento) {
	t.Kcal = arred(t.Kcal + l.Kcal)
	t.Prot = arred(t.Prot + l.Prot)
	t.Carb = arred(t.Carb + l.Carb)
	t.Gord = arred(t.Gord + l.Gord)
	t.Fibra = arred(t.Fibra + l.Fibra)
}

// RefeicaoDoDia agrupa os lançamentos de uma refeição.
type RefeicaoDoDia struct {
	Refeicao
	Itens  []store.Lancamento
	Totais Totais
}

// Dia é o diário de uma data: refeições, totais e a meta vigente.
type Dia struct {
	Data      time.Time
	Refeicoes []RefeicaoDoDia
	Totais    Totais
	Meta      store.Meta
	TemMeta   bool
}

// Diario monta o dia: lançamentos por refeição, totais e meta.
func (s *Servico) Diario(ctx context.Context, data time.Time) (Dia, error) {
	d := Dia{Data: data}
	ls, err := s.db.LancamentosDoDia(ctx, data.Format(FormatoData))
	if err != nil {
		return d, err
	}
	for _, ref := range Refeicoes {
		rd := RefeicaoDoDia{Refeicao: ref}
		for _, l := range ls {
			if l.Refeicao == ref.Chave {
				rd.Itens = append(rd.Itens, l)
				rd.Totais.somar(l)
				d.Totais.somar(l)
			}
		}
		d.Refeicoes = append(d.Refeicoes, rd)
	}
	d.Meta, d.TemMeta, err = s.Metas(ctx, data)
	return d, err
}

// Hoje devolve a data de hoje, sem horário.
func (s *Servico) Hoje() time.Time {
	t := s.agora()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Restante é quanto falta para a meta (negativo quando estourou). Zero sem meta.
func (d Dia) Restante() Totais {
	if !d.TemMeta {
		return Totais{}
	}
	return Totais{
		Kcal: arred(d.Meta.Kcal - d.Totais.Kcal),
		Prot: arred(d.Meta.Prot - d.Totais.Prot),
		Carb: arred(d.Meta.Carb - d.Totais.Carb),
		Gord: arred(d.Meta.Gord - d.Totais.Gord),
	}
}

// RemoverVarios tira do diário os lançamentos ainda ativos da lista, numa transação só.
// Serve para desfazer de uma vez uma refeição registrada pelo Nutri.
func (s *Servico) RemoverVarios(ctx context.Context, ids []int64, origem string) ([]store.Lancamento, error) {
	if err := validarOrigem(origem); err != nil {
		return nil, err
	}
	var out []store.Lancamento
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		for _, id := range ids {
			antes, err := s.remover(ctx, r, id, origem)
			if errors.Is(err, store.ErrNaoEncontrado) {
				continue
			}
			if err != nil {
				return err
			}
			out = append(out, antes)
		}
		return nil
	})
	return out, err
}

// LerData interpreta AAAA-MM-DD; vazio é hoje.
func (s *Servico) LerData(v string) (time.Time, error) {
	if v == "" || v == "hoje" {
		return s.Hoje(), nil
	}
	t, err := time.ParseInLocation(FormatoData, v, time.Local)
	if err != nil {
		return t, invalido("data inválida %q: use AAAA-MM-DD", v)
	}
	return t, nil
}
