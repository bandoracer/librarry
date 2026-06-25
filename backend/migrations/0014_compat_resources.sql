create table if not exists compat_resources (
  id uuid primary key default gen_random_uuid(),
  resource_type text not null,
  compat_id integer not null,
  name text not null default '',
  payload jsonb not null default '{}'::jsonb,
  deleted_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (resource_type, compat_id)
);

create index if not exists compat_resources_type_idx on compat_resources(resource_type)
  where deleted_at is null;
