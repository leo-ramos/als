# Deep Planning: Gerenciador de Comandos & Shims Parametrizados (`als`)

> [!IMPORTANT]
> **Status do Planejamento:** `APPROVED` (Concluídas as Fases 1 a 6 do fluxo `deep-planning`, incluindo `brainstorming` e revisão adversarial `multi-agent-brainstorming`).
> **Nome provisório do binário:** `als` (configurável no `go.mod` e `GoReleaser`).

---

## 1. Entendimento do Problema, Escopo e Não-Escopo (Fase 1 — Brainstorming)

### 1.1 Entendimento do Problema
Desenvolvedores e engenheiros de infraestrutura repetem diariamente comandos extensos e propensos a erro no terminal (conexões SSH com chaves específicas, túneis, comandos `docker`, `kubectl`, `curl` autenticados). As soluções nativas possuem limitações estruturais:
1. **`alias` nativo do shell (`.zshrc`/`.bashrc`):** Exige edição manual de arquivos, requer recarregar a sessão (`source`) em cada aba aberta, não aceita parâmetros dinâmicos no meio do comando (`{{var}}`), falha silenciosamente com caminhos relativos de arquivos e expõe credenciais se o usuário versionar seus dotfiles no Git.
2. **Ferramentas de snippets existentes (`pet`, `navi`):** Focam em buscar e colar snippets no buffer do terminal, exigindo múltiplos passos em vez de transformar o comando salvo em um **executável de primeira classe** invocável diretamente pelo nome no terminal.

### 1.2 Escopo (v1)
* **Execução Direta via Shims no `$PATH`:** Qualquer comando salvo torna-se imediatamente executável em qualquer terminal aberto apenas digitando seu nome (ex.: `connectServerProd` ou `conectar app-prod`).
* **Parametrização Dinâmica (`{{var}}`) e Seletores (`choices`):** Suporte a variáveis preenchidas por argumento posicional/flag, variáveis locais por comando/globais ou seletor interativo TUI no momento da execução.
* **Isolamento de Segredos e Múltiplos Ambientes:** Separação física entre a definição pública do comando (`commands.toml`, versionável no Git) e os dados sensíveis/IPs reais (`secrets.toml`, estritamente local com escopo `[global]`, `[commands.<nome>]` e `[commands.<nome>.choices.<var>]`).
* **Linter Preventivo no Cadastro:** Detecção automática de caminhos relativos de arquivos (oferecendo conversão para caminho absoluto) e heurística de alerta para IPs/tokens hardcoded.
* **Interface TUI e CLI:** Cadastro interativo seguro (evitando quebra de aspas pelo shell), listagem, busca fuzzy, edição e remoção.
* **Sincronização Git Nativa (`als sync`):** Versionamento declarativo do `commands.toml` via repositório Git do próprio usuário, com trava de segurança contra vazamento do `secrets.toml` e reconstrução automática de shims locais.

### 1.3 Não-Escopo Explícito (Non-Goals para a v1)
* **Suporte nativo a Windows (CMD/PowerShell):** A v1 foca exclusivamente em sistemas Unix/POSIX (**macOS e Linux**).
* **Backend/Cloud Proprietária:** Zero servidores próprios, contas de usuário ou bancos de dados na nuvem (custo operacional \$0; sincronização 100% via Git/SSH/HTTPS do usuário).
* **Alteração de estado interno do processo pai do shell (`cd`, `export`):** Como os shims executam em processo filho via `execve`, comandos cujo único propósito seja alterar o diretório da sessão interativa pai (`cd /pasta`) não fazem parte do escopo do shim binário na v1.
* **Histórico automático de comandos:** A ferramenta gerencia apenas comandos explicitamente salvos pelo usuário (não substitui `atuin` ou `fzf CTRL+R`).

---

## 2. Consolidação do DESIGN V0 (Fase 2 — Hipótese Inicial)

O **DESIGN V0** produzido ao final do brainstorming inicial estabeleceu:
1. **Arquitetura Multicall Binary:** Binário único em Go (`als`) colocado no `$PATH`. Symlinks criados em `~/.local/share/als/bin/<alias>` apontam para o binário `als`.
2. **Detecção por `os.Args[0]`:** Se invocado como `als`, abre a CLI/TUI de gerenciamento; se invocado pelo nome de um symlink, executa o fluxo de Shim.
3. **Armazenamento Dual em `~/.config/als/`:**
   * `commands.toml` (sincronizado via Git).
   * `secrets.toml` (ignorado pelo Git, permissão `0600`), suportando `[global]`, `[commands.<alias>]` e `[commands.<alias>.choices.<var>]`.
4. **Execução:** Interpolação das variáveis `{{var}}` na string do comando com escape POSIX e chamada de `syscall.Exec("/bin/sh", ["sh", "-c", cmdFinal], os.Environ())`.

---

## 3. Revisão Adversarial Multi-Agente (Fase 3 — `multi-agent-brainstorming`)

O **DESIGN V0** foi submetido sequencialmente aos três agentes revisores restritos e arbitrado pelo Integrator/Arbiter.

### 3.1 Parecer do Agente 1: Skeptic / Challenger ("Por que este design vai falhar em produção?")

* **[Obj-S1] Bug de Dupla Citação (Double-Quoting) na Interpolação de Strings:**
  * *Ataque:* No DESIGN V0, o comando salvo é `ssh -i "{{key_path}}" ubuntu@{{server_ip}}` (com aspas duplas ao redor de `{{key_path}}`) e a Seção 3 afirma que o motor aplicará `shellquote` no valor interpolado. Se o caminho tiver espaços (`/Users/dev/My Keys/id.key`), o `shellquote` gerará `'/Users/dev/My Keys/id.key'`. Inserido dentro das aspas duplas já existentes no template, o `/bin/sh` receberá `ssh -i "'/Users/dev/My Keys/id.key'"`, passando aspas simples literais para o `ssh` e quebrando a execução com `No such file or directory`.
* **[Obj-S2] Colisão de Flags entre o Shim e o Comando Alvo:**
  * *Ataque:* Se o usuário rodar `connectServerProd -v -p 2222` querendo passar `-v -p 2222` para o `ssh`, o parser de flags do Go no Modo Shim tentará interpretar `-v` ou `-p` como flags do próprio `als` (ou engolirá `--help`), quebrando o repasse de argumentos para o comando real.
* **[Obj-S3] Quebra em Massa de Symlinks após Atualização do Binário (`brew upgrade`):**
  * *Ataque:* Se os symlinks em `~/.local/share/als/bin/<alias>` apontarem diretamente para `os.Executable()` no macOS com Homebrew (`/opt/homebrew/Cellar/als/0.1.0/bin/als`), assim que o usuário atualizar para a versão `0.2.0`, o diretório `0.1.0` deixará de existir e **todos os comandos salvos pelo usuário quebrarão simultaneamente** com `bad interpreter / no such file or directory`.
* **[Obj-S4] Condição de Corrida (Lost Update) em Escritas Simultâneas:**
  * *Ataque:* Escrita atômica via `temp + rename` evita arquivo truncado, mas se dois terminais salvarem variáveis em `secrets.toml` ao mesmo tempo, o segundo sobrescreverá os dados lidos pelo primeiro sem bloqueio de arquivo (`flock`).

### 3.2 Parecer do Agente 2: Constraint Guardian (Segurança, Performance, Confiabilidade)

* **[Obj-C1] Crítico — Vazamento de Segredos na Tabela de Processos (`ps aux` / `/proc/<pid>/cmdline`):**
  * *Ataque:* No DESIGN V0, o motor substitui `{{var}}` diretamente na string passada para `syscall.Exec("/bin/sh", ["sh", "-c", cmdFinal])`. Em qualquer sistema Unix, os argumentos de linha de comando de `/bin/sh -c ...` ficam **visíveis em texto puro para todos os processos/usuários da máquina via `ps aux`** durante toda a execução do comando. Se o comando contiver um token ou senha vindo do `secrets.toml`, o segredo é exposto no SO.
* **[Obj-C2] Falha de Segurança no `.gitignore` para Arquivos Já Rastreados:**
  * *Ataque:* O DESIGN V0 verifica apenas se `secrets.toml` está listado no `.gitignore` antes do `als sync`. No Git, se o usuário tiver dado `git add -f secrets.toml` manualmente antes, o `.gitignore` é ignorado pelo Git e o `als sync` fará commit e push de todos os segredos para o repositório remoto.
* **[Obj-C3] Travamento Indefinido de Rede no `als sync`:**
  * *Ataque:* Subprocessos `git pull` e `git push` sem `context.WithTimeout` travarão o terminal indefinidamente em redes instáveis ou conexões SSH penduradas.

### 3.3 Parecer do Agente 3: User Advocate (Usabilidade, Carga Cognitiva, UX de Erros)

* **[Obj-U1] "Inferno de Aspas" e Expansão Prematura no Cadastro Inline (`als add`):**
  * *Ataque:* Pedir ao usuário para digitar `als add nome 'ssh -i "key" user@ip'` na linha de comando é frágil. Se ele usar aspas duplas externas, tiver `$VAR`, pipes (`|`) ou `&&`, o próprio `zsh`/`bash` vai expandir a variável ou executar o pipe **antes** do binário `als` receber os argumentos.
* **[Obj-U2] Caixa-Preta Perigosa (Falta de Inspeção / Dry-Run e Confirmação):**
  * *Ataque:* Meses depois de criar um alias como `limparBanco` ou `deployProd`, o usuário não lembra exatamente o que o shim executa. Se ele digitar `deployProd --help` achando que verá ajuda, o comando pode disparar em produção.
* **[Obj-U3] Verbose Desnecessária para Selecionar `choices`:**
  * *Ataque:* Se o comando `conectar` tem apenas 1 variável de escolha (`{{servidor}}`), obrigar o usuário a digitar `conectar --servidor=app-prod` é ruim; ele espera digitar apenas `conectar app-prod` ou `conectar`.

### 3.4 Decisão do Agente 4: Integrator / Arbiter

* **Disposição Final da Revisão Multi-Agente:** **`APPROVED` (com incorporação obrigatória das 10 correções no DESIGN V1)**.
* **Racional do Árbitro:** Nenhuma objeção exige criar novas features fora do escopo, mas **Obj-S1 + Obj-C1** revelam uma falha arquitetural na interpolação por string que é resolvida de forma muito mais limpa e segura injetando variáveis via **Environment Variables nativas do `execve`** (`$ALS_VAR_<nome>`). Todas as 10 objeções foram aceitas e convertidas em especificações do **DESIGN V1**.

---

## 4. DESIGN V1 Consolidado (Fase 4 — Design Revision)

### 4.1 Arquitetura Revisada e Fluxo de Execução Seguro

```mermaid
flowchart TD
    A["Usuário digita no terminal:<br/><code>conectar app-prod -v</code>"] --> B["SO resolve symlink em<br/><code>~/.local/share/als/bin/conectar</code>"]
    B --> C["Symlink aponta para<br/><code>~/.local/share/als/bin/.als-bin</code> -> binário <code>als</code>"]
    C --> D{"<code>filepath.Base(os.Args[0])</code> == 'als'?"}
    D -- "Sim" --> E["CLI / TUI Manager<br/>(add, list, edit, rm, sync, doctor)"]
    D -- "Não (Modo Shim)" --> F["Intercepta flags reservadas<br/>(<code>--als-info</code> / <code>--als-dry-run</code>)"]
    F --> G["Carrega <code>commands.toml</code> + <code>secrets.toml</code>"]
    G --> H["Resolve variáveis <code>{{var}}</code>:<br/>1. Arg posicional de Choice / <code>@var=val</code><br/>2. <code>secrets.toml</code> (comando > global)<br/>3. Prompt TUI (se TTY ativo)"]
    H --> I{"Requer confirmação<br/>(<code>confirm = true</code>)?"}
    I -- "Sim" --> J["Exibe comando mascarado e pede [y/N]"]
    I -- "Não / Confirmado" --> K["Injeta valores como <code>ALS_VAR_x=val</code> no env<br/>e substitui <code>{{x}}</code> por <code>\"$ALS_VAR_x\"</code>"]
    K --> L["<code>syscall.Exec('/bin/sh', ['sh', '-c', cmd + ' \"$@\"', aliasName, argsExtras...], env)</code>"]
```

### 4.2 Soluções Técnicas Incorporadas no DESIGN V1

1. **Injeção de Variáveis via Ambiente do `execve` (Resolve `Obj-S1` e `Obj-C1` de uma só vez):**
   * Em vez de concatenar o valor bruto de `{{key_path}}` dentro da string do comando (o que causava bugs de aspas e vazava segredos no `ps aux`), o motor de template transforma `{{key_path}}` (ou `"{{key_path}}"`) na expressão de shell `"$ALS_VAR_key_path"`.
   * O valor real (`/Users/dev/My Keys/ssh.key` ou um token secreto) é injetado apenas no array `envv` passado para `syscall.Exec`.
   * **Resultado:** Zero vulnerabilidade de Command Injection, suporte nativo a caminhos com espaços/caracteres especiais e **zero exposição de segredos no `ps aux`**.

2. **Repasse Intacto de Argumentos Extras via `"$@"` Posicional do `/bin/sh` (Resolve `Obj-S2` e `Obj-U3`):**
   * No Modo Shim, o `als` **não** usa o parser de flags do `Cobra`. Ele analisa os argumentos na seguinte regra determinística:
     * Se o 1º argumento for `--als-info` ou `--als-dry-run`: exibe o template, descrição e variáveis do comando e encerra sem executar (`Obj-U2`).
     * Argumentos no formato explícito `@var=valor` (ou o 1º argumento posicional caso coincida exatamente com uma opção de `choices` da primeira variável do comando, ex.: `conectar app-prod`): são consumidos pelo `als` para preencher a variável.
     * **Todos os demais argumentos** (`-v`, `-p 2222`, `--help`, caminhos com espaços) são passados intactos como parâmetros posicionais do `/bin/sh -c`:
       `syscall.Exec("/bin/sh", []string{"sh", "-c", renderedCmd + ` "$@"`, aliasName, remainingArgs...}, env)`
     *Nota técnica:* Se o `renderedCmd` não referenciar `$1`, `$2`, `$@`, o `als` anexa ` "$@"` automaticamente ao final para repassar flags extras ao comando subjacente.

3. **Symlink Indireto Único `.als-bin` (Resolve `Obj-S3`):**
   * Todos os aliases em `~/.local/share/als/bin/<alias>` são symlinks **relativos** apontando para `./.als-bin` no mesmo diretório.
   * Apenas `.als-bin` aponta para o caminho do binário principal (descoberto via `exec.LookPath("als")` preferencialmente sobre `os.Executable()`).
   * Caso o binário mude de caminho em um upgrade de gerenciador de pacotes, basta atualizar `.als-bin` (ou qualquer invocação de `als` corrige `.als-bin` automaticamente).

4. **File Locking (`flock`) + Escrita Atômica (Resolve `Obj-S4`):**
   * Toda escrita em `commands.toml` e `secrets.toml` adquire `syscall.Flock` exclusivo em `~/.config/als/.lock`, escreve em arquivo temporário no mesmo filesystem com `fsync` e executa `os.Rename`.

5. **UX de Cadastro Seguro e Inspeção (Resolve `Obj-U1` e `Obj-U2`):**
   * **Modo Interativo Padrão (`als add` ou `als add <nome>`):** Abre formulário TUI (`Huh`) onde o usuário cola o comando bruto sem interferência de aspas, pipes ou expansão de `$VAR` pelo shell atual.
   * **Modo Histórico (`als add --last`):** Lê o último comando do `~/.zsh_history` ou `~/.bash_history` e abre o formulário pré-preenchido.
   * **Flag `confirm = true` opcional:** Comandos críticos podem exigir confirmação `[y/N]` antes do `execve`.

6. **Blindagem do Git Sync (Resolve `Obj-C2` e `Obj-C3`):**
   * Antes de qualquer commit/push, `als sync` verifica:
     1. `secrets.toml` está no `.gitignore`?
     2. `git ls-files --error-unmatch secrets.toml` retorna erro (garantindo que `secrets.toml` **não** está rastreado no índice do Git)?
   * Se `secrets.toml` estiver rastreado no Git, o `als sync` aborta imediatamente, executa `git rm --cached secrets.toml` sugerido e bloqueia o push.
   * Todos os comandos Git rodam sob `context.WithTimeout(ctx, 15*time.Second)`.

---

## 5. Registro Completo de Decisões (Decision Log)

| ID | Decisão | Alternativas Consideradas | Objeções Associadas | Resolução e Racional Final |
| :--- | :--- | :--- | :--- | :--- |
| **D1** | Sincronização via repositório Git sobre arquivo declarativo `commands.toml` | Backend próprio na nuvem; apenas uso local sem sync | Escopo/YAGNI | Custo de infraestrutura \$0, diffs legíveis e controle total dos dados pelo usuário. |
| **D2** | Linguagem **Go** + foco POSIX (macOS/Linux) na v1 | Rust, Node.js, Python; suporte Windows na v1 | Latência de runtime | Binário estático único, startup `< 5ms`, ecossistema TUI `Bubble Tea`/`Huh` e distribuição via `GoReleaser`. |
| **D3** | Dual-file (`commands.toml` + `secrets.toml`) com escopos `[global]`, `[commands.<id>]` e `choices` | `secrets.env` plano; criptografia E2EE de arquivo único | Dúvida do usuário sobre múltiplos IPs | Permite múltiplos servidores em aliases distintos (sem colisão de nome de variável) e múltiplos IPs selecionáveis num único alias. |
| **D4** | Multicall Binary via symlinks relativos para `.als-bin` no `$PATH` | Funções de shell via `eval`/`precmd`; edição direta do `.zshrc` | `Obj-S3` | Funciona instantaneamente em todas as abas abertas e scripts; `.als-bin` evita quebra de symlinks em atualizações do Homebrew. |
| **D5** | Passagem de variáveis via Environment (`$ALS_VAR_x`) + argumentos extras via `"$@"` posicional do `/bin/sh` | Interpolação direta de string com `shellquote` + parser de flags no shim | `Obj-S1`, `Obj-S2`, `Obj-C1`, `Obj-U3` | Elimina vulnerabilidade de Command Injection, resolve conflito de aspas duplas/simples, esconde segredos do `ps aux` e preserva flags nativas do comando alvo (`-v`, `-p`). |
| **D6** | Cadastro prioritário via Wizard TUI (`als add`) e `als add --last` | Apenas CLI inline `als add <nome> "<cmd>"` | `Obj-U1` | Evita que o shell do usuário interprete pipes (`\|`), redirecionamentos ou remova aspas internas durante o cadastro. |

---

## 6. Especificação Técnica Detalhada (Schemas, Estados e Fluxos)

### 6.1 Estrutura de Arquivos e Permissões
```text
~/.config/als/                  (drwx------ 0700)
├── .git/                       (Repositório Git opcional configurado via `als sync init`)
├── .gitignore                  (-rw-r--r-- 0644 -> ignora secrets.toml, .lock, *.tmp)
├── .lock                       (-rw------- 0600 -> usado para syscall.Flock)
├── commands.toml               (-rw-r--r-- 0644 -> versionado no Git)
└── secrets.toml                (-rw------- 0600 -> NUNCA versionado no Git)

~/.local/share/als/bin/         (drwxr-xr-x 0755 -> adicionado ao $PATH)
├── .als-bin                    (symlink -> /opt/homebrew/bin/als ou caminho do binário)
├── conectar                    (symlink relativo -> .als-bin)
└── connectServerProd           (symlink relativo -> .als-bin)
```

### 6.2 Exemplo Completo de `commands.toml` e `secrets.toml`

**`~/.config/als/commands.toml` (Seguro para subir no Git):**
```toml
version = 1

[commands.connectServerProd]
cmd = "ssh -i {{key_path}} ubuntu@{{server_ip}}"
desc = "Conecta no servidor principal de produção"
tags = ["ssh", "app", "prod"]
confirm = false

[commands.connectServerProd.vars.key_path]
secret = true
prompt = "Caminho absoluto da chave SSH"

[commands.connectServerProd.vars.server_ip]
secret = true
prompt = "IP do servidor de produção"

[commands.conectar]
cmd = "ssh -i {{default_key}} ubuntu@{{servidor}}"
desc = "Seletor multi-servidor SSH"
tags = ["ssh", "infra"]

[commands.conectar.vars.default_key]
secret = true
scope = "global"

[commands.conectar.vars.servidor]
secret = true
choices = ["app-prod", "app-staging", "ecommerce-prod"]
```

**`~/.config/als/secrets.toml` (Local na máquina, Permissão `0600`):**
```toml
[global]
default_key = "/Users/dev/.ssh/prod-key.pem"

[commands.connectServerProd]
key_path = "/Users/dev/.ssh/prod-key.pem"
server_ip = "203.0.113.10"

[commands.conectar.choices.servidor]
app-prod = "203.0.113.10"
app-staging = "203.0.113.11"
ecommerce-prod = "45.55.120.10"
```

### 6.3 Máquina de Estados de uma Variável (`{{var}}`) na Execução
1. **`UNRESOLVED`**: Variável identificada no parser de template `{{var}}`.
2. **`RESOLVED_BY_CLI`**: Usuário passou `@var=valor` ou o 1º argumento posicional bateu com uma chave de `choices` (`conectar app-prod`). Se a chave de `choices` apontar para um valor secreto em `secrets.toml`, resolve o valor correspondente (`203.0.113.10`).
3. **`RESOLVED_BY_STORE`**: Valor encontrado em `secrets.toml` (`[commands.<alias>]` tem precedência sobre `[global]`) ou no `default` do `commands.toml`.
4. **`PROMPTING_USER`**: Se continuar `UNRESOLVED` (ou se for uma `choice` sem argumento informado):
   * Verifica `term.IsTerminal(int(os.Stdin.Fd()))`.
   * Se **não** for terminal interativo -> Transita para **`FAILED_NON_INTERACTIVE`** (Exit Code `2`).
   * Se for terminal interativo -> Exibe `Huh` Select (para `choices`) ou Input (para texto/segredo) e pergunta se deseja persistir em `secrets.toml` para execuções futuras.
5. **`BOUND_TO_ENV`**: Valor associado a `ALS_VAR_<var>` no ambiente do processo e substituído por `"$ALS_VAR_<var>"` na string do `/bin/sh`.

---

## 7. Verificação de Completude (Fase 5 — Completeness Check)

| Dimensão | Status | Cobertura no DESIGN V1 |
| :--- | :---: | :--- |
| **Requisitos Funcionais** | ✅ | CRUD de aliases, shims instantâneos no `$PATH`, variáveis `{{var}}`, múltiplos IPs (`choices` + namespaces), TUI de busca e `als sync` via Git. |
| **Requisitos Não-Funcionais** | ✅ | Startup `< 5ms` no modo shim, binário único em Go sem dependências externas, escrita atômica com `flock`. |
| **Casos de Uso & Fluxos Alternativos** | ✅ | Alias fixo por servidor, alias único com seletor de servidores, cadastro via TUI, CLI inline ou `--last`, inspeção com `--als-info`. |
| **Edge Cases** | ✅ | Caminhos com espaços, aspas já presentes no template, repasse de flags (`-v`, `--help`) ao comando real, execução em pipes sem TTY, upgrade do binário no Homebrew. |
| **Failure Modes** | ✅ | Colisão com binários do sistema (`ls`, `ssh`, `sudo`), conflito de rebase no Git, rede offline no `sync`, corrupção por concorrência. |
| **Permissões e Segurança** | ✅ | Diretório `0700`, `secrets.toml` `0600`, variáveis passadas via `envv` (invisíveis no `ps aux`), trava contra `secrets.toml` rastreado no Git, linter de caminhos relativos. |
| **Testes e Observabilidade** | ✅ | Testes unitários e de integração em diretórios isolados, flag `ALS_DEBUG=1` para diagnóstico de timing e resolução de variáveis, comando `als doctor`. |
| **Custos Operacionais** | ✅ | \$0 (arquitetura 100% local + Git BYO-Repo). |

---

## 8. Plano de Implementação Faseado e Critérios de Aceite (Fase 6)

### Fase 1: Fundação, Schemas e Armazenamento Seguro (`internal/config`, `internal/store`)
* **Entregas:**
  * Inicialização do módulo Go, estrutura de diretórios XDG (`~/.config/als`, `~/.local/share/als/bin`).
  * Parser e serializer de `commands.toml` e `secrets.toml` (`go-toml/v2`).
  * Escrita atômica com `syscall.Flock`, aplicação estrita de permissões (`0700`/`0600`) e geração automática do `.gitignore`.
* **Critério de Aceite:** Testes de concorrência com 20 goroutines escrevendo simultaneamente sem perda de dados ou corrupção de TOML.

### Fase 2: Motor de Shims, Resolução de Variáveis e `execve` (`internal/shim`, `internal/exec`)
* **Entregas:**
  * Dispatcher em `cmd/als/main.go` diferenciando `als` vs chamada de Shim via `os.Args[0]`.
  * Criador de symlinks relativos (`<alias> -> .als-bin`) + validador de colisão contra `PATH` do sistema e built-ins POSIX.
  * Compilador de template `{{var}} -> "$ALS_VAR_var"` (removendo aspas redundantes adjacentes) + resolução hierárquica (`CLI > comando > global > prompt Huh`).
  * Execução via `syscall.Exec` repassando `"$@"` posicional e variáveis no ambiente.
* **Critério de Aceite:** Executar um comando parametrizado com caminhos contendo espaços e flags extras (`-v -p 2222`), verificando que `ps aux` não exibe o valor secreto em texto puro e que o exit code é preservado.

### Fase 3: CLI, Linter de Cadastro e Interface TUI (`internal/cli`, `internal/linter`, `internal/tui`)
* **Entregas:**
  * Comandos `als init`, `als add` (Wizard TUI + `--last` + inline), `als list` (TUI interativa com filtro fuzzy para buscar e rodar comandos), `als rm`, `als edit`, `als info`, `als doctor` / `als rebuild`.
  * Linter estático no `als add` detectando caminhos relativos de arquivos (ex.: `-i "chave.key"`) sugerindo conversão para caminho absoluto ou variável `{{key_path}}`, e detectando IPs/tokens para extração automática ao `secrets.toml`.
* **Critério de Aceite:** Cadastrar o comando `ssh -i "prod-key.pem" ubuntu@203.0.113.10` via `als add`, validar que o linter sugere parametrizar o caminho e o IP, e executar `connectServerProd` de qualquer diretório da máquina.

### Fase 4: Sincronização Git Segura e Distribuição Open-Source (`internal/sync`, `.github/workflows`)
* **Entregas:**
  * Comando `als sync init <repo-url>` e `als sync` (`git pull --rebase`, verificação anti-leak do `secrets.toml` via `git ls-files`, `commit`, `push` com timeout de 15s e `als rebuild` automático pós-pull).
  * Pipeline `GoReleaser` + GitHub Actions para gerar binários macOS (`arm64`/`amd64`) e Linux (`arm64`/`amd64`), Homebrew Tap e `README.md` com documentação e GIF de demonstração.
* **Critério de Aceite:** Simular sincronização entre 2 diretórios locais via repositório Git `bare`, garantindo que novos comandos geram syims automaticamente na segunda máquina e que os valores secretos permanecem isolados localmente.

---

## 9. Decisões Pendentes do Usuário para Iniciar a Codificação

1. **Nome definitivo da ferramenta/binário:** Vamos manter **`als`** ou você prefere outro nome para o comando principal e repositório no GitHub?
2. **Início da implementação:** Deseja que eu inicie agora a **Fase 1 e Fase 2 da implementação** neste repositório?


---

# Deep Planning v1.1: Descoberta de Parâmetros (`KEY=VAL` / URLs), Resolução Posicional e Comando `als edit`

> [!IMPORTANT]
> **Status do Planejamento v1.1:** `APPROVED` (Concluídas as Fases 1 a 6 do fluxo `deep-planning`, incluindo `brainstorming` e revisão adversarial `multi-agent-brainstorming`).

---

## 1. Exploração de Abordagens e Consolidação do DESIGN V0 (Fase 1 & Fase 2)

### 1.1 Alternativas Arquiteturais Avaliadas (Brainstorming Step 5)

| Abordagem | Descrição | Complexidade | Risco | Manutenção | Veredito |
| :--- | :--- | :---: | :---: | :---: | :---: |
| **Opção A (Recomendada): Placeholders `{{var}}` com Resolução Posicional Inteligente + Linter de `KEY=VAL`/URLs + Wizard `als edit`** | Expande o linter para detectar `CHAVE=VALOR` e URLs, adiciona etapa guiada no `als add`/`als edit` para definir o modo da variável (valor padrão salvo, perguntar sempre ou `choices`), e permite passar argumentos posicionais diretos na execução (`buildCdaForcaAdr https://...`) sem exigir `@var=`. | Média | Baixo | Alta coesão (reutiliza `store`, `compiler` e `huh`) | **Selecionada** |
| **Opção B: Flag dedicada do Shim (`-p` / `--param`) + Truncamento de String (`URL=`)** | O comando salvo termina em `URL=` e o shim intercepta `-p <valor>` para concatenar ao final da string antes de chamar `/bin/sh`. | Baixa | **Crítico** | Frágil | **Rejeitada** (colide com `-p` do `ssh`/`docker`/`mkdir` e só suporta 1 parâmetro no fim do comando) |
| **Opção C: Parser de Flags Dinâmicas por Variável (`--url=...`)** | Transforma cada `{{var}}` em uma flag POSIX `--<var>` interceptada pelo shim. | Média | Alto | Média | **Rejeitada** (colide com flags nativas do comando alvo como `--url`, `--env`, `--flavor` e exige digitar o nome da flag sempre) |

### 1.2 Consolidação do DESIGN V0 (Hipótese Inicial)
1. **Linter (`internal/linter/linter.go`):** Adicionar regex para capturar `([A-Za-z0-9_]+)=(https?://[^\s"']+)` e URLs soltas `https?://[^\s"']+`, sugerindo `{{<chave_lower>}}` (ex.: `URL=https://...` $\rightarrow$ `URL={{url}}`).
2. **Fluxo `als add` e Novo `als edit` (`internal/cli/add.go`, `internal/cli/edit.go`):**
   * `als edit [alias]`: Se `[alias]` for omitido, abre um seletor `huh.Select` com os aliases existentes. Em seguida, abre o formulário `huh.Form` pré-preenchido com `Alias Name`, `Command`, `Description`, `Tags` e `Confirm`.
   * Ao detectar variáveis `{{var}}` (seja digitadas manualmente ou geradas pelo Linter), apresenta um passo de configuração para cada variável:
     * Modo 1: **Valor padrão salvo** (usa o valor salvo quando chamado sem argumentos; permite sobrescrever na chamada).
     * Modo 2: **Perguntar sempre (Prompt)** (sempre abre o input interativo se não for passado na linha de comando).
     * Modo 3: **Lista de opções (`choices`)** (cadastra apelidos/opções como `prod`, `homol`).
3. **Resolução Posicional (`internal/exec/resolver.go`):**
   * Argumentos de linha de comando que não começam com `-` nem `@` preenchem na ordem as variáveis `{{var}}` declaradas em `compiled.Variables`.
4. **Compilador (`internal/exec/compiler.go`):**
   * Substituir `{{var}}` por `"$ALS_VAR_VAR"` no comando compilado.

---

## 2. Revisão Adversarial Multi-Agente sobre o DESIGN V0 (Fase 3 — `multi-agent-brainstorming`)

### 2.1 Parecer do Agente 1: Skeptic / Challenger ("Por que este DESIGN V0 vai falhar em produção?")

* **[Obj-S1] Conflito entre Sobrescrita Posicional e Argumentos Posicionais do Comando Alvo ("Bug do `conectar ls`"):**
  * *Ataque:* No DESIGN V0, qualquer argumento que não comece com `-` ou `@` preenche a primeira variável `{{var}}`. Considere o comando `conectar` (`ssh -i {{key_path}} ubuntu@{{server_ip}}`) onde `key_path` e `server_ip` são credenciais fixas salvas em `secrets.toml`. Se o usuário rodar `conectar ls -la` para listar arquivos no servidor remoto, o DESIGN V0 vai capturar `ls` como o valor da 1ª variável (`{{key_path}}`), tentando rodar `ssh -i "ls" ubuntu@203.0.113.10 -la`!
  * *Exigência:* É necessário distinguir variáveis de **credencial fixa** (que só mudam via `@var=val`) de variáveis de **parâmetro posicional/dinâmico** (ou exigir que argumentos posicionais preencham apenas variáveis que aceitem parâmetro de chamada, respeitando também o separador POSIX `--`).
* **[Obj-S2] Bug de Aspas Assimétricas no `compiler.go` Atual com `KEY={{var}}`:**
  * *Ataque:* O regex atual em `internal/exec/compiler.go` é `(["'])?\{\{([a-zA-Z0-9_-]+)\}\}(["'])?`. Os grupos de aspas de abertura (`sub[1]`) e fechamento (`sub[3]`) são opcionais e independentes! Se o usuário cadastrar `'URL={{url}}'` ou `"URL={{url}}"`, não há aspas *imediatamente antes* de `{{`, mas há uma aspa *imediatamente depois* de `}}`. O `compiler.go` consome a aspa de fechamento e a substitui por `"$ALS_VAR_URL"`, gerando `'URL="$ALS_VAR_URL"` — uma string com aspa simples não fechada que quebra o `/bin/sh` com `Syntax error: Unterminated quoted string`!
* **[Obj-S3] Corrupção de Comando por `strings.ReplaceAll` Cego no Auto-Fix do Linter:**
  * *Ataque:* Em `internal/cli/add.go` (linha 202), o auto-fix do Linter faz `strings.ReplaceAll(rawCmd, w.RawMatch, placeholder)`. Se o comando for `fvm flutter build appbundle --flavor cda -t lib/main_cda.dart --dart-define FLAVOR=cda` e o linter extrair `w.RawMatch = "cda"`, o `ReplaceAll` destruirá o comando trocando todas as ocorrências: `--flavor {{flavor}} -t lib/main_{{flavor}}.dart --dart-define FLAVOR={{flavor}}`.

### 2.2 Parecer do Agente 2: Constraint Guardian (Segurança, Concorrência, Integridade de Dados)

* **[Obj-C1] Vazamento de Segredo em `commands.toml` ao Alternar Modo da Variável no `als edit`:**
  * *Ataque:* Se uma variável tinha `default = "https://..."` salvo em `commands.toml` (público no Git) e no `als edit` o usuário marca `secret = true`, se o código apenas gravar no `secrets.toml` sem limpar `VarDef.Default = ""` no `commands.toml`, o valor sensível continuará exposto no repositório Git após `als sync`.
* **[Obj-C2] Perda de Segredos e Symlink Órfão ao Renomear Alias no `als edit`:**
  * *Ataque:* Se o usuário abrir `als edit buildCdaForcaAdr` e alterar o nome para `buildCda`, salvar apenas a nova chave em `commands.toml` deixará `buildCdaForcaAdr` duplicado no `commands.toml`, deixará o symlink antigo `~/.local/share/als/bin/buildCdaForcaAdr` ativo e perderá o vínculo com `[commands.buildCdaForcaAdr]` no `secrets.toml`. A operação de renomeação precisa ser atômica sob `WithLock` migrando `commands.toml` e `secrets.toml` e reconciliando os symlinks.

### 2.3 Parecer do Agente 3: User Advocate (Usabilidade, Carga Cognitiva e Descoberta)

* **[Obj-U1] Invisibilidade de Parâmetros no `als list`:**
  * *Ataque:* Mesmo após parametrizar `buildCdaForcaAdr` com `URL={{url}}`, quando o usuário roda `als list` (como na captura de tela enviada), todas as palavras do comando aparecem na mesma cor cinza (`lipgloss.Color("248")`), sem destacar `{{url}}` e sem mostrar se `{{url}}` tem um valor padrão salvo ou como passar outro valor no terminal.
* **[Obj-U2] Fricção no Prompt Quando Já Existe Valor Salvo/Default:**
  * *Ataque:* Quando o usuário escolhe o modo "Perguntar sempre (com sugestão padrão)" ou quando edita uma variável no `als edit`, o campo de input do `huh.NewInput()` vem em branco se não for pré-populado com o valor atual salvo em `secrets.toml` / `default`, obrigando o usuário a redigitar ou colar a URL inteira toda vez.

### 2.4 Decisão do Agente 4: Integrator / Arbiter

* **Disposição Final da Revisão Multi-Agente:** **`APPROVED` (com incorporação obrigatória das 7 correções no DESIGN V1)**.
* **Racional do Árbitro:** As objeções `Obj-S1`, `Obj-S2` e `Obj-S3` identificaram bugs reais no código existente (`compiler.go` e `add.go`) e um caso de borda importante na resolução posicional (`resolver.go`). Todas as 7 correções são cirúrgicas, não inflam o escopo e tornam a experiência de criar, editar e executar comandos parametrizados consistente e à prova de falhas.

---

## 3. DESIGN V1 Consolidado (Fase 4 — Design Revision)

### 3.1 Correção do Compilador de Template (`internal/exec/compiler.go` — Resolve `Obj-S2`)
* O compilador deixa de capturar aspas soltas assimétricas. Ele passa a:
  1. Reconhecer `"{{var}}"` ou `'{{var}}'` **apenas quando as aspas de abertura e fechamento forem idênticas e imediatamente adjacentes ao placeholder**, convertendo para `"$ALS_VAR_VAR"`.
  2. Quando `{{var}}` estiver colado a um prefixo como `URL={{url}}` (fora de aspas) ou dentro de aspas maiores (`"--dart-define=URL={{url}}"`):
     * Se estiver fora de aspas: converte `{{url}}` para `"$ALS_VAR_URL"`, resultando em `URL="$ALS_VAR_URL"` (que no `/bin/sh` POSIX concatena `URL=` com o conteúdo protegido de `$ALS_VAR_URL` sem dividir em espaços).
     * Se estiver dentro de aspas duplas (`"URL={{url}}"`): converte `{{url}}` para `$ALS_VAR_URL`, preservando as aspas duplas externas (`"URL=$ALS_VAR_URL"`).
     * Se estiver dentro de aspas simples (`'URL={{url}}'`): converte `{{url}}` para `'"$ALS_VAR_URL"'`, fechando a aspa simples, expandindo a variável entre aspas duplas e reabrindo a aspa simples.

### 3.2 Linter Preciso por Span (`internal/linter/linter.go` — Resolve `Obj-S3`)
* A struct `LintWarning` passa a conter:
  * `Type`: `"raw_ip"`, `"relative_path"`, `"raw_token"`, `"key_value_url"`, `"raw_url"`.
  * `TargetSpan`: trecho exato a ser substituído no comando (ex.: `URL=https://tabletserveroci.casadoadubo.com.br:8601`).
  * `ReplacementSpan`: trecho parametrizado (ex.: `URL={{url}}`).
  * `RawMatch`: apenas o valor extraído para persistência (ex.: `https://tabletserveroci.casadoadubo.com.br:8601`).
  * `VariableKey`: nome sugerido da variável em `snake_case` minúsculo (ex.: `url`).
* A substituição usa `strings.Replace(rawCmd, w.TargetSpan, w.ReplacementSpan, 1)`, impedindo que substrings curtas corrompam outras partes do comando.

### 3.3 Resolução Posicional Determinística e Segura (`internal/exec/resolver.go` — Resolve `Obj-S1` e `Obj-U2`)
* Adicionamos o campo `Positional bool` (`toml:"positional,omitempty"`) ou regra clara em `VarDef`:
  * Ordem de resolução dos argumentos recebidos no Modo Shim (`args`):
    1. **Respeito ao delimitador POSIX `--`:** Tudo após `--` vai direto para `RemainingArgs` (`"$@"`), sem ser inspecionado pelo `als`.
    2. **Overrides explícitos `@var=valor` / `@var:valor`:** Sempre têm prioridade máxima para a variável nomeada.
    3. **Matches exatos de `choices`:** Se um argumento posicional bater com uma opção de `choices` de uma variável ainda não resolvida, resolve essa variável.
    4. **Argumentos Posicionais Diretos (`buildCdaForcaAdr https://...`):**
       * Todo argumento que **não** comece com `-` (não é flag) é associado, na ordem, às variáveis do comando que aceitam entrada posicional:
         * Variáveis sem valor fixo salvo (modo **Prompt / Dinâmico**); OU
         * Variáveis com `Default` / valor salvo que estejam marcadas como **Parâmetro Posicional (`positional = true`)** ou quando o comando possuir variáveis de parâmetro (como `{{url}}`), permitindo que `buildCdaForcaAdr https://outra-url:8601` sobrescreva o valor padrão de `{{url}}` diretamente sem `@url=`!
       * Variáveis de credencial estrita (como `key_path` em comandos SSH com múltiplas variáveis onde `positional = false`) não engolem subcomandos como `ls`.
    5. **Fallback para Valor Salvo (`secrets.toml` > `default` > `env`) e Prompt Interativo (`huh`):**
       * Se nenhuma entrada CLI foi passada, busca em `secrets.toml` e depois em `VarDef.Default`.
       * Se a variável for configurada como `prompt_always = true` (Modo "Perguntar toda vez"), abre o prompt interativo `huh.NewInput()` **já pré-preenchido** com o último valor/default (`Obj-U2`), permitindo apenas apertar `Enter` para confirmar ou digitar/colar outro valor.

### 3.4 Fluxo Unificado de Variáveis no `als add` e Novo Comando `als edit` (`internal/cli/add.go`, `internal/cli/edit.go` — Resolve `Obj-C1`, `Obj-C2`, `Obj-U1`)
* **Dica explícita na TUI:** O campo `Command to execute` no `als add` e `als edit` exibe na `Description`:
  `Use {{var}} for dynamic parameters (e.g. --dart-define URL={{url}}).`
* **Configuração Guiada de Variáveis (`configureVariablesWizard`):**
  * Sempre que o comando final contiver `{{var}}` (adicionado manualmente pelo usuário ou convertido pelo Linter), para cada variável `vName`, o wizard pergunta de forma enxuta:
    1. **Modo de Preenchimento da variável `{{vName}}`:**
       * `Valor padrão salvo (roda direto, mas aceita outro valor na chamada: alias <valor>)`
       * `Perguntar sempre que executar (abre prompt no terminal se não for passado na chamada)`
       * `Lista de opções pré-definidas (choices: ex. prod, homol, dev)`
    2. **Valor padrão / Opções** (já pré-preenchido se o Linter acabou de extrair `https://...` do comando ou se já existia no `als edit`).
    3. **Isolamento (Segredo local vs Público no Git):** Se deseja salvar o valor em `secrets.toml` (local, fora do Git) ou como `default` em `commands.toml`. Se `secret = true`, limpa obrigatoriamente `Default = ""` em `commands.toml` (`Obj-C1`).
* **Novo Comando `als edit [alias]`:**
  * Se chamado sem argumento (`als edit`), lista os comandos cadastrados num seletor interativo `huh.Select`.
  * Abre o formulário pré-preenchido com os dados atuais de `buildCdaForcaAdr` (incluindo o comando atual), passa pelo Linter (permitindo converter `URL=https://...` em `URL={{url}}` com 1 confirmação) e pelo wizard de variáveis.
  * Persiste via `st.UpdateCommand(oldAlias, newAlias, cmdDef)` sob `WithLock` e atualiza os symlinks no `shimMgr` (`Obj-C2`).
* **Destaque Visual no `als list` (`internal/cli/list.go` — Resolve `Obj-U1`):**
  * Destaca placeholders `{{var}}` em cor diferenciada dentro da string do comando e exibe uma linha resumo abaixo dos comandos parametrizados (ex.: `↳ params: {{url}} (usage: buildCdaForcaAdr [<url>] or @url=<val>)`).

---

## 4. Registro de Decisões da v1.1 (Decision Log)

| ID | Decisão | Alternativas Consideradas | Objeções Associadas | Resolução e Racional Final |
| :--- | :--- | :--- | :--- | :--- |
| **D7** | Sobrescrita de parâmetro via argumento posicional direto (`alias <valor>`) + `@var=valor` | Flag `-p <valor>`; flags dinâmicas `--<var>=<valor>` | `Obj-S1`, `Obj-S2 (v1)` | Mais curto de digitar que `-p` e `@var=`, e zero risco de colidir com flags `-p` nativas de comandos Unix (`ssh`, `docker`). |
| **D8** | Linter de `KEY=VAL` e URLs baseado em `TargetSpan` -> `ReplacementSpan` | `strings.ReplaceAll` apenas no valor bruto | `Obj-S3` | Impede que valores curtos ou repetidos adulterem outras partes do comando ao parametrizar. |
| **D9** | Compilador sensível ao contexto de aspas (`compiler.go`) | Regex gulosa com aspas opcionais independentes | `Obj-S2` | Corrige bug de aspas não fechadas em `'KEY={{var}}'` e `"KEY={{var}}"`, garantindo expansão segura no `/bin/sh`. |
| **D10** | Comando `als edit [alias]` compartilhando pipeline de Linter + Wizard de Variáveis com `als add` | Editar arquivos TOML manualmente ou recriar com `als add` | `Obj-C1`, `Obj-C2`, `Obj-U2` | Permite parametrizar comandos já cadastrados (como `buildCdaForcaAdr`) em segundos sem perder configurações e sem vazar segredos. |

---

## 5. Verificação de Completude da v1.1 (Fase 5 — Completeness Check)

| Dimensão | Status | Cobertura no DESIGN V1 (v1.1) |
| :--- | :---: | :--- |
| **Requisitos Funcionais** | ✅ | Linter detecta URLs e `KEY=VAL`, wizard configura modo da variável, `als edit` edita/renomeia comandos existentes, shim aceita argumento posicional direto. |
| **Requisitos Não-Funcionais** | ✅ | Execução do shim permanece `< 5ms`, zero dependências novas no `go.mod`, lock exclusivo em edições/renomeações. |
| **Edge Cases** | ✅ | `URL={{url}}` colado sem espaço, `"URL={{url}}"` entre aspas duplas/simples, argumentos extras com flags (`-v`), delimitador `--`, valores de URL contendo `:`, `/`, `?`, `&`, `=`. |
| **Failure Modes & Segurança** | ✅ | Limpeza automática de `Default` público quando `Secret = true` (`Obj-C1`), migração atômica de `secrets.toml` ao renomear comando (`Obj-C2`), substituição única por span no Linter (`Obj-S3`). |
| **Testes** | ✅ | Testes unitários para `CompileTemplate` (`URL={{url}}`, `'URL={{url}}'`, `"URL={{url}}"`), `ResolveVariables` (posicional direto + flags extras), `linter.Analyze` (URLs e `--dart-define`) e `Store.UpdateCommand`. |

---

## 6. Plano de Implementação Faseado e Critérios de Aceite da v1.1 (Fase 6)

### Etapa 1: Compilador Contextual e Resolução Posicional (`internal/exec`, `internal/store`)
* **Arquivos modificados:**
  * `internal/store/model.go` e `internal/store/store.go`: Adição de metadados de variável (`Positional`, `PromptAlways`) em `VarDef` e método atômico `UpdateCommand(oldName, newName string, cmd Command)`.
  * `internal/exec/compiler.go` e `internal/exec/compiler_test.go`: Correção do `CompileTemplate` para lidar com `KEY={{var}}`, `"KEY={{var}}"` e `'KEY={{var}}'` sem corromper aspas.
  * `internal/exec/resolver.go` e `internal/exec/exec_test.go`: Suporte a argumentos posicionais diretos para variáveis dinâmicas/posicionais (ex.: `buildCdaForcaAdr https://...`), suporte ao separador `--` e pré-preenchimento do input interativo quando houver valor padrão/salvo.

### Etapa 2: Evolução do Linter para URLs e `KEY=VALUE` (`internal/linter`)
* **Arquivos modificados:**
  * `internal/linter/linter.go` e `internal/linter/linter_test.go`: Detecção de `CHAVE=https://...`, `--dart-define CHAVE=VALOR` e URLs avulsas, preenchendo `TargetSpan`, `ReplacementSpan`, `RawMatch` e `VariableKey` (ex.: `URL=https://...` $\rightarrow$ `URL={{url}}`).

### Etapa 3: Wizard de Variáveis, Comando `als edit` e Destaque no `als list` (`internal/cli`)
* **Arquivos modificados/criados:**
  * `internal/cli/add.go`: Instrução de `{{var}}` no formulário, substituição segura via `TargetSpan` do Linter e wizard interativo de configuração de variáveis (`configureVariablesWizard`).
  * `internal/cli/edit.go` (novo): Comando `als edit [alias]` com seletor interativo (se omitido o nome), formulário pré-preenchido, execução opcional do Linter para parametrizar trechos existentes, configuração de variáveis e atualização atômica de store + shims.
  * `internal/cli/list.go`: Destaque colorido de `{{var}}` e dica de uso dos parâmetros na listagem.

### Critérios de Aceite Finais
1. Rodar `als edit buildCdaForcaAdr`, aceitar a sugestão do Linter para transformar `URL=https://tabletserveroci.casadoadubo.com.br:8601` em `URL={{url}}` e salvar o valor padrão.
2. Rodar `buildCdaForcaAdr --als-dry-run` (sem argumentos) e verificar que resolve `URL=https://tabletserveroci.casadoadubo.com.br:8601` via `$ALS_VAR_URL`.
3. Rodar `buildCdaForcaAdr https://outro-servidor.com.br:8601 --als-dry-run` e verificar que `{{url}}` recebe `https://outro-servidor.com.br:8601` sem precisar digitar `@url=` nem `-p`.
4. Todos os testes automatizados (`go test ./...`) passando sem regressões.
