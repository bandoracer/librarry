-- M6.4 native tags: labels are the canonical identity; wanted_items.tags and
-- author_subscriptions.tags keep the 0016 comma-separated text format but now
-- carry labels. Rename/delete rewrites both text columns transactionally.
create table if not exists tags (
  id uuid primary key default gen_random_uuid(),
  label text not null unique,
  created_at timestamptz not null default now()
);

-- M6.5 author add-filters (metadata-profile analog), applied by the author
-- monitor before candidates reach the review queue.
alter table author_subscriptions add column if not exists allowed_languages text not null default '';
alter table author_subscriptions add column if not exists must_not_contain text not null default '';
alter table author_subscriptions add column if not exists skip_missing_isbn boolean not null default false;
alter table author_subscriptions add column if not exists min_pages integer not null default 0;
