alter table wanted_items add column if not exists tags text not null default '';
alter table author_subscriptions add column if not exists tags text not null default '';

create index if not exists wanted_items_tags_idx on wanted_items(tags);
create index if not exists author_subscriptions_tags_idx on author_subscriptions(tags);
