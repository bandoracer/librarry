create table if not exists root_folders (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  path text not null unique,
  media_format text not null default 'ebook' check (media_format in ('ebook', 'audiobook')),
  default_quality_profile text not null default '',
  default_missing_book_policy text not null default '',
  default_tags text not null default '',
  is_default boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists root_folders_media_format_idx on root_folders(media_format);

alter table wanted_items add column if not exists root_folder_id uuid references root_folders(id) on delete set null;

-- Naming token support ({Series}, {SeriesPosition}, {Year}): persist series and
-- first-publish-year onto wanted items so import naming can render them.
alter table wanted_items add column if not exists series text not null default '';
alter table wanted_items add column if not exists series_position text not null default '';
alter table wanted_items add column if not exists first_publish_year integer not null default 0;

-- Backfill the year from the linked work for existing rows.
update wanted_items wi
set first_publish_year = coalesce(w.first_publish_year, 0)
from works w
where w.id = wi.work_id
  and wi.first_publish_year = 0
  and coalesce(w.first_publish_year, 0) <> 0;
