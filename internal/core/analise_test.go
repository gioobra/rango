package core

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/gioobra/rango/internal/store"
)

func data(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation(FormatoData, s, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDistribuicaoUsa449(t *testing.T) {
	// 100 g P = 400 kcal, 200 g C = 800, 50 g G = 450: total 1650
	d := DistribuicaoDe(Totais{Kcal: 9999, Prot: 100, Carb: 200, Gord: 50})
	if d != (Distribuicao{Prot: 24.2, Carb: 48.5, Gord: 27.3}) {
		t.Fatalf("distribuição %+v", d)
	}
	if DistribuicaoDe(Totais{}) != (Distribuicao{}) {
		t.Fatal("sem macros deveria dar zero")
	}
}

func TestAnaliseIgnoraDiasVaziosEContaAderencia(t *testing.T) {
	l := func(dia, ref string, id int64, nome string, kcal, prot float64, est bool) store.Lancamento {
		return store.Lancamento{Data: dia, Refeicao: ref, AlimentoID: id, AlimentoNome: nome, Kcal: kcal, Prot: prot, Estimado: est}
	}
	ls := []store.Lancamento{
		l("2026-11-01", "almoco", 1, "Arroz", 1000, 20, false),
		l("2026-11-01", "jantar", 2, "Frango", 1000, 100, false), // 2000 kcal, 120 g: na meta, proteína ok
		l("2026-11-02", "almoco", 1, "Arroz", 1500, 30, true),    // 1500 kcal: 25% abaixo
		// 2026-11-03 sem registro
		l("2026-11-04", "almoco", 2, "Frango", 2500, 125, false), // 2500 kcal: 25% acima, proteína ok
	}
	metas := []store.Meta{{VigenteDesde: "2026-10-01", Kcal: 2000, Prot: 120}}
	a := analisar(data(t, "2026-11-01"), 4, ls, metas)

	if a.DiasRegistrados != 3 || a.Cobertura() != 0.75 {
		t.Fatalf("registrados %d, cobertura %v", a.DiasRegistrados, a.Cobertura())
	}
	if a.Media.Kcal != 2000 || a.Media.Prot != 91.7 { // (2000+1500+2500)/3, (120+30+125)/3
		t.Fatalf("média %+v", a.Media)
	}
	if want := math.Sqrt((0 + 250000 + 250000) / 3.0); math.Abs(a.DesvioKcal-want) > 0.1 {
		t.Fatalf("desvio %v, quer %v", a.DesvioKcal, want)
	}
	if a.DiasComMeta != 3 || a.DiasNaMetaKcal != 1 || a.DiasProteinaOK != 2 {
		t.Fatalf("aderência: com meta %d, na meta %d, proteína %d", a.DiasComMeta, a.DiasNaMetaKcal, a.DiasProteinaOK)
	}
	if a.PctEstimado != 25 { // 1500 de 6000
		t.Fatalf("estimado %v%%", a.PctEstimado)
	}
	almoco := a.PorRefeicao[1]
	if almoco.Chave != "almoco" || almoco.Pct != 83.3 || almoco.KcalMedia != 1666.7 {
		t.Fatalf("almoço %+v", almoco)
	}
	if a.TopKcal[0].Nome != "Frango" || a.TopKcal[0].Total != 3500 || a.TopKcal[0].Vezes != 2 {
		t.Fatalf("top kcal %+v", a.TopKcal)
	}
}

func TestTendenciaSuavizaEMantemDiasSemPesagem(t *testing.T) {
	pesos := []store.Pesagem{{Data: "2026-11-01", Kg: 80}, {Data: "2026-11-03", Kg: 81}}
	s := Tendencia(pesos, data(t, "2026-11-04"))
	// 80; sem pesagem repete; 80 + 0,1 × (81 − 80); sem pesagem repete
	got := [4]float64{s[0].Tendencia, s[1].Tendencia, s[2].Tendencia, s[3].Tendencia}
	if got != [4]float64{80, 80, 80.1, 80.1} {
		t.Fatalf("tendência %v", got)
	}
	if s[1].Kg != 0 || s[2].Kg != 81 {
		t.Fatalf("pesagens na série: %+v", s)
	}
}

func TestTDEEPeloBalanco(t *testing.T) {
	ini := data(t, "2026-11-01")
	var serie []PontoPeso
	var dias []DiaDoPeriodo
	for i := range 28 {
		d := ini.AddDate(0, 0, i)
		serie = append(serie, PontoPeso{Data: d, Tendencia: 80 - 0.02*float64(i)}) // cai 0,54 kg em 27 dias
		dias = append(dias, DiaDoPeriodo{Data: d, Registrado: true, Totais: Totais{Kcal: 2000}})
	}
	td, ok, falta := CalcularTDEE(serie, dias)
	// 2000 + 0,54 × 7700 ÷ 27 = 2154
	if !ok || td.Kcal != 2154 || td.VariacaoKg != -0.54 || td.Dias != 28 {
		t.Fatalf("tdee %+v ok=%v %q", td, ok, falta)
	}

	if _, ok, falta := CalcularTDEE(serie[:10], dias); ok || !strings.Contains(falta, "4 dias") {
		t.Fatalf("com 10 dias deveria faltar 4: ok=%v %q", ok, falta)
	}
	for i := 0; i < 28; i += 2 { // metade dos dias sem anotação
		dias[i].Registrado = false
	}
	if _, ok, falta := CalcularTDEE(serie, dias); ok || !strings.Contains(falta, "6 dias registrados") {
		t.Fatalf("com 14 de 28 dias deveria faltar 6 (mínimo 20): ok=%v %q", ok, falta)
	}
}

func TestFibraCongeladaEReescalada(t *testing.T) {
	s, ctx := novoServico(t)
	feijao := primeiro(t, s, ctx, "feijao carioca cozido")
	ls, err := s.Registrar(ctx, Registro{Data: s.Hoje(), Refeicao: "almoco", Origem: OrigemManual,
		Itens: []Item{{AlimentoID: feijao.ID, Gramas: 200}}})
	if err != nil {
		t.Fatal(err)
	}
	if ls[0].Fibra == 0 || ls[0].Fibra != arred(feijao.Fibra*2) {
		t.Fatalf("fibra %v, quer %v", ls[0].Fibra, arred(feijao.Fibra*2))
	}
	g := 100.0
	ed, err := s.Editar(ctx, ls[0].ID, Edicao{Gramas: &g}, OrigemManual)
	if err != nil || ed.Fibra != arred(ls[0].Fibra/2) {
		t.Fatalf("fibra editada %v (%v)", ed.Fibra, err)
	}
	if err := s.RegistrarPeso(ctx, s.Hoje(), 80); err != nil {
		t.Fatal(err)
	}
	a, err := s.AnalisePeriodo(ctx, s.Hoje(), 7)
	if err != nil || a.Media.Fibra != ed.Fibra || a.ProteinaPorKg != arred(ed.Prot/80) {
		t.Fatalf("análise: fibra %v, proteína/kg %v (%v)", a.Media.Fibra, a.ProteinaPorKg, err)
	}
}
