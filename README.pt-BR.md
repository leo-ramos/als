# `als` — Gerenciador de Comandos & Aliases Parametrizados no Terminal

[![Licença: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)

[English](README.md) · [**Português (BR)**](README.pt-BR.md) · [Por que o `als`?](#why-als) · [Passo a Passo](#quick-start) · [Segurança & Git Sync](#security) · [Uso em Equipes](#team-sharing) · [Licença](#license)

> Transforme comandos longos e repetitivos em executáveis diretos no seu terminal (`$PATH`) com parâmetros dinâmicos (`{{var}}`), isolamento estrito de segredos e sincronização via Git.

---

<a id="why-als"></a>
## 💡 Por que o `als` existe?

O comando `alias` nativo do shell possui limitações graves no dia a dia:
1. **Exige edição manual:** É preciso alterar arquivos `.zshrc`/`.bashrc` e recarregar cada aba aberta do terminal (`source`).
2. **Não aceita parâmetros dinâmicos:** Um alias comum não permite passar variáveis no meio do comando (ex.: um IP que varia ou uma chave específica).
3. **Quebra com caminhos relativos:** Comandos que referenciam arquivos locais (ex.: `ssh -i "chave.key"`) falham ao serem chamados de qualquer outro diretório.
4. **Risco de vazamento de credenciais:** Salvar IPs de servidores, chaves privadas ou tokens no `.zshrc` vaza dados sensíveis ao versionar dotfiles no GitHub.

O **`als`** resolve isso criando **shims executáveis nativos no seu `$PATH`**: qualquer comando cadastrado fica imediatamente disponível em todas as abas do terminal, com tempo de inicialização `< 5ms`, repasse transparente de TTY (SSH, Docker, Vim) e separação física entre o modelo do comando (público) e os dados sensíveis (locais).

---

<a id="quick-start"></a>
## 🛠️ Passo a Passo Completo

### 1. Instalação

Dentro do diretório clonado do projeto, compile e copie o executável para o seu sistema:

```bash
# 1. Compilar o binário
go build -o als ./cmd/als

# 2. Mover para o diretório de executáveis do sistema
sudo cp ./als /usr/local/bin/

# 3. Confirmar a instalação
als --help
```

---

### 2. Configuração Inicial do Ambiente

Execute o comando de inicialização uma única vez:

```bash
als init
```

Esse comando cria os diretórios seguros e o shim mestre do `als`. 
* Caso o diretório de shims (`~/.local/share/als/bin`) ainda não esteja no seu `$PATH`, o `als init` instruirá a linha exata a ser adicionada ao seu `~/.zshrc` ou `~/.bashrc`:
  ```bash
  echo 'export PATH="$HOME/.local/share/als/bin:$PATH"' >> ~/.zshrc
  source ~/.zshrc
  ```

---

### 3. Cadastrar um Comando Parametrizado

Para registrar um comando contendo variáveis dinâmicas ou dados sensíveis, utilize a sintaxe `{{nome_da_variavel}}`.

Por exemplo, para criar o comando **`conectarServidor`**:

```bash
als add conectarServidor 'ssh -i "{{key}}" ubuntu@{{server_ip}}' --desc "Acesso SSH ao servidor principal"
```

> **Dica:** O `als` possui um analisador estático integrado (Linter). Ele detecta automaticamente que `key` e `server_ip` são credenciais sensíveis e as isola fora do arquivo sincronizado com o Git.

Você também pode utilizar o modo interativo simplesmente digitando:
```bash
als add
```

---

### 4. Configurar os Segredos (`key` e `server_ip`)

O `als` nunca salva dados sensíveis (como IPs de servidores e caminhos de chaves) no arquivo de comandos público (`commands.toml`). Ele armazena esses valores em `~/.config/als/secrets.toml` com permissão estrita `0600` (leitura e escrita apenas pelo seu usuário).

Você pode definir os valores de **duas formas**:

#### Opção A: Interativa (No primeiro uso)
Na primeira vez que você executar o comando, o terminal detectará que as variáveis estão vazias e abrirá um prompt seguro:
1. Digite o caminho da chave (ex.: `~/.ssh/minha-chave-producao.key`).
2. Digite o IP do servidor (ex.: `203.0.113.10`).
3. O terminal perguntará se deseja salvar esses valores localmente no `secrets.toml` para não precisar digitá-los novamente.

#### Opção B: Direta (Editando o arquivo local `secrets.toml`)
Abra o arquivo `~/.config/als/secrets.toml` (ele é criado automaticamente pelo `als`):

```toml
# ~/.config/als/secrets.toml
# Arquivo local da máquina — NUNCA comitado no Git (permissão 0600)

[commands.conectarServidor]
key = "/Users/seu-usuario/.ssh/minha-chave-producao.key"
server_ip = "203.0.113.10"
```

*Se você utiliza a mesma chave SSH para múltiplos comandos, pode configurá-la no escopo global:*
```toml
[global]
key = "/Users/seu-usuario/.ssh/chave-geral.key"
```

---

### 5. Chamar o Comando no Terminal

Depois de cadastrado, **você não precisa digitar `als run`**. Digite apenas o nome do comando criado em qualquer terminal ou aba:

```bash
conectarServidor
```

O `als` resolverá as variáveis `$key` e `$server_ip` no ambiente em milissegundos e iniciará a sessão SSH diretamente.

#### Repasse de Flags e Argumentos Extras
Qualquer argumento adicional passado para o alias é repassado intacto ao comando real via `"$@"`:

```bash
# Adiciona modo verboso (-v) e porta customizada (-p 2222) ao SSH
conectarServidor -v -p 2222
```

#### Sobrescrita Rápida de Parâmetros
Você pode passar valores de parâmetros de duas formas:

1. **Posicional Direto (Mais rápido e curto):**
   Passe o valor diretamente no terminal sem precisar de flags:
   ```bash
   buildCdaForcaAdr https://tabletserveroci.casadoadubo.com.br:8601
   ```
2. **Nomeado Explícito (`@var=valor`):**
   Ideal para comandos com múltiplos parâmetros quando você quer alterar uma variável específica:
   ```bash
   conectarServidor @server_ip=203.0.113.25
   ```

---

### 6. Inspeção e Teste Seguro (Dry-Run & Info)

Para inspecionar o que um comando faz antes de executá-lo:

```bash
# Exibe template, variáveis cadastradas e descrição
conectarServidor --als-info

# Simula a execução completa sem disparar o processo real
conectarServidor --als-dry-run
```

---

<a id="security"></a>
## 🔒 Arquitetura de Segurança e Git Sync

O `als` implementa uma barreira estrita entre modelos de comando e credenciais:

```text
~/.config/als/                  (drwx------ 0700)
├── .git/                       (Repositório Git de dotfiles)
├── .gitignore                  (Bloqueia secrets.toml, .lock, *.tmp)
├── .lock                       (Bloqueio POSIX flock para concorrência)
├── commands.toml               (Templates públicos — SEGURO para o Git)
└── secrets.toml                (Valores locais — NUNCA enviado ao Git)

~/.local/share/als/bin/         (No seu $PATH)
├── .als-bin                    (Symlink mestre -> /usr/local/bin/als)
└── conectarServidor            (Symlink relativo -> .als-bin)
```

### Por que é seguro enviar para o Git?
1. **Proteção contra `ps aux`:** Ao executar o comando, os segredos são passados como variáveis de ambiente (`$ALS_VAR_<NOME>`), impedindo que senhas ou IPs fiquem expostos como texto puro na lista de processos do sistema operacional.
2. **Trava de Segurança no Git Sync:** O comando `als sync` contém uma validação automática que bloqueia a sincronização caso detecte que o arquivo `secrets.toml` foi acidentalmente rastreado pelo Git.
3. **Escrita Atômica:** Alterações concorrentes em abas simultâneas utilizam `flock` consultivo para evitar arquivos corrompidos.

---

<a id="team-sharing"></a>
### 👥 Compartilhamento em Equipes: Comandos vs. Credenciais

Um dos casos de uso mais poderosos do `als` é a padronização de rotinas e fluxos operacionais em equipes de TI e engenharia. Entenda a separação prática entre os 3 níveis:

#### 1. O Software Open-Source (Este Repositório Público)
* Contém apenas o motor e binário do `als`.
* **Nenhum comando ou configuração sua é enviada para cá.** Seus comandos salvos ficam guardados exclusivamente dentro da pasta `~/.config/als/` da sua máquina pessoal.

#### 2. O Repositório Próprio da sua Equipe (`als sync`)
* Um líder de TI ou equipe pode criar um repositório Git próprio (geralmente privado, ex.: `git@github.com:sua-empresa/comandos-ti.git`).
* Ao configurar `als sync --remote <url>`, apenas os **modelos de comandos** (`commands.toml`) são sincronizados.
* Todos os membros da equipe que rodarem `als sync` passam a ter exatamente os mesmos comandos disponíveis no terminal (ex.: `conectarProducao`, `backupBanco`, `tunnelsCluster`), garantindo padronização imediata.

#### 3. Fronteira de Segurança: Por que chaves NUNCA vão para o Git
* O `als` **bloqueia deliberadamente** o envio do arquivo `secrets.toml` para o Git, mesmo que o repositório seja privado da empresa.
* **Motivo:** Versionar chaves privadas SSH (`.key`, `.pem`) ou tokens em repositórios Git é uma grave falha de conformidade (qualquer clone ou histórico vazado expõe a infraestrutura). Além disso, caminhos de arquivos diferem entre máquinas (ex.: `/Users/chefe/.ssh/id_rsa` no macOS vs `/home/dev/.ssh/id_rsa` no Linux).
* **Como funciona na prática:** O time compartilha a **estrutura do comando** via Git (`ssh -i "{{key}}" ubuntu@{{server_ip}}`). Cada desenvolvedor mantém sua respectiva chave e permissões configuradas no seu `secrets.toml` local (ou responde ao prompt interativo na primeira execução).

---

## 📋 Comandos Utilitários

```bash
# Listar todos os aliases cadastrados
als list

# Editar um alias existente (ou escolher interativamente se omitir o nome)
als edit buildCdaForcaAdr

# Ver detalhes e localização do shim de um alias
als info conectarServidor

# Remover um alias e limpar seu symlink no sistema
als rm conectarServidor

# Executar diagnóstico de saúde do sistema e reconstruir shims
als doctor

# Sincronizar comandos públicos com seu repositório de dotfiles
als sync
```

---

<a id="license"></a>
## 📄 Licença

Distribuído sob a licença **Apache 2.0**. Consulte [LICENSE](LICENSE) para mais informações.
