# Arquitetura Da API

O serviço é uma aplicação Go única com duas áreas de API, controladas por rotas e permissões de API key.

## Áreas

O serviço suporta dois modos de operação:

```txt
stateful
  usa PostgreSQL para salvar tabelas, chaves gerenciadas, logs e orçamentos.

stateless
  gera um arquivo JSON autocontido e calcula orçamento recebendo esse JSON no payload.
```

O modo stateless é o mais simples para operação local: a API não precisa guardar tabela nem orçamento; o ERP guarda a tabela vigente.

### API Admin

Prefixo:

```txt
/v1/admin
```

Responsável por gerar e controlar a tabela anual.

Fluxo:

```txt
POST /v1/admin/tabelas/gerar
  -> baixa dados oficiais do IBGE
  -> calcula tabela por plano, cobertura e faixa
  -> salva como draft

POST /v1/admin/tabelas/{id}/aprovar
  -> muda draft para approved

POST /v1/admin/tabelas/{id}/ativar
  -> muda approved para active
  -> arquiva tabela ativa anterior

POST /v1/admin/tabelas/simular
  -> baixa os mesmos dados oficiais do IBGE
  -> calcula cenários de mutualidade sem persistir tabela

POST /v1/admin/tabelas/gerar-json
  -> baixa dados oficiais do IBGE
  -> calcula tabela por plano, cobertura e faixa
  -> inclui qx por idade, metadados, hash e assinatura opcional
  -> retorna JSON autocontido sem persistir no banco

POST /v1/admin/api-keys
  -> cria uma chave admin ou erp
  -> retorna o segredo uma única vez

GET /v1/admin/logs/requisicoes
  -> lista chamadas recentes para auditoria
```

### API ERP

Prefixo:

```txt
/v1/erp
```

Responsável por gerar orçamento comercial usando a tabela ativa.

Fluxo:

```txt
POST /v1/erp/orcamentos
  -> recebe integrantes completos
  -> resolve idade por data_nascimento ou idade
  -> busca qx individual na versão IBGE da tabela ativa
  -> calcula idade média ponderada por mortalidade
  -> define faixa-base
  -> busca valor-base da tabela ativa
  -> aplica Fórmula C, A ou comparativo
  -> salva resultado idempotente por erp_orcamento_id

POST /v1/erp/orcamentos/calcular
  -> recebe tabela JSON autocontida no payload
  -> valida hash e assinatura, se configurada
  -> calcula orçamento sem buscar tabela ativa e sem salvar resultado
```

## Segurança

Chaves:

```txt
ADMIN_API_KEY acessa /v1/admin e /v1/erp
ERP_API_KEY acessa apenas /v1/erp
```

Essas chaves são bootstrap e ficam no ambiente. Chaves operacionais são criadas por `POST /v1/admin/api-keys`, salvas no banco apenas como SHA-256 e podem ser revogadas sem alterar arquivo `.env`.

No modo stateless, a tabela JSON pode ser assinada com `TABLE_SIGNATURE_SECRET`.

```txt
hash
  SHA-256 canônico do conteúdo da tabela, sem os campos hash e assinatura.

assinatura
  HMAC-SHA256 do hash usando TABLE_SIGNATURE_SECRET.
```

Se `TABLE_SIGNATURE_SECRET` estiver configurado, a rota `/v1/erp/orcamentos/calcular` exige assinatura válida. Isso impede que um preço seja alterado manualmente no JSON sem a API perceber.

Cabeçalhos aceitos:

```txt
X-API-Key: chave
Authorization: Bearer chave
```

## Persistência

Com `DATABASE_URL`, usa PostgreSQL.

Sem `DATABASE_URL`, usa memória apenas para desenvolvimento.

Tabelas:

```txt
ibge_data
pricing_table_versions
quote_results
api_keys
request_logs
```

O `erp_orcamento_id` em `quote_results` é único. Reenvio com o mesmo payload retorna o orçamento existente; reenvio com payload diferente retorna `409 IDEMPOTENCY_CONFLICT`.

`request_logs` grava método, rota, status, duração, chave autenticada, papel e endereço remoto. Falhas de gravação de log não derrubam a requisição principal.

## Cálculo Da Tabela

Para cada faixa:

```txt
qx_faixa = Σ(pop_idade × qx_idade) ÷ Σ(pop_idade)
```

Carregamento comum:

```txt
qx_0_17 = Σ(pop_idade × qx_idade) ÷ Σ(pop_idade), idade 0 a 17
```

Taxa ajustada:

```txt
qA = qx_faixa + qx_0_17
```

Custo atuarial:

```txt
CA = cobertura × qA ÷ 12
```

Mutualidade:

```txt
CA_mut = CA_faixa × (1 - fator) + CA_ref × fator
```

Métodos de `CA_ref`:

```txt
simples      = média simples dos CAs das faixas
populacional = média ponderada pela população da faixa
mortalidade  = média ponderada pelas mortes esperadas da faixa
```

Valor base:

```txt
custo_tecnico = CA_mut + custo_operacional + taxa_bancaria
valor_base = custo_tecnico × (1 + lucro + comissao)
```

Valores monetários são arredondados sempre para cima com 2 casas.

## Cálculo Do Orçamento

Idade média ponderada:

```txt
idade_media = Σ(idade_integrante × qx_integrante) ÷ Σ(qx_integrante)
```

Fórmula A:

```txt
valor = valor_base × [1 + Σ(qx_integrantes) × (n - 1)]
```

Fórmula C:

```txt
valor = valor_base × [1 + Σ(qx_adicionais) + 0.01 × (n - 1)]
```

Fórmula padrão:

```txt
C
```

## Fontes IBGE

A API separa população e mortalidade para evitar confundir projeção com tábua anual.

```txt
ano_calculo
  Define a coluna de população usada no cálculo.

revisao_populacional
  Define a revisão da Projeção da População do IBGE. O padrão atual é 2024.

ano_tabua_mortalidade
  Define a tábua completa anual. Se omitido, a API usa a última disponível no diretório oficial.
```

A API usa arquivos oficiais baixados automaticamente:

- população por idade simples da Projeção da População;
- tábua completa de mortalidade, ambos os sexos.

Cada tabela salva:

- ano de cálculo;
- revisão populacional;
- ano efetivo da tábua de mortalidade;
- URLs usadas;
- SHA-256 dos arquivos;
- quantidade de idades importadas.
