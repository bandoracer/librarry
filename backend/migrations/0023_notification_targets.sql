create table if not exists notification_targets (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  type text not null check (type in ('webhook', 'ntfy', 'discord', 'telegram')),
  settings jsonb not null default '{}',
  on_grab boolean not null default true,
  on_import boolean not null default true,
  on_upgrade boolean not null default true,
  on_download_failure boolean not null default true,
  on_health_issue boolean not null default false,
  enabled boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
