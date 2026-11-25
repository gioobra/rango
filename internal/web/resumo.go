package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gioobra/rango/internal/core"
)

var periodos = []int{7, 14, 30}

// barraDia é uma coluna do gráfico de kcal por dia (alturas em % da escala).
type barraDia struct {
	Rotulo     string // "Seg"
	Dica       string // tooltip
	Altura     float64
	Registrado bool
	Acima      bool // acima da faixa de ±10% da meta
}

// graficoKcal guarda a geometria do gráfico de kcal por dia.
type graficoKcal struct {
	Barras   []barraDia
	MetaPct  float64 // altura da linha da meta; 0 sem meta
	Escala   float64 // kcal no topo
	Rotular  bool    // cabe rótulo de dia embaixo de cada barra
	TemAcima bool    // alguma barra passou da faixa da meta (mostra a legenda)
	MetaKcal float64
}

func montarGraficoKcal(a core.Analise) graficoKcal {
	g := graficoKcal{Rotular: len(a.PorDia) <= 14}
	maior := 0.0
	for _, d := range a.PorDia {
		maior = math.Max(maior, d.Totais.Kcal)
	}
	if a.Meta != nil {
		g.MetaKcal = a.Meta.Kcal
		maior = math.Max(maior, a.Meta.Kcal)
	}
	if maior == 0 {
		maior = 2000
	}
	g.Escala = maior * 1.1
	if a.Meta != nil {
		g.MetaPct = a.Meta.Kcal / g.Escala * 100
	}
	for _, d := range a.PorDia {
		b := barraDia{Rotulo: diasCurtos[d.Data.Weekday()], Registrado: d.Registrado,
			Altura: d.Totais.Kcal / g.Escala * 100}
		dia := fmt.Sprintf("%s %02d/%02d", diasCurtos[d.Data.Weekday()], d.Data.Day(), d.Data.Month())
		if d.Registrado {
			b.Dica = fmt.Sprintf("%s: %s kcal · P %s g", dia, num(d.Totais.Kcal), dec(d.Totais.Prot))
			b.Acima = d.Meta != nil && d.Totais.Kcal > d.Meta.Kcal*(1+core.MargemMeta)
			g.TemAcima = g.TemAcima || b.Acima
		} else {
			b.Dica = dia + ": sem registro"
		}
		g.Barras = append(g.Barras, b)
	}
	return g
}

// graficoPeso é a linha da tendência e os pontos das pesagens, num SVG de 300 × 100.
type graficoPeso struct {
	Linha  string
	Pontos []pontoGrafico
	Min    float64
	Max    float64
	Inicio string
	Fim    string
}

type pontoGrafico struct {
	X, Y float64
	Dica string
}

func montarGraficoPeso(serie []core.PontoPeso) *graficoPeso {
	if len(serie) < 2 {
		return nil
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range serie {
		for _, v := range []float64{p.Tendencia, p.Kg} {
			if v > 0 {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
		}
	}
	if hi-lo < 1 { // ao menos 1 kg de faixa, para a linha não exagerar oscilações mínimas
		meio := (hi + lo) / 2
		lo, hi = meio-0.5, meio+0.5
	}
	x := func(i int) float64 { return 4 + float64(i)/float64(len(serie)-1)*292 }
	y := func(v float64) float64 { return 6 + (hi-v)/(hi-lo)*88 }
	g := &graficoPeso{Min: lo, Max: hi,
		Inicio: serie[0].Data.Format("02/01"), Fim: serie[len(serie)-1].Data.Format("02/01")}
	var pts []string
	for i, p := range serie {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x(i), y(p.Tendencia)))
		if p.Kg > 0 {
			g.Pontos = append(g.Pontos, pontoGrafico{X: x(i), Y: y(p.Kg),
				Dica: fmt.Sprintf("%s: %s kg (tendência %s)", p.Data.Format("02/01"), dec(p.Kg), dec2(p.Tendencia))})
		}
	}
	g.Linha = strings.Join(pts, " ")
	return g
}

// dec2 formata com até duas casas e vírgula: 80.14 -> "80,14".
func dec2(v float64) string {
	return strings.Replace(strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64), ".", ",", 1)
}

func (s *Servidor) resumo(w http.ResponseWriter, r *http.Request) {
	dias, _ := strconv.Atoi(r.URL.Query().Get("dias"))
	if dias != 14 && dias != 30 {
		dias = 7
	}
	hoje := s.svc.Hoje()
	a, err := s.svc.AnalisePeriodo(r.Context(), hoje, dias)
	if err != nil {
		s.falha(w, err)
		return
	}
	p, err := s.svc.Peso(r.Context(), hoje, max(dias, 30))
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "resumo", "pagina", map[string]any{
		"Titulo": "Resumo", "Aba": "resumo", "Dias": dias, "Periodos": periodos,
		"A": a, "Kcal": montarGraficoKcal(a), "Peso": p, "GraficoPeso": montarGraficoPeso(p.Serie),
		"Hoje": hoje, "ErroPeso": r.URL.Query().Get("erro"),
	})
}

func (s *Servidor) registrarPeso(w http.ResponseWriter, r *http.Request) {
	dias := "7"
	if n, _ := strconv.Atoi(r.FormValue("dias")); n == 14 || n == 30 {
		dias = strconv.Itoa(n)
	}
	kg, err := lerNumero(r.FormValue("kg"))
	if err == nil {
		err = s.svc.RegistrarPeso(r.Context(), s.svc.Hoje(), kg)
	} else {
		err = &core.ErroValidacao{Msg: "peso inválido"}
	}
	if core.EhValidacao(err) {
		http.Redirect(w, r, "/resumo?dias="+dias+"&erro="+url.QueryEscape(err.Error())+"#peso", http.StatusSeeOther)
		return
	}
	if err != nil {
		s.falha(w, err)
		return
	}
	http.Redirect(w, r, "/resumo?dias="+dias+"#peso", http.StatusSeeOther)
}
