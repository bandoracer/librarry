create table kindle_settings (
  singleton boolean primary key default true check (singleton),
  settings jsonb not null,
  updated_at timestamptz not null default now()
);
-- Snapshot identities survive book/file removal. No credentials or file bytes here.
create table kindle_deliveries (
  id uuid primary key default gen_random_uuid(),
  request_id text not null unique,
  wanted_id text not null,
  file_id text not null,
  recipient text not null,
  sender text not null,
  title text not null,
  state text not null check (state in ('sending','accepted','failed','unknown')),
  message text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index kindle_deliveries_book on kindle_deliveries(wanted_id,created_at desc,id);
-- Serialize sends for the same file/recipient even across API instances.
create unique index kindle_delivery_active on kindle_deliveries(file_id,recipient) where state='sending';
