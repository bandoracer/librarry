-- Only untouched, unassigned records created by scanning may be folded into an
-- older file identity. Keep this evidence across interrupted/cancelled scans.
alter table files add column scan_file_stamp text not null default '';
create function scan_discovery_identity(f files) returns jsonb language sql immutable as $$
 select jsonb_build_object('path',f.path,'editionId',f.edition_id,
   'format',f.media_format,'title',f.title,'author',f.author_name,
   'sourcePath',f.source_path,'importStatus',f.import_status,
   'metadata',f.metadata-'scanEvidence')
$$;
create table library_scan_discoveries (
 file_id uuid primary key references files(id) on delete cascade,
 job_id uuid not null references library_scan_jobs(id),
 identity jsonb not null
);
create index files_content_identity on files(checksum,size_bytes,media_format);
alter table library_scan_jobs add column moved integer not null default 0;
-- Historical IDs/paths remain evidence even if the current file is later removed.
create table library_scan_moves (
 job_id uuid not null references library_scan_jobs(id),
 file_id uuid not null,
 discovered_file_id uuid not null,
 previous_path text not null,
 current_path text not null,
 sha256 text not null check(sha256 ~ '^[0-9a-f]{64}$'),
 size_bytes bigint not null check(size_bytes>0),
 created_at timestamptz not null default now(),
 primary key(job_id,file_id)
);
