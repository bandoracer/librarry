-- Explicit associations coexist with the legacy JSON/scalar projections.
create table file_wanted_links (
  file_id uuid not null references files(id) on delete cascade,
  wanted_item_id uuid not null references wanted_items(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key(file_id, wanted_item_id)
);
create index file_wanted_links_wanted_idx on file_wanted_links(wanted_item_id, file_id);

create table file_download_links (
  file_id uuid not null references files(id) on delete cascade,
  download_record_id uuid not null references downloads(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key(file_id, download_record_id)
);
create index file_download_links_download_idx on file_download_links(download_record_id, file_id);

create table import_reconciliation_issues (
  file_id uuid not null references files(id) on delete cascade,
  kind text not null check(kind in ('wanted', 'download')),
  reason text not null,
  evidence jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  resolved_at timestamptz,
  primary key(file_id, kind)
);

-- Maintain the compatibility projection for all file writers, including legacy
-- import/scan/Readarr paths. Missing JSON never erases a proven relationship.
create function reconcile_file_links() returns trigger language plpgsql as $$
declare
  wanted_key text := nullif(trim(new.metadata->>'wantedId'), '');
  download_key text := coalesce(nullif(trim(new.metadata->'verifiedDownload'->>'id'), ''), nullif(trim(new.metadata->>'downloadId'), ''));
  client_key text := coalesce(nullif(trim(new.metadata->'verifiedDownload'->>'client'), ''), nullif(trim(new.metadata->>'downloadClient'), ''));
  wanted_uuid uuid;
  download_uuid uuid;
  matches integer;
begin
  if wanted_key is not null then
    select id into wanted_uuid from wanted_items where id::text = lower(wanted_key);
    if wanted_uuid is not null then
      delete from file_wanted_links where file_id=new.id and wanted_item_id<>wanted_uuid;
      insert into file_wanted_links(file_id,wanted_item_id) values(new.id,wanted_uuid) on conflict do nothing;
      update import_reconciliation_issues set resolved_at=now() where file_id=new.id and kind='wanted';
    else
      insert into import_reconciliation_issues(file_id,kind,reason,evidence)
      values(new.id,'wanted','wanted identifier does not resolve',jsonb_build_object('wantedId',wanted_key))
      on conflict(file_id,kind) do update set reason=excluded.reason,evidence=excluded.evidence,resolved_at=null;
    end if;
  end if;
  if download_key is not null then
    select count(*), (array_agg(id order by id))[1] into matches,download_uuid
    from downloads where external_id=download_key and (client_key is null or lower(client)=lower(client_key));
    if matches=1 then
      insert into file_download_links(file_id,download_record_id) values(new.id,download_uuid) on conflict do nothing;
      update import_reconciliation_issues set resolved_at=now() where file_id=new.id and kind='download';
    else
      insert into import_reconciliation_issues(file_id,kind,reason,evidence)
      values(new.id,'download',case when matches=0 then 'download identifier does not resolve' else 'download identifier is ambiguous across clients' end,
        jsonb_build_object('downloadId',download_key,'client',client_key,'candidateCount',matches))
      on conflict(file_id,kind) do update set reason=excluded.reason,evidence=excluded.evidence,resolved_at=null;
    end if;
  end if;
  return new;
end $$;
create trigger files_reconcile_links after insert or update of metadata on files
  for each row execute function reconcile_file_links();

create function link_download_import_projection() returns trigger language plpgsql as $$
begin
  if new.imported_file_id is not null then
    insert into file_download_links(file_id,download_record_id) values(new.imported_file_id,new.id) on conflict do nothing;
  end if;
  return new;
end $$;
create trigger downloads_link_import after insert or update of imported_file_id on downloads
  for each row execute function link_download_import_projection();

-- No verification receipts or operations are manufactured by this backfill.
update files set metadata=metadata;
insert into file_download_links(file_id,download_record_id)
select imported_file_id,id from downloads where imported_file_id is not null on conflict do nothing;

create table import_migration_reports (
  migration text primary key,
  report jsonb not null,
  created_at timestamptz not null default now()
);
insert into import_migration_reports(migration,report) values('0030_import_operations',jsonb_build_object(
  'filesBefore',(select count(*) from files),
  'filesAfter',(select count(*) from files),
  'wantedLinks',(select count(*) from file_wanted_links),
  'downloadLinks',(select count(*) from file_download_links),
  'unresolved',(select count(*) from import_reconciliation_issues where resolved_at is null),
  'verifiedLegacyImports',0
));

create table import_operations (
  id uuid primary key default gen_random_uuid(),
  download_record_id uuid not null unique references downloads(id) on delete restrict,
  wanted_item_id uuid not null references wanted_items(id) on delete restrict,
  source_root text not null,
  destination_root text not null,
  media_format text not null check(media_format in ('ebook','audiobook')),
  import_mode text not null check(import_mode in ('copy','hardlink','hardlinkOrCopy')),
  state text not null default 'planned' check(state in ('planned','transferring','verified','committed','failed','review')),
  cleanup_state text not null default 'blocked' check(cleanup_state in ('blocked','eligible','cleaned')),
  last_error text not null default '',
  cleanup_error text not null default '',
  lease_token uuid,
  lease_expires_at timestamptz,
  attempts integer not null default 0,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  committed_at timestamptz,
  check(cleanup_state='blocked' or state='committed')
);
create index import_operations_state_idx on import_operations(state,updated_at,id);

create table import_operation_files (
  id uuid primary key default gen_random_uuid(),
  operation_id uuid not null references import_operations(id) on delete cascade,
  file_order integer not null,
  relative_path text not null,
  source_path text not null,
  destination_path text not null,
  size_bytes bigint not null check(size_bytes>0),
  sha256 text not null check(sha256 ~ '^[0-9a-f]{64}$'),
  media_format text not null check(media_format in ('ebook','audiobook','sidecar')),
  required boolean not null default true,
  state text not null default 'planned' check(state in ('planned','transferred','verified','committed')),
  file_id uuid references files(id) on delete set null,
  updated_at timestamptz not null default now(),
  unique(operation_id,relative_path),
  unique(operation_id,destination_path)
);
create index import_operation_files_file_idx on import_operation_files(file_id);
