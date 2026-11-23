# Rango

@../PLANO.md

## Regras de trabalho

- Siga a arquitetura da seção 4 do plano: `web` e `mcp` → `core` → `store`. Toda regra de negócio fica no `core`.
- Não crie arquivos `.md` novos. Decisões vão para a seção correspondente do `PLANO.md`, e instruções de uso vão para o `README.md`.
- Não crie arquivos soltos na raiz. Cada arquivo novo tem lugar definido na estrutura de pastas do plano.
- Testes só nos pontos listados no plano. Não escreva testes para handlers, templates ou código trivial.
- Antes de adicionar uma dependência, justifique. Prefira a stdlib.
- Antes de dar uma tarefa por concluída, rode `make test lint`.
- Commits em Conventional Commits, em português.
- Ao terminar uma tarefa, sugira rodar `/docs` numa linha no fim da resposta quando: uma fase foi concluída, uma decisão mudou o plano (stack, modelo de dados, ferramentas MCP), algo novo ficou usável (subcomando, variável `RANGO_*`, passo de instalação) ou o usuário avisou que vai pausar. Não sugira em correções pequenas, refatorações ou ajustes visuais.
