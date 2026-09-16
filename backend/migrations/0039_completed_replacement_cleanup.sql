-- Replacement backups and remote download-source cleanup are independent.
alter table import_operations add column replacement_cleanup_state text not null default 'none'
 check(replacement_cleanup_state in ('none','pending','cleaned'));
alter table import_operations add column replacement_cleanup_error text not null default '';
create index import_operations_replacement_cleanup on import_operations(updated_at,id)
 where replacement_cleanup_state='pending';
