---
adr: 0005
titulo: A catraca só afrouxa depois do merge
status: aceita
data: 2026-09-05
codigo: tools/ratchet/main.go, .github/workflows/gate-qualidade.yml
verificar: go test ./tools/ratchet/
---

# ADR-0005 — A catraca só afrouxa depois do merge

## Decisão

O quality gate compara os achados com um baseline versionado em
`.ci/baseline.txt` e reprova qualquer chave que suba. Em PR o job **nunca
escreve**; o baseline só desce no job de `push` na `main`.

## Contexto

Gate novo em projeto que já existe tem dois destinos comuns: nasce vermelho e
todo mundo aprende a ignorar, ou nasce frouxo e não segura nada. A catraca é a
saída — o estado de hoje vira o teto, e o teto só desce.

Mas o jeito óbvio de fazer isso é o PR atualizar o baseline sozinho quando
melhora, e é justamente o que não se pode fazer.

## Alternativas descartadas

- **PR atualiza o próprio baseline.** Não funciona e não deveria: em PR vindo de
  fork o `GITHUB_TOKEN` é read-only por trava do GitHub, independente do que o
  YAML declare. E se funcionasse seria pior — um fork empurraria um teto mais
  alto junto com o código, e o gate autorizaria a própria regressão.
- **Contagem global por ferramenta.** Simples e furada: deixa trocar um achado
  por outro sem o número mudar.
- **GitHub Code Scanning (SARIF).** Interface nativa melhor, com diff por
  commit. Descartada por amarrar a catraca a um recurso do GitHub em vez de a um
  arquivo que qualquer um lê no diff do PR — e o baseline como arquivo continua
  funcionando se o projeto sair do GitHub.

## Consequências

Quem melhora não precisa lembrar de atualizar arquivo nenhum: o job da `main`
faz sozinho, com `[skip ci]` para não gastar um run recalculando o que acabou de
calcular.

O preço é que a catraca não desce **durante** a revisão, então um PR que corrige
achados mostra o baseline antigo até o merge. É o preço certo por não abrir a
porta acima.

A chave é `ferramenta|regra|arquivo` **com contagem**, e sem número de linha. Sem
número porque refatoração viraria churn; com contagem porque `errcheck` e
`ineffassign` não têm código de regra estável, então onze achados no mesmo
arquivo colapsam numa chave — e sem contar, um décimo segundo entraria sem
ninguém notar. Usar o texto da mensagem como chave resolveria a colisão e
criaria pior: o texto cita o identificador, então renomear uma variável
reprovaria um PR que não piorou nada.

Fixar a versão das ferramentas deixou de ser detalhe e virou requisito: linter
que muda de versão sozinho muda a contagem sozinho.

## Onde vive

`tools/ratchet/main.go` — normalização e comparação, com teste nos dois
sentidos: achado novo reprova, correção real passa como melhora.
`.github/workflows/gate-qualidade.yml` — o `atualizar-baseline` só chega ligado
em `push`.
