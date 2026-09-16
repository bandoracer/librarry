-- Renames reuse manual staging and cleanup, with one unfinished plan per file.
-- The original file row and its relationships survive the visibility commit.
create unique index import_operations_active_rename_file_idx
  on import_operations ((metadata->>'renameFileId'))
  where source_kind='manual' and metadata ? 'renameFileId'
    and (state<>'committed' or cleanup_state<>'cleaned');
create index import_operations_rename_file_history_idx
  on import_operations ((metadata->>'renameFileId'),created_at desc)
  where source_kind='manual' and metadata ? 'renameFileId';

-- A committed rename may still be removing its old name. A scan must not
-- create a duplicate file at that reserved source while cleanup is pending.
create index import_operation_files_source_idx on import_operation_files(source_path);
create or replace function guard_import_file_visibility() returns trigger language plpgsql as $$
begin
  perform pg_advisory_xact_lock(hashtextextended(new.path,1));
  if exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id
            where f.destination_path=new.path and o.state<>'committed') then
    raise exception 'file belongs to an unfinished import operation' using errcode='23514';
  end if;
  if exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id
            where f.source_path=new.path and o.source_kind='manual' and o.metadata ? 'renameFileId'
              and (o.state<>'committed' or o.cleanup_state<>'cleaned')
              and f.file_id is distinct from new.id) then
    raise exception 'file path belongs to an unfinished rename operation' using errcode='23514';
  end if;
  return new;
end $$;
