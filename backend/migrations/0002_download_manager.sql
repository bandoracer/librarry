alter table downloads add column if not exists name text not null default '';
alter table downloads add column if not exists tags text not null default '';
alter table downloads add column if not exists size_bytes bigint not null default 0;
alter table downloads add column if not exists downloaded_bytes bigint not null default 0;
alter table downloads add column if not exists uploaded_bytes bigint not null default 0;
alter table downloads add column if not exists download_rate bigint not null default 0;
alter table downloads add column if not exists upload_rate bigint not null default 0;
alter table downloads add column if not exists eta_seconds bigint not null default 0;
alter table downloads add column if not exists ratio numeric(8,4) not null default 0;
alter table downloads add column if not exists seeders integer not null default 0;
alter table downloads add column if not exists peers integer not null default 0;
alter table downloads add column if not exists added_at timestamptz;
alter table downloads add column if not exists completed_at timestamptz;
alter table downloads add column if not exists last_seen_at timestamptz;

create unique index if not exists downloads_client_external_id_unique_idx
  on downloads(client, external_id)
  where external_id is not null;

create index if not exists downloads_category_idx on downloads(category);
create index if not exists downloads_last_seen_at_idx on downloads(last_seen_at desc);
