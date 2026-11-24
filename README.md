# Rango

Diário alimentar local, leve e em português, com um agente (**Nutri**) que registra, corrige, planeja e analisa o que você come.

O Rango é um binário Go único que roda no seu PC. Você acessa pelo celular através da Tailscale. O Nutri é o Claude Code em modo headless, conversando com o Rango via MCP.

## Status

Em desenvolvimento. A fase 0 (design) está concluída, e ainda não há nada para instalar. O plano e as decisões estão em [`PLANO.md`](PLANO.md).

As telas de referência, exportadas do Google Stitch, ficam em `design/stitch/`. Abra qualquer `code.html` no navegador para ver.

## Dados

As informações nutricionais vêm da **Tabela Brasileira de Composição de Alimentos (TACO)**, do NEPA/Unicamp.
