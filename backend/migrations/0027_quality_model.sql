-- Readarr-style quality model: quality ladders on profiles, release profiles
-- for required/ignored/preferred terms, and per-quality size definitions.
-- Release scores become a composite of ladder rank and preferred-word score
-- ((ladderLen - ladderIndex) * 1000 + preferredScore), so the score columns
-- need headroom beyond numeric(6,3).

alter table releases alter column score type numeric(9,3);
alter table wanted_items alter column current_release_score type numeric(9,3);

-- Ordered quality ladder (most-preferred first) and the upgrade cutoff
-- quality. Legacy score columns stay for compat readers but stop driving
-- evaluation.
alter table quality_profiles add column if not exists qualities jsonb not null default '[]';
alter table quality_profiles add column if not exists cutoff_quality text not null default '';

create table if not exists release_profiles (
  id uuid primary key default gen_random_uuid(),
  name text not null default '',
  enabled boolean not null default true,
  required text not null default '',
  ignored text not null default '',
  preferred jsonb not null default '[]',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists quality_definitions (
  quality text primary key,
  title text not null default '',
  min_size_mb integer not null default 0,
  max_size_mb integer not null default 0
);

-- Sane book sizes: text formats top out around 500MB, audio formats larger.
insert into quality_definitions (quality, title, min_size_mb, max_size_mb) values
  ('azw3', 'AZW3', 0, 500),
  ('epub', 'EPUB', 0, 500),
  ('mobi', 'MOBI', 0, 500),
  ('pdf', 'PDF', 0, 500),
  ('unknownText', 'Unknown Text', 0, 500),
  ('flac', 'FLAC', 0, 20480),
  ('m4b', 'M4B', 0, 10240),
  ('mp3', 'MP3', 0, 10240),
  ('unknownAudio', 'Unknown Audio', 0, 20480)
on conflict (quality) do nothing;

-- Existing profiles adopt the default ladder for their media format
-- (ebook: azw3 > epub > mobi > pdf > unknownText, cutoff epub;
--  audiobook: flac > m4b > mp3 > unknownAudio, cutoff m4b).
update quality_profiles set
  qualities = case
    when media_format = 'audiobook' then
      '[{"id":"flac","allowed":true},{"id":"m4b","allowed":true},{"id":"mp3","allowed":true},{"id":"unknownAudio","allowed":true}]'::jsonb
    else
      '[{"id":"azw3","allowed":true},{"id":"epub","allowed":true},{"id":"mobi","allowed":true},{"id":"pdf","allowed":true},{"id":"unknownText","allowed":true}]'::jsonb
  end
where qualities = '[]'::jsonb;

update quality_profiles set
  cutoff_quality = case when media_format = 'audiobook' then 'm4b' else 'epub' end
where cutoff_quality = '';

-- Convert legacy per-profile term columns into one seeded release profile so
-- existing required/ignored/preferred behavior survives the model switch.
-- Skipped when every profile has empty term columns.
insert into release_profiles (name, enabled, required, ignored, preferred)
select
  'Migrated terms',
  true,
  coalesce((
    select string_agg(term, ',')
    from (
      select distinct trim(t) as term
      from quality_profiles qp, unnest(string_to_array(qp.required_terms, ',')) as t
      where trim(t) <> ''
    ) required_terms
  ), ''),
  coalesce((
    select string_agg(term, ',')
    from (
      select distinct trim(t) as term
      from quality_profiles qp, unnest(string_to_array(qp.rejected_terms, ',')) as t
      where trim(t) <> ''
    ) ignored_terms
  ), ''),
  coalesce((
    select jsonb_agg(jsonb_build_object('term', term, 'score', 8))
    from (
      select distinct trim(t) as term
      from quality_profiles qp, unnest(string_to_array(qp.preferred_terms, ',')) as t
      where trim(t) <> ''
    ) preferred_terms
  ), '[]'::jsonb)
where exists (
  select 1 from quality_profiles qp
  where trim(coalesce(qp.required_terms, '')) <> ''
     or trim(coalesce(qp.rejected_terms, '')) <> ''
     or trim(coalesce(qp.preferred_terms, '')) <> ''
)
and not exists (
  select 1 from release_profiles rp where rp.name = 'Migrated terms'
);

create index if not exists release_profiles_enabled_idx on release_profiles(enabled);
