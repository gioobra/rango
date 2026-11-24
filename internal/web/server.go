// Package web serve a interface do Rango: telas em html/template, htmx e PWA.
// Os handlers são finos: validam a entrada, chamam o core e renderizam.
package web

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/gioobra/rango/internal/agent"
	"github.com/gioobra/rango/internal/core"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Servidor é a aplicação web.
type Servidor struct {
	svc      *core.Servico
	agente   *agent.Agente
	conversa *conversa
	sessao   *sessao
	paginas  map[string]*template.Template
	log      *slog.Logger
}

// Novo monta o servidor. A senha é obrigatória.
func Novo(svc *core.Servico, agente *agent.Agente, senha string, log *slog.Logger) (*Servidor, error) {
	if senha == "" {
		return nil, errors.New("defina RANGO_SENHA: a web sempre pede senha")
	}
	s := &Servidor{svc: svc, agente: agente, conversa: &conversa{}, sessao: novaSessao(senha), log: log}
	if err := s.carregarTemplates(); err != nil {
		return nil, err
	}
	return s, nil
}

// ValidarEndereco aceita só localhost ou um IP da Tailscale (100.64.0.0/10 ou fd7a:115c:a1e0::/48).
func ValidarEndereco(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("endereço inválido %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("use um IP, não um nome: %q", host)
	}
	tailscale4 := netip.MustParsePrefix("100.64.0.0/10")
	tailscale6 := netip.MustParsePrefix("fd7a:115c:a1e0::/48")
	if ip.IsLoopback() || tailscale4.Contains(ip) || tailscale6.Contains(ip) {
		return nil
	}
	return fmt.Errorf("o Rango só escuta em localhost ou na Tailscale, não em %s", host)
}

func (s *Servidor) carregarTemplates() error {
	funcs := template.FuncMap{
		"num":        num,
		"dec":        dec,
		"pct":        pct,
		"rotuloData": rotuloData,
		"iso":        func(t time.Time) string { return t.Format(core.FormatoData) },
		"hora":       func(t time.Time) string { return t.Format("15:04") },
		"campo":      campo,
		"dataDe": func(v string) time.Time {
			t, _ := time.ParseInLocation(core.FormatoData, v, time.Local)
			return t
		},
		"atalhoDe": func(rotulo, icone, mensagem string) map[string]string {
			return map[string]string{"Rotulo": rotulo, "Icone": icone, "Mensagem": mensagem}
		},
		"diaMais":  func(t time.Time, n int) time.Time { return t.AddDate(0, 0, n) },
		"refeicao": core.NomeRefeicao,
		"anel":     anel,
		"menos":    func(a, b float64) float64 { return a - b },
		"maior":    func(a, b float64) bool { return a > b },
		"barraDe": func(nome, cor string, valor, meta float64) map[string]any {
			return map[string]any{"Nome": nome, "Cor": cor, "Valor": valor, "Meta": meta}
		},
		"iconeRefeicao": func(chave string) string {
			return map[string]string{"cafe": "bakery_dining", "almoco": "lunch_dining", "lanche": "egg_alt", "jantar": "dinner_dining"}[chave]
		},
		"campoDe": func(nome, rotulo, unidade string, valor float64, tem bool) map[string]any {
			return map[string]any{"Nome": nome, "Rotulo": rotulo, "Unidade": unidade, "Valor": valor, "Tem": tem}
		},
		"abaDe": func(chave, icone, nome, atual string) map[string]any {
			return map[string]any{"Chave": chave, "Icone": icone, "Nome": nome, "Ativa": chave == atual}
		},
	}
	base, err := template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/base.html")
	if err != nil {
		return err
	}
	paginas, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		return err
	}
	s.paginas = map[string]*template.Template{}
	for _, p := range paginas {
		nome := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".html")
		if nome == "base" {
			continue
		}
		t, err := template.Must(base.Clone()).ParseFS(templatesFS, p)
		if err != nil {
			return err
		}
		s.paginas[nome] = t
	}
	return nil
}

// Handler devolve as rotas.
func (s *Servidor) Handler() http.Handler {
	static, _ := fs.Sub(staticFS, "static")
	m := http.NewServeMux()
	m.Handle("GET /static/", http.StripPrefix("/static/", estatico(http.FileServerFS(static))))
	m.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Service-Worker-Allowed", "/")
		http.ServeFileFS(w, r, static, "sw.js")
	})
	m.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		http.ServeFileFS(w, r, static, "manifest.webmanifest")
	})

	m.HandleFunc("GET /login", s.paginaLogin)
	m.HandleFunc("POST /login", s.entrar)
	m.HandleFunc("POST /sair", s.sair)

	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/diario", http.StatusSeeOther)
	})
	m.HandleFunc("GET /diario", s.diario)
	m.HandleFunc("GET /adicionar", s.adicionar)
	m.HandleFunc("GET /adicionar/busca", s.busca)
	m.HandleFunc("GET /adicionar/alimento/{id}", s.painelAlimento)
	m.HandleFunc("GET /adicionar/previa", s.previa)
	m.HandleFunc("POST /porcoes", s.novaPorcao)
	m.HandleFunc("POST /lancamentos", s.registrar)
	m.HandleFunc("POST /lancamentos/{id}/remover", s.remover)
	m.HandleFunc("POST /desfazer", s.desfazer)
	m.HandleFunc("GET /metas", s.paginaMetas)
	m.HandleFunc("POST /metas", s.salvarMetas)
	m.HandleFunc("GET /chat", s.chat)
	m.HandleFunc("POST /chat", s.enviar)
	m.HandleFunc("GET /chat/turnos/{n}", s.eventos)
	m.HandleFunc("POST /chat/cartoes/{n}/desfazer", s.desfazerCartao)
	m.HandleFunc("POST /chat/nova", s.novaConversa)
	m.HandleFunc("GET /receitas", s.receitas)
	m.HandleFunc("GET /receitas/nova", s.novaReceita)
	m.HandleFunc("POST /receitas", s.salvarReceita)
	m.HandleFunc("GET /receitas/{id}/editar", s.editarReceita)
	m.HandleFunc("POST /receitas/{id}", s.salvarReceita)
	m.HandleFunc("GET /ingredientes", s.buscaIngrediente)
	m.HandleFunc("GET /ingredientes/{id}", s.ingrediente)
	for _, aba := range []string{"resumo"} {
		m.HandleFunc("GET /"+aba, s.emBreve(aba))
	}
	return s.exigirLogin(cabecalhos(m))
}

func estatico(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Os arquivos vêm embutidos e mudam a cada build: o navegador sempre revalida.
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

func cabecalhos(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		h.ServeHTTP(w, r)
	})
}

// render escreve a página (ou o bloco, para respostas htmx) só se o template rodou inteiro.
func (s *Servidor) render(w http.ResponseWriter, status int, pagina, bloco string, dados any) {
	t, ok := s.paginas[pagina]
	if !ok {
		s.falha(w, fmt.Errorf("página %q não existe", pagina))
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, bloco, dados); err != nil {
		s.falha(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Servidor) falha(w http.ResponseWriter, err error) {
	switch {
	case core.EhValidacao(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, core.ErrNaoEncontrado):
		http.Error(w, "não encontrado", http.StatusNotFound)
	default:
		s.log.Error("erro interno", "err", err)
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

// --- funções dos templates ---

// num formata como inteiro no padrão brasileiro: 1420 -> "1.420".
func num(v float64) string {
	n := int64(math.Round(v))
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// dec formata com no máximo uma casa decimal e vírgula: 4.3 -> "4,3", 90 -> "90".
func dec(v float64) string {
	v = math.Round(v*10) / 10
	if v == math.Trunc(v) {
		return num(v)
	}
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", ",", 1)
}

// campo formata para um input de texto: sem separador de milhar e com vírgula, 1100.5 -> "1100,5".
func campo(v float64) string {
	return strings.Replace(strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64), ".", ",", 1)
}

// pct devolve a fração em porcentagem, limitada a 0..100.
func pct(valor, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Max(0, math.Min(100, valor/total*100))
}

// anel calcula o stroke-dashoffset do anel de calorias (r = 68).
func anel(valor, total float64) string {
	c := 2 * math.Pi * 68
	return strconv.FormatFloat(c*(1-pct(valor, total)/100), 'f', 1, 64)
}

var mesesCurtos = []string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
var diasCurtos = []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}

func rotuloData(t time.Time) string {
	hoje := time.Now()
	hoje = time.Date(hoje.Year(), hoje.Month(), hoje.Day(), 0, 0, 0, 0, t.Location())
	dia := fmt.Sprintf("%d %s", t.Day(), mesesCurtos[t.Month()-1])
	switch int(math.Round(t.Sub(hoje).Hours() / 24)) {
	case 0:
		return "Hoje, " + dia
	case -1:
		return "Ontem, " + dia
	case 1:
		return "Amanhã, " + dia
	}
	return diasCurtos[t.Weekday()] + ", " + dia
}
