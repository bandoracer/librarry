alter table wanted_items add column if not exists title text not null default '';
alter table wanted_items add column if not exists author_name text not null default '';
alter table wanted_items add column if not exists cover_url text;
alter table wanted_items add column if not exists metadata_provider text not null default '';
alter table wanted_items add column if not exists source_key text not null default '';
alter table wanted_items add column if not exists last_search_at timestamptz;

alter table releases add column if not exists source_id text not null default '';
alter table releases add column if not exists info_hash text;
alter table releases add column if not exists info_url text;
alter table releases add column if not exists categories text not null default '';
alter table releases add column if not exists leechers integer not null default 0;
alter table releases add column if not exists approved boolean not null default false;
alter table releases add column if not exists searched_at timestamptz not null default now();

create unique index if not exists wanted_items_source_unique_idx
  on wanted_items(metadata_provider, source_key, wanted_format)
  where metadata_provider <> '' and source_key <> '';

create unique index if not exists releases_wanted_source_unique_idx
  on releases(wanted_item_id, source_id)
  where source_id <> '';

create index if not exists wanted_items_last_search_at_idx on wanted_items(last_search_at desc);
create index if not exists releases_approved_idx on releases(approved);
