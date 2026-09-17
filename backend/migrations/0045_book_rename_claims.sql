-- Complete-book renames reserve every tracked member, not just a primary file.
create table file_rename_claims (
  file_id uuid primary key references files(id) on delete cascade,
  operation_id uuid not null references import_operations(id) on delete cascade
);
create index file_rename_claims_operation_idx on file_rename_claims(operation_id);
insert into file_rename_claims(file_id,operation_id)
select f.file_id,o.id from import_operations o join import_operation_files f on f.operation_id=o.id
where o.source_kind='manual' and o.metadata ? 'renameFileId' and f.file_id is not null
  and (o.state<>'committed' or o.cleanup_state<>'cleaned');

-- Sidecars have no files row. Preserve their original manifest identity through
-- later folder renames so historical receipts can resolve their current path.
alter table import_operation_files add column rename_origin_file_id uuid references import_operation_files(id);
create index import_operation_files_rename_origin_idx on import_operation_files(rename_origin_file_id)
  where rename_origin_file_id is not null;

create function release_finished_rename_claims() returns trigger language plpgsql as $$
begin
  if new.state='committed' and new.cleanup_state='cleaned' then
    delete from file_rename_claims where operation_id=new.id;
  end if;
  return new;
end $$;
create trigger import_operations_release_rename_claims after update of state,cleanup_state on import_operations
  for each row execute function release_finished_rename_claims();

-- Import receipts may also follow a verified scan reconciliation of the same
-- file identity and bytes, in sequence with explicit rename operations.
create index library_scan_moves_file_created_idx on library_scan_moves(file_id,created_at);
