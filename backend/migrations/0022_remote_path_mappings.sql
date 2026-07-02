create table if not exists remote_path_mappings (
  id uuid primary key default gen_random_uuid(),
  host text not null default '',
  remote_prefix text not null,
  local_prefix text not null,
  created_at timestamptz not null default now()
);
