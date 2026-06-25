alter table files add column if not exists title text not null default '';
alter table files add column if not exists author_name text not null default '';
alter table files add column if not exists source_path text not null default '';
alter table files add column if not exists extension text not null default '';
alter table files add column if not exists modified_at timestamptz;

create index if not exists files_media_format_idx on files(media_format);
create index if not exists files_import_status_idx on files(import_status);
create index if not exists files_title_idx on files(title);
create index if not exists files_author_name_idx on files(author_name);
