create table if not exists compat_root_folders (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  path text not null unique,
  media_format text not null default 'mixed' check (media_format in ('ebook', 'audiobook', 'mixed')),
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists compat_root_folders_media_format_idx on compat_root_folders(media_format);
