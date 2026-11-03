create table if not exists ibge_data (
    year integer primary key,
    data jsonb not null,
    updated_at timestamptz not null default now()
);

create table if not exists pricing_table_versions (
    id text primary key,
    version text not null unique,
    ibge_year integer not null,
    status text not null check (status in ('draft', 'approved', 'active', 'archived')),
    request jsonb not null,
    result jsonb not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_pricing_table_versions_status
    on pricing_table_versions (status);

create table if not exists quote_results (
    id text primary key,
    erp_quote_id text not null unique,
    request_hash text not null default '',
    request jsonb not null,
    result jsonb not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_quote_results_erp_quote_id
    on quote_results (erp_quote_id);

create table if not exists api_keys (
    id text primary key,
    name text not null,
    role text not null check (role in ('admin', 'erp')),
    key_hash text not null unique,
    key_prefix text not null,
    active boolean not null default true,
    created_at timestamptz not null default now(),
    revoked_at timestamptz,
    last_used_at timestamptz
);

create index if not exists idx_api_keys_active
    on api_keys (active);

create table if not exists request_logs (
    id text primary key,
    request_id text not null,
    method text not null,
    path text not null,
    status integer not null,
    duration_ms bigint not null,
    api_key_id text,
    role text,
    remote_addr text,
    created_at timestamptz not null default now()
);

create index if not exists idx_request_logs_created_at
    on request_logs (created_at desc);
