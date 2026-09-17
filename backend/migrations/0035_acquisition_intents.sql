-- A durable submission boundary shared by manual and scheduled acquisitions.
-- URLs, provider keys, credentials and torrent/NZB bodies are not stored here.
create table acquisition_intents (
    id uuid primary key default gen_random_uuid(),
    scope_key text not null check (scope_key ~ '^[0-9a-f]{64}$'),
    request_key text not null check (request_key ~ '^[0-9a-f]{64}$'),
    wanted_item_id uuid references wanted_items(id) on delete set null,
    media_format text not null default '',
    client text not null,
    endpoint_hash text not null,
    info_hash text not null default '',
    title text not null default '',
    category text not null default '',
    state text not null check (state in ('submitting','uncertain','accepted','released')),
    external_id text not null default '',
    result jsonb,
    lease_token uuid,
    lease_expires_at timestamptz,
    attempts integer not null default 1,
    last_error text not null default '',
    next_check_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create unique index acquisition_intents_active_scope on acquisition_intents(scope_key) where state <> 'released';
create index acquisition_intents_recovery on acquisition_intents(state,updated_at);
