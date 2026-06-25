create table if not exists monitor_runs (
  id uuid primary key default gen_random_uuid(),
  trigger text not null default 'manual',
  status text not null default 'running',
  wanted_checked integer not null default 0,
  releases_found integer not null default 0,
  approved_count integer not null default 0,
  rejected_count integer not null default 0,
  grabbed_count integer not null default 0,
  error_count integer not null default 0,
  message text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz
);

create table if not exists history_events (
  id uuid primary key default gen_random_uuid(),
  event_type text not null,
  entity_type text not null default '',
  entity_id text not null default '',
  severity text not null default 'info',
  message text not null,
  data jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists monitor_runs_started_at_idx on monitor_runs(started_at desc);
create index if not exists monitor_runs_status_idx on monitor_runs(status);
create index if not exists history_events_created_at_idx on history_events(created_at desc);
create index if not exists history_events_entity_idx on history_events(entity_type, entity_id);
