package web

import (
	"net/http"
	"strconv"

	"github.com/gioobra/rango/internal/core"
	"github.com/gioobra/rango/internal/store"
)

func (s *Servidor) receitas(w http.ResponseWriter, r *http.Request) {
	rcs, err := s.svc.Receitas(r.Context())
	if err != nil {
		s.falha(w, err)
		return
	}
	aberta, _ := strconv.ParseInt(r.URL.Query().Get("aberta"), 10, 64)
	s.render(w, http.StatusOK, "receitas", "pagina", map[string]any{
		"Titulo": "Receitas", "Aba": "receitas", "Receitas": rcs, "Aberta": aberta,
		"Refeicao": lerRefeicao(""), "Refeicoes": core.Refeicoes, "Hoje": s.svc.Hoje(),
	})
}

// formReceita é o que a tela de edição mostra: os campos digitados e os ingredientes.
type formReceita struct {
	ID          int64
	Nome        string
	Itens       []core.ItemDaReceita
	RendimentoG float64
	NPorcoes    int
	NomePorcao  string
	Erro        string
}

func (s *Servidor) paginaReceita(w http.ResponseWriter, status int, f formReceita) {
	titulo := "Nova receita"
	if f.ID > 0 {
		titulo = "Editar receita"
	}
	s.render(w, status, "receita", "pagina", map[string]any{"Titulo": titulo, "Form": f})
}

func (s *Servidor) novaReceita(w http.ResponseWriter, r *http.Request) {
	s.paginaReceita(w, http.StatusOK, formReceita{NPorcoes: 1, NomePorcao: "1 marmita"})
}

func (s *Servidor) editarReceita(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rc, err := s.svc.Receita(r.Context(), id)
	if err != nil {
		s.falha(w, err)
		return
	}
	f := formReceita{ID: rc.ID, Nome: rc.Nome, Itens: rc.Itens,
		RendimentoG: rc.RendimentoG, NPorcoes: rc.NPorcoes, NomePorcao: rc.NomePorcao}
	if !rc.PesoInformado {
		f.RendimentoG = 0 // campo vazio: a soma é refeita se os ingredientes mudarem
	}
	s.paginaReceita(w, http.StatusOK, f)
}

// salvarReceita cria (sem id) ou edita (com id). Com erro de validação, volta ao formulário com o que foi digitado.
func (s *Servidor) salvarReceita(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.falha(w, &core.ErroValidacao{Msg: "formulário inválido"})
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	n := core.NovaReceita{Nome: r.FormValue("nome"), NomePorcao: r.FormValue("nome_porcao")}
	n.NPorcoes, _ = strconv.Atoi(r.FormValue("n_porcoes"))
	if v, err := lerNumero(r.FormValue("rendimento_g")); err == nil {
		n.RendimentoG = v
	}
	ids, gramas := r.Form["alimento_id"], r.Form["gramas"]
	f := formReceita{ID: id, Nome: n.Nome, RendimentoG: n.RendimentoG, NPorcoes: n.NPorcoes, NomePorcao: n.NomePorcao}
	for i := range ids {
		aid, _ := strconv.ParseInt(ids[i], 10, 64)
		g := 0.0
		if i < len(gramas) {
			g, _ = lerNumero(gramas[i])
		}
		n.Itens = append(n.Itens, store.ItemReceita{AlimentoID: aid, Gramas: g})
		if a, _, err := s.svc.Alimento(r.Context(), aid); err == nil {
			f.Itens = append(f.Itens, core.ItemDaReceita{ItemReceita: store.ItemReceita{AlimentoID: aid, AlimentoNome: a.Nome, Gramas: g}})
		}
	}

	var rc core.Receita
	var err error
	if id > 0 {
		rc, err = s.svc.EditarReceita(r.Context(), id, n)
	} else {
		rc, err = s.svc.CriarReceita(r.Context(), n)
	}
	if core.EhValidacao(err) {
		f.Erro = err.Error()
		s.paginaReceita(w, http.StatusBadRequest, f)
		return
	}
	if err != nil {
		s.falha(w, err)
		return
	}
	http.Redirect(w, r, "/receitas?aberta="+strconv.FormatInt(rc.ID, 10)+"#receita-"+strconv.FormatInt(rc.ID, 10), http.StatusSeeOther)
}

// buscaIngrediente lista alimentos para adicionar à receita.
func (s *Servidor) buscaIngrediente(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	as, err := s.svc.BuscarAlimentos(r.Context(), q, 10)
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "receita", "ingredientes-busca", map[string]any{"Busca": q, "Resultados": as})
}

// ingrediente devolve uma linha nova da lista de ingredientes.
func (s *Servidor) ingrediente(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	a, _, err := s.svc.Alimento(r.Context(), id)
	if err != nil {
		s.falha(w, err)
		return
	}
	s.render(w, http.StatusOK, "receita", "ingrediente",
		core.ItemDaReceita{ItemReceita: store.ItemReceita{AlimentoID: a.ID, AlimentoNome: a.Nome}})
}
