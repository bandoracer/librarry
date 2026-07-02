-- M6.3 import lists: Hardcover-native list sync with auto-add options plus a
-- shared exclusions table (also honored by the Readarr-compat sync command).
create table if not exists import_lists (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  type text not null default 'hardcover' check (type in ('hardcover')),
  enabled boolean not null default true,
  settings jsonb not null default '{}',
  monitor text not null default 'all' check (monitor in ('all', 'none')),
  quality_profile text not null default 'standard',
  root_folder_id uuid references root_folders(id) on delete set null,
  search_on_add boolean not null default false,
  last_synced_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists import_list_exclusions (
  id uuid primary key default gen_random_uuid(),
  title text not null default '',
  author_name text not null default '',
  source_key text not null unique,
  created_at timestamptz not null default now()
);
