create table if not exists blocklist (
  id uuid primary key default gen_random_uuid(),
  wanted_item_id uuid references wanted_items(id) on delete set null,
  title text not null,
  indexer text not null default '',
  protocol text not null default '',
  download_url_hash text not null default '',
  infohash text not null default '',
  reason text not null default '',
  source text not null default 'auto-failed',
  created_at timestamptz default now()
);

create index if not exists blocklist_infohash_idx on blocklist(infohash);
create index if not exists blocklist_download_url_hash_idx on blocklist(download_url_hash);
