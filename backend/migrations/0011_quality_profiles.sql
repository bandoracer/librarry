create table if not exists quality_profiles (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  media_format text not null default 'any' check (media_format in ('any', 'ebook', 'audiobook')),
  min_score numeric(6,3) not null default 60,
  cutoff_score numeric(6,3) not null default 85,
  min_seeders integer not null default 1,
  max_size_bytes bigint not null default 0,
  preferred_terms text not null default '',
  required_terms text not null default '',
  rejected_terms text not null default 'summary,review',
  preferred_score numeric(6,3) not null default 8,
  upgrade_allowed boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (name, media_format)
);

insert into quality_profiles (
  name, media_format, min_score, cutoff_score, min_seeders, max_size_bytes,
  preferred_terms, rejected_terms, preferred_score, upgrade_allowed
) values
  ('standard', 'ebook', 60, 85, 1, 786432000, 'epub,azw3', 'summary,review', 8, true),
  ('large', 'ebook', 65, 90, 1, 2147483648, 'epub,azw3,pdf', 'summary,review', 8, true),
  ('standard', 'audiobook', 60, 85, 1, 8589934592, 'm4b,mp3', 'summary,review', 8, true),
  ('large', 'audiobook', 65, 90, 1, 21474836480, 'm4b,mp3', 'summary,review', 8, true)
on conflict (name, media_format) do nothing;

create index if not exists quality_profiles_name_idx on quality_profiles(name);
create index if not exists quality_profiles_media_format_idx on quality_profiles(media_format);
