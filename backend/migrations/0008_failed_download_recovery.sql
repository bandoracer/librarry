alter table downloads add column if not exists last_activity_at timestamptz;
alter table downloads add column if not exists failure_reason text not null default '';
alter table downloads add column if not exists failed_at timestamptz;
alter table downloads add column if not exists retry_count integer not null default 0;
alter table downloads add column if not exists replacement_external_id text not null default '';

create table if not exists failed_download_runs (
  id uuid primary key default gen_random_uuid(),
  trigger text not null default 'manual',
  status text not null default 'running',
  downloads_checked integer not null default 0,
  failed_count integer not null default 0,
  replacements_found integer not null default 0,
  grabbed_count integer not null default 0,
  removed_count integer not null default 0,
  error_count integer not null default 0,
  message text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz
);

create index if not exists downloads_failed_at_idx on downloads(failed_at desc);
create index if not exists failed_download_runs_started_at_idx on failed_download_runs(started_at desc);
