-- A shared due time avoids sequential duplicate startup passes after restarts.
-- The advisory session lock, not an expired clock lease, owns an active run.
create table worker_tasks (
 task_id text primary key,
 run_id uuid,
 next_run_at timestamptz not null default now()
);
create table worker_task_runs (
 id uuid primary key default gen_random_uuid(),
 task_id text not null references worker_tasks(task_id),
 trigger text not null,
 backend_pid integer not null,
 state text not null check(state in ('running','completed','failed','interrupted')),
 started_at timestamptz not null default now(),
 heartbeat_at timestamptz not null default now(),
 finished_at timestamptz,
 outcome text not null default '',
 error text not null default ''
);
create index worker_task_runs_history_idx on worker_task_runs(task_id,started_at desc,id);
