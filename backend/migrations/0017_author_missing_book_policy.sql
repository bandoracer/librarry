alter table author_subscriptions
  add column if not exists missing_book_policy text not null default 'all'
  check (missing_book_policy in ('all', 'future', 'none'));

update author_subscriptions
set missing_book_policy = case when monitor_new_items then 'all' else 'none' end
where missing_book_policy = '';
