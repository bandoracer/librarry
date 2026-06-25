create table if not exists import_reviews (
  id uuid primary key default gen_random_uuid(),
  source_path text not null,
  download_id text not null default '',
  wanted_item_id uuid references wanted_items(id) on delete set null,
  media_format text not null default 'unknown',
  title text not null default '',
  author_name text not null default '',
  size_bytes bigint,
  reason text not null default '',
  status text not null default 'pending',
  decision text not null default '',
  destination_path text not null default '',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  resolved_at timestamptz
);

create index if not exists import_reviews_status_idx on import_reviews(status);
create index if not exists import_reviews_created_at_idx on import_reviews(created_at desc);
create index if not exists import_reviews_download_id_idx on import_reviews(download_id);
create unique index if not exists import_reviews_pending_source_path_idx
  on import_reviews(source_path)
  where status = 'pending';
