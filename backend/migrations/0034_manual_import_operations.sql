-- Native manual imports share the durable manifest/execution engine. They may
-- have no wanted book or download; neither association is invented.
alter table import_operations alter column download_record_id drop not null;
alter table import_operations alter column wanted_item_id drop not null;
alter table import_operations add column source_kind text not null default 'completed' check (source_kind in ('completed','manual'));
alter table import_operations add column request_key text not null default '';
create unique index import_operations_manual_request_idx on import_operations(request_key) where source_kind='manual';
alter table import_operations drop constraint import_operations_import_mode_check;
alter table import_operations add constraint import_operations_import_mode_check check(import_mode in ('copy','hardlink','hardlinkOrCopy','move'));
alter table import_operations add constraint import_operations_completed_identity check(source_kind<>'completed' or (download_record_id is not null and wanted_item_id is not null and import_mode<>'move'));
alter table import_operation_files add column previous_path text not null default '';
alter table import_operation_files add column previous_sha256 text not null default '';
alter table import_operation_files add column previous_size_bytes bigint not null default 0;
alter table import_operation_files add column source_removed boolean not null default false;
alter table import_operation_files add constraint import_previous_identity check ((previous_path='' and previous_sha256='' and previous_size_bytes=0) or (previous_path<>'' and previous_sha256 ~ '^[0-9a-f]{64}$' and previous_size_bytes>=0));
create index import_operations_request_scope_idx on import_operations((metadata->>'requestScope'),created_at) where source_kind='manual';
