# Rango

Diário alimentar local, leve e em português, com um agente (**Nutri**) que registra, corrige, planeja e analisa o que você come.

O Rango é um binário Go único que roda no seu PC. Você acessa pelo celular através da Tailscale. O Nutri é o Claude Code em modo headless, conversando com o Rango via MCP.

O que já funciona (fases 1 a 4): diário do dia com metas, busca nos alimentos da TACO, porções caseiras, remoção com desfazer, tema claro e escuro, PWA e senha, o chat com o Nutri, que registra e corrige por texto, receitas e marmitas, e a análise: médias, aderência à meta, peso e gasto diário estimado, todos calculados pelo app. O plano e as decisões estão em [`PLANO.md`](PLANO.md).

## Requisitos

- Go 1.27 ou mais novo
- Para o Nutri: [Claude Code](https://claude.com/claude-code) instalado e com login feito (`claude` no PATH). Sem ele, o resto do app funciona e o chat avisa que o Nutri está indisponível.
- Para desenvolver: [Tailwind CLI standalone](https://github.com/tailwindlabs/tailwindcss/releases) (`tailwindcss` no PATH) e `golangci-lint`

No Arch: `sudo pacman -S go golangci-lint && paru -S tailwindcss-bin`.

## Configuração

Crie um arquivo `.env` na raiz do projeto. Ele não vai para o git.

```sh
RANGO_SENHA=troque-por-uma-senha-forte
RANGO_ADDR=127.0.0.1:8080
```

Escreva um valor por linha, sem aspas e sem comentário no fim da linha: o `make` leria os espaços como parte da senha.

| Variável | Padrão | O que é |
|---|---|---|
| `RANGO_SENHA` | (nenhum) | Senha pedida pela web. Sem ela, o servidor não sobe. Trocar a senha desconecta todas as sessões. |
| `RANGO_ADDR` | `127.0.0.1:8080` | Endereço de escuta. Só aceita localhost ou um IP da Tailscale. |
| `RANGO_DB` | `~/.local/share/rango/rango.db` | Arquivo SQLite. É criado na primeira execução, já com a TACO. |
| `RANGO_CLAUDE` | `claude` | Binário do Claude Code usado pelo Nutri. Defina o caminho completo se o `claude` não estiver no PATH do serviço. |

O `make` lê o `.env` sozinho. Se rodar o binário direto, exporte as variáveis antes.

## Uso

```sh
make run      # gera o CSS e sobe em RANGO_ADDR
make build    # gera bin/rango
```

Abra `http://127.0.0.1:8080`, entre com a senha, defina as metas no ícone do robô e comece a registrar, pela tela do diário ou pela aba Chat ("almocei um prato de arroz e uma concha de feijão"). Cada registro do Nutri aparece num cartão com botão de desfazer. O histórico do chat fica na memória e some quando o servidor reinicia.

Na aba Receitas, cadastre a panela (ingredientes crus, peso pronto se pesou e em quantas porções rendeu) e registre cada marmita com um toque. Pelo chat também funciona: "fiz 4 marmitas com 500 g de frango e 600 g de batata" e, depois, "comi uma marmita". Produtos fora da TACO podem ser cadastrados pelo Nutri com os valores do rótulo.

Na aba Resumo, escolha 7, 14 ou 30 dias para ver dias anotados, médias, aderência à meta, de onde vêm as calorias, fibra, refeições e os alimentos que mais pesaram. Registre o peso de manhã no mesmo lugar: com 14 dias de pesagem e a maior parte dos dias anotada, aparece o gasto diário estimado (TDEE). O botão "Revisão da semana com o Nutri" e os atalhos do chat pedem ao Nutri uma revisão ou sugestões de refeição, sempre com os números calculados pelo app.

### MCP em outros clientes

`rango mcp` é um servidor MCP no stdio com as mesmas ferramentas e prompts do Nutri (`revisao_semanal`, `sugerir_refeicao`). Com `--read-only`, expõe só as de consulta. Para usar no Claude Code ou no Claude Desktop:

```json
{"mcpServers": {"rango": {"command": "/caminho/para/bin/rango", "args": ["mcp"]}}}
```

### Pelo celular (Tailscale)

1. Com a Tailscale ativa no PC e no celular, descubra o IP do PC: `tailscale ip -4` (algo como `100.x.y.z`).
2. No `.env`, defina `RANGO_ADDR=100.x.y.z:8080` e rode `make run`.
3. No celular, abra `http://100.x.y.z:8080`. Para instalar como app, use "Adicionar à tela inicial".

## Desenvolvimento

```sh
make test lint   # testes e lint, antes de cada commit
make css         # regera internal/web/static/app.css (versionado)
```

As telas de referência, exportadas do Google Stitch, ficam em `design/stitch/`. Abra qualquer `code.html` no navegador para ver.

## Dados

As informações nutricionais vêm da **Tabela Brasileira de Composição de Alimentos (TACO), 4ª edição, do NEPA/Unicamp**. A conversão para CSV usada como base é a do projeto [raulfdm/taco-api](https://github.com/raulfdm/taco-api).

Ícones: [Material Symbols](https://fonts.google.com/icons) (Apache 2.0). Fonte: [Plus Jakarta Sans](https://fonts.google.com/specimen/Plus+Jakarta+Sans) (OFL). Interatividade: [htmx](https://htmx.org) (0BSD).

## Licença

O código do Rango é distribuído sob a licença [MIT](LICENSE). Os dados nutricionais da TACO, fontes e bibliotecas de terceiros mantêm suas respectivas licenças e atribuições acima.
