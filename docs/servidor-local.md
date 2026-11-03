# Servidor Local 24/7 Sem Abrir Porta No Roteador

Este cenário funciona bem quando você quer manter a API e o PostgreSQL em uma máquina própria, mas o roteador não permite redirecionar portas.

## Arquitetura Recomendada

Para o fluxo stateless, sem banco local:

```txt
ERP externo
  -> URL pública HTTPS do túnel
  -> servidor local
  -> API Go em 127.0.0.1:8080

API Admin
  -> gera tabela JSON anual
  -> ERP salva o arquivo

API ERP
  -> recebe integrantes + tabela JSON
  -> calcula orçamento
```

Para o fluxo stateful, com banco local:

```txt
ERP externo
  -> URL pública HTTPS do túnel
  -> Cloudflare Tunnel ou Tailscale Funnel
  -> servidor local
  -> API Go em 127.0.0.1:8080
  -> PostgreSQL local em 127.0.0.1:5432
```

O banco não precisa ser exposto para a internet. Apenas a API recebe tráfego externo.

Minha recomendação atual para baixo volume é começar stateless. O PostgreSQL local fica opcional para histórico, auditoria longa e gestão de chaves em banco.

## Opção Principal: Cloudflare Tunnel

Use quando você tiver ou puder usar um domínio próprio.

Vantagens:

- não precisa abrir porta no roteador;
- fornece URL HTTPS com seu domínio;
- permite proteger a origem e esconder o IP local;
- é adequado para deixar rodando como serviço 24/7.

Fluxo:

1. A API roda localmente em `127.0.0.1:8080`.
2. O `cloudflared` cria conexões de saída para a Cloudflare.
3. A Cloudflare publica `https://api.seudominio.com`.
4. O ERP chama essa URL.

Referência oficial: https://developers.cloudflare.com/tunnel/

## Alternativa: Tailscale Funnel

Use quando você quer expor rápido sem domínio próprio, aceitando usar uma URL do Tailscale.

Vantagens:

- não precisa abrir porta;
- cria uma URL pública HTTPS;
- simples para testes e integrações pequenas.

Pontos de atenção:

- o Funnel tem limitações de porta e banda;
- a URL fica no domínio do seu tailnet;
- para acesso privado entre seus próprios dispositivos, Tailscale Serve/VPN costuma ser melhor que Funnel.

Referência oficial: https://tailscale.com/docs/features/tailscale-funnel

Para o Funnel voltar sozinho depois de reiniciar, use `--bg`. Sem `--bg`, o comando fica em primeiro plano e precisa ser executado novamente depois de reboot ou restart do Tailscale.

Helper do projeto:

```bash
sudo ./scripts/install-tailscale-funnel.sh
```

Esse helper:

- habilita `tailscaled` no boot;
- testa `http://localhost:8080/health`;
- ativa `tailscale funnel --bg --https=443 localhost:8080`;
- mostra o status do Funnel.

Comandos manuais equivalentes:

```bash
sudo systemctl enable --now tailscaled
sudo tailscale funnel --bg --https=443 localhost:8080
tailscale funnel status
```

O endereço público fica no domínio do seu tailnet, algo como:

```txt
https://nome-do-dispositivo.nome-tailnet.ts.net
```

Esse nome tende a ser estável enquanto o dispositivo mantém o mesmo nome no Tailscale. Se você renomear o dispositivo ou trocar de tailnet, a URL muda.

Limitações oficiais importantes do Funnel:

- exige Tailscale 1.38.3 ou superior;
- exige MagicDNS e HTTPS habilitados no tailnet;
- só publica nas portas `443`, `8443` ou `10000`;
- é público para a internet;
- está sujeito a limites de banda do Tailscale.

## Alternativa Para Teste: ngrok

Use para homologação rápida, demonstração ou teste com o ERP.

Vantagens:

- muito simples para subir;
- gera uma URL pública apontando para sua porta local.

Pontos de atenção:

- no plano gratuito, a URL e limites podem não ser ideais para produção;
- é melhor como ferramenta de teste do que como operação 24/7 definitiva.

Referência oficial: https://ngrok.com/docs/agent/cli

## Subir PostgreSQL Local

Instale PostgreSQL diretamente no servidor ou rode por Docker.

Exemplo com Docker Compose incluso no projeto:

```bash
docker compose -f docker-compose.postgres.yml up -d
```

Connection string padrão desse compose:

```txt
DATABASE_URL=postgres://pagliaro:troque-esta-senha@127.0.0.1:5432/pagliaro_precificador?sslmode=disable
```

Exemplo manual com Docker:

```bash
docker volume create pagliaro_pgdata

docker run -d \
  --name pagliaro-postgres \
  --restart unless-stopped \
  -e POSTGRES_DB=pagliaro_precificador \
  -e POSTGRES_USER=pagliaro \
  -e POSTGRES_PASSWORD=troque-esta-senha \
  -p 127.0.0.1:5432:5432 \
  -v pagliaro_pgdata:/var/lib/postgresql/data \
  postgres:16
```

Connection string:

```txt
DATABASE_URL=postgres://pagliaro:troque-esta-senha@127.0.0.1:5432/pagliaro_precificador?sslmode=disable
```

Rode as migrations:

```bash
psql "$DATABASE_URL" -f migrations/001_init.sql
psql "$DATABASE_URL" -f migrations/002_auth_audit_idempotency.sql
```

## Subir API Local Manualmente

Configure `.env`:

```txt
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgres://pagliaro:troque-esta-senha@127.0.0.1:5432/pagliaro_precificador?sslmode=disable
ADMIN_API_KEY=gere-uma-chave-admin-forte
ERP_API_KEY=gere-uma-chave-erp-inicial
```

Compile:

```bash
go build -o pagliaro-api ./cmd/api
```

Rode:

```bash
./pagliaro-api
```

Teste:

```bash
curl http://127.0.0.1:8080/health
```

## Rodar API 24/7 Com systemd

O helper do projeto compila a API, instala o binário em `/opt/pagliaro-precificador`, cria `/etc/pagliaro-precificador.env`, cadastra o serviço e habilita início automático no boot:

```bash
sudo ./scripts/install-systemd-service.sh
```

Depois revise:

```bash
sudo nano /etc/pagliaro-precificador.env
sudo systemctl restart pagliaro-api
```

Comandos úteis:

```bash
sudo systemctl status pagliaro-api
sudo journalctl -u pagliaro-api -f
sudo systemctl restart pagliaro-api
```

O passo manual equivalente fica abaixo, caso você prefira controlar tudo na mão.

Crie um usuário de serviço:

```bash
sudo useradd --system --home /opt/pagliaro --shell /usr/sbin/nologin pagliaro
```

Crie diretórios:

```bash
sudo mkdir -p /opt/pagliaro/app
sudo chown -R pagliaro:pagliaro /opt/pagliaro
```

Copie o binário compilado para:

```txt
/opt/pagliaro/app/pagliaro-api
```

Crie o arquivo:

```txt
/opt/pagliaro/app/.env
```

Crie o serviço:

```ini
[Unit]
Description=Pagliaro Precificador API
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
User=pagliaro
Group=pagliaro
WorkingDirectory=/opt/pagliaro/app
EnvironmentFile=/opt/pagliaro/app/.env
ExecStart=/opt/pagliaro/app/pagliaro-api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Salve em:

```txt
/etc/systemd/system/pagliaro-api.service
```

Ative:

```bash
sudo systemctl daemon-reload
sudo systemctl enable pagliaro-api
sudo systemctl start pagliaro-api
```

Ver logs:

```bash
sudo journalctl -u pagliaro-api -f
```

## Publicar Sem Abrir Porta Com Cloudflare Tunnel

Exemplo conceitual:

```bash
cloudflared tunnel login
cloudflared tunnel create pagliaro-api
cloudflared tunnel route dns pagliaro-api api.seudominio.com
```

Configure o túnel para apontar para a API local:

```yaml
tunnel: pagliaro-api
credentials-file: /etc/cloudflared/pagliaro-api.json

ingress:
  - hostname: api.seudominio.com
    service: http://127.0.0.1:8080
  - service: http_status:404
```

Instale como serviço:

```bash
sudo cloudflared service install
sudo systemctl enable cloudflared
sudo systemctl start cloudflared
```

Depois, teste:

```bash
curl https://api.seudominio.com/health
```

## Segurança Mínima

- Não exponha o PostgreSQL.
- Deixe `HTTP_ADDR=127.0.0.1:8080` quando usar túnel no mesmo servidor.
- Entregue ao ERP apenas uma API key com papel `erp`.
- Use uma API key diferente para cada ERP, parceiro ou ambiente.
- Revogue chaves antigas em `/v1/admin/api-keys/{id}/revogar`.
- Faça backup do PostgreSQL.

Backup simples:

```bash
pg_dump "$DATABASE_URL" > backup-pagliaro.sql
```

## Diagnóstico Depois De Reiniciar

Se o computador caiu durante a noite e a API não voltou, rode:

```bash
./scripts/diagnose-local-server.sh
```

Ele verifica:

- serviço `pagliaro-api`;
- health local da API;
- container do PostgreSQL;
- serviço `tailscaled`;
- status do Tailscale;
- status do Funnel;
- unidades systemd com nome Pagliaro.

Também dá para conferir manualmente:

```bash
sudo systemctl status pagliaro-api
sudo journalctl -u pagliaro-api -n 100 --no-pager
sudo systemctl status tailscaled
tailscale funnel status
tailscale serve get-config --all
```

## Corrigir Serviço Com Nome Errado

Se apareceu algo como `pagliaro"` ou `pagliaro\`` no systemd, provavelmente algum serviço foi criado com nome digitado/escapado errado.

Liste as unidades:

```bash
systemctl list-unit-files | grep -i pagliaro
```

Veja o arquivo da unidade suspeita:

```bash
systemctl cat 'nome-exato.service'
```

Depois de confirmar que é uma unidade errada, desative e remova:

```bash
sudo systemctl disable --now 'nome-exato.service'
sudo rm '/etc/systemd/system/nome-exato.service'
sudo systemctl daemon-reload
```

Em seguida instale novamente pelo helper:

```bash
sudo ./scripts/install-systemd-service.sh
sudo ./scripts/install-tailscale-funnel.sh
```

## Minha Recomendação

Para operação local 24/7 sem porta aberta:

1. PostgreSQL local em Docker ou instalado no Linux.
2. API Go rodando com `systemd`.
3. Cloudflare Tunnel publicando `https://api.seudominio.com`.
4. Banco acessível apenas via `127.0.0.1`.
5. Backups automáticos diários do PostgreSQL.

## Sobre Usar Um Tablet

Depende do sistema do tablet.

### Tablet Com Linux

Se o tablet roda Linux comum com `systemd`, ele pode funcionar bem para esse volume baixo de chamadas, algo como 50 requisições por dia.

Nesse caso, use exatamente o caminho recomendado:

```txt
PostgreSQL local
API como serviço systemd
Cloudflare Tunnel como serviço
tablet sempre carregando
backup automático
```

Cuidados:

- desativar suspensão automática;
- manter Wi-Fi estável;
- configurar reinício automático após queda de energia;
- usar armazenamento confiável, porque PostgreSQL escreve em disco;
- monitorar temperatura se ele ficar sempre carregando.

### Tablet Android

Eu não recomendo Android como produção 24/7 para API + PostgreSQL.

Dá para fazer testes com Termux, Termux:Boot e Termux services. A documentação do Termux:Boot orienta criar scripts em `~/.termux/boot/` e usar `termux-wake-lock` para tentar manter o processo acordado. Também existe `termux-services` para serviços com runit.

Referências:

- https://github.com/termux/termux-boot
- https://github.com/termux/termux-services

O problema é confiabilidade: Android pode matar processos em segundo plano, aplicar economia agressiva de bateria, limitar inicialização automática, atualizar apps em momentos ruins e tornar PostgreSQL local mais frágil.

Para homologação, Android pode servir. Para uma API que o ERP depende, prefira Linux.

## Melhor Custo-Benefício

Para esse volume baixo, eu escolheria nesta ordem:

1. mini PC Linux usado ou barato;
2. notebook velho com bateria boa;
3. tablet com Linux;
4. tablet Android apenas para teste/homologação.

O consumo de energia de um mini PC ou notebook simples costuma ser baixo, e você ganha `systemd`, Docker, backup e manutenção muito mais previsíveis.
