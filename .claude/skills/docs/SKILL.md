---
name: docs
description: Sincroniza a documentação do Rango (PLANO.md, README.md, .claude/CLAUDE.md) com o estado atual do código. Use no fim de uma fase ou depois de uma mudança relevante.
disable-model-invocation: true
---

# /docs

Atualize a documentação existente para refletir o que mudou. **Nunca crie arquivos novos.**

## Arquivos permitidos

| Arquivo | Conteúdo |
|---|---|
| `PLANO.md` | Visão, arquitetura, decisões, checklist das fases e itens em aberto |
| `README.md` | Como instalar, configurar e usar. Curto e prático |
| `.claude/CLAUDE.md` | Só regras de trabalho. Mexa apenas se uma regra mudou |

## Passos

1. Veja o que mudou: `git log` e `git diff` desde a última atualização da documentação, mais a conversa atual.
2. **PLANO.md**:
   - marque os itens concluídos no checklist das fases;
   - registre as decisões novas na seção correspondente (stack, arquitetura, modelo de dados, ferramentas MCP), com uma linha de motivo;
   - tire de "Em aberto" o que foi resolvido e acrescente as dúvidas novas;
   - corrija o que ficou desatualizado (nomes, tabelas, ferramentas).
3. **README.md**: crie ou ajuste apenas o necessário para alguém instalar e usar (requisitos, build, subcomandos, variáveis `RANGO_*`, acesso pelo celular). Cite a fonte da TACO (NEPA/Unicamp).
4. **.claude/CLAUDE.md**: altere só se uma regra de trabalho mudou. Mantenha em torno de 10 linhas.

## Regras de escrita

- Português, direto e sem enrolação.
- Edite as seções existentes antes de criar uma nova.
- Não duplique conteúdo entre os arquivos: o `README` explica o uso, o `PLANO` explica o porquê.
- Sem changelog e sem histórico de alterações, porque isso já está no git.
- No fim, mostre ao usuário um resumo curto do que mudou em cada arquivo.
