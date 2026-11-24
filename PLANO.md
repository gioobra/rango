# Rango — Plano

Diário alimentar local, leve e em português, com um agente (**Nutri**) que registra, corrige, planeja e analisa o que você come.

- **Rango** é o app: binário Go único, roda no PC, acessado pelo celular via Tailscale.
- **Nutri** é o agente: Claude Code em modo headless, conversando com o Rango via MCP.

---

## 1. Visão geral

```
Celular (navegador / PWA) ──Tailscale──► rango serve (PC)
                                           ├─ telas: diário, receitas, resumo
                                           └─ chat ──► claude -p  (Nutri)
                                                         └─ MCP stdio ─► rango mcp ─► SQLite
```

- Tudo roda no PC. O celular só abre a interface web.
- O chat da web é **apenas um cliente** do MCP. O mesmo MCP funciona no Claude Code, no Claude Desktop ou em qualquer outro cliente. Trocar o motor do chat (API direta, Ollama) não muda as ferramentas.
- O modelo roda na nuvem da Anthropic. Dados e programa ficam no PC.

## 2. Funcionalidades

| Função | Exemplo |
|---|---|
| Registro por texto | "almocei um prato de arroz, uma concha de feijão e um bife" |
| Correção | "na verdade foram 200g", "desfaz o último" |
| Receitas e marmitas | "fiz 4 marmitas com 500g de frango, 600g de batata e 100g de azeite" |
| Planejar | "o que janto pra fechar a proteína de hoje?", plano da semana e lista de compras |
| Analisar | revisão semanal, tendências, médias e (com peso) estimativa de TDEE |

Fora do escopo: exercícios e treinos.

## 3. Stack

| Camada | Escolha | Motivo |
|---|---|---|
| Linguagem | Go | Binário único, compilação cruzada simples |
| HTTP | `net/http` da stdlib (rotas com padrões, Go 1.22+) | Sem dependência de router |
| Templates | `html/template` + `embed` | Stdlib, sem arquivos gerados |
| Interatividade | htmx + SSE (streaming do chat) | Sem SPA e sem build de JS |
| Estilo | Tailwind (CLI standalone, sem Node) | É o formato que o Stitch exporta |
| Design | Google Stitch | Telas geradas e adaptadas aos templates |
| Banco | SQLite via `modernc.org/sqlite` | Sem CGO, um arquivo só |
| MCP | SDK oficial em Go (`modelcontextprotocol/go-sdk`) | Transporte stdio |
| Agente | `claude -p` (headless, `stream-json`, `--resume`) | Usa o login do Claude Code local |
| Alimentos | Tabela TACO (Unicamp) embutida como seed | Alimentos brasileiros desde a primeira execução |

Referência de código: `grafana/mcp-grafana` (MCP oficial em Go).

### Design (fase 0)
Projeto no Stitch: "Rango Diário Alimentar". O export fica em `design/stitch/`, com um `code.html` por tela, em versão clara e escura.
- **Telas:** Diário do dia, Chat com o Nutri, Adicionar alimento, Receitas, Resumo. Navegação inferior com 4 abas (Diário, Chat, Receitas, Resumo). Adicionar alimento abre por cima e não tem a barra.
- **Tema:** tokens no estilo Material 3, definidos em `design/stitch/*/DESIGN.md`. No claro, primária verde-folha `#006b2c`, superfície `#f8f9ff` e laranja `#994100` para estouro de meta. No escuro, superfície grafite `#111316`, primária `#4be277` e laranja `#ffb784`. O tema segue o sistema (`prefers-color-scheme`).
- **Tipografia e ícones:** Plus Jakarta Sans e Material Symbols Outlined.
- **Identidade:** o ícone do app é um robozinho nutricionista com uma folha, usado também como avatar do Nutri. O app não usa fotos de pessoas nem de comida.
- **Ao adaptar na fase 1:** o HTML do Stitch puxa Tailwind via CDN, fontes do Google Fonts e imagens do `googleusercontent`. Tudo isso vira asset embutido: CSS pelo Tailwind CLI, fontes e ícone em `internal/web/static/`. O app não pode depender de rede externa.

## 4. Arquitetura e organização

### Princípios
- **Camadas com dependência em uma direção só:** `web` e `mcp` → `core` → `store`. O `core` não conhece HTTP, MCP nem SQL.
- **Toda regra de negócio fica no `core`.** Handlers web e ferramentas MCP são finos: validam a entrada, chamam o `core` e formatam a saída.
- **Um binário, vários subcomandos.** Nada de serviços separados.
- **Assets embutidos** (`embed.FS`): templates, CSS, JS, migrations e seed da TACO. O binário é autossuficiente.
- **Dados fora do repositório**, seguindo XDG: banco em `~/.local/share/rango/rango.db`.
- **Poucas dependências.** Cada dependência nova precisa de um motivo claro.

### Estrutura de pastas
```
rango/
├── .claude/              # CLAUDE.md (regras de trabalho) e skills/docs/
├── design/stitch/        # export bruto do Stitch (referência, não vai no binário)
├── cmd/rango/            # main: subcomandos serve | mcp | import
├── internal/
│   ├── core/             # domínio: alimentos, porções, lançamentos, receitas, metas, resumos
│   ├── store/            # SQLite: repositórios, migrations/, seed/ (TACO)
│   ├── web/              # handlers, templates/, static/
│   ├── mcp/              # ferramentas e prompts MCP → core
│   └── agent/            # ponte com o claude -p, prompt do Nutri (embutido)
├── go.mod
├── Makefile
├── README.md
└── PLANO.md
```

### Regras de organização
- **Documentação enxuta:** na raiz, só `README.md` (como instalar e usar) e este `PLANO.md`, que guarda as decisões. Nada de pasta `docs/` cheia de `.md`. Exceções:
  - `.claude/CLAUDE.md`: curto, importa este plano e lista as regras de trabalho pro Claude Code;
  - `.claude/skills/docs/`: a skill `/docs`, que sincroniza a documentação no fim de cada fase;
  - o prompt do Nutri, em `internal/agent/`, embutido no binário;
  - os `DESIGN.md` do export do Stitch, em `design/stitch/`, mantidos como vieram.
- **Poucos testes, nos pontos que importam.** É um projeto pequeno. Os testes ficam ao lado do código (`*_test.go`) e cobrem só o que é fácil errar sem perceber: cálculo de macros e porções, receitas (rendimento e macros por porção), desfazer e auditoria, e as migrations. Handlers, templates e código trivial não precisam de teste.
- **Migrations numeradas** (`0001_inicial.sql`, ...), aplicadas automaticamente ao iniciar.
- **Formatação e lint:** `gofmt` e `golangci-lint`, sem exceção.
- **Commits** no padrão Conventional Commits (`feat:`, `fix:`, `refactor:` ...).
- **Configuração** por flags e variáveis de ambiente (`RANGO_*`). Sem arquivo de config enquanto não for necessário.
- **Makefile** com poucos alvos: `run`, `build`, `test`, `lint`, `css`.

## 5. Modelo de dados (rascunho)

```
alimentos       id, nome, fonte (taco|usuario|receita), kcal, prot, carb, gord, fibra   -- por 100g
porcoes         alimento_id, nome ("1 concha"), gramas
receitas        alimento_id, rendimento_g, n_porcoes
receita_itens   receita_id, alimento_id, gramas
lancamentos     id, data, refeicao, alimento_id, gramas,
                kcal, prot, carb, gord,          -- congelados no momento do registro
                origem (manual|agente), estimado, criado_em
metas           vigente_desde, kcal, prot, carb, gord
auditoria       id, lancamento_id, acao, antes, depois, origem, em
peso            data, kg                         -- fase 4
```

- Os macros ficam **congelados no lançamento**, então editar uma receita não altera o histórico.
- As **porções caseiras** são do usuário ("minha concha = 90g"), e o Nutri passa a usá-las.
- A **auditoria** guarda o antes e o depois, o que viabiliza o desfazer e mostra o que o agente mexeu.

## 6. Ferramentas MCP

Conjunto pequeno e bem escolhido (lição do Karakeep), com a escrita sob controle (lição do Actual Budget e do Grafana).

| Fase | Ferramentas |
|---|---|
| 2 | `buscar_alimento`, `registrar_refeicao` (vários itens), `editar_lancamento`, `remover_lancamento`, `desfazer`, `resumo_dia`, `definir_porcao`, `metas`, `definir_metas` |
| 3 | `criar_receita`, `listar_receitas` |
| 4 | `historico(periodo)`, `registrar_peso` + prompts `revisao_semanal`, `sugerir_refeicao` |

- A flag `--read-only` expõe só as ferramentas de leitura.
- Os botões rápidos da web ("Resumo da semana", "O que janto?") usam os mesmos prompts do MCP.

## 7. O Nutri

O prompt fica embutido no binário e é passado ao `claude -p`. Regras base:
- Sempre usar `buscar_alimento` antes de registrar, preferindo os itens da TACO.
- Usar as porções salvas do usuário. Quando estimar, marcar `estimado`.
- Depois de registrar, mostrar o resumo do dia comparado com a meta.
- Nunca remover sem confirmação.
- Responder em português, de forma curta.

## 8. Segurança

- `rango serve` escuta **só** na interface da Tailscale ou em localhost, nunca em `0.0.0.0`.
- A web pede senha, mesmo dentro da Tailscale.
- O `claude -p` roda com `--allowedTools "mcp__nutri__*"`: sem Bash, sem edição de arquivos e sem web.
- O agente roda num diretório de trabalho dedicado, isolado do repositório.
- O MCP é só stdio, então não precisa de autenticação nem de exposição na rede.

## 9. Fases

- [x] **0. Design**: telas no Stitch (diário do dia, chat, adicionar alimento, receitas, resumo), pensadas primeiro pro celular
- [ ] **1. Núcleo**: estrutura do projeto, schema e migrations, seed da TACO, diário e metas na web, PWA, senha
- [ ] **2. Nutri**: `rango mcp` (stdio), chat via `claude -p` com streaming, cartões de lançamento com desfazer
- [ ] **3. Receitas e marmitas**: alimento composto, rendimento, porções
- [ ] **4. Planejar e analisar**: histórico, peso, revisão semanal, sugestões

## 10. Em aberto

- Confirmar as flags atuais do `claude -p` (`--output-format stream-json`, `--resume`, `--allowedTools`, prompt de sistema).
- Termos de uso da TACO: citar a fonte (NEPA/Unicamp) no app e no README.
- Fotos (de prato e de rótulo): avaliar depois da fase 2.
