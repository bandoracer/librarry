-- Bound file lookups even before ANALYZE runs after a large import.
-- Preserve the evidence rules; only fence the linked-file lookup plan.
create or replace function librarry_book_file_evidence(wanted_ids uuid[])
returns table(wanted_id uuid, file_state text, file_reason text, present_files bigint, required_files bigint)
language sql stable as $$
with selected as materialized (
  select id,wanted_format from wanted_items where wanted_ids is null or id=any(wanted_ids)
), observations as (
  select wi.id,wi.wanted_format,
    case when f.id is null then 'absent'
    when exists(select 1 from import_operation_files m join import_operations o on o.id=m.operation_id
      where m.destination_path=f.path and o.state<>'committed') then 'unknown'
    when f.presence_state='missing' then 'missing'
    when f.presence_state='present' and f.import_status in ('available','imported') then 'present'
    else 'unknown' end as state
  from selected wi left join file_wanted_links l on l.wanted_item_id=wi.id
  -- Keep the file lookup keyed by the linked ID before testing its format.
  -- Without this lateral boundary, cold statistics can choose a format index
  -- scan for every wanted row, including books with no linked files.
  left join lateral (select f.* from files f where f.id=l.file_id offset 0) f
    on f.media_format=wi.wanted_format
), observed as (
  select id,wanted_format,count(*) filter(where state='present') as present,
    count(*) filter(where state='unknown') as unknown
  from observations group by id,wanted_format
), manifest_observations as (
  select m.wanted_item_id,m.operation_id,
    case when f.id is null or f.presence_state='missing' then 'missing'
    when m.state='committed' and f.presence_state='present' and f.import_status in ('available','imported')
      and f.media_format=m.media_format and f.checksum=m.sha256 and f.size_bytes=m.size_bytes
      and exists(select 1 from file_wanted_links l where l.file_id=f.id and l.wanted_item_id=m.wanted_item_id)
      and not exists(select 1 from import_operation_files pending join import_operations p on p.id=pending.operation_id
        where pending.destination_path=f.path and p.state<>'committed')
      then 'present' else 'unknown' end as state
  from selected wi join import_operation_files m on m.wanted_item_id=wi.id and m.media_format=wi.wanted_format
  join import_operations o on o.id=m.operation_id
  left join files f on f.id=m.file_id
  where wi.wanted_format='audiobook' and o.state='committed' and m.required
), manifests as (
  select wanted_item_id,operation_id,count(*) as required,
    count(*) filter(where state='present') as present,
    count(*) filter(where state='missing') as missing,
    count(*) filter(where state='unknown') as unknown
  from manifest_observations group by wanted_item_id,operation_id
), candidates as (
  select id,
    case when wanted_format='ebook' and present>0 then 'present'
      when unknown>0 or (wanted_format='audiobook' and present>0) then 'unknown'
      else 'missing' end as state,
    case when wanted_format='ebook' and present>0 then 'Library media was observed present by an import or scan.'
      when unknown>0 or (wanted_format='audiobook' and present>0) then 'File presence or audiobook completeness has not been verified.'
      else 'No present library media is recorded.' end as reason,
    present,0::bigint as required
  from observed
  union all
  select wanted_item_id,
    case when present=required then 'present' when present>0 and missing>0 then 'incomplete'
      when unknown>0 then 'unknown' else 'missing' end,
    case when present=required then 'Every required audiobook media file has matching committed import evidence.'
      when present>0 and missing>0 then 'Required audiobook media is missing.'
      when unknown>0 then 'Required audiobook media could not be verified against its import manifest.'
      else 'All required audiobook media is recorded missing.' end,
    present,required
  from manifests
), ranked as (
  select *,row_number() over(partition by id order by
    case state when 'present' then 4 when 'incomplete' then 3 when 'unknown' then 2 else 1 end desc,
    present desc,required desc,reason) as rank
  from candidates
)
select id,state,reason,present,required from ranked where rank=1
$$;
