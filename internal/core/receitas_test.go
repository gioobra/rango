package core

import (
	"testing"

	"github.com/gioobra/rango/internal/store"
)

func TestReceitaRendimentoEPorcao(t *testing.T) {
	s, ctx := novoServico(t)
	arroz := primeiro(t, s, ctx, "arroz")
	feijao := primeiro(t, s, ctx, "feijao")
	itens := []store.ItemReceita{{AlimentoID: arroz.ID, Gramas: 600}, {AlimentoID: feijao.ID, Gramas: 400}}
	kcalPanela := arroz.Kcal*6 + feijao.Kcal*4

	// Sem peso pronto, rende a soma dos ingredientes.
	rc, err := s.CriarReceita(ctx, NovaReceita{Nome: "Marmita", Itens: itens, NPorcoes: 4, NomePorcao: "1 marmita"})
	if err != nil {
		t.Fatal(err)
	}
	if rc.RendimentoG != 1000 || rc.PorcaoG != 250 || rc.PesoInformado {
		t.Fatalf("rendimento %v, porção %v, informado %v", rc.RendimentoG, rc.PorcaoG, rc.PesoInformado)
	}
	if rc.Kcal != arred(kcalPanela/10) {
		t.Fatalf("kcal por 100 g %v, quer %v", rc.Kcal, arred(kcalPanela/10))
	}
	if d := rc.MacrosPorcao.Kcal - kcalPanela/4; d > 0.5 || d < -0.5 {
		t.Fatalf("kcal da porção %v, quer ~%v", rc.MacrosPorcao.Kcal, kcalPanela/4)
	}
	_, ps, _ := s.Alimento(ctx, rc.ID)
	if len(ps) != 1 || ps[0].Nome != "1 marmita" || ps[0].Gramas != 250 {
		t.Fatalf("porção salva: %+v", ps)
	}

	// Com peso pronto menor (a água evaporou), cada 100 g concentra mais.
	rc2, err := s.CriarReceita(ctx, NovaReceita{Nome: "Marmita seca", Itens: itens, RendimentoG: 800, NPorcoes: 4})
	if err != nil {
		t.Fatal(err)
	}
	if rc2.Kcal != arred(kcalPanela/8) || rc2.PorcaoG != 200 || !rc2.PesoInformado {
		t.Fatalf("kcal por 100 g %v, porção %v", rc2.Kcal, rc2.PorcaoG)
	}

	if _, err := s.CriarReceita(ctx, NovaReceita{Nome: "marmita", Itens: itens}); !EhValidacao(err) {
		t.Fatalf("nome repetido deveria falhar, veio %v", err)
	}
}

func TestEditarReceitaNaoMudaOHistorico(t *testing.T) {
	s, ctx := novoServico(t)
	arroz := primeiro(t, s, ctx, "arroz")
	rc, err := s.CriarReceita(ctx, NovaReceita{Nome: "Arroz da semana",
		Itens: []store.ItemReceita{{AlimentoID: arroz.ID, Gramas: 1000}}, NPorcoes: 5, NomePorcao: "1 pote"})
	if err != nil {
		t.Fatal(err)
	}
	ls, err := s.Registrar(ctx, Registro{Data: s.Hoje(), Refeicao: "almoco", Origem: OrigemManual,
		Itens: []Item{{AlimentoID: rc.ID, Gramas: rc.PorcaoG}}})
	if err != nil {
		t.Fatal(err)
	}

	ed, err := s.EditarReceita(ctx, rc.ID, NovaReceita{Nome: "Arroz da semana",
		Itens: []store.ItemReceita{{AlimentoID: arroz.ID, Gramas: 1000}}, RendimentoG: 500, NPorcoes: 2, NomePorcao: "1 marmita"})
	if err != nil {
		t.Fatal(err)
	}
	if ed.Kcal != arred(rc.Kcal*2) || ed.PorcaoG != 250 {
		t.Fatalf("depois de editar: kcal %v, porção %v", ed.Kcal, ed.PorcaoG)
	}
	_, ps, _ := s.Alimento(ctx, rc.ID)
	if len(ps) != 1 || ps[0].Nome != "1 marmita" {
		t.Fatalf("a porção antiga devia sair: %+v", ps)
	}
	d, _ := s.Diario(ctx, s.Hoje())
	if d.Totais.Kcal != ls[0].Kcal {
		t.Fatalf("o lançamento mudou: %v, era %v", d.Totais.Kcal, ls[0].Kcal)
	}
	if _, err := s.EditarReceita(ctx, rc.ID, NovaReceita{Nome: "x", Itens: []store.ItemReceita{{AlimentoID: rc.ID, Gramas: 10}}}); !EhValidacao(err) {
		t.Fatalf("receita dentro dela mesma deveria falhar, veio %v", err)
	}
}
