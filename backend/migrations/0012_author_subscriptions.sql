create table if not exists author_subscriptions (
  id uuid primary key default gen_random_uuid(),
  provider text not null default '',
  provider_key text not null default '',
  author_name text not null,
  wanted_format text not null default 'ebook' check (wanted_format in ('ebook', 'audiobook')),
  quality_profile text not null default 'standard',
  status text not null default 'monitored',
  monitor_new_items boolean not null default true,
  last_sync_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists author_subscriptions_provider_key_format_idx
  on author_subscriptions(provider, provider_key, wanted_format)
  where provider <> '' and provider_key <> '';

create index if not exists author_subscriptions_status_idx on author_subscriptions(status);
create index if not exists author_subscriptions_last_sync_at_idx on author_subscriptions(last_sync_at);

create table if not exists author_subscription_runs (
  id uuid primary key default gen_random_uuid(),
  trigger text not null default 'manual',
  status text not null default 'running',
  authors_checked integer not null default 0,
  items_found integer not null default 0,
  wanted_created integer not null default 0,
  error_count integer not null default 0,
  message text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz
);

create index if not exists author_subscription_runs_started_at_idx
  on author_subscription_runs(started_at desc);
