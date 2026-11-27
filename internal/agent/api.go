package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// limites do provedor de API.
const (
	maxRodadas = 20 // respostas do modelo por turno (cada rodada pode chamar ferramentas)
	maxSessoes = 8  // conversas guardadas na memória
)

// ConfigAPI aponta para uma API compatível com a da OpenAI (Ollama, Gemini, Groq,
// OpenRouter, LM Studio, OpenAI).
type ConfigAPI struct {
	URL    string      // base da API, como http://localhost:11434/v1; vazio é a da OpenAI
	Modelo string      // obrigatório
	Chave  string      // opcional (o Ollama não pede)
	MCP    *sdk.Server // servidor MCP do Rango, atendido na memória
}

// API conversa com o Nutri por uma API de chat. As ferramentas vêm do MCP do Rango,
// ligado na memória do mesmo processo: o modelo só enxerga o que o MCP expõe.
type API struct {
	cliente     openai.Client
	modelo      string
	mcp         *sdk.ClientSession
	ferramentas []openai.ChatCompletionToolUnionParam
	prompts     map[string][]string // prompt -> nomes dos argumentos, na ordem

	mu      sync.Mutex
	sessoes map[string][]openai.ChatCompletionMessageParamUnion
	ordem   []string // sessões da mais antiga para a mais nova
}

// NovoAPI liga o MCP na memória e lê as ferramentas e os prompts.
func NovoAPI(ctx context.Context, cfg ConfigAPI) (*API, error) {
	if cfg.Modelo == "" {
		return nil, errors.New("defina RANGO_LLM_MODEL")
	}
	opts := []option.RequestOption{option.WithMaxRetries(2)}
	if cfg.URL != "" {
		opts = append(opts, option.WithBaseURL(cfg.URL))
	}
	if cfg.Chave != "" {
		opts = append(opts, option.WithAPIKey(cfg.Chave))
	}
	a := &API{cliente: openai.NewClient(opts...), modelo: cfg.Modelo,
		prompts: map[string][]string{}, sessoes: map[string][]openai.ChatCompletionMessageParamUnion{}}

	tServ, tCli := sdk.NewInMemoryTransports()
	if _, err := cfg.MCP.Connect(ctx, tServ, nil); err != nil {
		return nil, err
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "rango-nutri"}, nil).Connect(ctx, tCli, nil)
	if err != nil {
		return nil, err
	}
	a.mcp = cs
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(t.InputSchema)
		if err != nil {
			return nil, err
		}
		var params shared.FunctionParameters
		if err := json.Unmarshal(b, &params); err != nil {
			return nil, err
		}
		a.ferramentas = append(a.ferramentas, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name: t.Name, Description: openai.String(t.Description), Parameters: params,
		}))
	}
	for p, err := range cs.Prompts(ctx, nil) {
		if err != nil {
			return nil, err
		}
		var nomes []string
		for _, arg := range p.Arguments {
			nomes = append(nomes, arg.Name)
		}
		a.prompts[p.Name] = nomes
	}
	return a, nil
}

// Fechar desliga o MCP.
func (a *API) Fechar() error { return a.mcp.Close() }

// Disponivel implementa Provedor. A API só é testada de verdade na primeira mensagem.
func (a *API) Disponivel() error { return nil }

// Conversar implementa Provedor. O histórico fica na memória, por sessão.
func (a *API) Conversar(ctx context.Context, sessao, mensagem string, comando bool, emitir func(Evento)) (string, error) {
	if comando {
		m, err := a.expandir(ctx, mensagem)
		if err != nil {
			return sessao, err
		}
		mensagem = m
	}
	hist := a.historico(sessao)
	if sessao == "" || hist == nil {
		sessao = novoID()
		hist = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(promptNutri)}
	}
	hist = append(hist, openai.UserMessage(comAgora(mensagem)))

	for range maxRodadas {
		msg, err := a.rodada(ctx, hist, emitir)
		if err != nil {
			return sessao, err
		}
		hist = append(hist, msg.ToParam())
		if len(msg.ToolCalls) == 0 {
			a.guardar(sessao, hist)
			return sessao, nil
		}
		for _, tc := range msg.ToolCalls {
			nome := tc.Function.Name
			emitir(Evento{Tipo: Ferramenta, Ferramenta: nome})
			saida, falhou := a.chamar(ctx, nome, tc.Function.Arguments)
			emitir(Evento{Tipo: Resultado, Ferramenta: nome, Saida: saida, Erro: falhou})
			hist = append(hist, openai.ToolMessage(saida, tc.ID))
		}
	}
	return sessao, fmt.Errorf("o Nutri passou de %d rodadas de ferramentas sem responder", maxRodadas)
}

// rodada pede uma resposta ao modelo, emitindo o texto aos pedaços, e devolve a mensagem
// completa. O acumulador do SDK junta os pedaços das chamadas de ferramenta.
func (a *API) rodada(ctx context.Context, hist []openai.ChatCompletionMessageParamUnion, emitir func(Evento)) (openai.ChatCompletionMessage, error) {
	stream := a.cliente.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: a.modelo, Messages: hist, Tools: a.ferramentas,
	})
	defer func() { _ = stream.Close() }()
	var acc openai.ChatCompletionAccumulator
	for stream.Next() {
		ch := stream.Current()
		acc.AddChunk(ch)
		if len(ch.Choices) > 0 && ch.Choices[0].Delta.Content != "" {
			emitir(Evento{Tipo: Texto, Texto: ch.Choices[0].Delta.Content})
		}
	}
	if err := stream.Err(); err != nil {
		if ctx.Err() != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("o Nutri demorou demais: %w", ctx.Err())
		}
		return openai.ChatCompletionMessage{}, fmt.Errorf("API do modelo: %w", err)
	}
	if len(acc.Choices) == 0 {
		return openai.ChatCompletionMessage{}, errors.New("a API do modelo não devolveu resposta")
	}
	return acc.Choices[0].Message, nil
}

// chamar roda uma ferramenta do MCP. Erros voltam ao modelo como texto, para ele corrigir.
func (a *API) chamar(ctx context.Context, nome, argumentos string) (string, bool) {
	if strings.TrimSpace(argumentos) == "" {
		argumentos = "{}"
	}
	if !json.Valid([]byte(argumentos)) {
		return "argumentos não são JSON válido: " + argumentos, true
	}
	res, err := a.mcp.CallTool(ctx, &sdk.CallToolParams{Name: nome, Arguments: json.RawMessage(argumentos)})
	if err != nil {
		return err.Error(), true
	}
	var out strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*sdk.TextContent); ok {
			out.WriteString(t.Text)
		}
	}
	return out.String(), res.IsError
}

// expandir troca "/mcp__nutri__<prompt> <args>" pelo texto do prompt. Os argumentos
// vão na ordem em que o prompt os declara.
func (a *API) expandir(ctx context.Context, comando string) (string, error) {
	campos := strings.Fields(strings.TrimPrefix(comando, "/"+prefixo))
	if len(campos) == 0 {
		return "", errors.New("comando vazio")
	}
	nomes, ok := a.prompts[campos[0]]
	if !ok {
		return "", fmt.Errorf("prompt desconhecido: %s", campos[0])
	}
	args := map[string]string{}
	for i, v := range campos[1:] {
		if i < len(nomes) {
			args[nomes[i]] = v
		}
	}
	res, err := a.mcp.GetPrompt(ctx, &sdk.GetPromptParams{Name: campos[0], Arguments: args})
	if err != nil {
		return "", err
	}
	var out []string
	for _, m := range res.Messages {
		if t, ok := m.Content.(*sdk.TextContent); ok {
			out = append(out, t.Text)
		}
	}
	return strings.Join(out, "\n\n"), nil
}

func (a *API) historico(sessao string) []openai.ChatCompletionMessageParamUnion {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]openai.ChatCompletionMessageParamUnion(nil), a.sessoes[sessao]...)
}

// guardar salva o histórico de um turno que terminou bem. Um turno com erro não entra,
// para o histórico nunca ficar com uma chamada de ferramenta sem resposta.
func (a *API) guardar(sessao string, hist []openai.ChatCompletionMessageParamUnion) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.sessoes[sessao]; !ok {
		a.ordem = append(a.ordem, sessao)
		if len(a.ordem) > maxSessoes {
			delete(a.sessoes, a.ordem[0])
			a.ordem = a.ordem[1:]
		}
	}
	a.sessoes[sessao] = hist
}

func novoID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
