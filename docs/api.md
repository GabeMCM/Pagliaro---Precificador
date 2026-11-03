# Pagliaro Precificador API

Um único serviço HTTP expõe duas áreas:

- `/v1/admin/tabelas`: geração anual da tabela base a partir de dados oficiais do IBGE.
- `/v1/erp/orcamentos`: orçamento comercial consumido pelo ERP.

A documentação interativa da API Go é Swagger UI gerada com Swaggo:

```txt
GET /docs
```

Ela permite testar as rotas no navegador informando a API key.

O contrato Swagger/OpenAPI gerado fica em:

```txt
GET /openapi.json
```

Para regenerar os arquivos `docs/docs.go`, `docs/swagger.json` e `docs/swagger.yaml`:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go --parseInternal --parseDependency -o docs
```

## Configuração

A aplicação carrega `.env` automaticamente na inicialização.

```bash
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5432/pagliaro_precificador?sslmode=disable
ADMIN_API_KEY=troque-esta-chave-admin
ERP_API_KEY=troque-esta-chave-erp
TABLE_SIGNATURE_SECRET=troque-este-segredo-para-assinar-tabelas-json
```

Se `HTTP_ADDR` não estiver definido e a plataforma expuser `PORT`, a API escuta automaticamente em `0.0.0.0:$PORT`.

`TABLE_SIGNATURE_SECRET` é opcional. Quando informado, a tabela JSON gerada recebe `assinatura`, e a rota stateless de orçamento exige que essa assinatura esteja válida.

## Autenticação

Envie a chave em `X-API-Key` ou `Authorization: Bearer`.

- `ADMIN_API_KEY`: pode chamar rotas `/v1/admin/*` e `/v1/erp/*`.
- `ERP_API_KEY`: pode chamar apenas rotas `/v1/erp/*`.

As chaves do `.env` são bootstrap. Para produção, use a chave admin bootstrap apenas para criar/revogar chaves gerenciadas pela API.

## Gestão De API Keys

`POST /v1/admin/api-keys`

```json
{
  "nome": "ERP principal",
  "papel": "erp"
}
```

`papel` aceita `admin` ou `erp`.

Resposta:

```json
{
  "registro": {
    "id": "key_...",
    "nome": "ERP principal",
    "papel": "erp",
    "prefixo": "pag_erp_...",
    "ativa": true,
    "criada_em": "2026-08-27T..."
  },
  "api_key": "pag_erp_..."
}
```

O campo `api_key` só aparece na criação. Depois disso, a API só guarda o hash.

Outras rotas:

```txt
GET  /v1/admin/api-keys
POST /v1/admin/api-keys/{id}/revogar
```

## Logs De Requisições

`GET /v1/admin/logs/requisicoes?limite=100`

Retorna as chamadas mais recentes com `request_id`, método, rota, status HTTP, duração, chave usada e papel. O limite padrão é 100 e o máximo aceito é 500.

```json
{
  "items": [
    {
      "id": "log_...",
      "request_id": "req_...",
      "method": "POST",
      "path": "/v1/erp/orcamentos",
      "status": 201,
      "duration_ms": 12,
      "api_key_id": "key_...",
      "role": "erp",
      "remote_addr": "127.0.0.1:50000",
      "created_at": "2026-08-27T..."
    }
  ]
}
```

## Gerar Tabela

`POST /v1/admin/tabelas/gerar`

```json
{
  "ano_calculo": 2026,
  "revisao_populacional": 2024,
  "ano_tabua_mortalidade": 2024,
  "versao": "CALC-2026-REVPOP-2024-TABUA-2024-v1",
  "planos": [
    { "nome": "Essencial", "cobertura": 4000 },
    { "nome": "Familiar", "cobertura": 7000 },
    { "nome": "Premium", "cobertura": 10000 }
  ],
  "custos": {
    "gasto_fixo_mensal": 50000,
    "clientes_totais": 3000,
    "clientes_inadimplentes": 500,
    "taxa_bancaria": 3
  },
  "margens": {
    "lucro": 0.2,
    "comissao": 0.1
  },
  "mutualidade": {
    "metodo": "populacional",
    "fator": 0.3
  }
}
```

Métodos de mutualidade:

- `simples`: média simples dos custos atuariais das faixas.
- `populacional`: média ponderada pela participação populacional da faixa.
- `mortalidade`: média ponderada pela participação da faixa nas mortes esperadas.

Fluxo:

```txt
gerar -> draft
aprovar -> approved
ativar -> active
```

## Gerar Tabela JSON

`POST /v1/admin/tabelas/gerar-json`

Usa o mesmo payload de `gerar`, baixa os dados do IBGE, calcula os valores e retorna um JSON autocontido para o ERP salvar. Não grava tabela no banco.

Headers de resposta:

```txt
Content-Type: application/json; charset=utf-8
Content-Disposition: attachment; filename="tabela-precos-CALC-2026-REVPOP-2024-LATEST-v1.json"
```

Estrutura resumida:

```json
{
  "tipo": "pagliaro_tabela_precos",
  "schema_version": "1.0",
  "versao": "CALC-2026-REVPOP-2024-LATEST-v1",
  "gerada_em": "2026-08-27T15:00:00Z",
  "ibge": {
    "ano_calculo": 2026,
    "revisao_populacional": 2024,
    "ano_tabua_mortalidade": 2024,
    "population_url": "https://...",
    "mortality_url": "https://...",
    "population_sha256": "...",
    "mortality_sha256": "..."
  },
  "parametros": {},
  "resultado": {
    "version": "CALC-2026-REVPOP-2024-LATEST-v1",
    "rows": []
  },
  "qx_mortalidade_por_idade": {
    "0": 0.008,
    "1": 0.0005
  },
  "hash": "...",
  "assinatura": "..."
}
```

O `hash` detecta alteração no conteúdo do JSON. A `assinatura` usa HMAC-SHA256 com `TABLE_SIGNATURE_SECRET`; se esse segredo estiver configurado na API, a rota de orçamento stateless recusa tabela sem assinatura válida.

## Simular Tabela

`POST /v1/admin/tabelas/simular`

Usa o mesmo payload de geração, mas não salva nada no banco. Serve para comparar fórmulas de mutualidade antes de gerar a versão anual oficial.

Se `cenarios` não for enviado, a API testa:

```json
[
  { "metodo": "simples", "fator": 0.3 },
  { "metodo": "populacional", "fator": 0.3 },
  { "metodo": "mortalidade", "fator": 0.3 },
  { "metodo": "populacional", "fator": 0.35 },
  { "metodo": "mortalidade", "fator": 0.35 }
]
```

Também é possível enviar cenários específicos:

```json
{
  "ano_calculo": 2026,
  "revisao_populacional": 2024,
  "versao": "SIMULACAO-2026",
  "planos": [{ "nome": "Familiar", "cobertura": 7000 }],
  "custos": {
    "gasto_fixo_mensal": 50000,
    "clientes_totais": 3000,
    "clientes_inadimplentes": 500,
    "taxa_bancaria": 3
  },
  "margens": { "lucro": 0.2, "comissao": 0.1 },
  "cenarios": [
    { "metodo": "simples", "fator": 0.3 },
    { "metodo": "populacional", "fator": 0.35 }
  ]
}
```

## Aprovar E Ativar

```txt
GET  /v1/admin/tabelas/ativa
POST /v1/admin/tabelas/{id}/aprovar
POST /v1/admin/tabelas/{id}/ativar
POST /v1/admin/tabelas/{id}/arquivar
```

Ao ativar uma tabela, a tabela ativa anterior é arquivada.
Somente tabelas `draft` podem ser aprovadas. Somente tabelas `approved` podem ser ativadas.

## Gerar Orçamento

`POST /v1/erp/orcamentos`

```json
{
  "erp_orcamento_id": "ERP-123",
  "nome_plano": "Familiar",
  "cobertura": 7000,
  "formula": "comparativo",
  "integrantes": [
    {
      "erp_pessoa_id": "P1",
      "data_nascimento": "1989-04-10",
      "parentesco": "titular"
    },
    {
      "erp_pessoa_id": "P2",
      "idade": 64,
      "parentesco": "pai"
    }
  ]
}
```

`formula` aceita:

- `C`: padrão comercial.
- `A`: alternativa conservadora.
- `comparativo`: retorna A e C, usando C como valor final recomendado.

`erp_orcamento_id` é idempotente. Se o ERP reenviar o mesmo identificador, a API retorna o orçamento já salvo.

Se o ERP reenviar o mesmo `erp_orcamento_id` com payload diferente, a API responde:

```json
{
  "error": {
    "code": "IDEMPOTENCY_CONFLICT",
    "message": "erp_orcamento_id já existe com payload diferente",
    "details": []
  }
}
```

com HTTP `409`.

## Calcular Orçamento Com Tabela JSON

`POST /v1/erp/orcamentos/calcular`

Não usa banco. O ERP envia a tabela JSON gerada em `/v1/admin/tabelas/gerar-json` dentro do campo `tabela`.

```json
{
  "tabela": {
    "tipo": "pagliaro_tabela_precos",
    "schema_version": "1.0",
    "versao": "CALC-2026-REVPOP-2024-LATEST-v1",
    "resultado": {},
    "qx_mortalidade_por_idade": {},
    "hash": "...",
    "assinatura": "..."
  },
  "erp_orcamento_id": "ERP-123",
  "nome_plano": "Familiar",
  "cobertura": 7000,
  "formula": "comparativo",
  "integrantes": [
    {
      "erp_pessoa_id": "P1",
      "data_nascimento": "1989-04-10",
      "parentesco": "titular"
    },
    {
      "erp_pessoa_id": "P2",
      "idade": 64,
      "parentesco": "pai"
    }
  ]
}
```

Resposta: mesma estrutura de resultado de orçamento da rota com banco.

Neste fluxo, `erp_orcamento_id` continua obrigatório para rastreabilidade, mas a API não consegue garantir idempotência global porque não salva histórico. Essa garantia passa a ser responsabilidade do ERP.

## IBGE

O contrato separa três conceitos:

```txt
ano_calculo
  Ano usado para selecionar a coluna de população dentro da projeção.

revisao_populacional
  Revisão da Projeção da População do IBGE. O padrão atual é 2024.

ano_tabua_mortalidade
  Ano da Tábua Completa de Mortalidade. Se omitido, a API consulta o diretório oficial do IBGE e usa a última tábua anual disponível até o ano_calculo.
```

Por padrão, a API baixa:

- população por idade simples da Projeção da População, revisão 2024;
- tábua completa de mortalidade anual, ambos os sexos, da última publicação disponível ou do ano informado.

O campo antigo `ano_ibge` ainda é aceito como compatibilidade e vira `ano_calculo`, mas o novo contrato deve usar os três campos acima.

Para teste ou contingência, é possível sobrescrever URLs:

```json
{
  "ibge": {
    "population_url": "https://...",
    "mortality_url": "https://..."
  }
}
```

A tabela gerada salva metadados da importação:

- ano de cálculo;
- revisão populacional;
- ano da tábua de mortalidade efetivamente usada;
- URL de população;
- URL de mortalidade;
- SHA-256 de cada arquivo;
- quantidade de idades importadas em cada fonte.

## Erros

Formato padrão:

```json
{
  "error": {
    "code": "QUOTE_VALIDATION_ERROR",
    "message": "mensagem do erro",
    "details": []
  }
}
```

Códigos HTTP mais comuns:

- `400`: JSON inválido ou query inválida.
- `401`: API key ausente.
- `403`: API key sem permissão.
- `404`: registro não encontrado.
- `409`: conflito de idempotência.
- `422`: erro de validação de negócio.
- `502`: falha ao importar dados do IBGE.
