---
adr: 0006
titulo: govulncheck não bloqueia PR
status: aceita
data: 2026-09-05
codigo: .github/workflows/vulnerabilidades.yml
verificar: grep -L govulncheck .github/workflows/gate-qualidade.yml
---

# ADR-0006 — `govulncheck` não bloqueia PR

## Decisão

O `govulncheck` roda num workflow próprio, diário, contra a `main`, e **abre
issue** quando acha vulnerabilidade que o código alcança. Ele não participa do
gate de PR.

## Contexto

O óbvio é pôr o scanner de segurança junto dos outros gates. Mas o
`govulncheck` é diferente dos outros em espécie, não em grau: **ele falha quando
o mundo muda, não quando o código muda.**

Um CVE publicado hoje numa dependência transitiva deixaria vermelho o PR de
alguém que não encostou em dependência nenhuma.

## Alternativas descartadas

- **`govulncheck` no gate de PR, bloqueante.** Reprovaria contribuição de
  terceiro por causa de um CVE publicado enquanto o PR estava aberto. O efeito
  previsível é o time aprender que vermelho às vezes não é culpa de ninguém — e
  a partir daí, vermelho deixa de significar alguma coisa. Um gate que ensina a
  ser ignorado é pior que gate nenhum.
- **CVE entrando no baseline da catraca.** Tecnicamente caberia, e é o que faz
  errado: baseline é para dívida que se aceita e se reduz com o tempo.
  Vulnerabilidade alcançável exige decisão — atualizar ou mitigar —, e aceitar
  em silêncio é exatamente o que não pode acontecer.

## Consequências

Vulnerabilidade nova chega como issue, com o rastro da chamada, e não como PR
vermelho de um terceiro. O custo é que ela pode ficar aberta alguns dias antes
de alguém agir: é assumido, e é por isso que a varredura é diária e não semanal.

A separação também torna o `gate-qualidade` puramente determinístico — ele mede
o código, e só o código. Dois gates com naturezas diferentes no mesmo job
teriam de compartilhar a mesma política de falha, e não há política que sirva
para os dois.

Vale registrar o que motivou tudo isto: na **primeira medição**, antes de o CI
existir, o `govulncheck` achou duas vulnerabilidades que o código realmente
alcançava — laço infinito em `x/text` chegando por `fmt.Fprintln`, e XSS no
`goldmark` via `glamour`. A primeira importa porque o `deck` renderiza texto de
terceiros, que é a superfície descrita no [`security.md`](../security.md). Ele
distingue o que o código **chama** do que só está na árvore: das 13
vulnerabilidades presentes, 2 eram alcançáveis.

## Onde vive

`.github/workflows/vulnerabilidades.yml` — `schedule` diário e
`workflow_dispatch`, com deduplicação de issue aberta.
