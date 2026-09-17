-- Scheduling progress is separate from a successful provider search/sync.
-- Skips, outages and failed searches must not pin the first batch forever.
alter table wanted_items add column last_monitor_checked_at timestamptz;
alter table wanted_items add column last_upgrade_checked_at timestamptz;
alter table author_subscriptions add column last_sync_attempt_at timestamptz;
create index wanted_monitor_fair_order on wanted_items
 (coalesce(last_monitor_checked_at,last_search_at,'epoch'::timestamptz),created_at,id)
 where monitored and status in ('wanted','grabbed','imported');
create index wanted_upgrade_fair_order on wanted_items
 (coalesce(last_upgrade_checked_at,last_upgrade_search_at,'epoch'::timestamptz),created_at,id)
 where monitored and status in ('wanted','grabbed','imported');
create index author_monitor_fair_order on author_subscriptions
 (coalesce(last_sync_attempt_at,last_sync_at,'epoch'::timestamptz),author_name,id)
 where status='monitored' and monitor_new_items and missing_book_policy<>'none';
