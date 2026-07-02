-- M6.1 calendar: wanted items carry a confident (full-precision) release date.
-- Items with only a year keep release_date null and stay off the calendar.
alter table wanted_items add column if not exists release_date date;

-- Backfill from edition publish dates where a full date is confidently
-- parseable. Row-by-row with an exception guard: a regex-passing but
-- impossible date (e.g. 2026-02-30) must not abort the migration.
do $$
declare
  row record;
begin
  for row in
    select wi.id, e.published_date
    from wanted_items wi
    join editions e on e.id = wi.edition_id
    where wi.release_date is null
      and e.published_date ~ '^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$'
  loop
    begin
      update wanted_items
      set release_date = to_date(row.published_date, 'YYYY-MM-DD')
      where id = row.id;
    exception when others then
      null; -- leave release_date null for unparseable dates
    end;
  end loop;
end $$;

create index if not exists wanted_items_release_date_idx on wanted_items(release_date);

-- M6.2 auth: arr-style in-app authentication (none | basic | forms).
create table if not exists users (
  id uuid primary key default gen_random_uuid(),
  username text not null unique,
  password_hash text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists sessions (
  token_hash text primary key,
  user_id uuid not null references users(id) on delete cascade,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);

create index if not exists sessions_expires_at_idx on sessions(expires_at);
