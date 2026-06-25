create table if not exists feed_sync_runs (
  id uuid primary key default gen_random_uuid(),
  trigger text not null default 'manual',
  status text not null default 'running',
  releases_seen integer not null default 0,
  matched_count integer not null default 0,
  approved_count integer not null default 0,
  rejected_count integer not null default 0,
  grabbed_count integer not null default 0,
  error_count integer not null default 0,
  message text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz
);

create table if not exists feed_releases (
  id uuid primary key default gen_random_uuid(),
  source_id text not null default '',
  info_hash text,
  indexer text not null default '',
  title text not null,
  protocol text not null default '',
  download_url text not null default '',
  info_url text,
  size_bytes bigint,
  seeders integer not null default 0,
  leechers integer not null default 0,
  categories text not null default '',
  published_at timestamptz,
  first_seen_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now()
);

create unique index if not exists feed_releases_source_unique_idx
  on feed_releases(source_id)
  where source_id <> '';

create index if not exists feed_releases_last_seen_at_idx on feed_releases(last_seen_at desc);
create index if not exists feed_sync_runs_started_at_idx on feed_sync_runs(started_at desc);
