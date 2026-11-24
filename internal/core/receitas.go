package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gioobra/rango/internal/store"
)

// NovaReceita descreve uma panela: ingredientes, peso pronto e em quantas porções ela rende.
type NovaReceita struct {
	Nome        string
	Itens       []store.ItemReceita // só AlimentoID e Gramas são lidos
	RendimentoG float64             // peso pronto; 0 usa a soma dos ingredientes
	NPorcoes    int                 // 0 vale 1
	NomePorcao  string              // "" vale "1 porção"
}

// Receita completa, com ingredientes e a porção calculada.
type Receita struct {
	store.Receita
	Itens        []ItemDaReceita
	PorcaoG      float64
	MacrosPorcao Macros
	Total        Macros // a panela inteira
	// PesoInformado é falso quando o rendimento é só a soma dos ingredientes.
	PesoInformado bool
}

// ItemDaReceita é um ingrediente com os macros da quantidade usada.
type ItemDaReceita struct {
	store.ItemReceita
	Macros Macros
}

// NovoAlimento é um alimento do usuário, com valores por 100 g (de um rótulo, por exemplo).
type NovoAlimento struct {
	Nome      string
	Categoria string
	Por100g   Macros
}

func validarMacros100(m Macros) error {
	for _, v := range []float64{m.Kcal, m.Prot, m.Carb, m.Gord, m.Fibra} {
		if v < 0 {
			return invalido("valores nutricionais não podem ser negativos")
		}
	}
	if m.Kcal > 900 || m.Prot+m.Carb+m.Gord+m.Fibra > 100 {
		return invalido("valores por 100 g fora do possível: confira se não são de uma porção")
	}
	if m.Kcal == 0 && m.Prot+m.Carb+m.Gord > 0 {
		return invalido("informe as calorias")
	}
	return nil
}

// CriarAlimento cadastra um alimento do usuário. O nome não pode repetir outro do usuário.
func (s *Servico) CriarAlimento(ctx context.Context, n NovoAlimento) (store.Alimento, error) {
	n.Nome = strings.TrimSpace(n.Nome)
	if n.Nome == "" {
		return store.Alimento{}, invalido("dê um nome ao alimento")
	}
	if err := validarMacros100(n.Por100g); err != nil {
		return store.Alimento{}, err
	}
	a := store.Alimento{Nome: n.Nome, Categoria: strings.TrimSpace(n.Categoria), Fonte: "usuario",
		Kcal: arred(n.Por100g.Kcal), Prot: arred(n.Por100g.Prot), Carb: arred(n.Por100g.Carb),
		Gord: arred(n.Por100g.Gord), Fibra: arred(n.Por100g.Fibra)}
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		if _, err := r.AlimentoPorNome(ctx, "usuario", a.Nome); err == nil {
			return invalido("já existe um alimento seu chamado %q", a.Nome)
		} else if !errors.Is(err, store.ErrNaoEncontrado) {
			return err
		}
		var err error
		a.ID, err = r.InserirAlimento(ctx, a)
		return err
	})
	return a, err
}

// CriarReceita grava a receita como um alimento, com valores por 100 g calculados
// dos ingredientes e uma porção ("1 marmita") de rendimento ÷ porções.
func (s *Servico) CriarReceita(ctx context.Context, n NovaReceita) (Receita, error) {
	return s.salvarReceita(ctx, 0, n)
}

// EditarReceita troca ingredientes, rendimento e porções. Os lançamentos já feitos
// não mudam, porque os macros ficam congelados neles.
func (s *Servico) EditarReceita(ctx context.Context, id int64, n NovaReceita) (Receita, error) {
	if id <= 0 {
		return Receita{}, ErrNaoEncontrado
	}
	return s.salvarReceita(ctx, id, n)
}

func (s *Servico) salvarReceita(ctx context.Context, id int64, n NovaReceita) (Receita, error) {
	n.Nome = strings.TrimSpace(n.Nome)
	n.NomePorcao = strings.TrimSpace(n.NomePorcao)
	if n.Nome == "" {
		return Receita{}, invalido("dê um nome à receita")
	}
	if len(n.Itens) == 0 {
		return Receita{}, invalido("a receita precisa de ao menos um ingrediente")
	}
	if n.NPorcoes == 0 {
		n.NPorcoes = 1
	}
	if n.NPorcoes < 0 || n.NPorcoes > 100 {
		return Receita{}, invalido("o número de porções precisa ficar entre 1 e 100")
	}
	if n.NomePorcao == "" {
		n.NomePorcao = "1 porção"
	}
	if n.RendimentoG < 0 || n.RendimentoG > 50000 {
		return Receita{}, invalido("o peso pronto precisa ficar entre 0 e 50 kg")
	}

	var salvo int64
	err := s.db.Tx(ctx, func(r *store.Repo) error {
		var antiga store.Receita
		if id > 0 {
			var err error
			if antiga, err = r.Receita(ctx, id); err != nil {
				return err
			}
		}
		if outra, err := r.AlimentoPorNome(ctx, "receita", n.Nome); err == nil && outra.ID != id {
			return invalido("já existe uma receita chamada %q", n.Nome)
		} else if err != nil && !errors.Is(err, store.ErrNaoEncontrado) {
			return err
		}

		// Soma sem arredondar e só arredonda o resultado por 100 g.
		var total Macros
		var cru float64
		for _, it := range n.Itens {
			if it.Gramas <= 0 || it.Gramas > 20000 {
				return invalido("cada ingrediente precisa ficar entre 0 e 20 kg")
			}
			if it.AlimentoID == id {
				return invalido("a receita não pode ser ingrediente dela mesma")
			}
			a, err := r.Alimento(ctx, it.AlimentoID)
			if err != nil {
				return err
			}
			f := it.Gramas / 100
			total.Kcal += a.Kcal * f
			total.Prot += a.Prot * f
			total.Carb += a.Carb * f
			total.Gord += a.Gord * f
			total.Fibra += a.Fibra * f
			cru += it.Gramas
		}
		rend := n.RendimentoG
		if rend == 0 {
			rend = cru
		}
		k := 100 / rend
		agora := s.agora().Format(time.RFC3339)
		x := store.Receita{
			Alimento: store.Alimento{ID: id, Nome: n.Nome, Categoria: "Receitas", Fonte: "receita",
				Kcal: arred(total.Kcal * k), Prot: arred(total.Prot * k), Carb: arred(total.Carb * k),
				Gord: arred(total.Gord * k), Fibra: arred(total.Fibra * k)},
			RendimentoG: arred(rend), NPorcoes: n.NPorcoes, NomePorcao: n.NomePorcao,
			CriadoEm: agora, EditadoEm: agora,
		}
		if id == 0 {
			var err error
			if x.ID, err = r.InserirAlimento(ctx, x.Alimento); err != nil {
				return err
			}
		} else {
			if err := r.AtualizarAlimento(ctx, x.Alimento); err != nil {
				return err
			}
			if err := r.RemoverPorcao(ctx, id, antiga.NomePorcao); err != nil {
				return err
			}
		}
		if err := r.SalvarReceita(ctx, x); err != nil {
			return err
		}
		if err := r.TrocarItensReceita(ctx, x.ID, n.Itens); err != nil {
			return err
		}
		_, err := r.SalvarPorcao(ctx, store.Porcao{AlimentoID: x.ID, Nome: n.NomePorcao, Gramas: arred(rend / float64(n.NPorcoes))})
		salvo = x.ID
		return err
	})
	if err != nil {
		return Receita{}, err
	}
	return s.Receita(ctx, salvo)
}

// Receita devolve a receita com ingredientes, porção e totais.
func (s *Servico) Receita(ctx context.Context, id int64) (Receita, error) {
	x, err := s.db.Receita(ctx, id)
	if err != nil {
		return Receita{}, err
	}
	itens, err := s.db.ItensReceita(ctx, id)
	if err != nil {
		return Receita{}, err
	}
	return s.montarReceita(ctx, x, itens)
}

// Receitas lista todas, completas.
func (s *Servico) Receitas(ctx context.Context) ([]Receita, error) {
	xs, err := s.db.Receitas(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Receita, 0, len(xs))
	for _, x := range xs {
		itens, err := s.db.ItensReceita(ctx, x.ID)
		if err != nil {
			return nil, err
		}
		rc, err := s.montarReceita(ctx, x, itens)
		if err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, nil
}

func (s *Servico) montarReceita(ctx context.Context, x store.Receita, itens []store.ItemReceita) (Receita, error) {
	rc := Receita{Receita: x, PorcaoG: arred(x.RendimentoG / float64(x.NPorcoes))}
	rc.MacrosPorcao = MacrosDe(x.Alimento, rc.PorcaoG)
	rc.Total = MacrosDe(x.Alimento, x.RendimentoG)
	var cru float64
	for _, it := range itens {
		cru += it.Gramas
		a, err := s.db.Alimento(ctx, it.AlimentoID)
		if err != nil {
			return Receita{}, err
		}
		rc.Itens = append(rc.Itens, ItemDaReceita{ItemReceita: it, Macros: MacrosDe(a, it.Gramas)})
	}
	rc.PesoInformado = arred(cru) != x.RendimentoG
	return rc, nil
}
