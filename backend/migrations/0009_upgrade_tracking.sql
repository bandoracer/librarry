alter table wanted_items add column if not exists current_release_id uuid references releases(id) on delete set null;
alter table wanted_items add column if not exists current_release_score numeric(6,3) not null default 0;
alter table wanted_items add column if not exists last_upgrade_search_at timestamptz;

create table if not exists upgrade_runs (
  id uuid primary key default gen_random_uuid(),
  trigger text not null default 'manual',
  status text not null default 'running',
  wanted_checked integer not null default 0,
  releases_found integer not null default 0,
  upgrade_count integer not null default 0,
  grabbed_count integer not null default 0,
  error_count integer not null default 0,
  message text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz
);

create index if not exists wanted_items_last_upgrade_search_at_idx on wanted_items(last_upgrade_search_at desc);
create index if not exists upgrade_runs_started_at_idx on upgrade_runs(started_at desc);
