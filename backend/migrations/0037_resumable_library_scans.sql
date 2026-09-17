create table library_scan_jobs (
 id uuid primary key default gen_random_uuid(),
 scope_key text not null,
 media_format text not null,
 roots jsonb not null,
 state text not null default 'queued' check(state in ('queued','running','failed','cancelled','completed')),
 phase text not null default 'discover' check(phase in ('discover','reconcile','complete')),
 lease_token uuid,
 lease_expires_at timestamptz,
 cancel_requested boolean not null default false,
 scanned integer not null default 0,
 upserted integer not null default 0,
 skipped integer not null default 0,
 missing integer not null default 0,
 last_error text not null default '',
 reconcile_after uuid,
 created_at timestamptz not null default now(),
 updated_at timestamptz not null default now(),
 finished_at timestamptz
);
create unique index library_scan_active_scope on library_scan_jobs(scope_key) where state in ('queued','running');
create table library_scan_roots (
 path text primary key,
 identity text not null,
 completed_job_id uuid references library_scan_jobs(id),
 updated_at timestamptz not null default now()
);
create table library_scan_entries (
 job_id uuid not null references library_scan_jobs(id) on delete cascade,
 root_path text not null,
 path text not null,
 kind text not null check(kind in ('directory','file')),
 done boolean not null default false,
 primary key(job_id,path)
);
create index library_scan_pending_entries on library_scan_entries(job_id,path) where not done;
create table library_scan_absent (
 job_id uuid not null references library_scan_jobs(id) on delete cascade,
 file_id uuid not null references files(id) on delete cascade,
 path text not null,
 observed_updated_at timestamptz not null,
 primary key(job_id,file_id)
);
alter table files add column presence_state text not null default 'unknown' check(presence_state in ('unknown','present','missing'));
alter table files add column scan_device text not null default '';
alter table files add column scan_root text not null default '';
alter table files add column last_seen_scan_id uuid references library_scan_jobs(id);
create index files_scan_root_id on files(scan_root,id);
