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
- O `claude -p` chama o próprio executável do Rango (`rango mcp`) como servidor MCP, apontando para o mesmo banco. A web e o MCP escrevem no mesmo SQLite (WAL com `busy_timeout`).

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
| MCP | SDK oficial em Go (`modelcontextprotocol/go-sdk` v1.8) | Transporte stdio, esquema das ferramentas gerado das structs |
| Agente | `claude -p` (headless, `stream-json`, `--resume`) | Usa o login do Claude Code local |
| Alimentos | Tabela TACO (Unicamp) embutida como seed | Alimentos brasileiros desde a primeira execução |

Referência de código: `grafana/mcp-grafana` (MCP oficial em Go).

### Design (fase 0)
Projeto no Stitch: "Rango Diário Alimentar". O export fica em `design/stitch/`, com um `code.html` por tela, em versão clara e escura.
- **Telas:** Diário do dia, Chat com o Nutri, Adicionar alimento, Receitas, Resumo. Navegação inferior com 4 abas (Diário, Chat, Receitas, Resumo). Adicionar alimento abre por cima e não tem a barra.
- **Tema:** tokens no estilo Material 3, definidos em `design/stitch/*/DESIGN.md`. No claro, primária verde-folha `#006b2c`, superfície `#f8f9ff` e laranja `#994100` para estouro de meta. No escuro, superfície grafite `#111316`, primária `#4be277` e laranja `#ffb784`. O tema segue o sistema (`prefers-color-scheme`).
- **Tipografia e ícones:** Plus Jakarta Sans e Material Symbols Outlined.
- **Identidade:** o ícone do app é um robozinho nutricionista com uma folha, usado também como avatar do Nutri. O app não usa fotos de pessoas nem de comida.
- **Adaptação para os templates:** o HTML do Stitch é só referência, porque puxa Tailwind por CDN, fontes do Google Fonts e imagens do `googleusercontent`. Os templates reescrevem as telas com os mesmos tokens, e tudo isso fica embutido no binário (ver Princípios na seção 4). O robô virou o ícone do PWA e o avatar (`nutri.png`); o logo do Stitch foi trocado pelo ícone `nutrition`.

## 4. Arquitetura e organização

### Princípios
- **Camadas com dependência em uma direção só:** `web` e `mcp` → `core` → `store`. O `core` não conhece HTTP, MCP nem SQL.
- **Toda regra de negócio fica no `core`.** Handlers web e ferramentas MCP são finos: validam a entrada, chamam o `core` e formatam a saída.
- **Um binário, vários subcomandos.** Nada de serviços separados.
- **Assets embutidos** (`embed.FS`): templates, CSS, JS, fontes, migrations e seed da TACO. O binário é autossuficiente e não busca nada na internet: htmx, Plus Jakarta Sans e o subconjunto do Material Symbols ficam em `internal/web/static/`.
- **CSS gerado e versionado:** `internal/web/static/app.css` sai do Tailwind CLI (`make css`) e vai no git, para `go build` funcionar sem o Tailwind instalado. A entrada é `internal/web/tailwind.css`. O tema escuro troca as variáveis de cor via `prefers-color-scheme`, sem classes `dark:`.
- **Dados fora do repositório**, seguindo XDG: banco em `~/.local/share/rango/rango.db`.
- **Poucas dependências.** Cada dependência nova precisa de um motivo claro.

### Estrutura de pastas
```
rango/
├── .claude/              # CLAUDE.md (regras de trabalho) e skills/docs/
├── design/stitch/        # export bruto do Stitch (referência, não vai no binário)
├── cmd/rango/            # main: subcomandos serve (fase 1) | mcp (fase 2) | import (em aberto)
├── internal/
│   ├── core/             # domínio: alimentos, porções, lançamentos, receitas, metas, resumos
│   ├── store/            # SQLite: repositórios, migrations/, seed/ (TACO)
│   ├── web/              # handlers, templates/, static/
│   ├── mcp/              # ferramentas e prompts MCP → core
│   └── agent/            # ponte com o claude -p, prompt do Nutri (embutido)
├── go.mod, go.sum
├── .gitignore            # .env e bin/
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
alimentos       id, nome, nome_busca, categoria, fonte (taco|usuario|receita), taco_id,
                kcal, prot, carb, gord, fibra                  -- por 100g
porcoes         id, alimento_id, nome ("1 concha"), gramas
receitas        alimento_id, rendimento_g, n_porcoes, nome_porcao, criado_em, editado_em
receita_itens   id, receita_id, alimento_id, gramas            -- gramas crus, como foram para a panela
lancamentos     id, data, refeicao (cafe|almoco|lanche|jantar), alimento_id, gramas,
                kcal, prot, carb, gord,                        -- congelados no momento do registro
                origem (manual|agente), estimado, criado_em, removido_em
metas           vigente_desde, kcal, prot, carb, gord
auditoria       id, lancamento_id, acao (criar|editar|remover|desfazer), antes, depois, origem, em, desfeito
peso            data, kg                                       -- fase 4
```

- Os macros ficam **congelados no lançamento**, então editar uma receita não altera o histórico.
- **Receita é um alimento** (`fonte = 'receita'`): os valores por 100 g saem da soma dos ingredientes ÷ peso pronto. Assim ela entra na busca, no registro e nas porções sem caminho especial. Ao salvar, cria a porção `nome_porcao` ("1 marmita") = peso pronto ÷ `n_porcoes`; editar troca essa porção e mantém as que o usuário criou.
- **Peso pronto** é opcional: sem ele, vale a soma dos ingredientes crus. Não há coluna para "informado"; quando o rendimento é igual à soma, a edição mostra o campo vazio e a soma é refeita se os ingredientes mudarem.
- **Alimentos do usuário** (`fonte = 'usuario'`) entram por `criar_alimento`, com valores por 100 g de um rótulo. Nomes não repetem dentro da mesma fonte.
- As **porções caseiras** são do usuário ("minha concha = 90g"), e o Nutri passa a usá-las.
- A **auditoria** guarda o antes e o depois (o lançamento em JSON), o que viabiliza o desfazer e mostra o que o agente mexeu.
- **Remoção lógica** (`removido_em`): remover não apaga a linha, então o desfazer só restaura o estado anterior. O desfazer é uma pilha global: reverte a mudança mais recente ainda não desfeita, de qualquer origem.
- **Editar a quantidade** reescala os macros congelados na proporção, sem reler o alimento.
- **Busca** em `nome_busca` (minúsculo e sem acento), exigindo todos os termos. Ordem: alimentos do usuário, depois os da TACO; crus por último; nomes mais curtos primeiro.
- **Seed da TACO:** `internal/store/seed/taco.csv`, gerado a partir do CSV do projeto [raulfdm/taco-api](https://github.com/raulfdm/taco-api) (conversão da planilha oficial). "Tr" e células vazias viram 0. Os 6 alimentos sem kcal na tabela original (leite de vaca integral, leite desnatado UHT, iogurte de abacaxi, sal dietético, sal grosso e coco verde cru) ficaram de fora, para não registrar 0 kcal por engano. Sobram 591 alimentos, carregados na primeira execução.

## 6. Ferramentas MCP

Conjunto pequeno e bem escolhido (lição do Karakeep), com a escrita sob controle (lição do Actual Budget e do Grafana).

| Fase | Ferramentas |
|---|---|
| 2 | `buscar_alimento`, `registrar_refeicao` (vários itens), `editar_lancamento`, `remover_lancamento`, `desfazer`, `resumo_dia`, `definir_porcao`, `metas`, `definir_metas` |
| 3 | `criar_receita`, `editar_receita`, `listar_receitas`, `criar_alimento` |
| 4 | `historico(periodo)`, `registrar_peso` + prompts `revisao_semanal`, `sugerir_refeicao` |

- A flag `--read-only` expõe só as ferramentas de leitura (`buscar_alimento`, `resumo_dia`, `metas`, `listar_receitas`).
- `editar_receita` entrou porque a marmita se repete com outras quantidades, e `criar_alimento` resolve os produtos fora da TACO. `editar_receita` recebe a receita inteira, para não ter regras de mescla.
- As saídas são JSON estruturado. `buscar_alimento` já traz as porções salvas, e `registrar_refeicao` devolve os lançamentos e o resumo do dia, para o Nutri responder sem outra chamada.
- Datas em `AAAA-MM-DD`; sem data, vale hoje.
- Os atalhos da web ("Resumo de hoje", "O que janto?", "Desfazer último") hoje só mandam texto pronto. Quando os prompts da fase 4 existirem, "Resumo da semana" e "O que janto?" passam a usá-los.

## 7. O Nutri

O prompt (`internal/agent/nutri.txt`) fica embutido no binário e é passado com `--system-prompt`. Cada mensagem leva `[agora: data, dia, hora]`, porque a conversa atravessa dias e o prompt de sistema fica fixo na sessão. Regras base:
- Sempre usar `buscar_alimento` antes de registrar, preferindo os itens da TACO.
- Usar as porções salvas do usuário. Quando estimar, marcar `estimado`.
- Depois de registrar, mostrar o resumo do dia comparado com a meta.
- Nunca remover sem confirmação.
- Responder em português, de forma curta e em texto simples (a web não renderiza markdown).

### Chat na web
- O turno roda numa goroutine e guarda os eventos (texto, ferramenta, cartão, erro, fim). O SSE (`/chat/turnos/{n}`) retoma pelo `Last-Event-ID`, então o celular pode dormir no meio da resposta. Um turno por vez, com limite de 5 min.
- O streaming usa `EventSource` num `chat.js` pequeno, em vez da extensão SSE do htmx: menos um arquivo de terceiros e controle do reenvio.
- Resultados de `registrar_refeicao`, `editar_lancamento`, `remover_lancamento` e `desfazer` viram cartões. O "Desfazer" do cartão de registro remove a refeição inteira (`RemoverVarios`), e cada remoção entra na pilha do desfazer.
- A conversa (sessão do claude, mensagens e cartões) fica na memória do servidor. Reiniciar limpa a tela, e "Nova" começa outra sessão.
- `criar_receita` e `editar_receita` viram o cartão "Receita salva", com link para a receita.
- Ícones: o subconjunto do Material Symbols só tem os glifos em uso, então o botão de enviar é o `arrow_back` girado.

### Receitas na web
- Lista em `<details>` (sem JS): ingredientes, peso pronto, panela inteira e "Registrar 1 marmita", que usa o mesmo `POST /lancamentos` do diário.
- Formulário de receita com busca de ingrediente pelo htmx (`/ingredientes`). Números em campos de texto aceitam o formato brasileiro ("1.100" é mil e cem, "4,5") e são mostrados sem separador de milhar.

## 8. Segurança

- `rango serve` escuta **só** na interface da Tailscale ou em localhost, nunca em `0.0.0.0`.
- `rango serve` recusa qualquer endereço que não seja loopback, `100.64.0.0/10` ou `fd7a:115c:a1e0::/48` (Tailscale).
- A web pede senha, mesmo dentro da Tailscale. A senha vem de `RANGO_SENHA` e o servidor não sobe sem ela. A sessão fica num cookie assinado com HMAC-SHA256, com chave derivada da senha (trocar a senha derruba as sessões), `HttpOnly` e `SameSite=Strict`, o que já protege os formulários de CSRF. Senha errada espera 1 s.
- O `claude -p` roda com `--restricted --tools "" --strict-mcp-config --mcp-config <só o Rango> --allowedTools "mcp__nutri__*" --permission-mode dontAsk`: sem Bash, sem edição de arquivos, sem web e sem os MCPs e configurações do usuário. A mensagem vai pelo stdin, para não ser lida como flag.
- O agente roda em `~/.local/share/rango/nutri/` (ao lado do banco), isolado do repositório. O `mcp.json` é gerado ali a cada início.
- O MCP é só stdio, então não precisa de autenticação nem de exposição na rede.

## 9. Fases

- [x] **0. Design**: telas no Stitch (diário do dia, chat, adicionar alimento, receitas, resumo), pensadas primeiro pro celular
- [x] **1. Núcleo**: estrutura do projeto, schema e migrations, seed da TACO, diário e metas na web, PWA, senha
- [x] **2. Nutri**: `rango mcp` (stdio), chat via `claude -p` com streaming, cartões de lançamento com desfazer
- [x] **3. Receitas e marmitas**: alimento composto, rendimento, porções
- [ ] **4. Planejar e analisar**: histórico, peso, revisão semanal, sugestões

## 10. Em aberto

- Leite de vaca, iogurte de abacaxi e coco verde não vieram da TACO (sem kcal na tabela). Já dá para cadastrar com `criar_alimento` pelo Nutri; falta decidir se vêm prontos no seed, de outra fonte.
- Cadastro de alimento do usuário pela web (hoje só pelo Nutri).
- Excluir ou arquivar receitas. Uma receita usada em lançamentos não pode sumir do banco.
- Na busca para ingredientes, o cru costuma ser o certo, mas a ordem geral põe o cru por último.
- Subcomando `import` (previsto em `cmd/rango`): definir o que importa (alimentos próprios? backup?) antes de implementar.
- Fotos (de prato e de rótulo): avaliar. O rótulo combinaria com `criar_alimento`.
- Guardar o histórico do chat no banco, para sobreviver a reinícios do servidor (hoje fica na memória).
- Regerar o subconjunto do Material Symbols com `send` e outros ícones; falta a fonte completa no repositório ou um passo no `make`.
- Um cartão de registro continua com "Desfazer" mesmo se o Nutri já desfez aquele registro. Clicar não estraga nada, mas confunde.
