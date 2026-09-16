-- Stable review browsing must not move a row when its evidence is refreshed.
create index import_reviews_page_idx on import_reviews(created_at desc,id desc);
create index import_reviews_pending_page_idx on import_reviews(created_at desc,id desc) where status='pending';
