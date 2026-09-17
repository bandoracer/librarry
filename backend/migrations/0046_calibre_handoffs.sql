-- A remote upload cannot be rolled back by a local transaction. Record the send
-- before contacting Calibre and keep its acknowledgement independently of files.
create table calibre_handoffs (
  id uuid primary key default gen_random_uuid(),
  source_path text not null unique,
  root_folder_id uuid not null references root_folders(id),
  wanted_item_id uuid references wanted_items(id),
  download_record_id uuid references downloads(id),
  file_id uuid references files(id) on delete set null,
  phase text not null default 'planned' check (phase in ('planned','uploading','accepted','converting','ready','committed')),
  book_id integer check (book_id > 0),
  plan jsonb not null,
  progress jsonb not null default '[]',
  run_token uuid,
  last_error text not null default '',
  attempts integer not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index calibre_handoffs_recovery_idx on calibre_handoffs ((phase <> 'committed'),updated_at);
create unique index calibre_handoffs_download_idx on calibre_handoffs(download_record_id) where download_record_id is not null;
