package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gioobra/rango/internal/core"
	"github.com/gioobra/rango/internal/mcp"
	"github.com/gioobra/rango/internal/store"
)

// apiFalsa responde em SSE, como uma API compatível com a da OpenAI. A cada pedido
// devolve a próxima lista de chunks e guarda o corpo recebido.
type apiFalsa struct {
	mu        sync.Mutex
	respostas [][]string
	pedidos   []map[string]any
}

func (f *apiFalsa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var corpo map[string]any
	_ = json.Unmarshal(b, &corpo)
	f.mu.Lock()
	n := len(f.pedidos)
	f.pedidos = append(f.pedidos, corpo)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	for _, c := range f.respostas[n] {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", c)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}

func chunk(delta string, fim string) string {
	f := "null"
	if fim != "" {
		f = `"` + fim + `"`
	}
	return `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":` +
		delta + `,"finish_reason":` + f + `}]}`
}

func novaAPI(t *testing.T, f *apiFalsa) *API {
	t.Helper()
	ctx := context.Background()
	db, err := store.Abrir(ctx, filepath.Join(t.TempDir(), "rango.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Fechar() })
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	a, err := NovoAPI(ctx, ConfigAPI{URL: srv.URL, Modelo: "m", Chave: "k", MCP: mcp.Novo(core.Novo(db), "teste", false)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Fechar() })
	return a
}

// A chamada de ferramenta chega em pedaços; o Nutri roda o MCP, devolve o resultado
// ao modelo e emite os mesmos eventos do claude -p.
func TestAPIFerramentaEmPedacos(t *testing.T) {
	f := &apiFalsa{respostas: [][]string{
		{
			chunk(`{"role":"assistant","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"resumo_dia","arguments":""}}]}`, ""),
			chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"da"}}]}`, ""),
			chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"ta\":\"2026-01-05\"}"}}]}`, ""),
			chunk(`{}`, "tool_calls"),
		},
		{
			chunk(`{"role":"assistant","content":"Dia "}`, ""),
			chunk(`{"content":"vazio."}`, ""),
			chunk(`{}`, "stop"),
		},
		{chunk(`{"role":"assistant","content":"ok"}`, "stop")},
	}}
	a := novaAPI(t, f)

	var evs []Evento
	sessao, err := a.Conversar(context.Background(), "", "como está meu dia?", false, func(e Evento) { evs = append(evs, e) })
	if err != nil {
		t.Fatal(err)
	}
	var texto strings.Builder
	var res *Evento
	for i, e := range evs {
		switch e.Tipo {
		case Texto:
			texto.WriteString(e.Texto)
		case Resultado:
			res = &evs[i]
		}
	}
	if texto.String() != "Dia vazio." {
		t.Errorf("texto = %q", texto.String())
	}
	if res == nil || res.Ferramenta != "resumo_dia" || res.Erro || !strings.Contains(res.Saida, `"data":"2026-01-05"`) {
		t.Fatalf("resultado = %+v", res)
	}

	// O segundo pedido leva a resposta da ferramenta ligada ao id da chamada.
	msgs := f.pedidos[1]["messages"].([]any)
	ultima := msgs[len(msgs)-1].(map[string]any)
	if ultima["role"] != "tool" || ultima["tool_call_id"] != "c1" {
		t.Errorf("última mensagem = %v", ultima)
	}
	if len(f.pedidos[0]["tools"].([]any)) == 0 {
		t.Error("pedido sem ferramentas")
	}

	// A sessão continua: o terceiro pedido traz o histórico (sistema, usuário,
	// chamada, ferramenta, resposta) e a mensagem nova.
	if _, err := a.Conversar(context.Background(), sessao, "e agora?", false, func(Evento) {}); err != nil {
		t.Fatal(err)
	}
	if n := len(f.pedidos[2]["messages"].([]any)); n != 6 {
		t.Errorf("histórico com %d mensagens, esperava 6", n)
	}
}

// Atalhos da web viram o texto do prompt do MCP, com os argumentos na ordem declarada.
func TestAPIComando(t *testing.T) {
	f := &apiFalsa{respostas: [][]string{{chunk(`{"role":"assistant","content":"ok"}`, "stop")}}}
	a := novaAPI(t, f)
	if _, err := a.Conversar(context.Background(), "", Comando("sugerir_refeicao", "jantar"), true, func(Evento) {}); err != nil {
		t.Fatal(err)
	}
	msgs := f.pedidos[0]["messages"].([]any)
	u := msgs[len(msgs)-1].(map[string]any)["content"].(string)
	if !strings.HasPrefix(u, "[agora: ") || !strings.Contains(u, "no jantar") {
		t.Errorf("mensagem = %q", u)
	}
}
