-- A scanner or compatibility writer must not expose a file while its durable
-- import is unfinished. The import transaction changes its own operation state
-- before inserting files; that state is invisible to other sessions until commit.
create index import_operation_files_destination_idx on import_operation_files(destination_path);
create function guard_import_file_visibility() returns trigger language plpgsql as $$
begin
  perform pg_advisory_xact_lock(hashtextextended(new.path,1));
  if exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id
            where f.destination_path=new.path and o.state<>'committed') then
    raise exception 'file belongs to an unfinished import operation' using errcode='23514';
  end if;
  return new;
end $$;
create trigger files_guard_import_visibility before insert or update of path on files
  for each row execute function guard_import_file_visibility();
