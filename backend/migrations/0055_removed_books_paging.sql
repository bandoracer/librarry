-- Inactive identities remain inspectable without loading the active library.
create index wanted_removed_books_page_idx on wanted_items(created_at desc,id desc) where status in ('removed','ignored');
