alter table downloads add column if not exists import_status text not null default 'pending';
alter table downloads add column if not exists imported_file_id uuid references files(id) on delete set null;
alter table downloads add column if not exists imported_at timestamptz;
alter table downloads add column if not exists import_error text not null default '';

create index if not exists downloads_import_status_idx on downloads(import_status);
create index if not exists downloads_imported_file_id_idx on downloads(imported_file_id);
