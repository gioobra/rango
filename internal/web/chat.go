package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gioobra/rango/internal/agent"
	"github.com/gioobra/rango/internal/core"
	"github.com/gioobra/rango/internal/store"
)

// limite de uma resposta do Nutri, com todas as chamadas de ferramenta.
const tempoTurno = 5 * time.Minute

// Mensagem do chat. Uma mensagem do Nutri tem texto ou um cartão.
type Mensagem struct {
	DoUsuario bool
	Texto     string
	Cartao    int // índice em conversa.cartoes; -1 se não for cartão
	Hora      time.Time
}

// Cartao mostra o que o Nutri mudou no diário.
type Cartao struct {
	N           int
	Acao        string // registrar | editar | remover | desfazer | receita
	Refeicao    string
	Data        string
	Lancamentos []store.Lancamento
	Total       float64
	Dia         core.Dia
	Desfeito    bool
	Receita     *core.Receita // em Acao "receita"
}

// conversa é o chat em andamento. Fica na memória: reiniciar o servidor limpa a tela,
// mas a sessão do claude continua no disco dele.
type conversa struct {
	mu        sync.Mutex
	sessao    string
	mensagens []Mensagem
	cartoes   []*Cartao
	turno     *turno
	nTurno    int
}

// eventoChat é o que o navegador recebe pelo SSE.
type eventoChat struct {
	Nome   string // texto | cartao | ferramenta | erro | fim
	Texto  string
	Cartao int
}

// turno é uma resposta do Nutri. Os eventos ficam guardados para o SSE poder
// recomeçar de onde parou (o celular dorme, a rede cai).
type turno struct {
	N        int
	Pergunta Mensagem
	mu       sync.Mutex
	eventos  []eventoChat
	fim      bool
	aviso    chan struct{}
}

func (t *turno) adicionar(e eventoChat) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.eventos = append(t.eventos, e)
	if e.Nome == "fim" {
		t.fim = true
	}
	close(t.aviso)
	t.aviso = make(chan struct{})
}

func (t *turno) desde(i int) ([]eventoChat, bool, <-chan struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if i > len(t.eventos) {
		i = len(t.eventos)
	}
	return t.eventos[i:], t.fim, t.aviso
}

var avisoFerramenta = map[string]string{
	"buscar_alimento":    "Procurando alimentos…",
	"registrar_refeicao": "Registrando…",
	"editar_lancamento":  "Corrigindo…",
	"remover_lancamento": "Removendo…",
	"desfazer":           "Desfazendo…",
	"resumo_dia":         "Olhando o dia…",
	"metas":              "Olhando as metas…",
	"definir_porcao":     "Salvando a porção…",
	"definir_metas":      "Salvando as metas…",
	"listar_receitas":    "Olhando as receitas…",
	"criar_receita":      "Salvando a receita…",
	"editar_receita":     "Atualizando a receita…",
	"criar_alimento":     "Cadastrando o alimento…",
}

func (s *Servidor) chat(w http.ResponseWriter, r *http.Request) {
	c := s.conversa
	c.mu.Lock()
	dados := map[string]any{
		"Titulo": "Chat", "Aba": "chat",
		"Mensagens": append([]Mensagem(nil), c.mensagens...),
	}
	cartoes := make([]Cartao, len(c.cartoes))
	for i, k := range c.cartoes {
		cartoes[i] = *k
	}
	dados["Cartoes"] = cartoes
	if t := c.turno; t != nil {
		dados["Turno"] = t
	}
	c.mu.Unlock()
	if err := s.agente.Disponivel(); err != nil {
		dados["Indisponivel"] = err.Error()
	}
	s.render(w, http.StatusOK, "chat", "pagina", dados)
}

// enviar começa um turno e devolve a bolha do usuário e o espaço da resposta.
func (s *Servidor) enviar(w http.ResponseWriter, r *http.Request) {
	texto := strings.TrimSpace(r.FormValue("mensagem"))
	if texto == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(texto) > 4000 {
		s.falha(w, &core.ErroValidacao{Msg: "mensagem longa demais"})
		return
	}
	c := s.conversa
	c.mu.Lock()
	if c.turno != nil {
		c.mu.Unlock()
		http.Error(w, "o Nutri ainda está respondendo", http.StatusConflict)
		return
	}
	c.nTurno++
	t := &turno{N: c.nTurno, Pergunta: Mensagem{DoUsuario: true, Texto: texto, Cartao: -1, Hora: time.Now()},
		aviso: make(chan struct{})}
	c.turno = t
	sessao := c.sessao
	c.mu.Unlock()

	go s.responder(t, sessao)
	s.render(w, http.StatusOK, "chat", "turno", t)
}

// responder roda o Nutri e guarda o que ele disse e fez.
func (s *Servidor) responder(t *turno, sessao string) {
	ctx, cancelar := context.WithTimeout(context.Background(), tempoTurno)
	defer cancelar()

	var respostas []Mensagem
	var atual strings.Builder
	fecharTexto := func() {
		if txt := strings.TrimSpace(atual.String()); txt != "" {
			respostas = append(respostas, Mensagem{Texto: txt, Cartao: -1, Hora: time.Now()})
		}
		atual.Reset()
	}
	nova, err := s.agente.Conversar(ctx, sessao, t.Pergunta.Texto, func(e agent.Evento) {
		switch e.Tipo {
		case agent.Texto:
			atual.WriteString(e.Texto)
			t.adicionar(eventoChat{Nome: "texto", Texto: e.Texto})
		case agent.Ferramenta:
			t.adicionar(eventoChat{Nome: "ferramenta", Texto: avisoFerramenta[e.Ferramenta]})
		case agent.Resultado:
			if e.Erro {
				return
			}
			if n, ok := s.novoCartao(e.Ferramenta, e.Saida); ok {
				fecharTexto()
				respostas = append(respostas, Mensagem{Cartao: n, Hora: time.Now()})
				t.adicionar(eventoChat{Nome: "cartao", Cartao: n})
			}
		}
	})
	fecharTexto()
	if err != nil {
		s.log.Error("Nutri", "err", err)
		msg := "Não consegui responder agora. " + err.Error()
		respostas = append(respostas, Mensagem{Texto: msg, Cartao: -1, Hora: time.Now()})
		t.adicionar(eventoChat{Nome: "erro", Texto: msg})
	}

	c := s.conversa
	c.mu.Lock()
	if nova != "" {
		c.sessao = nova
	}
	c.mensagens = append(c.mensagens, t.Pergunta)
	c.mensagens = append(c.mensagens, respostas...)
	c.turno = nil
	c.mu.Unlock()
	t.adicionar(eventoChat{Nome: "fim"})
}

// novoCartao transforma o resultado de uma ferramenta de escrita num cartão.
func (s *Servidor) novoCartao(ferramenta, saida string) (int, bool) {
	var out struct {
		Lancamentos []store.Lancamento `json:"lancamentos"`
		Lancamento  *store.Lancamento  `json:"lancamento"`
		ID          int64              `json:"id"`
	}
	if json.Unmarshal([]byte(saida), &out) != nil {
		return 0, false
	}
	if ferramenta == "criar_receita" || ferramenta == "editar_receita" {
		rc, err := s.svc.Receita(context.Background(), out.ID)
		if err != nil {
			return 0, false
		}
		return s.guardarCartao(&Cartao{Acao: "receita", Receita: &rc}), true
	}
	acao := map[string]string{
		"registrar_refeicao": "registrar", "editar_lancamento": "editar",
		"remover_lancamento": "remover", "desfazer": "desfazer",
	}[ferramenta]
	if acao == "" {
		return 0, false
	}
	if out.Lancamento != nil {
		out.Lancamentos = []store.Lancamento{*out.Lancamento}
	}
	if len(out.Lancamentos) == 0 {
		return 0, false
	}
	k := &Cartao{Acao: acao, Lancamentos: out.Lancamentos,
		Refeicao: out.Lancamentos[0].Refeicao, Data: out.Lancamentos[0].Data}
	for _, l := range out.Lancamentos {
		k.Total += l.Kcal
	}
	k.Dia = s.diaDoCartao(context.Background(), k.Data)
	return s.guardarCartao(k), true
}

// guardarCartao numera o cartão e o guarda na conversa.
func (s *Servidor) guardarCartao(k *Cartao) int {
	c := s.conversa
	c.mu.Lock()
	defer c.mu.Unlock()
	k.N = len(c.cartoes)
	c.cartoes = append(c.cartoes, k)
	return k.N
}

// cartao devolve uma cópia do cartão n.
func (s *Servidor) cartao(n int) (Cartao, bool) {
	c := s.conversa
	c.mu.Lock()
	defer c.mu.Unlock()
	if n < 0 || n >= len(c.cartoes) {
		return Cartao{}, false
	}
	return *c.cartoes[n], true
}

// eventos transmite o turno por SSE, a partir do Last-Event-ID se o navegador reconectou.
func (s *Servidor) eventos(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.PathValue("n"))
	c := s.conversa
	c.mu.Lock()
	t := c.turno
	c.mu.Unlock()
	fl, ok := w.(http.Flusher)
	if t == nil || t.N != n || !ok {
		// Turno já acabou: o navegador recarrega a página para ver o histórico.
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: fim\ndata: \n\n")
		return
	}
	i := 0
	if id, err := strconv.Atoi(r.Header.Get("Last-Event-ID")); err == nil {
		i = id + 1
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	for {
		evs, fim, aviso := t.desde(i)
		for _, e := range evs {
			dados := e.Texto
			if e.Nome == "cartao" {
				dados = s.htmlCartao(e.Cartao)
			}
			escreverSSE(w, i, e.Nome, dados)
			i++
		}
		fl.Flush()
		if fim { // "fim" é sempre o último evento, e já foi escrito
			return
		}
		select {
		case <-aviso:
		case <-r.Context().Done():
			return
		}
	}
}

func escreverSSE(w http.ResponseWriter, id int, nome, dados string) {
	var b strings.Builder
	fmt.Fprintf(&b, "id: %d\nevent: %s\n", id, nome)
	for _, linha := range strings.Split(dados, "\n") {
		fmt.Fprintf(&b, "data: %s\n", linha)
	}
	b.WriteString("\n")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Servidor) htmlCartao(n int) string {
	k, ok := s.cartao(n)
	if !ok {
		return ""
	}
	var buf bytes.Buffer
	if err := s.paginas["chat"].ExecuteTemplate(&buf, "cartao", k); err != nil {
		s.log.Error("cartão", "err", err)
	}
	return buf.String()
}

// desfazerCartao tira do diário o que o cartão registrou.
func (s *Servidor) desfazerCartao(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.PathValue("n"))
	k, ok := s.cartao(n)
	if !ok || k.Acao != "registrar" || k.Desfeito {
		s.falha(w, core.ErrNaoEncontrado)
		return
	}
	ids := make([]int64, len(k.Lancamentos))
	for i, l := range k.Lancamentos {
		ids[i] = l.ID
	}
	if _, err := s.svc.RemoverVarios(r.Context(), ids, core.OrigemManual); err != nil {
		s.falha(w, err)
		return
	}
	k.Desfeito = true
	k.Dia = s.diaDoCartao(r.Context(), k.Data)
	c := s.conversa
	c.mu.Lock()
	if n < len(c.cartoes) {
		*c.cartoes[n] = k
	}
	c.mu.Unlock()
	s.render(w, http.StatusOK, "chat", "cartao", k)
}

// novaConversa esquece a sessão atual. No meio de uma resposta, não faz nada.
func (s *Servidor) novaConversa(w http.ResponseWriter, r *http.Request) {
	c := s.conversa
	c.mu.Lock()
	if c.turno == nil {
		c.sessao, c.mensagens, c.cartoes = "", nil, nil
	}
	c.mu.Unlock()
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

// diaDoCartao monta o resumo do dia mostrado no cartão; sem ele, o cartão só não mostra a meta.
func (s *Servidor) diaDoCartao(ctx context.Context, data string) core.Dia {
	t, err := s.svc.LerData(data)
	if err != nil {
		return core.Dia{}
	}
	d, err := s.svc.Diario(ctx, t)
	if err != nil {
		s.log.Warn("dia do cartão", "err", err)
	}
	return d
}
