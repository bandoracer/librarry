-- Readarr-style metadata profiles: named, reusable author add-filter sets
-- (language allowlist, must-not-contain terms, ISBN requirement, minimum page
-- count). Author subscriptions can reference a profile; when set, the profile
-- filters win over the legacy per-author override columns (which stay for
-- subscriptions without a profile).

create table if not exists metadata_profiles (
  id uuid primary key default gen_random_uuid(),
  name text not null unique,
  allowed_languages text not null default '',
  must_not_contain text not null default '',
  skip_missing_isbn boolean not null default false,
  min_pages integer not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- Seed the no-filter default profile.
insert into metadata_profiles (name) values ('Standard')
on conflict (name) do nothing;

alter table author_subscriptions
  add column if not exists metadata_profile_id uuid references metadata_profiles(id);

create index if not exists author_subscriptions_metadata_profile_idx
  on author_subscriptions(metadata_profile_id)
  where metadata_profile_id is not null;
