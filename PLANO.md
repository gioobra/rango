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
| Analisar | revisão semanal, tendências, médias, aderência à meta e (com peso) TDEE adaptativo, sempre calculados pelo app |

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
                kcal, prot, carb, gord, fibra,                 -- congelados no momento do registro
                origem (manual|agente), estimado, criado_em, removido_em
metas           vigente_desde, kcal, prot, carb, gord
auditoria       id, lancamento_id, acao (criar|editar|remover|desfazer), antes, depois, origem, em, desfeito
peso            data, kg                                       -- uma pesagem por dia (a última vale)
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
- **Fibra congelada** (fase 4): entrou depois. A migration preenche os lançamentos antigos a partir do alimento (gramas × fibra por 100 g), o que é exato para a TACO e aproximado para receitas editadas depois do registro.
- **Seed da TACO:** `internal/store/seed/taco.csv`, gerado a partir do CSV do projeto [raulfdm/taco-api](https://github.com/raulfdm/taco-api) (conversão da planilha oficial). "Tr" e células vazias viram 0. Os 6 alimentos sem kcal na tabela original (leite de vaca integral, leite desnatado UHT, iogurte de abacaxi, sal dietético, sal grosso e coco verde cru) ficaram de fora, para não registrar 0 kcal por engano. Sobram 591 alimentos, carregados na primeira execução.

### Análise determinística (fase 4)

Toda métrica é calculada no `core`, testada e devolvida pronta. O Nutri não faz conta: ele lê os números das ferramentas e explica. A aba Resumo mostra os mesmos números, então chat e tela nunca divergem.

- **Energia dos macros:** proteína e carboidrato valem 4 kcal/g e gordura 9 kcal/g. A distribuição (% das kcal de P, C e G) usa a soma dessas três, não a kcal da tabela, para fechar em 100%.
- **Dia registrado:** tem ao menos um lançamento. Dias vazios ficam fora das médias, e a cobertura (dias registrados ÷ dias do período) aparece junto, para não confundir "comi pouco" com "não anotei".
- **Período** (7, 14 ou 30 dias até uma data):
  - média diária e desvio padrão de kcal; médias de P, C, G e fibra;
  - aderência: dias a até ±10% da meta de kcal e dias com proteína ≥ meta, contados só entre os dias registrados que têm meta;
  - fatia de cada refeição nas kcal do período;
  - os 5 alimentos que mais trouxeram kcal e os 5 que mais trouxeram proteína;
  - fatia das kcal que veio de quantidade estimada.
- **Peso:** tendência por média móvel exponencial (α = 0,1, como no Hacker's Diet). Começa na primeira pesagem e, em dia sem pesagem, repete o valor anterior. Assim a oscilação de água do dia a dia não engana.
- **TDEE adaptativo:** média das kcal dos dias registrados − (tendência no fim − tendência no início) × 7.700 kcal/kg ÷ dias, numa janela de até 28 dias. Só é calculado com ao menos 14 dias desde a primeira pesagem dentro da janela e 70% dos dias registrados. Senão, a resposta diz o que falta. Substitui fórmulas de gasto basal (que pediriam idade, sexo e altura) porque mede o próprio usuário.
- **Proteína por kg:** média de proteína do período ÷ tendência de peso atual.
- **Simulação:** `simular_refeicao` calcula itens hipotéticos sobre o dia (macros, dia depois, restante, se fica na meta) sem registrar. É o que deixa as sugestões do Nutri determinísticas: ele propõe e ajusta os gramas, o `core` faz a conta.
- **Meta de cada dia:** a aderência usa a meta vigente naquele dia, não a atual. Dias anteriores à primeira meta ficam fora da aderência.
- **Gráficos da aba Resumo:** HTML e SVG montados no servidor, sem biblioteca de JS. Cada gráfico tem uma série só (kcal por dia com a linha da meta; tendência de peso com os pontos das pesagens), dica nativa (`title`) ao passar o dedo ou o mouse, e a tabela dos dias para quem não lê o gráfico.
- **Cores dos macros:** azul (P), laranja (C) e verde-água (G), da paleta de referência da skill de visualização, validadas para daltonismo no claro e no escuro. As antigas (verde e laranja do tema) falhavam. Os dois design systems do Stitch (claro e escuro) descrevem essa paleta, para que telas novas geradas lá não voltem às cores antigas.
- **Layout da aba Resumo:** redesenhado a partir de variantes geradas no Stitch. A média de kcal virou o número principal, junto com a cobertura. Proteína e aderência aparecem em cartões menores, com selos ("na meta", "quase lá", "abaixo"). A faixa de ±10% sai de `core.FaixaKcal`, a mesma regra da aderência. O "quase lá" (≥ 90% da meta de proteína) é só um rótulo e não muda a aderência. No gráfico de kcal, cada barra mostra o valor (até 7 dias) e o dia de hoje fica destacado. O top 5 tem duas abas (calorias e proteína), feitas só com CSS (rádios e `:has`), sem JS.

## 6. Ferramentas MCP

Conjunto pequeno e bem escolhido (lição do Karakeep), com a escrita sob controle (lição do Actual Budget e do Grafana).

| Fase | Ferramentas |
|---|---|
| 2 | `buscar_alimento`, `registrar_refeicao` (vários itens), `editar_lancamento`, `remover_lancamento`, `desfazer`, `resumo_dia`, `definir_porcao`, `metas`, `definir_metas` |
| 3 | `criar_receita`, `editar_receita`, `listar_receitas`, `criar_alimento` |
| 4 | `analise_periodo`, `peso` (tendência e TDEE), `simular_refeicao`, `registrar_peso` + prompts `revisao_semanal`, `sugerir_refeicao` |

- A flag `--read-only` expõe só as ferramentas de leitura (`buscar_alimento`, `resumo_dia`, `metas`, `listar_receitas`).
- `editar_receita` entrou porque a marmita se repete com outras quantidades, e `criar_alimento` resolve os produtos fora da TACO. `editar_receita` recebe a receita inteira, para não ter regras de mescla.
- As saídas são JSON estruturado. `buscar_alimento` já traz as porções salvas, e `registrar_refeicao` devolve os lançamentos e o resumo do dia, para o Nutri responder sem outra chamada.
- Datas em `AAAA-MM-DD`; sem data, vale hoje.
- Os atalhos "Revisão da semana" e "O que eu como?" da web (e o botão da aba Resumo) mandam `/mcp__nutri__revisao_semanal` e `/mcp__nutri__sugerir_refeicao <refeição>`: o `claude -p` expande os prompts do MCP como comandos. A bolha mostra o rótulo. Esses comandos vão sem o prefixo `[agora: ...]`, que os quebraria; o texto digitado sempre leva o prefixo, então nunca vira comando.

## 7. O Nutri

O prompt (`internal/agent/nutri.txt`) fica embutido no binário e é passado com `--system-prompt`. Cada mensagem leva `[agora: data, dia, hora]`, porque a conversa atravessa dias e o prompt de sistema fica fixo na sessão. Regras base:
- Sempre usar `buscar_alimento` antes de registrar, preferindo os itens da TACO.
- Usar as porções salvas do usuário. Quando estimar, marcar `estimado`.
- Depois de registrar, mostrar o resumo do dia comparado com a meta.
- Não calcular médias, porcentagens nem TDEE: usar os números de `resumo_dia`, `analise_periodo` e `peso`.
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
- [x] **4. Planejar e analisar**: análise determinística no `core` (fibra, distribuição de energia, médias e aderência por período, tendência de peso, TDEE adaptativo), peso, aba Resumo, prompts de revisão semanal e sugestão

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
- "Dia registrado" é ter um lançamento. Um dia anotado pela metade puxa a média e o TDEE para baixo. Avaliar marcar o dia como completo, ou ignorar dias muito abaixo da média.
- Plano da semana e lista de compras (seção 2) ainda não existem. Dá para montar em cima de `simular_refeicao` e das receitas.
- Apagar uma pesagem pela web ou pelo Nutri (o `core` já tem `RemoverPeso`; hoje só dá para substituir a do dia).
- Microanimações, só com CSS e o que o htmx já tem: troca de abas e de páginas com View Transitions (`@view-transition` entre páginas e `transition:true` nas trocas do htmx), entrada de lançamentos e cartões do chat com `@starting-style` e resposta ao toque (`:active` com leve escala). Tudo desligado com `prefers-reduced-motion`.

## 11. Ideias

Evoluções pensadas para facilitar o self-hosting e permitir rodar o Nutri sem depender exclusivamente do Claude Code local, preservando a arquitetura atual (`web` e `mcp` → `core` → `store`).

### Provedor de API OpenAI-compatible (`internal/agent`)
- **Interface `agent.Provedor`:** extrair `Disponivel()` e `Conversar(...)` para uma interface, mantendo o backend `claude -p` intacto como padrão quando nenhuma API externa estiver configurada.
- **SDK oficial (`github.com/openai/openai-go/v3`):** preferido em vez de um cliente HTTP/SSE artesanal para evitar bugs em chunks fragmentados de `tool_calls` (`ChatCompletionAccumulator`), mantendo apenas uma dependência nova no `go.mod`.
- **Reaproveitamento do MCP em memória:** o backend de API conecta-se diretamente ao `mcp.Novo(...)` no mesmo processo via `InMemoryTransport` do `modelcontextprotocol/go-sdk`. `ListTools` alimenta as ferramentas da API, `CallTool` executa no `core` emitindo os mesmos eventos (`Ferramenta` e `Resultado`, preservando os cartões com "Desfazer" sem alterar `internal/web/chat.go`), e `GetPrompt` expande os atalhos (`revisao_semanal`, `sugerir_refeicao`).
- **Configuração por ambiente:** `RANGO_LLM_URL`, `RANGO_LLM_MODEL` e `RANGO_LLM_KEY` (opcional). Quando `RANGO_LLM_MODEL` está definido, o servidor usa o backend de API — compatível com Ollama local (gratuito), Gemini, Groq, OpenRouter, LM Studio e OpenAI.
- **Sessão e limites:** histórico da conversa mantido em memória por sessão (alinhado ao comportamento atual do chat) e limite de iterações de tool calls por turno.
- **Atalhos sem o `claude -p`:** quem expande `/mcp__nutri__<prompt> <args>` hoje é o `claude -p`. No backend de API, o `internal/agent` detecta o prefixo e chama `GetPrompt`, e `internal/web/chat.go` não muda.
- **Segurança:** as flags do `claude -p` (seção 8) não se aplicam aqui, mas não fazem falta. O modelo só enxerga as ferramentas do `mcp.Novo(...)` em memória, sem Bash, arquivos ou web. A `--read-only` continua valendo.
- **Modelos locais:** o `nutri.txt` foi afinado para o Claude. Modelos pequenos erram chamadas de ferramenta ou fazem conta por conta própria, então o README deve indicar um modelo mínimo testado.
- **Testes sem chave real:** `httptest.Server` simulando streaming SSE e chamadas de ferramenta contra o MCP em memória e SQLite temporário.

### Docker e self-hosting
- **Rede configurável via ambiente:** manter `ValidarEndereco` restrito a `localhost` e Tailscale por padrão, adicionando opt-in via `RANGO_PERMITIR_EXTERNO=1` para liberar bind em `0.0.0.0` dentro de containers sem exigir flags extras no CLI.
- **Imagem enxuta (`Dockerfile` + `.dockerignore`):** build multi-stage (`CGO_ENABLED=0`) gerando imagem final em `gcr.io/distroless/static-debian12:nonroot` (~20 MB, com CA certs para chamadas HTTPS às APIs de LLM e sem Node/Tailwind no build porque `app.css` já é versionado).
- **Só com o backend de API:** a imagem distroless não tem o `claude` nem Node, então no container o Nutri depende de `RANGO_LLM_MODEL`. Sem ele, a web sobe sem chat.
- **`docker-compose.yml`:** exemplo pronto com volume persistente em `/data` (`RANGO_DB=/data/rango.db`) e variáveis comentadas para Ollama no host (`http://host.docker.internal:11434/v1`) ou API externa. A porta é publicada só em `127.0.0.1:8080:8080`, ou por um sidecar da Tailscale, para que o `RANGO_PERMITIR_EXTERNO` valha só dentro do container e o app continue fora da rede do host (seção 8).

### Licença
- Código licenciado sob **MIT** (`LICENSE`), compatível com todas as dependências e assets embutidos; dados da TACO, fontes e bibliotecas mantêm suas atribuições no `README.md`.
