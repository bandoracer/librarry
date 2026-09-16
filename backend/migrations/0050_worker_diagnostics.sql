-- Routine successes may expire; unresolved failure evidence must not disappear.
alter table worker_task_runs drop constraint worker_task_runs_state_check;
alter table worker_task_runs add check(state in ('running','completed','degraded','failed','interrupted'));
alter table worker_task_runs add column details jsonb not null default '{}';
alter table worker_task_runs add column reviewed_at timestamptz;
alter table worker_tasks add column last_success_at timestamptz;
alter table worker_tasks add column last_success_run_id uuid;
update worker_tasks t set last_success_at=s.finished_at,last_success_run_id=s.id
from (select distinct on(task_id) task_id,id,finished_at from worker_task_runs where state='completed' and finished_at is not null order by task_id,finished_at desc,id desc) s where t.task_id=s.task_id;
create index worker_unreviewed_idx on worker_task_runs(task_id,started_at desc,id desc) where reviewed_at is null and state in ('degraded','failed','interrupted','running');
