# cloudflare-ddns-agent

Mantém um ou mais registros DNS tipo A na Cloudflare sempre apontando para o IP público atual da máquina onde roda. Feito para servidores com IP dinâmico (conexão residencial/DSL, sem IP fixo) que precisam de um hostname estável.

## Como funciona

A cada execução, o agente:
1. Descobre o IP público atual da máquina.
2. Consulta na Cloudflare qual é o valor atual de cada hostname configurado (nunca confia em cache local).
3. Cria o registro se não existir, atualiza se o IP mudou, ou não faz nada se já está correto.

Isso roda em loop via `systemd timer` (instalado automaticamente), não como processo contínuo.

## Instalação

Como root, numa máquina Linux (amd64 ou arm64):

```bash
curl -fsSL https://raw.githubusercontent.com/MX2Tech/cloudflare-ddns-agent/main/install.sh | sudo bash
```

Vai pedir 4 informações:
- **Cloudflare API Token** — veja abaixo como gerar.
- **Zona** — o domínio raiz na Cloudflare (ex: `tecnologiadsl.com.br`).
- **Hostname** — o registro a manter atualizado (ex: `hub.tecnologiadsl.com.br`).
- **Intervalo** — de quanto em quanto tempo checar (padrão 30 segundos).

O instalador testa a configuração na hora e já mostra se funcionou.

## Gerando o token da Cloudflare

Em [dash.cloudflare.com](https://dash.cloudflare.com) → My Profile → API Tokens → Create Token → template "Edit zone DNS". Restrinja à zona específica que você vai usar (não use a Global API Key).

## Configuração manual (múltiplos hostnames)

Edite `/etc/cloudflare-ddns-agent/config.yaml`:

```yaml
cloudflare:
  api_token: "cfut_..."
check_interval: 30s
records:
  - zone: tecnologiadsl.com.br
    hostname: hub.tecnologiadsl.com.br
  - zone: tecnologiadsl.com.br
    hostname: vpn.tecnologiadsl.com.br
```

Depois de editar, aplique com:
```bash
sudo cloudflare-ddns-agent install
```

## Comandos

```bash
cloudflare-ddns-agent update     # roda uma checagem/atualização manual
cloudflare-ddns-agent install    # instala e ativa o timer do systemd
cloudflare-ddns-agent uninstall  # remove o timer do systemd
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

- Só Linux (amd64/arm64).
- Só registros A (IPv4) — sem suporte a IPv6/AAAA.
- Só Cloudflare como provedor de DNS.

## Teste manual de fumaça

Depois de instalar numa VM limpa:

1. Rode `sudo cloudflare-ddns-agent update` e confirme que o registro aparece no painel da Cloudflare com o IP público correto da máquina.
2. No painel da Cloudflare, edite o registro manualmente para um IP qualquer diferente (ou apague o registro).
3. Espere um ciclo do timer (`check_interval` segundos) — ou force com `sudo systemctl start cloudflare-ddns-agent.service`.
4. Confirme que o registro voltou sozinho para o IP público real da máquina — esse é o comportamento auto-corretivo que o agente garante (ele nunca confia num valor "lembrado", sempre repergunta pra Cloudflare qual é o estado atual antes de decidir se precisa escrever).
