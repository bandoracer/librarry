alter table wanted_items add column if not exists monitored boolean not null default true;

create index if not exists wanted_items_monitored_idx on wanted_items(monitored);
