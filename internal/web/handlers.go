package web

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gioobra/rango/internal/core"
	"github.com/gioobra/rango/internal/store"
)

// lerData lê ?data=AAAA-MM-DD; sem ela, hoje.
func (s *Servidor) lerData(v string) time.Time {
	if t, err := time.ParseInLocation(core.FormatoData, v, time.Local); err == nil {
		return t
	}
	return s.svc.Hoje()
}

func lerRefeicao(v string) string {
	for _, r := range core.Refeicoes {
		if r.Chave == v {
			return v
		}
	}
	// sem refeição escolhida, sugere pelo horário
	switch h := time.Now().Hour(); {
	case h < 11:
		return "cafe"
	case h < 15:
		return "almoco"
	case h < 18:
		return "lanche"
	}
	return "jantar"
}

// milhar reconhece "1.100" e "12.500.000": ponto só como separador de milhar.
var milhar = regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`)

// lerNumero aceita o formato brasileiro ("1.100", "4,5", "1.100,5") e o do campo number ("4.5").
func lerNumero(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if strings.Contains(v, ",") || milhar.MatchString(v) {
		v = strings.ReplaceAll(v, ".", "")
	}
	return strconv.ParseFloat(strings.Replace(v, ",", ".", 1), 64)
}

func paraDiario(data, aviso string) string {
	q := url.Values{"data": {data}}
	if aviso != "" {
		q.Set("aviso", aviso)
	}
	return "/diario?" + q.Encode()
}

var avisos = map[string]string{
	"adicionado": "Adicionado ao diário.",
	"removido":   "Item removido.",
	"desfeito":   "Desfeito.",
	"nada":       "Nada para desfazer.",
}

func (s *Servidor) diario(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.Diario(r.Context(), s.lerData(r.URL.Query().Get("data")))
	if err != nil {
		s.falha(w, err)
		return
	}
	aviso := r.URL.Query().Get("aviso")
	s.render(w, http.StatusOK, "diario", "pagina", map[string]any{
		"Titulo": "Diário", "Aba": "diario", "Dia": d,
		"Aviso": avisos[aviso], "PodeDesfazer": aviso == "adicionado" || aviso == "removido",
	})
}

func (s *Servidor) adicionar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dados := map[string]any{
		"Titulo": "Adicionar alimento", "Data": s.lerData(q.Get("data")),
		"Refeicao": lerRefeicao(q.Get("refeicao")), "Refeicoes": core.Refeicoes, "Busca": q.Get("q"),
	}
	if q.Get("q") != "" {
		as, err := s.svc.BuscarAlimentos(r.Context(), q.Get("q"), 20)
		if err != nil {
			s.falha(w, err)
			return
		}
		dados["Resultados"] = as
	}
	s.render(w, http.StatusOK, "adicionar", "pagina", dados)
}

func (s *Servidor) busca(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	as, err := s.svc.BuscarAlimentos(r.Context(), q.Get("q"), 20)
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "adicionar", "resultados", map[string]any{
		"Busca": q.Get("q"), "Resultados": as,
		"Data": s.lerData(q.Get("data")), "Refeicao": lerRefeicao(q.Get("refeicao")),
	})
}

type painel struct {
	Alimento store.Alimento
	Porcoes  []store.Porcao
	Gramas   float64
	Macros   core.Macros
	Data     time.Time
	Refeicao string
	Erro     string
}

func (s *Servidor) montarPainel(r *http.Request, id int64, gramas float64) (painel, error) {
	a, ps, err := s.svc.Alimento(r.Context(), id)
	if err != nil {
		return painel{}, err
	}
	if gramas <= 0 {
		gramas = 100
		if len(ps) > 0 {
			gramas = ps[0].Gramas
		}
	}
	return painel{
		Alimento: a, Porcoes: ps, Gramas: gramas, Macros: core.MacrosDe(a, gramas),
		Data: s.lerData(r.FormValue("data")), Refeicao: lerRefeicao(r.FormValue("refeicao")),
	}, nil
}

func (s *Servidor) painelAlimento(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	p, err := s.montarPainel(r, id, 0)
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "adicionar", "painel", p)
}

func (s *Servidor) previa(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("alimento_id"), 10, 64)
	g, err := lerNumero(r.FormValue("gramas"))
	if err != nil || g < 0 {
		g = 0
	}
	a, _, err := s.svc.Alimento(r.Context(), id)
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "adicionar", "previa", painel{Alimento: a, Gramas: g, Macros: core.MacrosDe(a, g)})
}

func (s *Servidor) novaPorcao(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("alimento_id"), 10, 64)
	g, err := lerNumero(r.FormValue("gramas"))
	if err != nil {
		g = 0
	}
	_, errPorcao := s.svc.DefinirPorcao(r.Context(), id, r.FormValue("porcao_nome"), g)
	if errPorcao != nil && !core.EhValidacao(errPorcao) {
		s.falha(w, errPorcao)
		return
	}
	p, err := s.montarPainel(r, id, g)
	if err != nil {
		s.falha(w, err)
		return
	}
	if errPorcao != nil {
		p.Erro = errPorcao.Error()
	}
	s.render(w, http.StatusOK, "adicionar", "painel", p)
}

func (s *Servidor) registrar(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("alimento_id"), 10, 64)
	g, err := lerNumero(r.FormValue("gramas"))
	if err != nil {
		s.falha(w, &core.ErroValidacao{Msg: "quantidade inválida"})
		return
	}
	data := s.lerData(r.FormValue("data"))
	_, err = s.svc.Registrar(r.Context(), core.Registro{
		Data: data, Refeicao: r.FormValue("refeicao"), Origem: core.OrigemManual,
		Itens: []core.Item{{AlimentoID: id, Gramas: g}},
	})
	if err != nil {
		s.falha(w, err)
		return
	}
	http.Redirect(w, r, paraDiario(data.Format(core.FormatoData), "adicionado"), http.StatusSeeOther)
}

func (s *Servidor) remover(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	l, err := s.svc.Remover(r.Context(), id, core.OrigemManual)
	if err != nil {
		s.falha(w, err)
		return
	}
	http.Redirect(w, r, paraDiario(l.Data, "removido"), http.StatusSeeOther)
}

func (s *Servidor) desfazer(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.Desfazer(r.Context(), core.OrigemManual)
	if errors.Is(err, core.ErrNadaADesfazer) {
		http.Redirect(w, r, paraDiario(r.FormValue("data"), "nada"), http.StatusSeeOther)
		return
	}
	if err != nil {
		s.falha(w, err)
		return
	}
	http.Redirect(w, r, paraDiario(d.Lancamento.Data, "desfeito"), http.StatusSeeOther)
}

func (s *Servidor) paginaMetas(w http.ResponseWriter, r *http.Request) {
	m, ok, err := s.svc.Metas(r.Context(), s.svc.Hoje())
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "metas", "pagina", map[string]any{
		"Titulo": "Metas", "Meta": m, "TemMeta": ok, "Salvo": r.URL.Query().Has("salvo"),
	})
}

func (s *Servidor) salvarMetas(w http.ResponseWriter, r *http.Request) {
	var v [4]float64
	for i, campo := range []string{"kcal", "prot", "carb", "gord"} {
		n, err := lerNumero(r.FormValue(campo))
		if err != nil {
			n = -1 // o core recusa
		}
		v[i] = n
	}
	_, err := s.svc.DefinirMetas(r.Context(), v[0], v[1], v[2], v[3])
	if core.EhValidacao(err) {
		digitado := store.Meta{Kcal: max(v[0], 0), Prot: max(v[1], 0), Carb: max(v[2], 0), Gord: max(v[3], 0)}
		s.render(w, http.StatusBadRequest, "metas", "pagina", map[string]any{
			"Titulo": "Metas", "Meta": digitado, "TemMeta": true, "Erro": err.Error(),
		})
		return
	}
	if err != nil {
		s.falha(w, err)
		return
	}
	http.Redirect(w, r, "/metas?salvo", http.StatusSeeOther)
}
