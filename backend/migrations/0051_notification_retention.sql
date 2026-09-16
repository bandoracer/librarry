-- Resolved delivery history can expire; event identities remain replay barriers.
alter table notification_deliveries add column resolved_at timestamptz;
-- Acceptance is already terminal. Legacy automated cancellations are NOT proof
-- that an operator reviewed the missing notification; leave those unresolved.
update notification_deliveries set resolved_at=updated_at where state='accepted';
alter table notification_deliveries add constraint notification_resolution_state check(resolved_at is null or state in ('accepted','cancelled'));
alter table notification_events add column archived_at timestamptz;
alter table notification_events add column retention_summary jsonb not null default '{}';
alter table notification_events add constraint notification_archived_payload check(archived_at is null or (event='{}'::jsonb and compat_context='{}'::jsonb));
create index notification_retention_events_idx on notification_events(created_at,id) where archived_at is null;
