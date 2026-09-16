-- Recovery ordering must not move when a lease, phase, or progress changes.
create index import_operations_recovery_page_idx on import_operations(created_at desc,id desc);
create index import_operations_unfinished_page_idx on import_operations(created_at desc,id desc)
where state<>'committed' or (source_kind='manual' and cleanup_state<>'cleaned') or replacement_cleanup_state='pending';
create index calibre_handoffs_recovery_page_idx on calibre_handoffs(created_at desc,id desc);
create index calibre_handoffs_unfinished_page_idx on calibre_handoffs(created_at desc,id desc) where phase<>'committed';
create index import_reconciliation_page_idx on import_reconciliation_issues(created_at desc,file_id desc,kind desc) where resolved_at is null;
