---
adr: 0003
titulo: O deck não fala com API de modelo
status: aceita
data: 2026-08-28
codigo: internal/herdr/herdr.go:KnownKinds, internal/ui/agents.go:startAgent
verificar: go test ./internal/herdr/ -run TestUnknownKinds
---

# ADR-0003 — O deck não fala com API de modelo

## Decisão

Acesso a OpenRouter, OmniRouter e afins acontece **dentro do `opencode`**, que
entra na cadeia de `agent_kind` como qualquer outro tipo. O deck não tem chave
de API, não escolhe modelo e não faz requisição de inferência.

## Contexto

Havia um item de backlog para integrar OpenRouter, motivado por um problema
real: quando a cota de um provedor acaba, o trabalho para.

## Alternativas descartadas

- **Cliente de OpenRouter dentro do deck.** Resolveria a cota, mas criaria um
  **segundo modelo de execução**: hoje todo agente roda num pane do herdr, com
  estado observável e um caminho de captura. Um agente por API não tem pane, não
  tem `agent read`, não tem `f` para ir até ele — seria uma segunda
  implementação de backend inteira, com sua própria captura e seu próprio ciclo
  de vida.
- **Guardar a chave no `config.md`.** Segredo em arquivo que o usuário versiona
  junto com o board. Descartado por si só.

## Consequências

O problema da cota foi resolvido **sem código**: `agent_kind` virou uma cadeia
(`claude, opencode`), o `startAgent` tenta em ordem e segue no primeiro que
subir. O provedor usado vai para o `## Log` do card.

O deck fica sem chave de API, sem dependência de rede no caminho de execução, e
com um só modelo de execução.

O custo é real e recai no usuário: ele precisa instalar e configurar o
`opencode`, e a escolha de modelo acontece fora do deck — que portanto não pode
mostrá-la nem trocá-la. Foi julgado barato perto de manter dois backends.

`KnownKinds()` lê os tipos do próprio binário do herdr em vez de fixá-los aqui:
cada versão suporta um conjunto diferente, e uma cópia envelheceria em silêncio.

## Onde vive

`internal/ui/agents.go:startAgent` — a cadeia.
`internal/herdr/herdr.go:KnownKinds` — os tipos vêm do binário.
