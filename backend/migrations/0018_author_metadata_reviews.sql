create table if not exists author_metadata_reviews (
  id uuid primary key default gen_random_uuid(),
  author_subscription_id uuid references author_subscriptions(id) on delete cascade,
  provider text not null default '',
  candidate_key text not null,
  title text not null default '',
  author_name text not null default '',
  wanted_format text not null default 'ebook' check (wanted_format in ('ebook', 'audiobook')),
  quality_profile text not null default 'standard',
  tags text not null default '',
  policy text not null default '',
  reason text not null default '',
  status text not null default 'pending',
  decision text not null default '',
  wanted_item_id uuid references wanted_items(id) on delete set null,
  result jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  resolved_at timestamptz
);

create index if not exists author_metadata_reviews_status_idx
  on author_metadata_reviews(status, created_at desc);

create index if not exists author_metadata_reviews_author_idx
  on author_metadata_reviews(author_subscription_id, created_at desc);

create unique index if not exists author_metadata_reviews_candidate_idx
  on author_metadata_reviews(author_subscription_id, candidate_key, wanted_format);
