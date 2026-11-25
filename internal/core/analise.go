package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gioobra/rango/internal/store"
)

// Constantes da análise. As regras estão na seção 5 do PLANO.md.
const (
	KcalPorGProt = 4.0
	KcalPorGCarb = 4.0
	KcalPorGGord = 9.0
	KcalPorKg    = 7700.0 // energia de 1 kg de massa corporal, para o balanço do TDEE
	AlfaPeso     = 0.1    // suavização da tendência de peso (média móvel exponencial)
	MargemMeta   = 0.10   // dia "na meta" de kcal: até ±10%
	JanelaTDEE   = 28     // dias usados no TDEE
	MinDiasTDEE  = 14     // dias desde a primeira pesagem dentro da janela
	MinCobertura = 0.70   // fração mínima de dias registrados na janela do TDEE
	MaxDias      = 90     // maior período de análise
)

// Distribuicao é a porcentagem das kcal vinda de cada macro (soma 100, ou 0 sem macros).
type Distribuicao struct {
	Prot float64
	Carb float64
	Gord float64
}

// DistribuicaoDe usa 4/4/9 kcal por grama sobre a soma dos três, não a kcal da tabela,
// para fechar em 100%.
func DistribuicaoDe(t Totais) Distribuicao {
	p, c, g := t.Prot*KcalPorGProt, t.Carb*KcalPorGCarb, t.Gord*KcalPorGGord
	soma := p + c + g
	if soma <= 0 {
		return Distribuicao{}
	}
	return Distribuicao{Prot: arred(p / soma * 100), Carb: arred(c / soma * 100), Gord: arred(g / soma * 100)}
}

// DiaDoPeriodo é um dia dentro de uma análise.
type DiaDoPeriodo struct {
	Data         time.Time
	Totais       Totais
	Registrado   bool        // ao menos um lançamento
	Meta         *store.Meta // vigente no dia
	KcalEstimada float64     // kcal de quantidades estimadas
}

// FaixaKcal compara kcal com a meta: -1 abaixo, 0 a até ±10% dela, 1 acima.
func FaixaKcal(kcal, meta float64) int {
	switch {
	case math.Abs(kcal-meta) <= meta*MargemMeta:
		return 0
	case kcal < meta:
		return -1
	default:
		return 1
	}
}

// NaMetaKcal diz se o dia ficou a até ±10% da meta de kcal.
func (d DiaDoPeriodo) NaMetaKcal() bool {
	return d.Registrado && d.Meta != nil && d.Meta.Kcal > 0 && FaixaKcal(d.Totais.Kcal, d.Meta.Kcal) == 0
}

// ProteinaOK diz se o dia bateu a meta de proteína.
func (d DiaDoPeriodo) ProteinaOK() bool {
	return d.Registrado && d.Meta != nil && d.Meta.Prot > 0 && d.Totais.Prot >= d.Meta.Prot
}

// FatiaRefeicao é quanto uma refeição pesa nas kcal do período.
type FatiaRefeicao struct {
	Refeicao
	KcalMedia float64 // por dia registrado
	Pct       float64
}

// ItemRanking é um alimento somado no período.
type ItemRanking struct {
	AlimentoID int64
	Nome       string
	Total      float64 // kcal ou gramas de proteína
	Vezes      int
}

// Analise resume um período. Médias e aderência contam só os dias registrados.
type Analise struct {
	Inicio, Fim     time.Time
	Dias            int
	DiasRegistrados int
	PorDia          []DiaDoPeriodo

	Media        Totais
	DesvioKcal   float64
	Distribuicao Distribuicao
	PctEstimado  float64 // das kcal

	DiasComMeta    int
	DiasNaMetaKcal int
	DiasProteinaOK int
	Meta           *store.Meta // vigente no fim

	PorRefeicao []FatiaRefeicao
	TopKcal     []ItemRanking
	TopProt     []ItemRanking

	ProteinaPorKg float64 // 0 sem peso
}

// Cobertura é a fração de dias registrados.
func (a Analise) Cobertura() float64 {
	if a.Dias == 0 {
		return 0
	}
	return float64(a.DiasRegistrados) / float64(a.Dias)
}

func validarDias(dias int) error {
	if dias < 1 || dias > MaxDias {
		return invalido("o período precisa ter entre 1 e %d dias", MaxDias)
	}
	return nil
}

// AnalisePeriodo analisa os dias que terminam em fim.
func (s *Servico) AnalisePeriodo(ctx context.Context, fim time.Time, dias int) (Analise, error) {
	if err := validarDias(dias); err != nil {
		return Analise{}, err
	}
	ini := fim.AddDate(0, 0, -(dias - 1))
	ls, err := s.db.LancamentosEntre(ctx, ini.Format(FormatoData), fim.Format(FormatoData))
	if err != nil {
		return Analise{}, err
	}
	metas, err := s.db.MetasAte(ctx, fim.Format(FormatoData))
	if err != nil {
		return Analise{}, err
	}
	a := analisar(ini, dias, ls, metas)
	if len(metas) > 0 {
		m := metas[len(metas)-1]
		a.Meta = &m
	}
	pesos, err := s.db.PesosAte(ctx, fim.Format(FormatoData))
	if err != nil {
		return Analise{}, err
	}
	if serie := Tendencia(pesos, fim); len(serie) > 0 && a.DiasRegistrados > 0 {
		a.ProteinaPorKg = arred(a.Media.Prot / serie[len(serie)-1].Tendencia)
	}
	return a, nil
}

// metaEm acha a meta vigente na data (metas em ordem de vigência).
func metaEm(metas []store.Meta, data string) *store.Meta {
	var m *store.Meta
	for i := range metas {
		if metas[i].VigenteDesde <= data {
			m = &metas[i]
		}
	}
	return m
}

// analisar faz as contas sobre os lançamentos já carregados.
func analisar(ini time.Time, dias int, ls []store.Lancamento, metas []store.Meta) Analise {
	a := Analise{Inicio: ini, Fim: ini.AddDate(0, 0, dias-1), Dias: dias}
	indice := map[string]int{}
	for i := range dias {
		d := ini.AddDate(0, 0, i)
		chave := d.Format(FormatoData)
		indice[chave] = i
		a.PorDia = append(a.PorDia, DiaDoPeriodo{Data: d, Meta: metaEm(metas, chave)})
	}

	kcalRef := map[string]float64{}
	topKcal, topProt := map[int64]*ItemRanking{}, map[int64]*ItemRanking{}
	var kcalTotal, kcalEst float64
	for _, l := range ls {
		i, ok := indice[l.Data]
		if !ok {
			continue
		}
		d := &a.PorDia[i]
		d.Registrado = true
		d.Totais.somar(l)
		if l.Estimado {
			d.KcalEstimada += l.Kcal
			kcalEst += l.Kcal
		}
		kcalTotal += l.Kcal
		kcalRef[l.Refeicao] += l.Kcal
		for _, par := range []struct {
			m map[int64]*ItemRanking
			v float64
		}{{topKcal, l.Kcal}, {topProt, l.Prot}} {
			it := par.m[l.AlimentoID]
			if it == nil {
				it = &ItemRanking{AlimentoID: l.AlimentoID, Nome: l.AlimentoNome}
				par.m[l.AlimentoID] = it
			}
			it.Total += par.v
			it.Vezes++
		}
	}

	var soma Totais
	var kcals []float64
	for _, d := range a.PorDia {
		if !d.Registrado {
			continue
		}
		a.DiasRegistrados++
		kcals = append(kcals, d.Totais.Kcal)
		soma.Kcal += d.Totais.Kcal
		soma.Prot += d.Totais.Prot
		soma.Carb += d.Totais.Carb
		soma.Gord += d.Totais.Gord
		soma.Fibra += d.Totais.Fibra
		if d.Meta != nil {
			a.DiasComMeta++
			if d.NaMetaKcal() {
				a.DiasNaMetaKcal++
			}
			if d.ProteinaOK() {
				a.DiasProteinaOK++
			}
		}
	}
	if n := float64(a.DiasRegistrados); n > 0 {
		a.Media = Totais{Kcal: arred(soma.Kcal / n), Prot: arred(soma.Prot / n), Carb: arred(soma.Carb / n),
			Gord: arred(soma.Gord / n), Fibra: arred(soma.Fibra / n)}
		a.DesvioKcal = arred(desvio(kcals))
		a.Distribuicao = DistribuicaoDe(a.Media)
		for _, r := range Refeicoes {
			f := FatiaRefeicao{Refeicao: r, KcalMedia: arred(kcalRef[r.Chave] / n)}
			if kcalTotal > 0 {
				f.Pct = arred(kcalRef[r.Chave] / kcalTotal * 100)
			}
			a.PorRefeicao = append(a.PorRefeicao, f)
		}
	}
	if kcalTotal > 0 {
		a.PctEstimado = arred(kcalEst / kcalTotal * 100)
	}
	a.TopKcal = ranking(topKcal, 5)
	a.TopProt = ranking(topProt, 5)
	return a
}

// desvio padrão populacional.
func desvio(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var m float64
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	var q float64
	for _, x := range v {
		q += (x - m) * (x - m)
	}
	return math.Sqrt(q / float64(len(v)))
}

func ranking(m map[int64]*ItemRanking, n int) []ItemRanking {
	out := make([]ItemRanking, 0, len(m))
	for _, it := range m {
		if it.Total > 0 {
			it.Total = arred(it.Total)
			out = append(out, *it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Nome < out[j].Nome
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// --- peso ---

// PontoPeso é um dia da série de peso.
type PontoPeso struct {
	Data      time.Time
	Kg        float64 // 0 se não pesou no dia
	Tendencia float64
}

// Tendencia monta a série diária da primeira pesagem até fim: média móvel exponencial
// com α = 0,1. Dia sem pesagem repete a tendência anterior.
func Tendencia(pesos []store.Pesagem, fim time.Time) []PontoPeso {
	if len(pesos) == 0 {
		return nil
	}
	porData := map[string]float64{}
	for _, p := range pesos {
		porData[p.Data] = p.Kg
	}
	ini, err := time.ParseInLocation(FormatoData, pesos[0].Data, fim.Location())
	if err != nil || ini.After(fim) {
		return nil
	}
	var out []PontoPeso
	t := pesos[0].Kg
	for d := ini; !d.After(fim); d = d.AddDate(0, 0, 1) {
		kg := porData[d.Format(FormatoData)]
		if kg > 0 {
			t += AlfaPeso * (kg - t)
		}
		out = append(out, PontoPeso{Data: d, Kg: kg, Tendencia: arred2(t)})
	}
	return out
}

func arred2(v float64) float64 { return math.Round(v*100) / 100 }

// TDEE é o gasto diário estimado pelo balanço entre o que se comeu e a tendência de peso.
type TDEE struct {
	Kcal            float64
	MediaKcal       float64
	VariacaoKg      float64 // tendência no fim − no início
	Dias            int     // dias da janela
	DiasRegistrados int
}

// CalcularTDEE usa a janela de até 28 dias que termina no último ponto da série.
// Sem dados suficientes, devolve ok falso e o que falta.
func CalcularTDEE(serie []PontoPeso, dias []DiaDoPeriodo) (TDEE, bool, string) {
	if len(serie) == 0 {
		return TDEE{}, false, "registre o peso para estimar o gasto"
	}
	janela := serie
	if len(janela) > JanelaTDEE {
		janela = janela[len(janela)-JanelaTDEE:]
	}
	if len(janela) < MinDiasTDEE {
		return TDEE{}, false, fmt.Sprintf("faltam %s de acompanhamento do peso (são precisos %d)",
			plural(MinDiasTDEE-len(janela), "dia", "dias"), MinDiasTDEE)
	}
	ini, fim := janela[0].Data, janela[len(janela)-1].Data
	var soma float64
	var reg int
	for _, d := range dias {
		if d.Registrado && !d.Data.Before(ini) && !d.Data.After(fim) {
			soma += d.Totais.Kcal
			reg++
		}
	}
	minimo := int(math.Ceil(MinCobertura * float64(len(janela))))
	if reg < minimo {
		return TDEE{}, false, fmt.Sprintf("faltam %s: só %d dos últimos %d dias têm alimentação anotada",
			plural(minimo-reg, "dia registrado", "dias registrados"), reg, len(janela))
	}
	media := soma / float64(reg)
	delta := janela[len(janela)-1].Tendencia - janela[0].Tendencia
	intervalos := float64(len(janela) - 1)
	return TDEE{
		Kcal:      math.Round(media - delta*KcalPorKg/intervalos),
		MediaKcal: math.Round(media), VariacaoKg: arred2(delta),
		Dias: len(janela), DiasRegistrados: reg,
	}, true, ""
}

func plural(n int, um, varios string) string {
	if n == 1 {
		return "1 " + um
	}
	return fmt.Sprintf("%d %s", n, varios)
}

// Peso resume o acompanhamento até fim.
type Peso struct {
	Serie         []PontoPeso // últimos dias pedidos
	Ultima        *store.Pesagem
	Tendencia     float64 // no fim; 0 sem pesagem
	VariacaoSem   float64 // kg por semana, pela tendência dos últimos 7 dias
	TDEE          TDEE
	TemTDEE       bool
	FaltaParaTDEE string
}

// RegistrarPeso grava a pesagem do dia (a última do dia vale).
func (s *Servico) RegistrarPeso(ctx context.Context, data time.Time, kg float64) error {
	if kg < 20 || kg > 400 {
		return invalido("o peso precisa ficar entre 20 e 400 kg")
	}
	if data.After(s.Hoje()) {
		return invalido("não dá para registrar peso no futuro")
	}
	return s.db.SalvarPeso(ctx, store.Pesagem{Data: data.Format(FormatoData), Kg: math.Round(kg*10) / 10})
}

// RemoverPeso apaga a pesagem do dia.
func (s *Servico) RemoverPeso(ctx context.Context, data time.Time) error {
	return s.db.RemoverPeso(ctx, data.Format(FormatoData))
}

// Peso devolve a série dos últimos dias, a tendência e o TDEE adaptativo.
func (s *Servico) Peso(ctx context.Context, fim time.Time, dias int) (Peso, error) {
	if err := validarDias(dias); err != nil {
		return Peso{}, err
	}
	pesos, err := s.db.PesosAte(ctx, fim.Format(FormatoData))
	if err != nil {
		return Peso{}, err
	}
	var p Peso
	serie := Tendencia(pesos, fim)
	if len(serie) == 0 {
		_, _, p.FaltaParaTDEE = CalcularTDEE(nil, nil)
		return p, nil
	}
	ult := pesos[len(pesos)-1]
	p.Ultima = &ult
	p.Tendencia = serie[len(serie)-1].Tendencia
	if len(serie) > 7 {
		p.VariacaoSem = arred2(p.Tendencia - serie[len(serie)-8].Tendencia)
	}
	p.Serie = serie
	if len(serie) > dias {
		p.Serie = serie[len(serie)-dias:]
	}

	janela := min(len(serie), JanelaTDEE)
	ls, err := s.db.LancamentosEntre(ctx, serie[len(serie)-janela].Data.Format(FormatoData), fim.Format(FormatoData))
	if err != nil {
		return Peso{}, err
	}
	a := analisar(serie[len(serie)-janela].Data, janela, ls, nil)
	p.TDEE, p.TemTDEE, p.FaltaParaTDEE = CalcularTDEE(serie, a.PorDia)
	return p, nil
}

// Simulacao mostra como o dia fecharia com itens a mais, sem registrar nada.
type Simulacao struct {
	Itens      []ItemSimulado
	Soma       Totais
	DiaDepois  Totais
	Restante   Totais // vs meta, depois dos itens; zero sem meta
	TemMeta    bool
	Distrib    Distribuicao // do dia depois
	MetaKcalOK bool         // o dia fica a até ±10% da meta
}

// ItemSimulado é um item da simulação com seus macros.
type ItemSimulado struct {
	Alimento store.Alimento
	Gramas   float64
	Macros   Macros
}

// Simular calcula itens hipotéticos sobre o dia.
func (s *Servico) Simular(ctx context.Context, data time.Time, itens []Item) (Simulacao, error) {
	if len(itens) == 0 {
		return Simulacao{}, invalido("nada para simular")
	}
	d, err := s.Diario(ctx, data)
	if err != nil {
		return Simulacao{}, err
	}
	var sim Simulacao
	for _, it := range itens {
		if err := validarGramas(it.Gramas); err != nil {
			return Simulacao{}, err
		}
		a, err := s.db.Alimento(ctx, it.AlimentoID)
		if err != nil {
			return Simulacao{}, fmt.Errorf("alimento %d: %w", it.AlimentoID, err)
		}
		m := MacrosDe(a, it.Gramas)
		sim.Itens = append(sim.Itens, ItemSimulado{Alimento: a, Gramas: it.Gramas, Macros: m})
		sim.Soma.somar(store.Lancamento{Kcal: m.Kcal, Prot: m.Prot, Carb: m.Carb, Gord: m.Gord, Fibra: m.Fibra})
	}
	depois := d
	depois.Totais.Kcal = arred(d.Totais.Kcal + sim.Soma.Kcal)
	depois.Totais.Prot = arred(d.Totais.Prot + sim.Soma.Prot)
	depois.Totais.Carb = arred(d.Totais.Carb + sim.Soma.Carb)
	depois.Totais.Gord = arred(d.Totais.Gord + sim.Soma.Gord)
	depois.Totais.Fibra = arred(d.Totais.Fibra + sim.Soma.Fibra)
	sim.DiaDepois = depois.Totais
	sim.Distrib = DistribuicaoDe(depois.Totais)
	sim.TemMeta = d.TemMeta
	if d.TemMeta {
		sim.Restante = depois.Restante()
		dp := DiaDoPeriodo{Registrado: true, Totais: depois.Totais, Meta: &d.Meta}
		sim.MetaKcalOK = dp.NaMetaKcal()
	}
	return sim, nil
}
