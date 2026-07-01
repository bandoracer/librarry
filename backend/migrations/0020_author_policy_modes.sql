-- 0017 added missing_book_policy with an inline check constraint limited to
-- ('all', 'future', 'none'). Relax it so author monitoring supports the full
-- Readarr-style monitor modes.
alter table author_subscriptions
  drop constraint if exists author_subscriptions_missing_book_policy_check;

alter table author_subscriptions
  add constraint author_subscriptions_missing_book_policy_check
  check (missing_book_policy in ('all', 'future', 'none', 'missing', 'existing', 'first', 'latest'));
