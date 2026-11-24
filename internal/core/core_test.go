package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gioobra/rango/internal/store"
)

func novoServico(t *testing.T) (*Servico, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Fechar() })
	s := Novo(db)
	s.agora = func() time.Time { return time.Date(2026, 11, 23, 12, 0, 0, 0, time.Local) }
	return s, ctx
}

func primeiro(t *testing.T, s *Servico, ctx context.Context, busca string) store.Alimento {
	t.Helper()
	as, err := s.BuscarAlimentos(ctx, busca, 5)
	if err != nil || len(as) == 0 {
		t.Fatalf("busca %q: %v %v", busca, as, err)
	}
	return as[0]
}

func TestMacrosDe(t *testing.T) {
	a := store.Alimento{Kcal: 124, Prot: 2.6, Carb: 25.8, Gord: 1.0, Fibra: 2.7}
	got := MacrosDe(a, 150)
	want := Macros{Kcal: 186, Prot: 3.9, Carb: 38.7, Gord: 1.5, Fibra: 4.1}
	if got != want {
		t.Fatalf("MacrosDe = %+v, quer %+v", got, want)
	}
	if z := MacrosDe(a, 0); z != (Macros{}) {
		t.Fatalf("0 g deveria dar zero, deu %+v", z)
	}
}

func TestBuscaIgnoraAcento(t *testing.T) {
	s, ctx := novoServico(t)
	a := primeiro(t, s, ctx, "feijao carioca cozido")
	if a.Nome != "Feijão, carioca, cozido" {
		t.Fatalf("achou %q", a.Nome)
	}
}

func TestRegistrarCongelaMacrosEDiarioSoma(t *testing.T) {
	s, ctx := novoServico(t)
	arroz := primeiro(t, s, ctx, "arroz tipo 1 cozido")
	feijao := primeiro(t, s, ctx, "feijao carioca cozido")
	ls, err := s.Registrar(ctx, Registro{
		Data: s.Hoje(), Refeicao: "almoco", Origem: OrigemManual,
		Itens: []Item{{AlimentoID: arroz.ID, Gramas: 150}, {AlimentoID: feijao.ID, Gramas: 90, Estimado: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := MacrosDe(arroz, 150).Kcal; ls[0].Kcal != want {
		t.Fatalf("kcal do arroz = %v, quer %v", ls[0].Kcal, want)
	}
	d, err := s.Diario(ctx, s.Hoje())
	if err != nil {
		t.Fatal(err)
	}
	want := arred(ls[0].Kcal + ls[1].Kcal)
	if d.Totais.Kcal != want || d.Refeicoes[1].Totais.Kcal != want || len(d.Refeicoes[1].Itens) != 2 {
		t.Fatalf("totais do dia errados: %+v", d.Totais)
	}
	if !d.Refeicoes[1].Itens[1].Estimado {
		t.Fatal("item estimado perdeu a marca")
	}
}

func TestRegistrarEhAtomico(t *testing.T) {
	s, ctx := novoServico(t)
	arroz := primeiro(t, s, ctx, "arroz")
	_, err := s.Registrar(ctx, Registro{
		Data: s.Hoje(), Refeicao: "jantar", Origem: OrigemAgente,
		Itens: []Item{{AlimentoID: arroz.ID, Gramas: 100}, {AlimentoID: 999999, Gramas: 100}},
	})
	if err == nil {
		t.Fatal("deveria falhar com alimento inexistente")
	}
	d, _ := s.Diario(ctx, s.Hoje())
	if d.Totais.Kcal != 0 {
		t.Fatal("o primeiro item não deveria ter ficado gravado")
	}
}

func TestEditarReescalaValoresCongelados(t *testing.T) {
	s, ctx := novoServico(t)
	arroz := primeiro(t, s, ctx, "arroz")
	ls, _ := s.Registrar(ctx, Registro{Data: s.Hoje(), Refeicao: "almoco", Origem: OrigemManual,
		Itens: []Item{{AlimentoID: arroz.ID, Gramas: 100}}})
	g := 200.0
	l, err := s.Editar(ctx, ls[0].ID, Edicao{Gramas: &g}, OrigemAgente)
	if err != nil {
		t.Fatal(err)
	}
	if l.Kcal != arred(ls[0].Kcal*2) || l.Gramas != 200 {
		t.Fatalf("edição não dobrou: %+v", l)
	}
}

func TestDesfazerReverteNaOrdemInversa(t *testing.T) {
	s, ctx := novoServico(t)
	arroz := primeiro(t, s, ctx, "arroz")
	ls, _ := s.Registrar(ctx, Registro{Data: s.Hoje(), Refeicao: "almoco", Origem: OrigemManual,
		Itens: []Item{{AlimentoID: arroz.ID, Gramas: 100}}})
	id := ls[0].ID
	g := 250.0
	if _, err := s.Editar(ctx, id, Edicao{Gramas: &g}, OrigemAgente); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remover(ctx, id, OrigemAgente); err != nil {
		t.Fatal(err)
	}
	kcalDia := func() float64 {
		d, err := s.Diario(ctx, s.Hoje())
		if err != nil {
			t.Fatal(err)
		}
		return d.Totais.Kcal
	}
	if kcalDia() != 0 {
		t.Fatal("removido ainda aparece no dia")
	}

	passos := []struct {
		acao string
		kcal float64
	}{
		{"remover", arred(ls[0].Kcal * 2.5)}, // volta o item com 250 g
		{"editar", ls[0].Kcal},               // volta para 100 g
		{"criar", 0},                         // some o registro
	}
	for _, p := range passos {
		d, err := s.Desfazer(ctx, OrigemManual)
		if err != nil {
			t.Fatal(err)
		}
		if d.Acao != p.acao || kcalDia() != p.kcal {
			t.Fatalf("desfazer %s: ação %s, kcal %v (quer %v)", p.acao, d.Acao, kcalDia(), p.kcal)
		}
	}
	if _, err := s.Desfazer(ctx, OrigemManual); !errors.Is(err, ErrNadaADesfazer) {
		t.Fatalf("esperava ErrNadaADesfazer, veio %v", err)
	}
}

func TestMetaVigente(t *testing.T) {
	s, ctx := novoServico(t)
	if _, ok, _ := s.Metas(ctx, s.Hoje()); ok {
		t.Fatal("sem meta definida ainda")
	}
	if _, err := s.DefinirMetas(ctx, 2200, 130, 240, 65); err != nil {
		t.Fatal(err)
	}
	if m, ok, _ := s.Metas(ctx, s.Hoje().AddDate(0, 0, 3)); !ok || m.Kcal != 2200 {
		t.Fatal("meta deveria valer nos dias seguintes")
	}
	if _, ok, _ := s.Metas(ctx, s.Hoje().AddDate(0, 0, -1)); ok {
		t.Fatal("meta não vale antes da vigência")
	}
}
