# cloudflare-ddns-agent

Mantém um ou mais registros DNS tipo A na Cloudflare sempre apontando para o IP público atual da máquina onde roda. Feito para servidores com IP dinâmico (conexão residencial/DSL, sem IP fixo) que precisam de um hostname estável.

## Como funciona

A cada execução, o agente:
1. Descobre o IP público atual da máquina.
2. Consulta na Cloudflare qual é o valor atual de cada hostname configurado (nunca confia em cache local).
3. Cria o registro se não existir, atualiza se o IP mudou, ou não faz nada se já está correto.

No Linux isso roda via `systemd timer` (instalado automaticamente), não como processo contínuo. No Windows roda como uma tarefa agendada (veja abaixo).

## Instalação no Linux

Como root, numa máquina Linux (amd64 ou arm64):

```bash
curl -fsSL https://raw.githubusercontent.com/MX2Tech/cloudflare-ddns-agent/main/install.sh | sudo bash
```

Vai pedir 4 informações:
- **Cloudflare API Token** — veja abaixo como gerar.
- **Zona** — o domínio raiz na Cloudflare (ex: `mx2tech.cloud`).
- **Hostname** — o registro a manter atualizado (ex: `hub.mx2tech.cloud`).
- **Intervalo** — de quanto em quanto tempo checar (padrão 30 segundos).

O instalador testa a configuração na hora e já mostra se funcionou.

## Instalação no Windows

Windows 10/11 ou Server 2016+ (amd64). Abra o **PowerShell como administrador** e rode:

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/MX2Tech/cloudflare-ddns-agent/main/install.ps1))) -Zona mx2tech.cloud -Hostname cliente.mx2tech.cloud
```

O token é pedido de forma oculta (não fica no histórico do PowerShell). O instalador:
- baixa o `.exe` da última release e confere o SHA-256 contra o `checksums.txt`;
- grava o binário em `C:\Program Files\cloudflare-ddns-agent\` (só administrador escreve lá, e ele roda como SYSTEM);
- grava a config em `C:\ProgramData\cloudflare-ddns-agent\config.yaml`, numa pasta acessível **só por SYSTEM e Administradores** (o `ProgramData` é legível por qualquer usuário por padrão);
- testa com `update` e registra a tarefa agendada `cloudflare-ddns-agent`.

A tarefa roda `cloudflare-ddns-agent.exe run` como **SYSTEM**: sobe no boot sem precisar de ninguém logado, checa a cada `check_interval` e tem um watchdog de 5 minutos que relança o processo se ele cair (sem abrir uma segunda instância). A config é relida a cada ciclo, então editar o `config.yaml` não exige reiniciar nada.

Parâmetros úteis: `-IntervaloSegundos 60`, `-Versao v0.2.0`, `-Reconfigurar` (regrava a config e pede o token de novo), `-BinarioLocal C:\caminho\agente.exe` (máquina sem acesso ao GitHub).

Log (só mudanças de IP e erros, mais um "unchanged" por hora como sinal de vida; limitado a ~10 MB):

```powershell
Get-Content C:\ProgramData\cloudflare-ddns-agent\agent.log -Tail 20 -Wait
Get-ScheduledTask cloudflare-ddns-agent | Get-ScheduledTaskInfo
```

Desinstalar: `& "C:\Program Files\cloudflare-ddns-agent\cloudflare-ddns-agent.exe" uninstall` (remove a tarefa; config e registros DNS ficam).

## Gerando o token da Cloudflare

**Nunca use a Global API Key** — gere um token escopado só pra isso:

1. Acesse [dash.cloudflare.com](https://dash.cloudflare.com) e faça login.
2. Clique no seu ícone de perfil (canto superior direito) → **My Profile**.
3. No menu lateral, vá em **API Tokens**.
4. Clique em **Create Token**.
5. Na lista de templates, ache **Edit zone DNS** e clique em **Use template**.
6. Em **Permissions**, confirme que está `Zone` / `DNS` / `Edit` (já vem assim pelo template — não precisa mexer).
7. Em **Zone Resources**, troque de "All zones" para **Specific zone** e selecione a zona que você vai usar (ex: `tecnologiadsl.com.br`). Isso garante que o token só enxerga esse domínio, nada mais da sua conta.
8. Clique em **Continue to summary**, confira o resumo e clique em **Create Token**.
9. Copie o token exibido (formato `cfut_...`) — ele só aparece uma vez nessa tela. Cole ele quando o `install.sh` pedir.

Se perder o token, é só repetir o processo e gerar um novo (o antigo pode ser revogado na mesma tela de **API Tokens**).

## Configuração manual (múltiplos hostnames)

Edite `/etc/cloudflare-ddns-agent/config.yaml`:

```yaml
cloudflare:
  api_token: "cfut_..."
check_interval: 30s
records:
  - zone: mx2tech.cloud
    hostname: hub.mx2tech.cloud
  - zone: mx2tech.cloud
    hostname: vpn.mx2tech.cloud
```

Depois de editar, aplique com:
```bash
sudo cloudflare-ddns-agent install
```

## Comandos

```bash
cloudflare-ddns-agent update     # roda uma checagem/atualização manual
cloudflare-ddns-agent run        # checa para sempre, a cada check_interval (é o que a tarefa do Windows roda)
cloudflare-ddns-agent install    # Linux: instala e ativa o timer do systemd | Windows: registra e inicia a tarefa agendada
cloudflare-ddns-agent uninstall  # Linux: remove o timer | Windows: remove a tarefa agendada
```

## Ver logs

```bash
journalctl -u cloudflare-ddns-agent -f
```

## Desinstalar

```bash
sudo cloudflare-ddns-agent uninstall
```
Isso remove o timer/service do systemd, mas não apaga `/etc/cloudflare-ddns-agent/config.yaml` nem os registros já criados na Cloudflare.

## Limitações

- Linux (amd64/arm64) e Windows (amd64).
- O token tem permissão de DNS na **zona inteira** (a Cloudflare não escopa token por registro): quem for administrador da máquina consegue editar qualquer registro da zona. Em máquina de terceiros, prefira uma zona dedicada só para DDNS e aponte o nome final para ela por CNAME.
- Só registros A (IPv4) — sem suporte a IPv6/AAAA.
- Só Cloudflare como provedor de DNS.

## Teste manual de fumaça

Depois de instalar numa VM limpa:

1. Rode `sudo cloudflare-ddns-agent update` e confirme que o registro aparece no painel da Cloudflare com o IP público correto da máquina.
2. No painel da Cloudflare, edite o registro manualmente para um IP qualquer diferente (ou apague o registro).
3. Espere um ciclo do timer (`check_interval` segundos) — ou force com `sudo systemctl start cloudflare-ddns-agent.service`.
4. Confirme que o registro voltou sozinho para o IP público real da máquina — esse é o comportamento auto-corretivo que o agente garante (ele nunca confia num valor "lembrado", sempre repergunta pra Cloudflare qual é o estado atual antes de decidir se precisa escrever).
