alter table quote_results
    add column if not exists request_hash text not null default '';

do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'quote_results_erp_quote_id_key'
    ) then
        alter table quote_results add constraint quote_results_erp_quote_id_key unique (erp_quote_id);
    end if;
end $$;

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
