# Deploy Gratuito Da API E Do Banco

Este guia considera um ambiente gratuito ou quase gratuito para homologação, teste com ERP e primeira operação com baixo volume.

## Recomendação Principal

Use:

- API Go: Render Free Web Service.
- Banco PostgreSQL: Neon Free.

Motivo: o Render hospeda a API Go gratuitamente, mas o Postgres gratuito do Render expira depois de 30 dias. Para manter dados por mais tempo sem pagar, Neon é melhor para o banco.

Referências oficiais:

- Render Free: https://render.com/docs/free
- Neon Pricing: https://neon.com/pricing
- Supabase Pricing: https://supabase.com/pricing

## Limites Importantes

Render Free Web Service:

- Tem cota mensal de horas grátis.
- Pode "dormir" quando fica sem tráfego e demorar na primeira chamada depois de inativo.
- Não deve ser tratado como ambiente crítico de produção.

Render Free Postgres:

- 1 GB.
- Expira depois de 30 dias.
- Sem backups gerenciados no plano gratuito.

Neon Free:

- PostgreSQL serverless.
- Plano gratuito sem cartão, conforme página oficial.
- 0,5 GB de storage por projeto e cota mensal de compute.
- Pode pausar quando inativo e acordar na próxima conexão.

Supabase Free:

- PostgreSQL gerenciado.
- 500 MB de database size por projeto.
- Pode pausar depois de inatividade.

## Passo A Passo Com Neon + Render

### 1. Criar Banco No Neon

1. Crie uma conta em https://neon.com.
2. Crie um projeto PostgreSQL.
3. Copie a connection string.
4. Garanta que a URL tenha SSL habilitado:

```txt
postgresql://usuario:senha@host/dbname?sslmode=require
```

### 2. Rodar Migrations

No seu computador, com `DATABASE_URL` apontando para o Neon:

```bash
psql "$DATABASE_URL" -f migrations/001_init.sql
psql "$DATABASE_URL" -f migrations/002_auth_audit_idempotency.sql
```

Se você ainda não tiver `psql`, instale o cliente PostgreSQL ou use um console SQL do próprio provedor e execute o conteúdo dos arquivos em ordem.

### 3. Subir API No Render

1. Suba este repositório em um GitHub privado ou público.
2. No Render, crie um `Web Service`.
3. Conecte o repositório.
4. Configure:

```txt
Runtime: Go
Build Command: go build -o app ./cmd/api
Start Command: ./app
Health Check Path: /health
```

5. Configure variáveis de ambiente:

```txt
DATABASE_URL=postgresql://usuario:senha@host/dbname?sslmode=require
ADMIN_API_KEY=gere-uma-chave-admin-forte
ERP_API_KEY=gere-uma-chave-erp-inicial-ou-desative-depois
```

Não precisa configurar `HTTP_ADDR` no Render. A API usa `PORT` automaticamente quando a plataforma fornece essa variável.

### 4. Criar Chaves Operacionais

Depois do deploy, crie uma chave para cada integração:

```bash
curl -X POST https://sua-api.onrender.com/v1/admin/api-keys \
  -H "X-API-Key: gere-uma-chave-admin-forte" \
  -H "Content-Type: application/json" \
  -d '{"nome":"ERP produção","papel":"erp"}'
```

Guarde o valor de `api_key`. Ele só aparece uma vez.

Depois disso:

- entregue ao ERP apenas a chave `erp`;
- mantenha a chave `admin` com poucas pessoas;
- revogue chaves antigas pela rota `/v1/admin/api-keys/{id}/revogar`.

### 5. Testar Deploy

```bash
curl https://sua-api.onrender.com/health
curl https://sua-api.onrender.com/docs
```

Na documentação interativa, troque a Base URL para a URL pública do Render e informe a API key.

## Alternativa Com Supabase

Também funciona usar Supabase apenas como PostgreSQL:

1. Crie projeto em https://supabase.com.
2. Copie a connection string Postgres.
3. Use `sslmode=require`.
4. Rode as migrations.
5. Configure essa `DATABASE_URL` no Render.

O Supabase Free pode ser mais simples se você quiser painel SQL e inspeção visual do banco. O Neon tende a ser mais confortável para Postgres serverless pequeno e persistente.

## Quando Sair Do Gratuito

Considere plano pago quando:

- a API precisar ficar sempre acordada;
- o ERP tiver SLA real;
- precisar de backups automáticos;
- o banco passar de 500 MB;
- a geração anual de tabela precisar rodar sem risco de timeout;
- o orçamento virar operação crítica.

Para produção real, eu recomendo API paga pequena + Postgres com backup automático.
