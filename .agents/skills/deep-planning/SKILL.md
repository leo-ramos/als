---
name: deep-planning
description: "Use for deep software feature planning. Always execute brainstorming followed by multi-agent-brainstorming, then consolidate the reviewed design into a final implementation plan."
risk: critical
source: community
date_added: "2026-09-16"
---

# Deep Planning

Esta skill é uma orquestração obrigatória de planejamento profundo para funcionalidades de software.

O objetivo é produzir um plano com alta cobertura de requisitos, casos de uso,
edge cases, failure modes e riscos, reduzindo ao máximo problemas descobertos
somente durante a implementação.

## REGRA FUNDAMENTAL

As fases abaixo são **OBRIGATÓRIAS e SEQUENCIAIS**.

A execução desta skill **SEMPRE deve passar pelas duas skills**:

1. `brainstorming`
2. `multi-agent-brainstorming`

**NUNCA pule `multi-agent-brainstorming`, mesmo que a solução pareça simples,
óbvia, de baixo risco ou suficientemente analisada pela fase de brainstorming.**

O agente **NÃO deve decidir** que apenas `brainstorming` é suficiente.

`multi-agent-brainstorming` não é uma etapa opcional de acordo com a complexidade
da tarefa. Ele é uma etapa obrigatória de validação adversarial de toda solução.

Não finalize o planejamento antes da execução das duas skills.

---

## Fase 1 — Brainstorming

**EXECUTE OBRIGATORIAMENTE a skill `brainstorming`.**

Use esta fase para compreender completamente:

* problema e objetivo da funcionalidade;
* contexto existente no projeto;
* requisitos explícitos e implícitos;
* casos de uso;
* fluxos principais e alternativos;
* restrições;
* dependências;
* impacto sobre funcionalidades existentes;
* decisões que precisam ser tomadas antes da implementação.

Faça perguntas e interrompa o processo sempre que informações relevantes
estiverem faltando ou houver ambiguidades importantes.

Não avance enquanto o entendimento não estiver suficientemente consolidado.

Ao final desta fase, produza uma solução inicial suficientemente definida
para ser submetida à revisão adversarial.

Essa solução será chamada de **DESIGN V0**.

---

## Fase 2 — Initial Design

Com base no resultado da fase de brainstorming, consolide o **DESIGN V0**.

Defina:

* arquitetura;
* componentes envolvidos;
* fluxo de dados;
* estados e transições;
* persistência;
* integrações;
* comportamento esperado;
* tratamento de erros;
* estratégia de implementação;
* impacto no sistema existente.

O DESIGN V0 é uma hipótese de solução e **não deve ser tratado como final**.

---

## Fase 3 — Adversarial Review

**EXECUTE OBRIGATORIAMENTE a skill `multi-agent-brainstorming`.**

Esta fase deve ser executada **sempre**, independentemente da complexidade
ou do nível de confiança na solução inicial.

A entrada desta fase é o **DESIGN V0 produzido nas fases anteriores**.

O objetivo é tentar encontrar problemas que não foram identificados durante
o brainstorming inicial.

A revisão deve procurar deliberadamente:

* casos de uso não considerados;
* edge cases;
* failure modes;
* inconsistências;
* estados inválidos;
* requisitos implícitos ausentes;
* problemas de segurança;
* concorrência;
* performance;
* UX;
* integridade e consistência dos dados;
* offline e instabilidade de rede;
* compatibilidade;
* migrações;
* impacto em funcionalidades existentes;
* problemas de manutenção;
* riscos de implementação;
* custos operacionais;
* comportamentos inesperados;
* dependências ocultas.

A solução deve ser tratada como algo que precisa ser atacado,
e não como uma proposta já validada.

**Nunca considere a tarefa concluída antes desta fase.**

Mesmo que a revisão conclua que o DESIGN V0 está correto,
a execução da `multi-agent-brainstorming` ainda é obrigatória.

---

## Fase 4 — Design Revision

Após concluir `multi-agent-brainstorming`, incorpore ao DESIGN V0
todas as descobertas relevantes.

Produza o **DESIGN V1 consolidado**.

Cada problema relevante identificado deve:

1. ser resolvido no design; ou
2. ser explicitamente registrado como uma decisão consciente de não resolvê-lo.

Não descarte uma descoberta sem registrar a decisão correspondente.

---

## Fase 5 — Completeness Check

Antes de finalizar, faça uma verificação completa do DESIGN V1.

Confirme que o planejamento cobre, quando aplicável:

* requisitos funcionais;
* requisitos não funcionais;
* casos de uso;
* fluxos alternativos;
* edge cases;
* failure modes;
* estados e transições;
* dados e persistência;
* permissões e segurança;
* integrações;
* impacto no sistema existente;
* testes;
* observabilidade;
* riscos;
* migração e compatibilidade;
* custos operacionais.

Esta verificação é adicional às duas skills obrigatórias.

Não finalize apenas porque existe uma implementação funcional possível.

O objetivo é maximizar a cobertura de cenários antes da implementação.

---

## Fase 6 — Final Implementation Plan

Somente depois de concluir:

1. `brainstorming`;
2. DESIGN V0;
3. `multi-agent-brainstorming`;
4. DESIGN V1;
5. completeness check;

produza o documento final de planejamento.

O documento final deve conter:

* entendimento do problema;
* escopo;
* não-escopo;
* requisitos;
* casos de uso;
* fluxos;
* edge cases;
* failure modes;
* arquitetura;
* estados e transições;
* decisões;
* alterações necessárias;
* impacto no sistema existente;
* riscos e mitigação;
* estratégia de testes;
* plano de implementação faseado;
* critérios de aceite;
* decisões pendentes do usuário.

O documento final deve representar o **DESIGN V1 após a revisão adversarial**,
e nunca apenas o primeiro design produzido durante o brainstorming.

---

## Critério de conclusão

A skill `deep-planning` só pode ser considerada concluída quando:

* `brainstorming` tiver sido executado;
* um DESIGN V0 tiver sido produzido;
* `multi-agent-brainstorming` tiver sido executado sobre o DESIGN V0;
* as descobertas da revisão tiverem sido incorporadas ou explicitamente rejeitadas;
* o DESIGN V1 tiver sido produzido;
* a verificação de completude tiver sido realizada;
* o plano final de implementação tiver sido documentado.

**É proibido encerrar a execução após somente `brainstorming`.**
