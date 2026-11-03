# Pagliaro Precificador

API em Go para precificação de planos funerários familiares.

Também existe uma versão Node.js/TypeScript em [node-api](node-api/README.md), criada para o fluxo stateless sem banco obrigatório.

## Ambiente

```bash
nix develop
```

## Configuração

A API carrega automaticamente o arquivo `.env` na raiz do projeto. Variáveis já exportadas no terminal têm prioridade.

Arquivo local criado:

```bash
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=
ADMIN_API_KEY=dev-admin-key
ERP_API_KEY=dev-erp-key
TABLE_SIGNATURE_SECRET=dev-table-signature-secret
```

Use `.env.example` como referência para configurar PostgreSQL e chaves reais.

Em provedores de hospedagem que expõem `PORT`, a API usa `0.0.0.0:$PORT` automaticamente quando `HTTP_ADDR` não estiver definido.

## Banco

Execute a migration em um PostgreSQL:

```bash
psql "$DATABASE_URL" -f migrations/001_init.sql
psql "$DATABASE_URL" -f migrations/002_auth_audit_idempotency.sql
```

Se `DATABASE_URL` não estiver configurada, a API sobe com storage em memória para desenvolvimento.

Para operar sem banco, use o fluxo stateless:

```txt
POST /v1/admin/tabelas/gerar-json
  -> baixa IBGE, calcula tabela e retorna um JSON autocontido

POST /v1/erp/orcamentos/calcular
  -> recebe esse JSON no payload e calcula o orçamento
```

Nesse modelo, o ERP guarda o arquivo JSON da tabela vigente e a API não precisa salvar tabelas nem orçamentos.

Para subir PostgreSQL local com Docker Compose:

```bash
docker compose -f docker-compose.postgres.yml up -d
```

## Rodar

```bash
go run ./cmd/api
```

Sem banco:

```bash
go run ./cmd/api
```

Para instalar como serviço Linux e iniciar automaticamente depois de reiniciar o computador:

```bash
sudo ./scripts/install-systemd-service.sh
```

Para publicar via Tailscale Funnel persistente:

```bash
sudo tailscale set --operator=$USER
./scripts/install-tailscale-funnel.sh
```

Se preferir não configurar operador, execute o apontamento direto com sudo:

```bash
sudo tailscale funnel --bg --https=443 localhost:8080
```

Para diagnosticar o servidor local depois de reiniciar:

```bash
./scripts/diagnose-local-server.sh
```

## Rotas

- `GET /health`
- `GET /docs`
- `GET /openapi.json`
- `POST /v1/admin/api-keys`
- `GET /v1/admin/api-keys`
- `POST /v1/admin/api-keys/{id}/revogar`
- `GET /v1/admin/logs/requisicoes`
- `POST /v1/admin/tabelas/simular`
- `POST /v1/admin/tabelas/gerar-json`
- `POST /v1/admin/tabelas/gerar`
- `GET /v1/admin/tabelas`
- `GET /v1/admin/tabelas/ativa`
- `GET /v1/admin/tabelas/{id}`
- `POST /v1/admin/tabelas/{id}/aprovar`
- `POST /v1/admin/tabelas/{id}/ativar`
- `POST /v1/admin/tabelas/{id}/arquivar`
- `POST /v1/erp/orcamentos`
- `POST /v1/erp/orcamentos/calcular`
- `GET /v1/erp/orcamentos/{id}`

Detalhes:

- Documentação Swagger UI: `http://127.0.0.1:8080/docs`
- JSON Swagger/OpenAPI gerado: `http://127.0.0.1:8080/openapi.json`
- [Contrato da API](docs/api.md)
- [Arquitetura e Fluxos](docs/arquitetura.md)
- [Deploy gratuito de API e banco](docs/deploy-gratis.md)
- [Servidor local 24/7 sem abrir porta](docs/servidor-local.md)

## Fonte IBGE

A geração da tabela separa:

- `ano_calculo`: coluna de população usada;
- `revisao_populacional`: revisão da Projeção da População, padrão `2024`;
- `ano_tabua_mortalidade`: tábua anual de mortalidade. Se omitido, a API usa a última disponível no diretório oficial do IBGE.

## Testes

```bash
go test ./...
```

## Documentação Swagger

A documentação da API Go é gerada com Swaggo a partir das anotações nos handlers.

Para regenerar depois de alterar rotas, payloads ou respostas:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go --parseInternal --parseDependency -o docs
```

## Gestão De Chaves

As chaves fixas do `.env` servem como bootstrap. Em produção, mantenha uma chave admin de emergência no ambiente e gere chaves por integração:

```bash
curl -X POST http://127.0.0.1:8080/v1/admin/api-keys \
  -H "X-API-Key: dev-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"nome":"ERP principal","papel":"erp"}'
```

O segredo `api_key` aparece apenas na criação. A API salva somente o hash.
