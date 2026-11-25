// Package agent liga o Rango ao Nutri: roda o claude -p em modo headless, com o
// MCP do próprio Rango como única ferramenta, e traduz o stream-json em eventos.
package agent

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed nutri.txt
var promptNutri string

// prefixo das ferramentas do MCP "nutri" vistas pelo claude.
const prefixo = "mcp__nutri__"

// Config diz onde estão as peças.
type Config struct {
	Claude string // binário do claude (no PATH ou caminho completo)
	Rango  string // executável do rango, que o claude chama como servidor MCP
	DB     string // banco que o MCP abre
	Dir    string // diretório de trabalho do agente, isolado do repositório
}

// Agente conversa com o Nutri.
type Agente struct {
	cfg       Config
	mcpConfig string
}

// Novo prepara o diretório de trabalho e a configuração do MCP.
func Novo(cfg Config) (*Agente, error) {
	if err := os.MkdirAll(cfg.Dir, 0o700); err != nil {
		return nil, err
	}
	conf := map[string]any{"mcpServers": map[string]any{"nutri": map[string]any{
		"command": cfg.Rango, "args": []string{"mcp"}, "env": map[string]string{"RANGO_DB": cfg.DB},
	}}}
	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return nil, err
	}
	a := &Agente{cfg: cfg, mcpConfig: filepath.Join(cfg.Dir, "mcp.json")}
	return a, os.WriteFile(a.mcpConfig, b, 0o600)
}

// Disponivel diz se o binário do claude foi encontrado.
func (a *Agente) Disponivel() error {
	_, err := exec.LookPath(a.cfg.Claude)
	if err != nil {
		return fmt.Errorf("claude não encontrado (%s): instale o Claude Code ou defina RANGO_CLAUDE", a.cfg.Claude)
	}
	return nil
}

// Tipos de evento.
const (
	Texto      = "texto"      // pedaço da resposta
	Ferramenta = "ferramenta" // o Nutri chamou uma ferramenta
	Resultado  = "resultado"  // a ferramenta respondeu
)

// Evento é algo que aconteceu durante a resposta.
type Evento struct {
	Tipo       string
	Texto      string // em Texto
	Ferramenta string // em Ferramenta e Resultado, sem o prefixo do MCP
	Saida      string // em Resultado: o JSON devolvido (ou a mensagem de erro)
	Erro       bool   // em Resultado: a ferramenta falhou
}

var diasSemana = []string{"domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"}

// Comando monta a chamada de um prompt do MCP do Rango ("/mcp__nutri__revisao_semanal").
// Só o código monta comandos: o texto digitado nunca vira comando, porque leva o prefixo de data.
func Comando(prompt string, args ...string) string {
	return strings.Join(append([]string{"/" + prefixo + prompt}, args...), " ")
}

// Conversar manda a mensagem ao Nutri, continuando a sessão se houver, e chama emitir
// a cada evento. Devolve o id da sessão para a próxima mensagem. Com comando, a mensagem
// vai como está (ver Comando).
func (a *Agente) Conversar(ctx context.Context, sessao, mensagem string, comando bool, emitir func(Evento)) (string, error) {
	if err := a.Disponivel(); err != nil {
		return sessao, err
	}
	args := []string{"-p",
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", a.mcpConfig,
		"--allowedTools", prefixo + "*", "--permission-mode", "dontAsk",
		"--system-prompt", promptNutri,
	}
	if sessao != "" {
		args = append(args, "--resume", sessao)
	}
	cmd := exec.CommandContext(ctx, a.cfg.Claude, args...)
	cmd.Dir = a.cfg.Dir
	cmd.WaitDelay = 5 * time.Second
	// A data vai na mensagem porque a conversa atravessa dias. Comandos vão sem prefixo,
	// senão o claude não os reconhece; os prompts do MCP já pedem os dados de hoje.
	if !comando {
		agora := time.Now()
		mensagem = fmt.Sprintf("[agora: %s, %s %s]\n%s",
			agora.Format("2006-01-02"), diasSemana[agora.Weekday()], agora.Format("15:04"), mensagem)
	}
	cmd.Stdin = strings.NewReader(mensagem)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	saida, err := cmd.StdoutPipe()
	if err != nil {
		return sessao, err
	}
	if err := cmd.Start(); err != nil {
		return sessao, err
	}

	l := leitor{nomes: map[string]string{}, emitir: emitir, sessao: sessao}
	sc := bufio.NewScanner(saida)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		l.linha(sc.Bytes())
	}
	errLeitura := sc.Err()
	errCmd := cmd.Wait()
	switch {
	case ctx.Err() != nil:
		return l.sessao, fmt.Errorf("o Nutri demorou demais: %w", ctx.Err())
	case l.falha != "":
		return l.sessao, errors.New(l.falha)
	case errLeitura != nil:
		return l.sessao, errLeitura
	case errCmd != nil:
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return l.sessao, fmt.Errorf("claude: %v: %s", errCmd, msg)
	}
	return l.sessao, nil
}

// leitor interpreta as linhas do stream-json.
type leitor struct {
	nomes  map[string]string // id da chamada -> ferramenta
	emitir func(Evento)
	sessao string
	falha  string
}

type bloco struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type linhaStream struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Event     struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
	Message struct {
		Content []bloco `json:"content"`
	} `json:"message"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

func (l *leitor) linha(b []byte) {
	var m linhaStream
	if json.Unmarshal(b, &m) != nil {
		return
	}
	if m.SessionID != "" {
		l.sessao = m.SessionID
	}
	switch m.Type {
	case "stream_event":
		if m.Event.Type == "content_block_delta" && m.Event.Delta.Type == "text_delta" {
			l.emitir(Evento{Tipo: Texto, Texto: m.Event.Delta.Text})
		}
	case "assistant":
		// O texto já chegou aos pedaços; daqui só interessam as chamadas de ferramenta.
		for _, c := range m.Message.Content {
			if c.Type == "tool_use" {
				nome := strings.TrimPrefix(c.Name, prefixo)
				l.nomes[c.ID] = nome
				l.emitir(Evento{Tipo: Ferramenta, Ferramenta: nome})
			}
		}
	case "user":
		for _, c := range m.Message.Content {
			if c.Type == "tool_result" {
				l.emitir(Evento{Tipo: Resultado, Ferramenta: l.nomes[c.ToolUseID], Saida: textoDe(c.Content), Erro: c.IsError})
			}
		}
	case "result":
		if m.IsError || m.Subtype != "success" {
			l.falha = "o Nutri não conseguiu responder"
			if m.Result != "" {
				l.falha += ": " + m.Result
			}
		}
	}
}

// textoDe junta o conteúdo de um tool_result, que vem como texto ou lista de blocos.
func textoDe(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var bs []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	_ = json.Unmarshal(raw, &bs)
	var out strings.Builder
	for _, b := range bs {
		if b.Type == "text" {
			out.WriteString(b.Text)
		}
	}
	return out.String()
}
