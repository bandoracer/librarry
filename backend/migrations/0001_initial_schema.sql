create extension if not exists pgcrypto;

create table if not exists authors (
  id uuid primary key default gen_random_uuid(),
  canonical_name text not null,
  sort_name text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists works (
  id uuid primary key default gen_random_uuid(),
  title text not null,
  sort_title text not null,
  first_publish_year integer,
  description text,
  cover_url text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists work_authors (
  work_id uuid not null references works(id) on delete cascade,
  author_id uuid not null references authors(id) on delete cascade,
  role text not null default 'author',
  primary key (work_id, author_id, role)
);

create table if not exists series (
  id uuid primary key default gen_random_uuid(),
  title text not null,
  sort_title text not null,
  provider_hint text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists series_items (
  series_id uuid not null references series(id) on delete cascade,
  work_id uuid not null references works(id) on delete cascade,
  position text not null,
  primary key (series_id, work_id)
);

create table if not exists editions (
  id uuid primary key default gen_random_uuid(),
  work_id uuid not null references works(id) on delete cascade,
  title text not null,
  media_format text not null check (media_format in ('ebook', 'audiobook', 'physical', 'unknown')),
  language text,
  publisher text,
  published_date text,
  asin text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists edition_identifiers (
  edition_id uuid not null references editions(id) on delete cascade,
  kind text not null,
  value text not null,
  primary key (kind, value)
);

create table if not exists provider_records (
  id uuid primary key default gen_random_uuid(),
  provider text not null,
  provider_key text not null,
  entity_type text not null,
  entity_id uuid,
  raw jsonb not null default '{}'::jsonb,
  confidence numeric(5,4) not null default 0,
  fetched_at timestamptz not null default now(),
  unique (provider, provider_key)
);

create table if not exists manual_overrides (
  id uuid primary key default gen_random_uuid(),
  entity_type text not null,
  entity_id uuid not null,
  field_name text not null,
  value jsonb not null,
  reason text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (entity_type, entity_id, field_name)
);

create table if not exists wanted_items (
  id uuid primary key default gen_random_uuid(),
  work_id uuid references works(id) on delete cascade,
  edition_id uuid references editions(id) on delete cascade,
  wanted_format text not null check (wanted_format in ('ebook', 'audiobook')),
  quality_profile text not null default 'standard',
  status text not null default 'wanted',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists releases (
  id uuid primary key default gen_random_uuid(),
  wanted_item_id uuid references wanted_items(id) on delete set null,
  indexer text not null,
  title text not null,
  protocol text not null,
  download_url text not null,
  size_bytes bigint,
  seeders integer,
  score numeric(6,3) not null default 0,
  rejected_reason text,
  published_at timestamptz,
  created_at timestamptz not null default now()
);

create table if not exists downloads (
  id uuid primary key default gen_random_uuid(),
  release_id uuid references releases(id) on delete set null,
  client text not null,
  external_id text,
  category text not null,
  save_path text not null,
  state text not null,
  progress numeric(5,4) not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists files (
  id uuid primary key default gen_random_uuid(),
  edition_id uuid references editions(id) on delete set null,
  media_format text not null check (media_format in ('ebook', 'audiobook')),
  path text not null unique,
  size_bytes bigint,
  checksum text,
  import_status text not null default 'pending_review',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists authors_sort_name_idx on authors(sort_name);
create index if not exists works_sort_title_idx on works(sort_title);
create index if not exists editions_work_id_idx on editions(work_id);
create index if not exists wanted_items_status_idx on wanted_items(status);
create index if not exists releases_wanted_item_id_idx on releases(wanted_item_id);
create index if not exists downloads_state_idx on downloads(state);
