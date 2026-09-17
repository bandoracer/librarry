-- A reviewed pack may contain several books; ownership belongs to each media
-- file. The operation's wanted ID remains its primary compatibility projection.
alter table import_operation_files add column wanted_item_id uuid references wanted_items(id) on delete restrict;
update import_operation_files f set wanted_item_id=o.wanted_item_id from import_operations o where o.id=f.operation_id and f.media_format<>'sidecar';
create index import_operation_files_wanted_idx on import_operation_files(wanted_item_id);

-- Identical paths in two clients must not share an operator decision.
drop index import_reviews_pending_source_path_idx;
create unique index import_reviews_pending_source_identity_idx on import_reviews(source_path,download_id,lower(coalesce(metadata->>'downloadClient',''))) where status='pending';
